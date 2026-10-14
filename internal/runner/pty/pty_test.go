package pty

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/runner"
)

func start(t *testing.T, spec runner.Spec) runner.Session {
	t.Helper()
	s, err := Runner{}.Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop(time.Second) })
	return s
}

// readAll reads the session's output until it ends, failing after a timeout.
func readAll(t *testing.T, s runner.Session) string {
	t.Helper()
	out := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, s)
		out <- buf.String()
	}()
	select {
	case got := <-out:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("output never ended")
		return ""
	}
}

// readUntil reads until want appears in the output.
func readUntil(t *testing.T, s runner.Session, want string) string {
	t.Helper()
	found := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		chunk := make([]byte, 1024)
		for {
			n, err := s.Read(chunk)
			buf.Write(chunk[:n])
			if strings.Contains(buf.String(), want) || err != nil {
				found <- buf.String()
				return
			}
		}
	}()
	select {
	case got := <-found:
		if !strings.Contains(got, want) {
			t.Fatalf("output ended without %q: %q", want, got)
		}
		return got
	case <-time.After(5 * time.Second):
		t.Fatalf("never saw %q", want)
		return ""
	}
}

func sh(script string) runner.Spec { return runner.Spec{Command: []string{"/bin/sh", "-c", script}} }

func TestOutputAndExitCode(t *testing.T) {
	s := start(t, sh(`printf 'hello\033[32m green\033[0m'; exit 3`))
	got := readAll(t, s)
	if !strings.Contains(got, "hello\x1b[32m green\x1b[0m") {
		t.Errorf("raw output (with ANSI) not preserved: %q", got)
	}
	code, err := s.Wait()
	if err != nil || code != 3 {
		t.Errorf("Wait = %d, %v; want 3", code, err)
	}
}

// The last bytes a process prints right before exiting must not be lost, even
// when the reader only gets to them after the process is gone.
func TestFinalOutputSurvivesTheExit(t *testing.T) {
	s := start(t, sh(`echo THE-END; exit 0`))
	if code, _ := s.Wait(); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := readAll(t, s); !strings.Contains(got, "THE-END") {
		t.Errorf("final output lost: %q", got)
	}
}

// A long output, read while it is produced, arrives complete and in order.
func TestLongOutputArrivesComplete(t *testing.T) {
	s := start(t, sh(`i=0; while [ $i -lt 5000 ]; do echo "line $i"; i=$((i+1)); done; echo THE-END`))
	got := readAll(t, s)
	if !strings.Contains(got, "line 4999\r\nTHE-END") || strings.Count(got, "line ") != 5000 {
		t.Errorf("got %d lines, ends %q", strings.Count(got, "line "), got[max(0, len(got)-40):])
	}
}

func TestInputReachesTheProcess(t *testing.T) {
	s := start(t, sh(`read line; echo "got:$line"`))
	if _, err := s.Write([]byte("ping\r")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, s, "got:ping")
}

func TestInitialSizeAndResize(t *testing.T) {
	s := start(t, runner.Spec{
		Command: []string{"/bin/sh", "-c", `stty size; read _; stty size`},
		Size:    runner.Size{Cols: 91, Rows: 33},
	})
	readUntil(t, s, "33 91")
	if err := s.Resize(runner.Size{Cols: 150, Rows: 50}); err != nil {
		t.Fatal(err)
	}
	_, _ = s.Write([]byte("\r"))
	readUntil(t, s, "50 150")
}

func TestDefaultSizeIsStable(t *testing.T) {
	s := start(t, sh(`stty size`))
	if got := readAll(t, s); !strings.Contains(got, "40 120") {
		t.Errorf("default size: %q, want 40 rows x 120 cols", got)
	}
}

func TestStopEndsAProcessThatIgnoresTERM(t *testing.T) {
	s := start(t, sh(`trap '' TERM; echo ready; while true; do sleep 1; done`))
	readUntil(t, s, "ready")
	begin := time.Now()
	if err := s.Stop(200 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if time.Since(begin) > 3*time.Second {
		t.Error("Stop didn't kill after the grace period")
	}
	if _, err := s.Wait(); err != nil {
		t.Errorf("Wait after Stop: %v", err)
	}
}

func TestEnvironmentIsMinimal(t *testing.T) {
	t.Setenv("FOREMAN_TEST_SECRET", "leak")
	spec := sh(`echo "secret=[$FOREMAN_TEST_SECRET] mine=[$MINE] term=[$TERM] pwd=[$(pwd)]"`)
	spec.Env = []string{"MINE=yes"}
	spec.Dir = t.TempDir()
	got := readAll(t, start(t, spec))
	for _, want := range []string{"secret=[]", "mine=[yes]", "term=[xterm-256color]"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	// The working directory is the spec's (symlinks resolved: macOS /var is /private/var).
	dir, err := filepath.EvalSymlinks(spec.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "pwd=["+dir+"]") {
		t.Errorf("working directory: want %q in %q", dir, got)
	}
}

func TestLabelsAreKept(t *testing.T) {
	spec := sh(`true`)
	spec.Labels = map[string]string{"foreman.instance": "mara@ledger"}
	if got := start(t, spec).Labels()["foreman.instance"]; got != "mara@ledger" {
		t.Errorf("label %q", got)
	}
}

func TestEmptyCommandAndMissingProgramFail(t *testing.T) {
	if _, err := (Runner{}).Start(context.Background(), runner.Spec{}); err == nil {
		t.Error("empty command started")
	}
	if _, err := (Runner{}).Start(context.Background(), runner.Spec{Command: []string{"/no/such/program"}}); err == nil {
		t.Error("missing program started")
	}
}

func TestListIsEmpty(t *testing.T) {
	if got, err := (Runner{}).List(context.Background()); err != nil || len(got) != 0 {
		t.Errorf("List = %v, %v", got, err)
	}
}
