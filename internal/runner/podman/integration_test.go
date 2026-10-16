//go:build podman

// Integration tests against a real Podman: `just test-podman` (macOS: the
// podman machine; Linux: rootless Podman). They pull alpine once.
package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/runner"
)

const testImage = "docker.io/library/alpine:latest"

var ctx = context.Background()

func engine(t *testing.T) *Runner {
	t.Helper()
	r := Open()
	if err := r.Connect(ctx, true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	pullOnce.Do(func() { pullErr = pull(r.c, testImage) })
	if pullErr != nil {
		t.Fatal(pullErr)
	}
	return r
}

var (
	pullOnce sync.Once
	pullErr  error
)

// pull fetches an image; the API streams progress as JSON lines.
func pull(c *client, ref string) error {
	resp, err := c.http.Post("http://podman"+apiPrefix+"/images/pull?reference="+url.QueryEscape(ref), "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	for {
		var line struct{ Error string }
		if err := dec.Decode(&line); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if line.Error != "" {
			return fmt.Errorf("pull: %s", line.Error)
		}
	}
}

// output collects a process's output in the background.
type output struct {
	mu  sync.Mutex
	buf bytes.Buffer
	end chan struct{}
}

func collect(p runner.Process) *output {
	o := &output{end: make(chan struct{})}
	go func() {
		defer close(o.end)
		chunk := make([]byte, 4096)
		for {
			n, err := p.Read(chunk)
			o.mu.Lock()
			o.buf.Write(chunk[:n])
			o.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return o
}

func (o *output) waitFor(t *testing.T, want string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		o.mu.Lock()
		got := o.buf.String()
		o.mu.Unlock()
		if strings.Contains(got, want) {
			return
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	t.Fatalf("never saw %q; output %q", want, o.buf.String())
}

func start(t *testing.T, r *Runner, spec runner.Spec) runner.Process {
	t.Helper()
	if spec.Image == "" {
		spec.Image = testImage
	}
	if spec.Name == "" {
		spec.Name = fmt.Sprintf("test-%s-%d", t.Name(), time.Now().UnixNano())
	}
	p, err := r.Start(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Stop(time.Second) })
	return p
}

func sh(script string) []string { return []string{"/bin/sh", "-c", script} }

// The first output is kept (attached before the start), input arrives, and
// the exit code comes back; the container is then removed.
func TestStartInputOutputAndExit(t *testing.T) {
	r := engine(t)
	name := fmt.Sprintf("test-exit-%d", time.Now().UnixNano())
	p := start(t, r, runner.Spec{Name: name, Command: sh(`echo first-line; read x; echo "got:$x:$FOO"; exit 3`), Env: []string{"FOO=bar"}})
	out := collect(p)
	out.waitFor(t, "first-line")
	if _, err := p.Write([]byte("hello\r")); err != nil {
		t.Fatal(err)
	}
	out.waitFor(t, "got:hello:bar")
	if code, err := p.Wait(); err != nil || code != 3 {
		t.Errorf("Wait = %d, %v", code, err)
	}
	if p.Labels()[managedLabel] != "true" {
		t.Errorf("labels %v", p.Labels())
	}
	// Removed once exited, so the name is free again.
	time.Sleep(2 * time.Second)
	start(t, r, runner.Spec{Name: name, Command: sh("true")})
}

func TestSizeAndResize(t *testing.T) {
	r := engine(t)
	p := start(t, r, runner.Spec{Command: sh(`read x; stty size; read x; stty size`), Size: runner.Size{Cols: 100, Rows: 30}})
	out := collect(p)
	_, _ = p.Write([]byte("\r"))
	out.waitFor(t, "30 100")
	if err := p.Resize(runner.Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	_, _ = p.Write([]byte("\r"))
	out.waitFor(t, "24 80")
}

// Podman's default detach keys must not swallow the agent's keystrokes.
func TestCtrlPCtrlQReachTheProgram(t *testing.T) {
	r := engine(t)
	p := start(t, r, runner.Spec{Command: sh(`stty -ixon; cat -v`)})
	out := collect(p)
	time.Sleep(300 * time.Millisecond)
	_, _ = p.Write([]byte("\x10\x11\r"))
	out.waitFor(t, "^P^Q")
}

func TestMountsAtTheSamePathReadWriteAndReadOnly(t *testing.T) {
	r := engine(t)
	// The workspace must be shared with the podman machine on macOS: use a
	// folder under the user's home rather than the system temp folder.
	home, _ := os.UserHomeDir()
	base, err := os.MkdirTemp(home, ".foreman-podman-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	rw, ro := filepath.Join(base, "rw"), filepath.Join(base, "ro")
	_ = os.MkdirAll(rw, 0o755)
	_ = os.MkdirAll(ro, 0o755)
	p := start(t, r, runner.Spec{
		Command: sh(fmt.Sprintf(`echo made > %s/f && pwd && (echo x > %s/f 2>/dev/null && echo RO-WRITABLE || echo ro-refused); echo done`, rw, ro)),
		Dir:     rw,
		Mounts:  []runner.Mount{{Source: rw, Target: rw}, {Source: ro, Target: ro, ReadOnly: true}},
	})
	out := collect(p)
	out.waitFor(t, "done")
	out.waitFor(t, rw)
	out.waitFor(t, "ro-refused")
	info, err := os.Stat(filepath.Join(rw, "f"))
	if err != nil {
		t.Fatalf("the file written inside isn't on the host: %v", err)
	}
	if owner := fileOwner(info); owner != os.Getuid() {
		t.Errorf("the file belongs to uid %d, not the CEO's %d (keep-id)", owner, os.Getuid())
	}
}

// A second engine (a Foreman restart) finds the running container and can
// use it.
func TestListReattaches(t *testing.T) {
	r := engine(t)
	name := fmt.Sprintf("test-list-%d", time.Now().UnixNano())
	p := start(t, r, runner.Spec{Name: name, Command: sh(`while read x; do echo "echo:$x"; done`), Labels: map[string]string{"foreman.instance": name}})
	_ = p

	again := engine(t)
	list, err := again.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found runner.Process
	for _, l := range list {
		if l.Labels()["foreman.instance"] == name {
			found = l
		}
	}
	if found == nil {
		t.Fatalf("List didn't find %s among %d", name, len(list))
	}
	out := collect(found)
	_, _ = found.Write([]byte("ping\r"))
	out.waitFor(t, "echo:ping")
	// The same name can't start twice while it runs.
	if _, err := again.Start(ctx, runner.Spec{Name: name, Image: testImage, Command: sh("true")}); err == nil {
		t.Error("a second start of a running name succeeded")
	}
}

func TestStopEndsAProgramThatIgnoresTERM(t *testing.T) {
	r := engine(t)
	name := "stop-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	p := start(t, r, runner.Spec{Name: name, Command: sh(`trap "" TERM; echo ready; while true; do sleep 1; done`)})
	out := collect(p)
	out.waitFor(t, "ready")
	begin := time.Now()
	if err := p.Stop(time.Second); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(begin); took > 10*time.Second {
		t.Errorf("Stop took %v", took)
	}
	// Gone once Stop returns: Foreman exiting right after leaves no container.
	if err := exec.Command("podman", "container", "exists", containerName(name)).Run(); err == nil {
		t.Error("the container is still there after Stop")
	}
	if err := p.Stop(time.Second); err != nil {
		t.Errorf("stopping twice: %v", err)
	}
}

// Many at once: Start serializes creation and start, so none fails.
func TestManyStartsAtOnce(t *testing.T) {
	r := engine(t)
	const n = 12
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p, err := r.Start(ctx, runner.Spec{Name: fmt.Sprintf("test-many-%d-%d", i, time.Now().UnixNano()), Image: testImage, Command: sh("echo ok")})
			if err != nil {
				errs <- err
				return
			}
			out := collect(p)
			if code, err := p.Wait(); err != nil || code != 0 {
				errs <- fmt.Errorf("exit %d %v", code, err)
			}
			<-out.end
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestNoImageOrCommandIsAnError(t *testing.T) {
	r := engine(t)
	if _, err := r.Start(ctx, runner.Spec{Command: sh("true")}); err == nil {
		t.Error("started without an image")
	}
	if _, err := r.Start(ctx, runner.Spec{Image: testImage}); err == nil {
		t.Error("started without a command")
	}
}

// Containers run on Foreman's own CPU type (on macOS, the Podman machine's is the
// Mac's), so Foreman mounts the foreman-agent built for runtime.GOARCH.
func TestContainersRunOnForemansCPUType(t *testing.T) {
	machine := map[string]string{"arm64": "aarch64", "amd64": "x86_64"}[runtime.GOARCH]
	if machine == "" {
		t.Skipf("Foreman ships no foreman-agent for %s", runtime.GOARCH)
	}
	collect(start(t, engine(t), runner.Spec{Command: sh("uname -m")})).waitFor(t, machine)
}

// Before Connect every call says Podman isn't running; Connect is idempotent,
// and State reports what it reached.
func TestOpenThenConnect(t *testing.T) {
	r := Open()
	t.Cleanup(r.Close)
	if _, err := r.List(ctx); !errors.Is(err, ErrNotRunning) {
		t.Errorf("List before Connect: %v", err)
	}
	if _, err := r.Start(ctx, runner.Spec{Image: testImage, Command: sh("true")}); !errors.Is(err, ErrNotRunning) {
		t.Errorf("Start before Connect: %v", err)
	}
	if s := r.State(ctx); s.Running || !s.Installed {
		t.Errorf("before Connect: %+v", s)
	}
	for range 2 {
		if err := r.Connect(ctx, true); err != nil {
			t.Fatal(err)
		}
	}
	if s := r.State(ctx); !s.Running || s.Version == "" || s.Error != "" {
		t.Errorf("after Connect: %+v", s)
	}
	if _, err := r.List(ctx); err != nil {
		t.Errorf("List after Connect: %v", err)
	}
}
