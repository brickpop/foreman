package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Only macOS's open, or xdg-open on a Linux with a display, can open a
// folder; anywhere else (a headless Linux, a container, a missing program,
// another OS) there is no opener, so the UI shows no Open button.
func TestDetect(t *testing.T) {
	found := func(name string) (string, error) { return "/usr/bin/" + name, nil }
	missing := func(string) (string, error) { return "", errors.New("not found") }
	env := func(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }
	for _, tc := range []struct {
		name, goos string
		env        map[string]string
		lookPath   func(string) (string, error)
		want       string
	}{
		{"macOS", "darwin", nil, found, "/usr/bin/open"},
		{"macOS without open", "darwin", nil, missing, ""},
		{"Linux on X11", "linux", map[string]string{"DISPLAY": ":0"}, found, "/usr/bin/xdg-open"},
		{"Linux on Wayland", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, found, "/usr/bin/xdg-open"},
		{"Linux without a display", "linux", nil, found, ""},
		{"Linux without xdg-open", "linux", map[string]string{"DISPLAY": ":0"}, missing, ""},
		{"Windows", "windows", map[string]string{"DISPLAY": ":0"}, found, ""},
	} {
		o := detect(tc.goos, env(tc.env), tc.lookPath)
		if o.program != tc.want || o.CanOpen() != (tc.want != "") {
			t.Errorf("%s: program %q, want %q", tc.name, o.program, tc.want)
		}
	}
}

// fakeOpener is an opener whose program records the arguments it got.
func fakeOpener(t *testing.T) (Opener, string) {
	t.Helper()
	dir := t.TempDir()
	got := filepath.Join(dir, "args")
	program := filepath.Join(dir, "opener")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > '" + got + "'\n"
	if err := os.WriteFile(program, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return Opener{program: program}, got
}

// The folder reaches the program as its one argument, untouched by any
// shell, spaces and quotes included.
func TestOpenAFolder(t *testing.T) {
	o, got := fakeOpener(t)
	dir := filepath.Join(t.TempDir(), "my files; 'rm -rf'")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := o.Open(dir); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(got)
		if err == nil && strings.HasSuffix(string(data), "\n") {
			if string(data) != dir+"\n" {
				t.Errorf("the program got %q, want only %q", data, dir)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the program never ran")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Anything but a real folder is refused before any program runs: a relative
// path, a file, a missing folder, a symlink (to a folder or not), a folder
// with an extension (macOS would launch x.app), and every path without a
// desktop.
func TestOpenRefuses(t *testing.T) {
	o, got := fakeOpener(t)
	root := t.TempDir()
	file := filepath.Join(root, "notes.txt")
	folder := filepath.Join(root, "real")
	link := filepath.Join(root, "link")
	bundle := filepath.Join(root, "Evil.app")
	for _, d := range []string{folder, bundle} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(folder, link); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"real", "", file, filepath.Join(root, "missing"), link, bundle} {
		if err := o.Open(dir); !errors.Is(err, ErrNotAFolder) {
			t.Errorf("%q: %v, want ErrNotAFolder", dir, err)
		}
	}
	if err := (Opener{}).Open(folder); !errors.Is(err, ErrUnavailable) {
		t.Errorf("no desktop: %v, want ErrUnavailable", err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Errorf("the program ran for a refused path")
	}
}
