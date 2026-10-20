package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/userdata"
	"github.com/brickpop/foreman/internal/workspace"
)

func TestFirstRunDeclinedTouchesNothing(t *testing.T) {
	for _, answer := range []string{"\n", "n\n", "yes\n", "", "Y es\n"} {
		dir := t.TempDir()
		var out bytes.Buffer
		_, err := openOrCreate(dir, false, strings.NewReader(answer), &out)
		if err == nil {
			t.Errorf("answer %q: created a workspace", answer)
		}
		if _, statErr := os.Stat(filepath.Join(dir, workspace.DirName)); !os.IsNotExist(statErr) {
			t.Errorf("answer %q: .foreman/ exists after declining", answer)
		}
		if !strings.Contains(out.String(), "isn't a Foreman workspace yet") {
			t.Errorf("answer %q: no prompt shown: %q", answer, out.String())
		}
	}
}

func TestFirstRunAcceptedCreatesWorkspace(t *testing.T) {
	for _, answer := range []string{"y\n", "Y\n", "  y  \n"} {
		dir := t.TempDir()
		ws, err := openOrCreate(dir, false, strings.NewReader(answer), &bytes.Buffer{})
		if err != nil {
			t.Fatalf("answer %q: %v", answer, err)
		}
		if _, err := os.Stat(ws.Path()); err != nil {
			t.Errorf("answer %q: %v", answer, err)
		}
	}
}

func TestYesSkipsThePromptAndExistingWorkspacesDontAsk(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if _, err := openOrCreate(dir, true, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if _, err := openOrCreate(dir, false, strings.NewReader(""), &out); err != nil {
		t.Fatalf("reopening asked again or failed: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("prompted although it shouldn't have: %q", out.String())
	}
}

func TestFlagsOnEitherSideOfTheFolder(t *testing.T) {
	for _, args := range [][]string{
		{"ws", "--no-open", "--port", "1"},
		{"--no-open", "ws", "--port", "1"},
		{"--no-open", "--port", "1", "ws"},
	} {
		flags := flag.NewFlagSet("t", flag.ContinueOnError)
		noOpen := flags.Bool("no-open", false, "")
		port := flags.String("port", "8888", "")
		dir, err := parseArgs(flags, args)
		if err != nil || dir != "ws" || !*noOpen || *port != "1" {
			t.Errorf("%v: dir=%q noOpen=%v port=%q err=%v", args, dir, *noOpen, *port, err)
		}
	}
	flags := flag.NewFlagSet("t", flag.ContinueOnError)
	if dir, err := parseArgs(flags, nil); err != nil || dir != "." {
		t.Errorf("no args: dir=%q err=%v, want the current folder", dir, err)
	}
	if _, err := parseArgs(flag.NewFlagSet("t", flag.ContinueOnError), []string{"a", "b"}); err == nil {
		t.Error("two folders accepted")
	}
}

func TestRelativeProgramsResolveAgainstTheStartFolder(t *testing.T) {
	cwd, _ := os.Getwd()
	for in, want := range map[string]string{
		".dev/fakeharness": filepath.Join(cwd, ".dev/fakeharness"),
		"./x":              filepath.Join(cwd, "x"),
		"bash":             "bash", // left to the PATH lookup
		"/usr/bin/env":     "/usr/bin/env",
	} {
		if got, err := resolveProgram(in); err != nil || got != want {
			t.Errorf("resolveProgram(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// The fake harness runs on the host only, so a workspace on Podman refuses
// it before starting anything.
func TestFakeHarnessNeedsTheDirectEngine(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	dir := t.TempDir()
	err := run([]string{dir, "--yes", "--no-open", "--home", t.TempDir(), "--fake-harness", "/bin/true"}, strings.NewReader(""), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "drop --fake-harness") {
		t.Errorf("on podman: %v", err)
	}
}

func TestTheAgentPortIsTheNextOne(t *testing.T) {
	if p, err := nextPort("8888"); err != nil || p != "8889" {
		t.Errorf("8888: %q %v", p, err)
	}
	for _, bad := range []string{"65535", "0", "x", ""} {
		if _, err := nextPort(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// A signal stops serving, puts the signals back (so a second Ctrl-C ends
// Foreman at once) and only then runs the shutdown.
func TestServeUntilShutsDownOnASignal(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	quit, signal := context.WithCancel(context.Background())
	released, shutdown := false, false
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- serveUntil(quit, func() { released = true }, &out, &http.Server{Handler: http.NotFoundHandler()}, l, func() {
			if !released {
				t.Error("the shutdown ran before the signals were released")
			}
			if _, err := http.Get("http://" + l.Addr().String()); err == nil {
				t.Error("still serving during the shutdown")
			}
			shutdown = true
		})
	}()
	if _, err := http.Get("http://" + l.Addr().String()); err != nil {
		t.Fatalf("not serving: %v", err)
	}
	signal()
	if err := <-done; err != nil || !shutdown {
		t.Errorf("err %v, shutdown %v", err, shutdown)
	}
	if !bytes.Contains(out.Bytes(), []byte("Ctrl-C again")) {
		t.Errorf("printed %q", out.String())
	}
}

// Serving that fails returns its error, with no shutdown: nothing to stop.
func TestServeUntilReturnsAServingError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	err = serveUntil(context.Background(), func() {}, &bytes.Buffer{}, &http.Server{}, l, func() { t.Error("shut down") })
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		t.Errorf("err %v", err)
	}
}

// One Foreman per machine: a second one says where the first serves, and
// touches nothing (no workspace made, no port taken).
func TestASecondForemanPointsAtTheFirst(t *testing.T) {
	home := t.TempDir()
	first, err := userdata.TakeLock(home)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if err := first.Announce("http://127.0.0.1:8888/?token=abc", "/Users/ceo/dev"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var out bytes.Buffer
	if err := run([]string{dir, "--yes", "--no-open", "--home", home}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "already running") || !strings.Contains(got, "/Users/ceo/dev") || !strings.Contains(got, "http://127.0.0.1:8888/?token=abc") {
		t.Errorf("%q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".foreman")); !os.IsNotExist(err) {
		t.Error("the second one made a workspace")
	}
}
