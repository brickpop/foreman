package home

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Foreman's folder (wiki/product/models/home.md#foremans-folder): what the CEO
// owns, apart from Foreman's machinery in its home (~/.foreman). Every path
// follows from it and a project's id:
//
//	hq/                                  Foreman's HQ: teams/, instructions.md
//	projects/<project>/hq/               the project's HQ
//	projects/<project>/files/            the files for and from its agents
//	repos/<path>/                        the code repos Foreman clones (clone.go)
//	worktrees/<project>/<agent>/<repo>/  the agents' worktrees: disposable
//
// Each agent's harness folder is in the home: harness/<project>/<agent>/.

const (
	hqDir        = "hq"        // an HQ's folder: Foreman's in the folder, a project's in projects/<project>/
	projectsDir  = "projects"  // in the folder: one folder per project
	worktreesDir = "worktrees" // in the folder: <project>/<agent>/<repo>
	reposDir     = "repos"     // in the folder: the code repos Foreman clones, by their URL's path
	filesDir     = "files"     // in a project's folder: the files shared with its agents (Project.Files)
	harnessDir   = "harness"   // in the home: each agent's private harness folder, <project>/<agent>
)

// Folder is Foreman's folder, a real path (the foreman_folder setting).
func (h *Home) Folder() string { return h.folder }

// HQ is a path inside Foreman's HQ.
func (h *Home) HQ(elem ...string) string {
	return filepath.Join(append([]string{h.folder, hqDir}, elem...)...)
}

// projectDir is a project's folder: its HQ and its files.
func (h *Home) projectDir(id string) string { return filepath.Join(h.folder, projectsDir, id) }

// worktrees is the folder of a project's agents' worktrees, one folder each.
func (h *Home) worktrees(id string) string { return filepath.Join(h.folder, worktreesDir, id) }

// Sandbox is an agent's sandbox on the project: its working directory,
// holding a worktree of each code repo it opened.
func (p Project) Sandbox(agent string) string { return filepath.Join(p.worktrees, agent) }

// HarnessDir is an agent's private harness folder: its config and
// conversations, in Foreman's home, which only the CEO can read.
func (p Project) HarnessDir(agent string) string { return filepath.Join(p.harness, agent) }

const (
	FilesInbox  = "inbox"  // the CEO's files for the agents: read-only for them
	FilesShared = "shared" // the agents' data that outlives a task and a team: every agent writes it
)

// FileFolders are the folders of a project's files
// (wiki/product/models/project-config.md#the-projects-folder).
var FileFolders = []string{FilesInbox, FilesShared}

// MakeFiles makes the project's file folders that are missing.
func (p Project) MakeFiles() error {
	for _, f := range FileFolders {
		if err := os.MkdirAll(filepath.Join(p.Files, f), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// WorktreeAgent is the agent whose sandbox holds a worktree (its folder is
// <sandbox>/<repo>), sandbox giving an agent's sandbox; "" when it is no
// agent's (the CEO's checkout, another project's agent). Folders compare as
// the file system sees them: git reports worktrees with symlinks resolved
// (macOS's /var is /private/var).
func WorktreeAgent(tree string, sandbox func(agent string) string) string {
	dir := filepath.Dir(tree)
	agent := filepath.Base(dir)
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	if resolve(sandbox(agent)) == resolve(dir) {
		return agent
	}
	return ""
}

// foremanFolder reads foreman_folder from settings.toml (the default while
// it doesn't exist) and checks it: after ~, an absolute path inside the CEO's
// home folder (agents reach it there; on macOS, what the Podman machine
// shares), neither inside Foreman's home nor holding it. It returns its real
// path, made if missing. Foreman can't start without it, so a settings.toml
// that doesn't parse stops the start too.
func (h *Home) foremanFolder() (string, error) {
	folder := DefaultSettings().ForemanFolder
	data, err := os.ReadFile(h.Path(settingsFile))
	switch {
	case err == nil:
		s := struct {
			ForemanFolder string `toml:"foreman_folder"`
		}{folder}
		if _, err := toml.Decode(string(data), &s); err != nil {
			return "", fmt.Errorf("%s: %w (foreman_folder is read from it)", h.Path(settingsFile), err)
		}
		folder = s.ForemanFolder
	case !errors.Is(err, os.ErrNotExist):
		return "", err
	}
	path, err := expandHome(folder)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("foreman_folder: %q must be an absolute path or start with ~/ (%s)", folder, h.Path(settingsFile))
	}
	real, err := realPath(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	home, err := filepath.EvalSymlinks(h.root)
	if err != nil {
		return "", err
	}
	switch {
	case !inside(real, h.allowed):
		return "", fmt.Errorf("foreman_folder: %s is outside your home folder (%s), where agents can't reach it", path, h.allowed)
	case inside(real, home) || inside(home, real):
		return "", fmt.Errorf("foreman_folder: %s overlaps Foreman's home (%s), which only Foreman uses", path, h.root)
	}
	if err := os.MkdirAll(real, 0o755); err != nil {
		return "", err
	}
	return real, nil
}

// usable refuses a path Foreman may not use for a repo (real: its real
// path): outside the CEO's home folder, or inside Foreman's home (the CEO,
// 2026-10-28).
func (h *Home) usable(path, real string) error {
	if !inside(real, h.allowed) {
		return fmt.Errorf("%w: %s is outside your home folder (%s)", ErrInvalid, path, h.allowed)
	}
	if home, err := filepath.EvalSymlinks(h.root); err == nil && inside(real, home) {
		return fmt.Errorf("%w: %s is inside Foreman's home (%s)", ErrInvalid, path, h.root)
	}
	return nil
}

// realPath is path's real path, resolving the folders of it that exist (a
// folder may not exist yet).
func realPath(path string) (string, error) {
	var rest []string
	for dir := path; ; dir = filepath.Dir(dir) {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(append([]string{real}, rest...)...), nil
		}
		if filepath.Dir(dir) == dir {
			return "", fmt.Errorf("%w: %s can't be resolved", ErrInvalid, path)
		}
		rest = append([]string{filepath.Base(dir)}, rest...)
	}
}

// expandHome turns a leading ~ into the CEO's home folder.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}

// makeDir creates dir and its missing parents, and returns the outermost one
// it created ("" when dir existed), which undoes it whole.
func makeDir(dir string) (string, error) {
	made := ""
	for d := dir; !exists(d); d = filepath.Dir(d) {
		made = d
	}
	return made, os.MkdirAll(dir, 0o755)
}
