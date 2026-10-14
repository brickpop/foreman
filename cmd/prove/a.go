package main

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"
)

// Experiment A (wiki/backlog/prove-a-podman-engine.md): drive Podman from Go
// through its REST API: start a container with a terminal, stream it both
// ways, resize it, and find and re-attach to it from a second process.

const (
	aImage = "docker.io/library/alpine:latest"
	aLabel = "foreman.prove"
	aValue = "a"
)

func connectPodman(r *report) *podman {
	socket, err := findSocket(r)
	if !r.check(err, "found the Podman API socket") {
		return nil
	}
	r.info("socket: %s", socket)
	p := newPodman(socket)
	v, err := p.version()
	if !r.check(err, "talked to the REST API") {
		return nil
	}
	r.info("Podman %s, %s/%s", v, runtime.GOOS, runtime.GOARCH)
	return p
}

func proveAStart(r *report) {
	p := connectPodman(r)
	if p == nil {
		return
	}
	// Leftovers from an earlier, interrupted run.
	if old, _ := p.listByLabel(aLabel, aValue); len(old) > 0 {
		for _, id := range old {
			_ = p.remove(id)
		}
		r.info("removed %d leftover container(s) from an earlier run", len(old))
	}

	begin := time.Now()
	if !r.check(p.pull(aImage), "pulled "+aImage) {
		return
	}
	r.info("pull took %v", time.Since(begin).Round(time.Millisecond))

	cwd, _ := os.Getwd()
	opts := []string{"rw"}
	if _, err := os.Stat("/sys/fs/selinux/enforce"); err == nil {
		opts = append(opts, "z") // SELinux hosts (Fedora): relabel the bind mount
	}
	spec := createSpec{
		Name:     fmt.Sprintf("foreman-prove-a-%d", time.Now().Unix()),
		Image:    aImage,
		Command:  []string{"/bin/sh"},
		Terminal: true,
		Stdin:    true,
		WorkDir:  cwd,
		Labels:   map[string]string{aLabel: aValue},
		Mounts:   []mount{{Destination: cwd, Source: cwd, Type: "bind", Options: opts}},
	}
	begin = time.Now()
	id, err := p.create(spec)
	if !r.check(err, "created a container with a terminal, the current folder mounted at the same path") {
		return
	}
	if !r.check(p.start(id), "started it") {
		return
	}
	r.info("create + start took %v", time.Since(begin).Round(time.Millisecond))

	s, err := p.attach(id)
	if !r.check(err, "attached to its terminal (raw two-way stream)") {
		return
	}
	defer s.close() // closing the stream detaches; the container keeps running

	if !r.check(p.resize(id, 120, 40), "set the terminal size to 120x40 through the API") {
		return
	}
	_ = s.write("stty size; echo hello\r")
	at, err := s.waitFor("hello\r\n", 0, 5*time.Second)
	if r.check(err, "typed a command and read its output") {
		if _, err := s.waitFor("40 120", 0, time.Second); err != nil {
			r.fail("the program saw the initial size: %v", err)
		} else {
			r.pass("the program saw the initial size (40 rows, 120 columns)")
		}
	}

	if r.check(p.resize(id, 100, 30), "resized to 100x30 through the API") {
		_ = s.write("stty size\r")
		at, err = s.waitFor("30 100", at, 5*time.Second)
		r.check(err, "the program saw the new size (30 rows, 100 columns)")
	}

	_ = s.write("pwd\r")
	at, err = s.waitFor(cwd, at, 5*time.Second)
	r.check(err, "the mounted folder is the working directory, at the host path")

	// Ctrl-P Ctrl-Q is Podman's default detach sequence: it must reach the program.
	// -ixon first: in its default mode the terminal itself consumes Ctrl-Q (and
	// Ctrl-S) as flow control; full-screen programs turn that off too.
	_ = s.write("stty -ixon; cat -v\r")
	time.Sleep(200 * time.Millisecond)
	_ = s.write("\x10\x11\r\x04")
	_, err = s.waitFor("^P^Q", at, 5*time.Second)
	r.check(err, "Ctrl-P Ctrl-Q reached the program (no detach keys swallow input)")

	// A keystroke's round trip: write one character, wait for the terminal's echo.
	var samples []time.Duration
	for i := 0; i < 20; i++ {
		mark := s.len()
		sent := time.Now()
		_ = s.write("x")
		if _, err := s.waitFor("x", mark, 2*time.Second); err != nil {
			r.fail("keystroke echo: %v", err)
			return
		}
		samples = append(samples, time.Since(sent))
	}
	_ = s.write("\x15") // Ctrl-U: clear the typed x's
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	median, worst := samples[len(samples)/2], samples[len(samples)-1]
	if worst < 50*time.Millisecond {
		r.pass("keystroke round trip under 50ms (median %v, worst %v)", median.Round(time.Microsecond), worst.Round(time.Microsecond))
	} else {
		r.fail("keystroke round trip: median %v, worst %v (want under 50ms)", median.Round(time.Microsecond), worst.Round(time.Microsecond))
	}

	r.info("exiting WITHOUT stopping the container: the next step is a separate process (a Foreman restart)")
}

func proveAReattach(r *report) {
	p := connectPodman(r)
	if p == nil {
		return
	}
	ids, err := p.listByLabel(aLabel, aValue)
	if !r.check(err, "listed containers by label") {
		return
	}
	if len(ids) != 1 {
		r.fail("found %d containers labelled %s=%s, want 1 (run `prove a start` first)", len(ids), aLabel, aValue)
		return
	}
	id := ids[0]
	defer func() { _ = p.remove(id) }()
	st, err := p.inspect(id)
	if r.check(err, "inspected it") && !st.Running {
		r.fail("the container didn't survive the first process: it isn't running")
		return
	}
	r.pass("the container survived the first process exiting, still running")

	s, err := p.attach(id)
	if !r.check(err, "re-attached from a new process") {
		return
	}
	defer s.close()
	_ = s.write("echo again\r")
	_, err = s.waitFor("again\r\n", 0, 5*time.Second)
	r.check(err, "the re-attached terminal works both ways")

	_ = s.write("trap 'exit 7' TERM; echo trapped\r")
	if _, err := s.waitFor("trapped\r\n", 0, 5*time.Second); err != nil {
		r.fail("setting up the stop test: %v", err)
		return
	}
	begin := time.Now()
	if !r.check(p.stop(id, 3), "stopped it with a grace period") {
		return
	}
	r.info("stop took %v", time.Since(begin).Round(time.Millisecond))
	st, err = p.inspect(id)
	if r.check(err, "inspected the stopped container") {
		if st.Running {
			r.fail("still running after stop")
		} else {
			r.pass("exited with code %d after the stop", st.ExitCode)
		}
	}
	r.check(p.remove(id), "removed it")
}
