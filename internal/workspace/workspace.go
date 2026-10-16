// Package workspace is the files the CEO owns (wiki/product/models/workspace.md):
// it locates a workspace, keeps its .foreman/ folder (the workspace repo, the
// access token), loads and validates the config files, saves them version-
// guarded with one commit each, and watches them change on disk.
package workspace

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DirName is the folder holding Foreman's own data inside a workspace.
const DirName = ".foreman"

const tokenFile = "access-token"

// Workspace is a folder whose subfolders are projects.
type Workspace struct {
	root string // absolute path of the workspace folder

	// Set by Load.
	git    Git
	mu     sync.Mutex // guards snap and saves
	snap   snapshot
	notify func(Change) // set by Watch
}

// Root is the absolute path of the workspace folder.
func (w *Workspace) Root() string { return w.root }

// ErrNotInitialized means the folder has no .foreman/ yet.
var ErrNotInitialized = errors.New("not a Foreman workspace")

// Open returns the workspace at dir, or ErrNotInitialized if dir has no .foreman/.
func Open(dir string) (*Workspace, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a folder", root)
	}
	info, err = os.Stat(filepath.Join(root, DirName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotInitialized
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s exists but is not a folder", filepath.Join(root, DirName))
	}
	return &Workspace{root: root}, nil
}

// Init creates .foreman/ in dir and returns the workspace.
func Init(dir string) (*Workspace, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	// 0700: the folder holds the access token and, later, secrets.
	if err := os.Mkdir(filepath.Join(root, DirName), 0o700); err != nil {
		return nil, err
	}
	return &Workspace{root: root}, nil
}

// Path returns a path inside .foreman/.
func (w *Workspace) Path(elem ...string) string {
	return filepath.Join(append([]string{w.root, DirName}, elem...)...)
}

// AccessToken returns the per-install access token, generating it on first use.
// With rotate, a new token replaces the stored one, which logs every browser out.
func (w *Workspace) AccessToken(rotate bool) (string, error) {
	return secret(w.Path(tokenFile), rotate)
}

// AgentSecret signs the agent link's tokens (internal/agentlink), generated on
// first use. Kept under credentials/, which the workspace repo ignores.
func (w *Workspace) AgentSecret() ([]byte, error) {
	if err := os.MkdirAll(w.Path("credentials"), 0o700); err != nil {
		return nil, err
	}
	s, err := secret(w.Path("credentials", "agent-link"), false)
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

// Git is what the workspace needs of git: the workspace repo
// (wiki/product/models/workspace.md#the-workspace-repo).
type Git interface {
	Init(ctx context.Context, dir, branch string) error
	Commit(ctx context.Context, dir, message string, files ...string) error
}
