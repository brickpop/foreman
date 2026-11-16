package home

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/brickpop/foreman/internal/git"
)

// Syncing the HQs with their origins (wiki/backlog/portable-projects.md):
// on start, each is brought up to date and pushed; on stop, pushed. Through
// the CEO's own git, its credentials and signing; Foreman never commits for
// anyone here. One machine at a time, so what comes in is a fast-forward.

// hqBranch is the branch every HQ is on.
const hqBranch = "main"

// syncTimeout bounds each step of a start's sync on one HQ (a pull, a push):
// a remote that hangs delays the start that long at most, never blocks it.
const syncTimeout = 30 * time.Second

// pushTimeout bounds each HQ's push on stop: the next start pushes what a
// slow one left.
const pushTimeout = 10 * time.Second

// Backup is an HQ not backed up by its origin, and why: none set, or the
// last sync failed.
type Backup struct {
	Project string `json:"project"` // "" for Foreman's HQ
	Path    string `json:"path"`
	Problem string `json:"problem"`
}

// Sync brings Foreman's HQ and every project's HQ up to date with its
// origin, then pushes it: Foreman's HQ first (projects.toml may bring
// projects in), then the projects' together, then each project's origin is
// written to projects.toml when it changed (a `git remote add` made since),
// and Foreman's HQ pushed last, with that commit. What fails is a Backup.
func (h *Home) Sync(ctx context.Context) {
	problems := map[string]string{}
	pulled := h.step(ctx, func(ctx context.Context) error { return h.git.Pull(ctx, h.HQ(), hqBranch) })
	h.reload()

	h.mu.Lock()
	var projects []Project
	for _, p := range values(h.snap.projects) {
		if exists(p.Path) {
			projects = append(projects, p)
		}
	}
	h.mu.Unlock()
	var mu sync.Mutex
	var wg sync.WaitGroup
	origins := map[string]string{}
	for _, p := range projects {
		wg.Go(func() {
			err := h.step(ctx, func(ctx context.Context) error { return h.git.Pull(ctx, p.Path, hqBranch) })
			if err == nil || !errors.Is(err, git.ErrNoRemote) {
				err = errors.Join(err, h.step(ctx, func(ctx context.Context) error { return h.git.Push(ctx, p.Path, hqBranch) }))
			}
			origin := h.git.Origin(ctx, p.Path)
			mu.Lock()
			defer mu.Unlock()
			problems[p.ID] = syncProblem(p.Path, err)
			origins[p.ID] = origin
		})
	}
	wg.Wait()

	h.mu.Lock()
	if err := h.refreshOrigins(ctx, origins); err != nil {
		slog.Warn("recording the projects' HQ origins", "err", err)
	}
	h.unlock()
	pushed := error(nil)
	if pulled == nil || !errors.Is(pulled, git.ErrNoRemote) {
		pushed = h.step(ctx, func(ctx context.Context) error { return h.git.Push(ctx, h.HQ(), hqBranch) })
	}
	problems[""] = syncProblem(h.HQ(), errors.Join(pulled, pushed))

	h.mu.Lock()
	h.synced = problems
	h.publish([]Change{{Kind: "projects"}})
	h.unlock()
	for id, problem := range problems {
		if problem != "" {
			slog.Warn("an HQ isn't backed up", "project", id, "problem", problem)
		}
	}
}

// step runs one git step on an HQ, bounded by syncTimeout.
func (h *Home) step(ctx context.Context, do func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	return do(ctx)
}

// refreshOrigins writes each project's HQ origin to projects.toml where it
// differs, in one commit. Called with h.mu held.
func (h *Home) refreshOrigins(ctx context.Context, origins map[string]string) error {
	changed := false
	for id, origin := range origins {
		if r, ok := h.snap.records.Records[id]; ok && r.HQ != origin {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return h.changeRecords(ctx, "Projects: HQ origins", func(records map[string]Record) {
		for id, origin := range origins {
			if r, ok := records[id]; ok {
				r.HQ = origin
				records[id] = r
			}
		}
	})
}

// syncProblem says why the HQ at dir isn't backed up after a sync's err; ""
// when it is.
func syncProblem(dir string, err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, git.ErrNoRemote):
		return noOrigin(dir)
	}
	return "not synced with its origin: " + strings.ReplaceAll(err.Error(), "\n", " ")
}

// noOrigin is the Backup problem of an HQ with no origin: what to run.
func noOrigin(dir string) string {
	return fmt.Sprintf("it has no origin: make an empty repo for it, then git -C %q remote add origin <its URL> && git -C %q push -u origin %s. Foreman pushes it at every start and stop from then on", dir, dir, hqBranch)
}

// Backups lists the HQs not backed up: Foreman's HQ and each project's, as
// the start's sync left them, and a project made since without an origin.
func (h *Home) Backups() []Backup {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := []Backup{}
	if problem := h.synced[""]; problem != "" {
		out = append(out, Backup{Path: h.HQ(), Problem: problem})
	}
	for _, p := range values(h.snap.projects) {
		problem, synced := h.synced[p.ID]
		if !synced && h.snap.records.Records[p.ID].HQ == "" && exists(p.Path) {
			problem = noOrigin(p.Path)
		}
		if problem != "" {
			out = append(out, Backup{Project: p.ID, Path: p.Path, Problem: problem})
		}
	}
	return out
}

// PushAll pushes every HQ to its origin, together, each bounded by
// pushTimeout: on stop. A failure is logged; the next start pushes it. What
// a project's HQ holds uncommitted isn't pushed (Foreman never commits for
// the lead or the CEO), and the log says so.
func (h *Home) PushAll(ctx context.Context) {
	h.mu.Lock()
	dirs := map[string]string{"": h.HQ()}
	for _, p := range h.snap.projects {
		if exists(p.Path) {
			dirs[p.ID] = p.Path
		}
	}
	h.mu.Unlock()
	var wg sync.WaitGroup
	for id, dir := range maps.All(dirs) {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(ctx, pushTimeout)
			defer cancel()
			if err := h.git.Push(ctx, dir, hqBranch); err != nil && !errors.Is(err, git.ErrNoRemote) {
				slog.Warn("pushing an HQ; the next start pushes it", "project", id, "err", err)
			}
			if files, err := h.git.Uncommitted(ctx, dir); err == nil && len(files) > 0 {
				slog.Warn("an HQ has changes not committed, so not pushed", "project", id, "hq", dir, "files", files)
			}
		})
	}
	wg.Wait()
}
