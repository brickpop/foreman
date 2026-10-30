package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/userdata"
)

// Foreman takes no folder: it runs from its home. A folder given (an old
// habit) is refused before anything is made.
func TestAFolderArgumentIsRefused(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	err := run([]string{"--no-open", "--home", home, "~/dev"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "foreman takes no folder (~/dev)") {
		t.Errorf("%v", err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Error("the home was made anyway")
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

// The fake harness runs on the host only, so a Foreman on Podman refuses it
// before starting anything.
func TestFakeHarnessNeedsTheDirectEngine(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	err := run([]string{"--no-open", "--home", t.TempDir(), "--fake-harness", "/bin/true"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "drop --fake-harness") {
		t.Errorf("on podman: %v", err)
	}
}

// --port wins over settings.toml's port, which serves when no flag is given.
func TestThePortFlagWinsOverTheSetting(t *testing.T) {
	if p := portOf("9000", 7000); p != "9000" {
		t.Errorf("flag and setting: %q", p)
	}
	if p := portOf("", 7000); p != "7000" {
		t.Errorf("setting alone: %q", p)
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
// touches nothing (no global repo made, no port taken).
func TestASecondForemanPointsAtTheFirst(t *testing.T) {
	home := t.TempDir()
	first, err := userdata.TakeLock(home)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if err := first.Announce("http://127.0.0.1:8888/?token=abc"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"--no-open", "--home", home}, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "already running") || !strings.Contains(got, "http://127.0.0.1:8888/?token=abc") {
		t.Errorf("%q", got)
	}
	if _, err := os.Stat(filepath.Join(home, "config")); !os.IsNotExist(err) {
		t.Error("the second one made the global repo")
	}
}
