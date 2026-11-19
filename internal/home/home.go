// Package home is Foreman on this machine (wiki/product/models/home.md): its
// home (~/.foreman: settings.toml, the access token, the harness folders) and
// its folder (~/foreman: Foreman's HQ with the teams and the CEO's
// instructions, each project's HQ and files, the agents' worktrees), and the
// projects (records in the database). It loads and validates the config
// files, checks each project's repos, saves version-guarded (in an HQ, one
// commit each), and watches its files change on disk.
package home

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/brickpop/foreman/internal/store"
	"github.com/fsnotify/fsnotify"
)

const tokenFile = "access-token"

// Home is Foreman's home, ~/.foreman (or --home), and its folder.
type Home struct {
	root   string // absolute
	folder string // Foreman's folder, a real path (Folder); set by Load

	// Set by Load.
	git      Git
	machine  store.Projects // this machine's part of each project (projects.toml has the rest)
	deployed func(ctx context.Context, project string) bool
	allowed  string // Foreman's folder and repos must be inside it: the CEO's home folder, its real path
	start    string // the folder browser opens there: where Foreman was started (inside allowed), its real path

	mu      sync.Mutex // guards snap, saves and pending
	snap    snapshot
	synced  map[string]Backup // each HQ after the start's sync, by project ("" Foreman's HQ); Problem "" for none
	notify  func(Change)      // set by Watch
	watcher *fsnotify.Watcher // set by Watch
	// rewatch: a project was made, its HQ to watch (unlock).
	rewatch bool
	// pending are the changes to tell the subscriber once h.mu is released
	// (unlock): never under it, since the subscriber may read the home.
	pending []Change
}

// Root is the home's absolute path.
func (h *Home) Root() string { return h.root }

// Open returns the home at dir, creating the folder if it isn't there.
func Open(dir string) (*Home, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	// 0700: it holds the access token, logins and secrets.
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	return &Home{root: root}, nil
}

// Path returns a path inside the home.
func (h *Home) Path(elem ...string) string {
	return filepath.Join(append([]string{h.root}, elem...)...)
}

// AccessToken returns the per-install access token, generating it on first use.
// With rotate, a new token replaces the stored one, which logs every browser out.
func (h *Home) AccessToken(rotate bool) (string, error) {
	return secret(h.Path(tokenFile), rotate)
}

// AgentSecret signs the agent link's tokens (internal/agentlink), generated on
// first use. Kept under credentials/.
func (h *Home) AgentSecret() ([]byte, error) {
	if err := os.MkdirAll(h.Path("credentials"), 0o700); err != nil {
		return nil, err
	}
	s, err := secret(h.Path("credentials", "agent-link"), false)
	return []byte(s), err
}

// secret reads a random secret from path, or makes one (also with rotate).
func secret(path string, rotate bool) (string, error) {
	if !rotate {
		data, err := os.ReadFile(path)
		if err == nil {
			if token := strings.TrimSpace(string(data)); token != "" {
				return token, nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	// Write then rename, so a crash never leaves a truncated token behind.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(token+"\n"), 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return token, nil
}

// Git is what the home needs of git: the HQs it makes and commits to
// (wiki/product/models/home.md#foremans-hq), and telling a project's repos
// apart.
type Git interface {
	Init(ctx context.Context, dir, branch string) error
	Commit(ctx context.Context, dir, message string, files ...string) error
	// RootCommit is a first commit of the repo, which names it wherever it goes.
	RootCommit(ctx context.Context, dir string) (string, error)
	// Resolve is the commit ref names; an error when the repo hasn't it.
	Resolve(ctx context.Context, dir, ref string) (string, error)
	// Clone clones source (a URL or a local path) into dir, as the CEO.
	Clone(ctx context.Context, source, dir string) error
	// MoveWorktree moves one of repo's worktrees (a repo renamed).
	MoveWorktree(ctx context.Context, repo, from, to string) error
	// PruneWorktrees makes repo forget its worktrees whose folder is gone.
	PruneWorktrees(ctx context.Context, repo string) error
	// Branches lists dir's branches that start with prefix, by name.
	Branches(ctx context.Context, dir, prefix string) ([]string, error)
	// Branch is the branch dir's checkout is on; "" when it is detached.
	Branch(ctx context.Context, dir string) (string, error)
	HasBranch(ctx context.Context, dir, branch string) bool
	// Origin is the repo's origin URL, without credentials; "" when none.
	Origin(ctx context.Context, dir string) string
	// MatchOrigin verifies whether a managed checkout can be shared for remote.
	MatchOrigin(ctx context.Context, dir, remote string) error
	// Pull brings origin's branch into dir's checkout, never over
	// uncommitted changes, aborting a conflict (git.ErrNoRemote: no origin).
	Pull(ctx context.Context, dir, branch string) error
	// Push pushes dir's branch to origin's, never forcing.
	Push(ctx context.Context, dir, branch string) error
	// Uncommitted lists what dir's checkout holds not committed.
	Uncommitted(ctx context.Context, dir string) ([]string, error)
}

// Deps is what Load needs besides the home's files.
type Deps struct {
	Git      Git
	Projects store.Projects
	// Deployed says whether a team is deployed on the project: its repos
	// can't be taken away from under its agents.
	Deployed func(ctx context.Context, project string) bool
	// Start is the folder Foreman was started in: the folder browser opens
	// there (it can go up to the CEO's home folder), or at the CEO's home
	// folder when it is outside that, or empty.
	Start string
}
