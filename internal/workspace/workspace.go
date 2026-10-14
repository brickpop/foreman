// Package workspace locates a Foreman workspace and owns the files in its
// .foreman/ folder that Foreman itself manages.
package workspace

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DirName is the folder holding Foreman's own data inside a workspace.
const DirName = ".foreman"

const tokenFile = "access-token"

// Workspace is a folder whose subfolders are projects.
type Workspace struct {
	root string // absolute path of the workspace folder
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
	path := w.Path(tokenFile)
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
