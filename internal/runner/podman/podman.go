// Package podman runs processes in rootless Podman containers, the production
// execution engine (wiki/product/architecture/session-runner.md#podman-engine-production).
// Each process is a container's main program, with a terminal Podman
// allocates; containers outlive Foreman, and List finds them again.
package podman

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/brickpop/foreman/internal/runner"
)

// managedLabel marks every container Foreman creates, so List finds them.
const managedLabel = "foreman.managed"

// imageUser is the user of Foreman's agent image (images/Containerfile: the
// node image's uid 1000). keep-id maps it to the CEO's user, so files agents
// write into the mounts belong to the CEO.
var imageUser = &namespace{Nsmode: "keep-id", Value: "uid=1000,gid=1000"}

// defaultSize is the terminal size until the first resize, like runner/direct.
var defaultSize = runner.Size{Cols: 120, Rows: 40}

// Runner is the Podman execution engine.
type Runner struct {
	c           *client
	stopService func() // ends a Podman service Foreman started; nil otherwise
	selinux     bool   // the host labels bind mounts (Fedora): mounts get :z

	// Creating and starting many keep-id containers at once races inside
	// Podman's runtime ("write to ping_group_range"); one at a time is
	// reliable and takes about 0.2 s each (just prove-storm, 2026-10-14).
	starting sync.Mutex
}

var _ runner.Runner = (*Runner)(nil)

// New connects to Podman, starting its service (or, on macOS, its machine)
// if needed.
func New(ctx context.Context) (*Runner, error) {
	socket, stop, err := findSocket(ctx)
	if err != nil {
		return nil, err
	}
	c := newClient(socket)
	if _, err := c.version(ctx); err != nil {
		if stop != nil {
			stop()
		}
		return nil, fmt.Errorf("podman API on %s: %w", socket, err)
	}
	_, err = os.Stat("/sys/fs/selinux/enforce")
	return &Runner{c: c, stopService: stop, selinux: err == nil}, nil
}

// Version is Podman's version, for Settings → Maintenance.
func (r *Runner) Version(ctx context.Context) (string, error) { return r.c.version(ctx) }

// Close ends the Podman service if Foreman started it. Containers keep running.
func (r *Runner) Close() {
	if r.stopService != nil {
		r.stopService()
	}
}

// Start creates a container for spec, attaches to its terminal before starting
// it (so its first output isn't lost), and starts it.
func (r *Runner) Start(ctx context.Context, spec runner.Spec) (runner.Process, error) {
	if len(spec.Command) == 0 {
		return nil, errors.New("podman: empty command")
	}
	if spec.Image == "" {
		return nil, errors.New("podman: no image")
	}
	cs := r.createSpec(spec)
	r.starting.Lock()
	defer r.starting.Unlock()
	if err := r.clearStale(ctx, cs.Name); err != nil {
		return nil, err
	}
	id, err := r.c.create(ctx, cs)
	if err != nil {
		return nil, err
	}
	bg := context.Background() // the process outlives ctx
	conn, out, err := r.c.attach(ctx, id)
	if err != nil {
		_ = r.c.remove(bg, id)
		return nil, err
	}
	if err := r.c.start(ctx, id); err != nil {
		conn.Close()
		_ = r.c.remove(bg, id)
		return nil, err
	}
	p := newProcess(r.c, id, conn, out, cs.Labels)
	size := spec.Size
	if size.Cols == 0 || size.Rows == 0 {
		size = defaultSize
	}
	_ = p.Resize(size)
	return p, nil
}

// clearStale removes a stopped container left with this name (its process
// exited while Foreman was down). A running one is a live process: an error.
func (r *Runner) clearStale(ctx context.Context, name string) error {
	var out struct{ State struct{ Running bool } }
	err := r.c.call(ctx, "GET", "/containers/"+name+"/json", nil, nil, &out)
	var api *apiError
	switch {
	case errors.As(err, &api) && api.Status == 404:
		return nil
	case err != nil:
		return err
	case out.State.Running:
		return fmt.Errorf("podman: %s is already running", name)
	}
	return r.c.remove(ctx, name)
}

// List re-attaches to the running containers Foreman started.
func (r *Runner) List(ctx context.Context) ([]runner.Process, error) {
	running, err := r.c.listRunning(ctx, managedLabel, "true")
	if err != nil {
		return nil, err
	}
	var list []runner.Process
	for _, l := range running {
		conn, out, err := r.c.attach(ctx, l.ID)
		if err != nil {
			return nil, err
		}
		list = append(list, newProcess(r.c, l.ID, conn, out, l.Labels))
	}
	return list, nil
}

var notInName = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

// containerName makes a Podman container name from a spec name (an instance
// id like mara@ledger becomes foreman-mara-ledger).
func containerName(name string) string {
	return "foreman-" + strings.Trim(notInName.ReplaceAllString(name, "-"), "-")
}

func (r *Runner) createSpec(spec runner.Spec) createSpec {
	env := map[string]string{"TERM": "xterm-256color"}
	for _, kv := range spec.Env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}
	labels := map[string]string{}
	for k, v := range spec.Labels {
		labels[k] = v
	}
	labels[managedLabel] = "true" // last: a spec can't unmark it
	var mounts []mount
	for _, m := range spec.Mounts {
		opts := []string{"rw"}
		if m.ReadOnly {
			opts = []string{"ro"}
		}
		if r.selinux {
			// Shared label: several containers mount the same repos; the
			// private :Z would take them away from the others.
			opts = append(opts, "z")
		}
		mounts = append(mounts, mount{Destination: m.Target, Source: m.Source, Type: "bind", Options: opts})
	}
	return createSpec{
		Name: containerName(spec.Name), Image: spec.Image, Command: spec.Command,
		Terminal: true, Stdin: true, WorkDir: spec.Dir,
		Labels: labels, Mounts: mounts, Env: env, Userns: imageUser,
	}
}

// process is one container: its attached terminal, and its end.
type process struct {
	c      *client
	id     string
	conn   net.Conn      // writes: the terminal's input
	out    *bufio.Reader // reads: the terminal's output
	labels map[string]string

	done     chan struct{} // closed when the container has exited
	exitCode int
	waitErr  error
}

func newProcess(c *client, id string, conn net.Conn, out *bufio.Reader, labels map[string]string) *process {
	p := &process{c: c, id: id, conn: conn, out: out, labels: labels, done: make(chan struct{})}
	go p.reap()
	return p
}

// reap waits for the container to exit, then removes it, so its name is free
// for the next start.
func (p *process) reap() {
	bg := context.Background()
	p.exitCode, p.waitErr = p.c.wait(bg, p.id)
	close(p.done)
	// Let the reader drain the last output before the stream goes away.
	time.Sleep(time.Second)
	p.conn.Close()
	_ = p.c.remove(bg, p.id)
}

func (p *process) Read(b []byte) (int, error) {
	n, err := p.out.Read(b)
	if err != nil && errors.Is(err, net.ErrClosed) {
		err = io.EOF
	}
	return n, err
}

func (p *process) Write(b []byte) (int, error) { return p.conn.Write(b) }

func (p *process) Labels() map[string]string { return p.labels }

func (p *process) Resize(size runner.Size) error {
	return p.c.resize(context.Background(), p.id, size.Cols, size.Rows)
}

// Stop asks the program to end (SIGTERM), and Podman kills it after grace.
func (p *process) Stop(grace time.Duration) error {
	select {
	case <-p.done:
		return nil
	default:
	}
	seconds := int(math.Ceil(grace.Seconds()))
	if err := p.c.stop(context.Background(), p.id, seconds); err != nil {
		var api *apiError
		if !errors.As(err, &api) || api.Status != 304 { // 304: already stopped
			return err
		}
	}
	<-p.done
	return nil
}

func (p *process) Wait() (int, error) {
	<-p.done
	return p.exitCode, p.waitErr
}
