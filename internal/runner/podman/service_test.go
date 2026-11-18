package podman

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
)

// What the podman CLI leaves running (`machine start`'s VM and gvproxy) is in
// no process group of Foreman's, so the terminal's Ctrl-C on Foreman never
// reaches it: a fake podman reports the group it runs in.
func TestPodmanRunsOutsideForemansGroup(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nps -o pgid= -p $$\n"
	if err := os.WriteFile(filepath.Join(bin, "podman"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := podmanOutput(t.Context(), "machine", "start")
	if err != nil {
		t.Fatal(err)
	}
	group, err := strconv.Atoi(out)
	if err != nil {
		t.Fatalf("pgid %q: %v", out, err)
	}
	if group == syscall.Getpgrp() {
		t.Errorf("podman ran in Foreman's process group %d", group)
	}
}
