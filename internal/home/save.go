package home

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ErrConflict means the file changed since the version the caller read:
// reload, then save again.
var ErrConflict = errors.New("home: changed since it was read")

// InvalidError lists why a save was refused; nothing was written.
type InvalidError struct{ Problems []string }

func (e *InvalidError) Error() string {
	return "invalid: " + strings.Join(e.Problems, "; ")
}

// SaveTeam writes a team file and commits it to Foreman's HQ. version
// is the one read ("" for a new team). It returns the saved team.
func (h *Home) SaveTeam(ctx context.Context, c TeamConfig, version string) (Team, error) {
	h.mu.Lock()
	defer h.unlock()
	if p := h.CheckTeam(c); len(p) > 0 {
		return Team{}, &InvalidError{p}
	}
	data, err := encodeTeam(c)
	if err != nil {
		return Team{}, err
	}
	if err := h.write(ctx, h.HQ(teamsDir, c.Team+".toml"), version, data, c.Name+": saved in the UI"); err != nil {
		return Team{}, err
	}
	return h.snap.teams[c.Team], nil
}

// SaveSettings writes settings.toml, this machine's, in no repo. version is
// the one read ("" while the file doesn't exist). It returns the new version.
func (h *Home) SaveSettings(ctx context.Context, s Settings, version string) (string, error) {
	h.mu.Lock()
	defer h.unlock()
	if p := s.validate(); len(p) > 0 {
		return "", &InvalidError{p}
	}
	data, err := encode(s)
	if err != nil {
		return "", err
	}
	if err := h.write(ctx, h.Path(settingsFile), version, data, ""); err != nil {
		return "", err
	}
	return h.snap.settings.Version, nil
}

// SaveInstructions writes Foreman's HQ's instructions.md and commits it.
// version is the one read. It returns them as saved.
func (h *Home) SaveInstructions(ctx context.Context, text, version string) (Instructions, error) {
	h.mu.Lock()
	defer h.unlock()
	if err := h.write(ctx, h.HQ(instructionsFile), version, []byte(text), "Instructions: saved in the UI"); err != nil {
		return Instructions{}, err
	}
	return h.snap.instructions, nil
}

// write replaces a file if it is still at version, commits it to Foreman's
// HQ with message (none for ""), then reloads and queues the changes for the
// subscriber. Called with h.mu held, released with unlock.
func (h *Home) write(ctx context.Context, path, version string, data []byte, message string) error {
	current, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if version != "" {
			return ErrConflict
		}
	case err != nil:
		return err
	case Version(current) != version:
		return ErrConflict
	case string(current) == string(data):
		return nil // nothing to write, nothing to commit
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Write then rename, so a reader (the watcher, the CEO's editor) never
	// sees half a file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	old := h.snap
	h.snap = h.scan(old)
	h.publish(diff(old, h.snap))
	if message == "" {
		return nil
	}
	rel, err := filepath.Rel(h.HQ(), path)
	if err != nil {
		return err
	}
	if err := h.git.Commit(ctx, h.HQ(), message, rel); err != nil {
		return fmt.Errorf("saved, but not committed to Foreman's HQ: %w", err)
	}
	return nil
}

// publish queues changes for the subscriber. Called with h.mu held, which
// is released with unlock.
func (h *Home) publish(changes []Change) {
	if h.notify != nil {
		h.pending = append(h.pending, changes...)
	}
	// A project made has an HQ to watch now (unlock).
	if slices.ContainsFunc(changes, func(c Change) bool { return c.Kind == "projects" || c.Kind == "project" }) {
		h.rewatch = true
	}
}

// unlock releases h.mu, then tells the subscriber what changed meanwhile.
// Under the lock, a subscriber reading the home (the instance manager
// reading max_sessions on a settings change) would wait on itself forever,
// and every request after it.
func (h *Home) unlock() {
	changes, notify, rewatch := h.pending, h.notify, h.rewatch && h.watcher != nil
	h.pending, h.rewatch = nil, false
	h.mu.Unlock()
	// Before telling: a change made right after it is then seen.
	if rewatch {
		h.watchDirs(h.watcher)
	}
	for _, c := range changes {
		notify(c)
	}
}

// CheckTeam is what a save of c would refuse, without saving: the team
// file's rules. None span teams: an agent's id and name are its team's own.
func (h *Home) CheckTeam(c TeamConfig) []string {
	return c.validate(c.Team)
}
