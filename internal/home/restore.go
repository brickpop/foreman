package home

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Coming to a new machine (wiki/backlog/portable-projects.md): Foreman's HQ
// restored from its origin, then each project brought here from the HQ
// origin its record keeps.

// restoreDir is where Restore clones, in Foreman's folder, before the clone
// replaces the HQ's contents.
const restoreDir = ".hq-restore"

// Pristine says whether Foreman's HQ is still as its first start made it,
// so Restore may replace it: its first commit only, nothing changed since,
// no team and no project.
func (h *Home) Pristine(ctx context.Context) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pristine(ctx)
}

// pristine is Pristine. Called with h.mu held.
func (h *Home) pristine(ctx context.Context) bool {
	if len(h.snap.teams) > 0 || exists(h.HQ(projectsFile)) {
		return false
	}
	if _, err := h.git.Resolve(ctx, h.HQ(), "HEAD~1"); err == nil {
		return false
	}
	files, err := h.git.Uncommitted(ctx, h.HQ())
	return err == nil && len(files) == 0
}

// Restore replaces a pristine Foreman's HQ with a clone of url (a URL or a
// local path, as the CEO's git reaches it): the HQ of another machine, its
// teams, instructions and projects. The clone must be on main, as every HQ
// is, and hold what an HQ holds. The HQ's folder stays (the watcher keeps
// following it): its contents are swapped.
func (h *Home) Restore(ctx context.Context, url string) error {
	h.mu.Lock()
	defer h.unlock()
	if !h.pristine(ctx) {
		return fmt.Errorf("%w: Foreman's HQ has teams, projects or changes of its own: Restore only replaces the one made at first start", ErrExists)
	}
	tmp := filepath.Join(h.folder, restoreDir)
	// What a restore stopped halfway left.
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := h.git.Clone(ctx, url, tmp); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}
	if !h.git.HasBranch(ctx, tmp, hqBranch) {
		return fmt.Errorf("%w: %s has no %s branch: Foreman's HQ is on %s", ErrInvalid, url, hqBranch, hqBranch)
	}
	if !exists(filepath.Join(tmp, instructionsFile)) && !exists(filepath.Join(tmp, teamsDir)) && !exists(filepath.Join(tmp, projectsFile)) {
		return fmt.Errorf("%w: %s doesn't look like Foreman's HQ (no %s, %s/ or %s)", ErrInvalid, url, instructionsFile, teamsDir, projectsFile)
	}
	if err := swapContents(h.HQ(), tmp); err != nil {
		return err
	}
	// What the start's sync said of the HQ replaced (no origin) is past.
	delete(h.synced, "")
	old := h.snap
	h.snap = h.scan(old)
	h.publish(diff(old, h.snap))
	return nil
}

// swapContents empties dir, then moves every entry of from into it.
func swapContents(dir, from string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	entries, err = os.ReadDir(from)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(from, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// BringHere clones the HQ of a project not on this machine (from another
// machine's records) from the origin its record keeps, into its place. Its
// code repos are then located or cloned one by one (CloneMissingRepo).
func (h *Home) BringHere(ctx context.Context, id string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, err := h.found(id)
	if err != nil {
		return Project{}, err
	}
	if p.Here {
		return Project{}, fmt.Errorf("%w: %s is on this machine already, at %s", ErrExists, id, p.Path)
	}
	url := p.Remote
	if url == "" {
		return Project{}, fmt.Errorf("%w: %s's record keeps no HQ origin to clone it from: its HQ never had one where it was made. Push it there, then run Foreman there once", ErrInvalid, id)
	}
	made, err := makeDir(h.projectDir(id))
	if err != nil {
		return Project{}, err
	}
	if err := h.git.Clone(ctx, url, p.Path); err != nil {
		if made != "" {
			_ = os.RemoveAll(made)
		}
		return Project{}, fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}
	return h.rescan(id)
}
