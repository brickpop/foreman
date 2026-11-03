package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/runner"
)

func TestContainerNamesAreValidPodmanNames(t *testing.T) {
	for in, want := range map[string]string{
		"mara@ledger":     "foreman-mara-ledger",
		"tove@ledger-web": "foreman-tove-ledger-web",
		"@odd name!":      "foreman-odd-name",
		"x.y_z":           "foreman-x.y_z",
	} {
		if got := containerName(in); got != want {
			t.Errorf("containerName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateSpec(t *testing.T) {
	spec := runner.Spec{
		Name: "mara@ledger", Image: "localhost/foreman/harness", Command: []string{"claude"},
		Dir: "/ws/ledger/.sandboxes/mara",
		Env: []string{"CLAUDE_CONFIG_DIR=/home/node/.claude", "A=b=c", "malformed"},
		Mounts: []runner.Mount{
			{Source: "/ws/ledger/.sandboxes/mara", Target: "/ws/ledger/.sandboxes/mara"},
			{Source: "/ws/ledger/ledger-api/.git", Target: "/ws/ledger/ledger-api/.git", ReadOnly: true},
		},
		Labels: map[string]string{"foreman.instance": "mara@ledger"},
	}
	for _, selinux := range []bool{false, true} {
		cs := (&Runner{selinux: selinux}).createSpec(spec)
		if cs.Name != "foreman-mara-ledger" || !cs.Terminal || !cs.Stdin || cs.WorkDir != spec.Dir || cs.Userns != imageUser {
			t.Errorf("spec %+v", cs)
		}
		// Podman's init, not the agent, is PID 1, so orphans are reaped.
		if !cs.Init {
			t.Error("no init")
		}
		if b, err := json.Marshal(cs); err != nil || !bytes.Contains(b, []byte(`"init":true`)) {
			t.Errorf("the create payload asks for no init: %s, %v", b, err)
		}
		wantEnv := map[string]string{"TERM": "xterm-256color", "CLAUDE_CONFIG_DIR": "/home/node/.claude", "A": "b=c"}
		if !reflect.DeepEqual(cs.Env, wantEnv) {
			t.Errorf("env %v", cs.Env)
		}
		if cs.Labels[managedLabel] != "true" || cs.Labels["foreman.instance"] != "mara@ledger" {
			t.Errorf("labels %v", cs.Labels)
		}
		rw, ro := []string{"rw"}, []string{"ro"}
		if selinux {
			rw, ro = []string{"rw", "z"}, []string{"ro", "z"}
		}
		if !reflect.DeepEqual(cs.Mounts[0].Options, rw) || !reflect.DeepEqual(cs.Mounts[1].Options, ro) || cs.Mounts[1].Destination != spec.Mounts[1].Target {
			t.Errorf("selinux %v: mounts %+v", selinux, cs.Mounts)
		}
	}
	// The spec's labels can't unmark a container: List would lose it.
	spec.Labels[managedLabel] = "false"
	if cs := (&Runner{}).createSpec(spec); cs.Labels[managedLabel] != "true" {
		t.Errorf("a spec label unmarked the container: %v", cs.Labels)
	}
}

func TestMachineBounds(t *testing.T) {
	for _, c := range []struct {
		hostGiB, cpus int
		want          MachineBounds
	}{
		// Half the Mac, within 4 GiB and 60%.
		{32, 10, MachineBounds{MinMemory: 4, MaxMemory: 19, Memory: 16, MaxCPUs: 10, CPUs: 5}},
		{16, 8, MachineBounds{MinMemory: 4, MaxMemory: 9, Memory: 8, MaxCPUs: 8, CPUs: 4}},
		// Half is under the minimum: the minimum.
		{7, 1, MachineBounds{MinMemory: 4, MaxMemory: 4, Memory: 4, MaxCPUs: 1, CPUs: 1}},
		// Too small for 4 GiB within 60%: MaxMemory < MinMemory.
		{6, 4, MachineBounds{MinMemory: 4, MaxMemory: 3, Memory: 4, MaxCPUs: 4, CPUs: 2}},
	} {
		if got := machineBounds(uint64(c.hostGiB)<<10, c.cpus); got != c.want {
			t.Errorf("%d GiB, %d CPUs: %+v, want %+v", c.hostGiB, c.cpus, got, c.want)
		}
	}
}

// top's rows, its command line whole or split at its spaces: PID 1 and the
// zombies are left out.
func TestOthers(t *testing.T) {
	// PID 1 is podman's init and its first child the container's command
	// (the harness): neither counts. An orphan the agent left, reparented to
	// PID 1, still does.
	got := others([][]string{
		{"1", "0", "S", "/run/podman-init", "--", "claude", "--continue"},
		{"2", "1", "S", "claude --continue"},
		{"7", "2", "S", "/opt/foreman/foreman-agent", "mcp"},
		{"9", "2", "Z", "[sh]"},
		{"12", "1", "R", "npm run dev"},
		{"13"},
	})
	if want := []string{"/opt/foreman/foreman-agent mcp", "npm run dev"}; !slices.Equal(got, want) {
		t.Errorf("others = %q, want %q", got, want)
	}
}

// A short or unreadable row is skipped, never a panic.
func TestOthersSkipsBadRows(t *testing.T) {
	if got := others([][]string{{}, {"x", "1", "S", "a"}, {"5", "1", "S"}}); len(got) != 0 {
		t.Errorf("others = %q, want none", got)
	}
}

// fakePodman serves Podman's API on a Unix socket with handle and returns an
// engine connected to it.
func fakePodman(t *testing.T, handle http.HandlerFunc) *Runner {
	t.Helper()
	dir, err := os.MkdirTemp("", "pod") // short: a socket path has a length limit
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	l, err := net.Listen("unix", filepath.Join(dir, "s"))
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: handle}
	go srv.Serve(l)
	t.Cleanup(func() { srv.Close() })
	return &Runner{c: newClient(filepath.Join(dir, "s"))}
}

// checkAPI answers a check's calls: no earlier check container unless left
// names one, and a container x that runs `true`. Each call is recorded as
// "METHOD /path" in calls; remove, when set, answers the DELETEs.
func checkAPI(left string, calls chan<- string, remove http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, apiPrefix)
		calls <- req.Method + " " + path
		switch {
		case req.Method == "GET" && path == "/containers/json":
			if left == "" {
				fmt.Fprint(w, `[]`)
			} else {
				fmt.Fprintf(w, `[{"Id":%q}]`, left)
			}
		case path == "/containers/create":
			fmt.Fprint(w, `{"Id":"x"}`)
		case strings.HasSuffix(path, "/start"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(path, "/wait"):
			fmt.Fprint(w, `0`)
		case req.Method == "DELETE" && remove != nil:
			remove(w, req)
		}
	}
}

// A container an earlier check left is removed before the check runs, and
// the check's own container after it.
func TestCheckInitClearsEarlierChecks(t *testing.T) {
	calls := make(chan string, 20)
	r := fakePodman(t, checkAPI("old", calls, nil))
	if err := r.CheckInit(context.Background(), "img"); err != nil {
		t.Fatal(err)
	}
	close(calls)
	var got []string
	for c := range calls {
		got = append(got, c)
	}
	want := []string{"GET /containers/json", "DELETE /containers/old", "POST /containers/create",
		"POST /containers/x/start", "POST /containers/x/wait", "DELETE /containers/x"}
	if !slices.Equal(got, want) {
		t.Errorf("calls %q\nwant  %q", got, want)
	}
}

// A Podman that hangs on the remove doesn't hold the check: the remove has
// its own limit, even when the check's context has none.
func TestCheckInitRemoveHasItsOwnLimit(t *testing.T) {
	defer func(d time.Duration) { removeTimeout = d }(removeTimeout)
	removeTimeout = 50 * time.Millisecond
	hang := make(chan struct{})
	defer close(hang)
	r := fakePodman(t, checkAPI("", make(chan string, 20), func(http.ResponseWriter, *http.Request) { <-hang }))
	done := make(chan error, 1)
	go func() { done <- r.CheckInit(context.Background(), "img") }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("a failed remove failed the check: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the check hung on the remove")
	}
}
