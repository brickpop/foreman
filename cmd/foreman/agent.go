package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// agentKeep is how long a foreman-agent copy no Foreman has started with is
// kept: a Foreman still running from before an upgrade mounts its own copy
// into every container it starts.
const agentKeep = 7 * 24 * time.Hour

// writeAgent puts foreman-agent (data) where containers can mount it: dir, in
// the user's cache folder, which Podman's machine shares on macOS. Each copy
// is named by its content, so Foreman versions don't overwrite each other's,
// and copies unused for agentKeep are removed. "" when there is none (not
// built).
func writeAgent(dir string, data []byte, now time.Time) (string, error) {
	if data == nil {
		return "", nil
	}
	sum := sha256.Sum256(data)
	path := filepath.Join(dir, "foreman-agent-"+hex.EncodeToString(sum[:6]))
	defer pruneAgents(dir, path, now)
	if _, err := os.Stat(path); err == nil {
		return path, os.Chtimes(path, now, now) // in use again: not pruned
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return "", err
	}
	if err := os.Chtimes(tmp, now, now); err != nil {
		return "", err
	}
	return path, os.Rename(tmp, path)
}

// pruneAgents removes the foreman-agent copies in dir, other than keep, that
// no Foreman has started with for agentKeep. Best effort: a copy left behind
// only takes space.
func pruneAgents(dir, keep string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if !strings.HasPrefix(e.Name(), "foreman-agent-") || path == keep {
			continue
		}
		if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) > agentKeep {
			_ = os.Remove(path)
		}
	}
}
