package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrConflict means the file changed since the version the caller read:
// reload, then save again.
var ErrConflict = errors.New("workspace: changed since it was read")

// InvalidError lists why a save was refused; nothing was written.
type InvalidError struct{ Problems []string }

func (e *InvalidError) Error() string {
	return "invalid: " + strings.Join(e.Problems, "; ")
}

// SaveTeam writes a team file and commits it to the workspace repo. version
// is the one read ("" for a new team). It returns the saved team.
func (w *Workspace) SaveTeam(ctx context.Context, c TeamConfig, version string) (Team, error) {
	w.mu.Lock()
	defer w.unlock()
	if p := w.checkTeam(c); len(p) > 0 {
		return Team{}, &InvalidError{p}
	}
	data, err := encodeTeam(c)
	if err != nil {
		return Team{}, err
	}
	rel := filepath.Join(teamsDir, c.Team+".toml")
	name := c.Name
	if err := w.write(ctx, rel, version, data, name+": saved in the UI"); err != nil {
		return Team{}, err
	}
	return w.snap.teams[c.Team], nil
}

// SaveSettings writes settings.toml and commits it. version is the one read
// ("" while the file doesn't exist). It returns the new version.
func (w *Workspace) SaveSettings(ctx context.Context, s Settings, version string) (string, error) {
	w.mu.Lock()
	defer w.unlock()
	if p := s.validate(); len(p) > 0 {
		return "", &InvalidError{p}
	}
	data, err := encode(s)
	if err != nil {
		return "", err
	}
	if err := w.write(ctx, settingsFile, version, data, "Settings: saved in the UI"); err != nil {
		return "", err
	}
	return w.snap.settings.Version, nil
}

// write replaces a file under .foreman/ if it is still at version, commits
// it, then reloads and queues the changes for the subscriber. Called with
// w.mu held, released with unlock.
func (w *Workspace) write(ctx context.Context, rel, version string, data []byte, message string) error {
	path := w.Path(rel)
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
	old := w.snap
	w.snap = w.scan(old)
	w.publish(diff(old, w.snap))
	if err := w.git.Commit(ctx, w.Path(), message, rel); err != nil {
		return fmt.Errorf("saved, but not committed to the workspace repo: %w", err)
	}
	return nil
}

// publish queues changes for the subscriber. Called with w.mu held, which
// is released with unlock.
func (w *Workspace) publish(changes []Change) {
	if w.notify != nil {
		w.pending = append(w.pending, changes...)
	}
}

// unlock releases w.mu, then tells the subscriber what changed meanwhile.
// Under the lock, a subscriber reading the workspace (the instance manager
// reading max_sessions on a settings change) would wait on itself forever,
// and every request after it.
func (w *Workspace) unlock() {
	changes, notify := w.pending, w.notify
	w.pending = nil
	w.mu.Unlock()
	for _, c := range changes {
		notify(c)
	}
}

// CheckTeam is what a save of c would refuse, without saving: the team
// file's rules, and those across teams against every other team as it is now.
func (w *Workspace) CheckTeam(c TeamConfig) []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.checkTeam(c)
}

func (w *Workspace) checkTeam(c TeamConfig) []string {
	if p := c.validate(c.Team); len(p) > 0 {
		return p
	}
	teams := union(w.snap.teams, map[string]Team{c.Team: {ID: c.Team, Config: c}})
	crossTeamProblems(teams)
	return teams[c.Team].Problems
}
