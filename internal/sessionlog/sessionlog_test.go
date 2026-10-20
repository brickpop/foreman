package sessionlog

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
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

func names(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

// An instance's folder stays under the cap: a session goes on in a new part
// each quarter of the cap, and the oldest files go first, older sessions
// before its own parts. Tail reads across the parts left.
func TestTheCapSplitsSessionsAndDropsTheOldest(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "mara@ledger")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, size := range map[string]int{"20261011T000000.000000000Z.log": 60, "20261012T000000.000000000Z.log": 30} {
		if err := os.WriteFile(filepath.Join(dir, name), bytes.Repeat([]byte("o"), size), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	d := Dir{Root: root, Now: fixed(time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)), Cap: func() int64 { return 100 }}
	l, err := d.Open("mara@ledger")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	const s = "20261013T000000.000000000Z"
	chunk := func(c byte) []byte { return bytes.Repeat([]byte{c}, 25) }
	// 90 + 25: over the cap once the first part closes, so the oldest goes.
	_, _ = l.Write(chunk('a'))
	if got := names(t, dir); !slices.Equal(got, []string{"20261012T000000.000000000Z.log", s + ".log", s + ".p000002.log"}) {
		t.Errorf("after one part: %v", got)
	}
	for _, c := range []byte("bcde") {
		_, _ = l.Write(chunk(c))
	}
	// The older session went, then the session's own first part.
	if got := names(t, dir); !slices.Equal(got, []string{s + ".p000002.log", s + ".p000003.log", s + ".p000004.log", s + ".p000005.log", s + ".p000006.log"}) {
		t.Errorf("after five parts: %v", got)
	}
	want := string(bytes.Join([][]byte{chunk('b'), chunk('c'), chunk('d'), chunk('e')}, nil))
	if got, err := l.Tail(1000); err != nil || string(got) != want {
		t.Errorf("Tail across parts = %q, %v", got, err)
	}
	if got, _ := l.Tail(30); string(got) != want[len(want)-30:] {
		t.Errorf("Tail(30) across two parts = %q", got)
	}
	if l.Path() != filepath.Join(dir, s+".p000006.log") {
		t.Errorf("writing %s", l.Path())
	}
}

// A new session prunes what older ones left over the cap, which is read when
// asked: a change applies from the next part.
func TestTheCapIsReadWhenAsked(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "a")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"20261010T000000.000000000Z.log", "20261011T000000.000000000Z.log", "20261012T000000.000000000Z.log"} {
		if err := os.WriteFile(filepath.Join(dir, name), bytes.Repeat([]byte("o"), 60), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	limit := int64(0)
	d := Dir{Root: root, Now: fixed(time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)), Cap: func() int64 { return limit }}
	l, err := d.Open("a")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = l.Write(bytes.Repeat([]byte("x"), 500))
	if got := names(t, dir); len(got) != 4 {
		t.Errorf("no cap: %v", got)
	}
	_ = l.Close()
	limit = 100
	if _, err := (Dir{Root: root, Now: fixed(time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)), Cap: d.Cap}).Open("a"); err != nil {
		t.Fatal(err)
	}
	// 3×60 + 500 over 100: everything older goes, the new (empty) session stays.
	if got := names(t, dir); !slices.Equal(got, []string{"20261014T000000.000000000Z.log"}) {
		t.Errorf("after the cap: %v", got)
	}
}
