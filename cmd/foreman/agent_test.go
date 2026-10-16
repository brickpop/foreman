package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAgent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "foreman") // created on demand
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	if path, err := writeAgent(dir, nil, now); path != "" || err != nil {
		t.Fatalf("not built: %q, %v", path, err)
	}
	v1, err := writeAgent(dir, []byte("v1"), now)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(v1); !bytes.Equal(data, []byte("v1")) {
		t.Fatalf("wrote %q", data)
	}
	if info, _ := os.Stat(v1); info.Mode().Perm()&0o111 == 0 {
		t.Errorf("not executable: %v", info.Mode())
	}

	// Another version gets its own copy, and v1, used a day ago, stays.
	v2, err := writeAgent(dir, []byte("v2"), now.Add(24*time.Hour))
	if err != nil || v2 == v1 {
		t.Fatalf("v2: %q, %v", v2, err)
	}
	if _, err := os.Stat(v1); err != nil {
		t.Fatalf("v1 was removed while recently used: %v", err)
	}

	// v1 starting again refreshes its copy, so v2 starting a week later keeps it.
	if again, err := writeAgent(dir, []byte("v1"), now.Add(5*24*time.Hour)); again != v1 || err != nil {
		t.Fatalf("v1 again: %q, %v", again, err)
	}
	stray := filepath.Join(dir, "unrelated")
	leftover := v1 + ".tmp" // a write cut short long ago
	for _, f := range []string{stray, leftover} {
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(f, now, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := writeAgent(dir, []byte("v2"), now.Add(8*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(v1); err != nil {
		t.Errorf("v1, started 3 days ago, was removed: %v", err)
	}
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Errorf("an old unfinished copy stays: %v", err)
	}
	if _, err := os.Stat(stray); err != nil {
		t.Errorf("a file that isn't foreman-agent was removed: %v", err)
	}

	// Unused for over a week: removed by whichever Foreman starts next.
	if _, err := writeAgent(dir, []byte("v2"), now.Add(13*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(v1); !os.IsNotExist(err) {
		t.Errorf("v1, unused for 8 days, stays: %v", err)
	}
	if _, err := os.Stat(v2); err != nil {
		t.Errorf("the copy in use was removed: %v", err)
	}
}
