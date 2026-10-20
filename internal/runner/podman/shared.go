package podman

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// sharedBuild is an image's build as every Foreman on this machine sees it:
// the image is one for every home, so one Foreman builds it at a time
// (a lock file), and each shows the same state and log, wherever the build
// runs. Files under dir: <key>.lock, <key>.state (building, ready or
// failed) and <key>.log, the last build's output.
type sharedBuild struct {
	dir, key string
}

// lockPoll is how often a Foreman waiting for another's build looks again.
const lockPoll = 200 * time.Millisecond

func (b sharedBuild) path(ext string) string { return filepath.Join(b.dir, b.key+ext) }

// lock waits for the build lock (another Foreman's build ends first) and
// returns its release.
func (b sharedBuild) lock(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(b.dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(b.path(".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { _ = f.Close() }, nil // closing releases it
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			_ = f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(lockPoll):
		}
	}
}

// busy reports whether a build holds the lock now, in this Foreman or another.
func (b sharedBuild) busy() bool {
	f, err := os.OpenFile(b.path(".lock"), os.O_RDWR, 0)
	if err != nil {
		return false // never built here
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return true
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}

// begin marks a build started (holding the lock): building, an empty log.
func (b sharedBuild) begin() error {
	if err := os.WriteFile(b.path(".log"), nil, 0o600); err != nil {
		return err
	}
	return b.set("building")
}

func (b sharedBuild) set(state string) error {
	return os.WriteFile(b.path(".state"), []byte(state), 0o600)
}

func (b sharedBuild) appendLog(s string) {
	f, err := os.OpenFile(b.path(".log"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(s)
}

// state is the last build's state and the end of its log. A build that says
// building without holding the lock was cut short (its Foreman stopped): it
// failed. "" when nothing was built here yet.
func (b sharedBuild) state() (string, string) {
	data, _ := os.ReadFile(b.path(".state"))
	state := strings.TrimSpace(string(data))
	logData, _ := os.ReadFile(b.path(".log"))
	if len(logData) > maxLog {
		logData = logData[len(logData)-maxLog:]
	}
	log := string(logData)
	busy := b.busy()
	switch {
	case busy:
		return "building", log
	case state == "building":
		return "failed", log + "\nThe build stopped before it ended: the Foreman running it exited.\n"
	}
	return state, log
}
