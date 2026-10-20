package userdata

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// RunningError means another Foreman holds the home's lock: it serves URL,
// on Workspace.
type RunningError struct{ URL, Workspace string }

func (e *RunningError) Error() string {
	return "Foreman is already running on this machine"
}

// Lock is the home's lock, held as long as its file stays open: the system
// releases it when the process ends, however it ends, so it is never left
// behind by a crash.
type Lock struct{ file *os.File }

// TakeLock takes the lock in home (server.lock), or says who holds it
// (*RunningError).
func TakeLock(home string) (*Lock, error) {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, err
	}
	// 0600: it will hold the URL with the access token.
	f, err := os.OpenFile(filepath.Join(home, "server.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		defer f.Close()
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("locking %s: %w", f.Name(), err)
		}
		data, _ := os.ReadFile(f.Name())
		url, workspace, _ := strings.Cut(strings.TrimSpace(string(data)), "\n")
		return nil, &RunningError{URL: url, Workspace: workspace}
	}
	return &Lock{file: f}, nil
}

// Announce writes where this Foreman serves, for the next one to say.
func (l *Lock) Announce(url, workspace string) error {
	if err := l.file.Truncate(0); err != nil {
		return err
	}
	_, err := l.file.WriteAt([]byte(url+"\n"+workspace+"\n"), 0)
	return err
}

// Release lets the lock go, and forgets the URL: for a server that stops
// without its process ending (tests).
func (l *Lock) Release() error {
	_ = l.file.Truncate(0)
	return l.file.Close()
}
