// Package desktop opens a folder on the desktop of the machine Foreman runs
// on, which is the CEO's (Foreman listens on 127.0.0.1 only): Finder on
// macOS, the file manager xdg-open picks on Linux
// (wiki/product/features/sessions.md#agent-tabs).
package desktop

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var (
	// ErrUnavailable: this machine has no desktop to open a folder on.
	ErrUnavailable = errors.New("no desktop to open folders on")
	// ErrNotAFolder: the path isn't one Foreman opens (see Open).
	ErrNotAFolder = errors.New("not a folder")
)

// Opener opens folders with the desktop's program; its zero value has none.
type Opener struct {
	program string // its full path; "" without a desktop
}

// Detect finds this machine's opener: open on macOS; xdg-open on Linux, and
// only with a display (none over SSH, in a container or a headless service).
func Detect() Opener { return detect(runtime.GOOS, os.Getenv, exec.LookPath) }

func detect(goos string, getenv func(string) string, lookPath func(string) (string, error)) Opener {
	name := ""
	switch goos {
	case "darwin":
		name = "open"
	case "linux":
		if getenv("DISPLAY") == "" && getenv("WAYLAND_DISPLAY") == "" {
			return Opener{}
		}
		name = "xdg-open"
	default:
		return Opener{}
	}
	program, err := lookPath(name)
	if err != nil {
		return Opener{}
	}
	return Opener{program: program}
}

// CanOpen says whether this machine can open folders.
func (o Opener) CanOpen() bool { return o.program != "" }

// Open opens dir in the desktop's file manager. Only a real folder: an
// absolute path, not a symlink (an agent could point one anywhere), and with
// no extension, since macOS's open launches a bundle (x.app) rather than
// showing it; Foreman's folders have none. It returns once the program has
// started, never waiting for the window: xdg-open may stay until the file
// manager closes.
func (o Opener) Open(dir string) error {
	if !o.CanOpen() {
		return ErrUnavailable
	}
	if !filepath.IsAbs(dir) || filepath.Ext(dir) != "" {
		return fmt.Errorf("%w: %s", ErrNotAFolder, dir)
	}
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrNotAFolder, dir)
	}
	// The program itself, no shell: the folder is its only argument, and
	// absolute, so it can't be taken for an option.
	cmd := exec.Command(o.program, dir)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening %s: %w", dir, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
