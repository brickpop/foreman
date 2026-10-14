package sessionlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixed(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestWriteAndTail(t *testing.T) {
	l, err := Dir{Root: t.TempDir()}.Open("mara@ledger")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, _ = l.Write([]byte("hello "))
	_, _ = l.Write([]byte("\x1b[32mworld\x1b[0m"))
	for _, c := range []struct {
		max  int64
		want string
	}{
		{1000, "hello \x1b[32mworld\x1b[0m"},
		{4, "\x1b[0m"},
		{0, ""},
	} {
		got, err := l.Tail(c.max)
		if err != nil || string(got) != c.want {
			t.Errorf("Tail(%d) = %q, %v; want %q", c.max, got, err, c.want)
		}
	}
}

func TestTailAfterClose(t *testing.T) {
	l, _ := Dir{Root: t.TempDir()}.Open("a")
	_, _ = l.Write([]byte("kept"))
	_ = l.Close()
	if got, err := l.Tail(100); err != nil || string(got) != "kept" {
		t.Errorf("Tail after Close = %q, %v", got, err)
	}
}

func TestLayoutAndPermissions(t *testing.T) {
	root := t.TempDir()
	l, err := Dir{Root: root, Now: fixed(time.Date(2026, 10, 13, 9, 30, 0, 5, time.UTC))}.Open("mara@ledger")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "mara@ledger", "20261013T093000.000000005Z.log")
	if l.Path() != want {
		t.Errorf("path %q, want %q", l.Path(), want)
	}
	info, _ := os.Stat(l.Path())
	if info.Mode().Perm() != 0o600 {
		t.Errorf("log permissions %o: logs can hold secrets a harness printed", info.Mode().Perm())
	}
}

func TestTwoSessionsNeverShareAFile(t *testing.T) {
	d := Dir{Root: t.TempDir(), Now: fixed(time.Unix(0, 0))}
	if _, err := d.Open("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Open("a"); err == nil {
		t.Error("a second session with the same timestamp reused the first one's log")
	}
}

func TestInstanceIdCannotEscapeTheFolder(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "../x", "a/b", `a\b`} {
		if _, err := (Dir{Root: t.TempDir()}).Open(bad); err == nil {
			t.Errorf("instance %q accepted", bad)
		} else if !strings.Contains(err.Error(), "invalid instance") {
			t.Errorf("instance %q: unexpected error %v", bad, err)
		}
	}
}
