package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
