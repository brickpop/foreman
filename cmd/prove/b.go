package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Experiment B (wiki/backlog/prove-b-shared-login.md), step 1: log in to
// Claude Code once, inside a login container, the way Settings → Connect will.
// The login lands in the shared credentials folder, separate from the host's
// own `claude` login.

// credentialsDir is the experiments' shared login folder (git-ignored).
func credentialsDir() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".dev", "prove", "credentials", "claude-code")
}

// configDir is where the shared folder appears inside containers, as the
// harness's config dir.
const configDir = "/home/node/.foreman-claude"

func proveBLogin(r *report) {
	p := connectPodman(r)
	if p == nil {
		return
	}
	creds := credentialsDir()
	if !r.check(os.MkdirAll(creds, 0o700), "created the shared credentials folder "+creds) {
		return
	}
	removeLabelled(r, p, "b-login")
	// Move any previous login aside, so Claude really asks to log in: its interface
	// opens without contacting Anthropic, so an old, dead login looks fine until used.
	if old := filepath.Join(creds, ".credentials.json"); fileExists(old) {
		aside := old + ".old-" + time.Now().Format("20060102-150405")
		if !r.check(os.Rename(old, aside), "moved the previous login aside ("+filepath.Base(aside)+")") {
			return
		}
	}
	id, err := p.create(createSpec{
		Name:     "foreman-prove-b-login-" + newToken()[:8],
		Image:    harnessImage,
		Command:  []string{"claude"},
		Terminal: true,
		Stdin:    true,
		Env:      map[string]string{"CLAUDE_CONFIG_DIR": configDir},
		Labels:   map[string]string{"foreman.prove": "b-login"},
		Mounts:   []mount{{Destination: configDir, Source: creds, Type: "bind", Options: bindOptions()}},
		Userns:   keepID,
	})
	if !r.check(err, "created the login container (run `just prove-image` first if the image is missing)") {
		return
	}
	defer func() { _ = p.remove(id) }()
	if !r.check(p.start(id), "started Claude Code in it") {
		return
	}

	fmt.Print(`
      ── Your terminal is now connected to Claude Code in the container ──
      1. Type /login and press Enter (Claude won't ask by itself: it believes it is
         logged in from its account record, even without valid credentials).
      2. Choose "Claude account with subscription", open the URL it shows in your
         browser, sign in, paste the code back.
      3. When it says you're logged in, type /exit (or press Ctrl-C twice).

`)
	time.Sleep(2 * time.Second)
	code, err := interactive(p, id)
	fmt.Print("\r\n")
	if !r.check(err, "relayed the terminal both ways until Claude exited") {
		return
	}
	r.info("Claude exited with code %d", code)

	files, _ := os.ReadDir(creds)
	var names []string
	for _, f := range files {
		names = append(names, f.Name())
	}
	r.info("the credentials folder now holds: %s", strings.Join(names, ", "))
	if !fileExists(filepath.Join(creds, ".credentials.json")) {
		r.fail("no .credentials.json in the shared folder: the login didn't land there")
		return
	}
	r.pass("the login was written to the shared folder (.credentials.json)")
	// Prove it with a real request, not just an interface that opened.
	if raceRound(r, p, "dir", 1, "checking the login with one real request") == 1 {
		r.pass("the new login works")
	} else {
		r.fail("the new login doesn't work for a real request")
	}
}

// interactive connects the user's terminal to the container's until the
// container exits: raw mode, both directions, the terminal's size.
func interactive(p *podman, id string) (int, error) {
	conn, br, err := p.attachRaw(id)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if size, err := exec.Command("sh", "-c", "stty size < /dev/tty").Output(); err == nil {
		var rows, cols int
		if _, err := fmt.Sscan(string(size), &rows, &cols); err == nil {
			_ = p.resize(id, cols, rows)
		}
	}
	saved, err := exec.Command("sh", "-c", "stty -g < /dev/tty").Output()
	if err != nil {
		return 0, fmt.Errorf("not a terminal: run this from a real terminal (%v)", err)
	}
	_ = exec.Command("sh", "-c", "stty raw -echo < /dev/tty").Run()
	defer func() { _ = exec.Command("sh", "-c", "stty "+strings.TrimSpace(string(saved))+" < /dev/tty").Run() }()

	go func() { _, _ = io.Copy(conn, os.Stdin) }()
	go func() { _, _ = io.Copy(os.Stdout, br) }()
	return p.wait(id)
}

// bindOptions are the options for a read-write bind mount on this host:
// SELinux hosts (Fedora) need the shared `z` relabel.
func bindOptions() []string {
	if _, err := os.Stat("/sys/fs/selinux/enforce"); err == nil {
		return []string{"rw", "z"}
	}
	return []string{"rw"}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
