package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/brickpop/foreman/internal/setup"
)

type fakeSetup struct {
	started int
	created [][2]int
}

func (f *fakeSetup) Steps(context.Context) []setup.Step {
	return []setup.Step{{Kind: "machine", State: "todo", Machine: &setup.MachineSize{MinMemory: 4, MaxMemory: 19, Memory: 16, MaxCPUs: 10, CPUs: 5}}}
}

func (f *fakeSetup) StartPodman(context.Context) { f.started++ }

func (f *fakeSetup) CreateMachine(_ context.Context, cpus, memory int) error {
	if memory < 4 {
		return errors.New("podman: machine size out of bounds: memory 4–19 GiB")
	}
	f.created = append(f.created, [2]int{cpus, memory})
	return nil
}

func TestSetup(t *testing.T) {
	f := &fakeSetup{}
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Setup: f, UI: builtUI})
	var steps []map[string]any
	decodeBody(t, get(t, h, "/api/setup"), &steps)
	if len(steps) != 1 || steps[0]["kind"] != "machine" || steps[0]["machine"].(map[string]any)["maxCpus"] != 10.0 {
		t.Errorf("steps %v", steps)
	}
	if w := send(h, "POST", "/api/setup/podman/start", ""); w.Code != http.StatusAccepted || f.started != 1 {
		t.Errorf("start: %d, started %d", w.Code, f.started)
	}
	if w := send(h, "POST", "/api/setup/podman/machine", `{"cpus":5,"memory":16}`); w.Code != http.StatusAccepted || len(f.created) != 1 || f.created[0] != [2]int{5, 16} {
		t.Errorf("create: %d %s, created %v", w.Code, w.Body, f.created)
	}
	for body, code := range map[string]string{`{"cpus":5,"memory":2}`: "CANT_CREATE", `nope`: "BAD_REQUEST"} {
		w := send(h, "POST", "/api/setup/podman/machine", body)
		var got map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if got["code"] != code || len(f.created) != 1 {
			t.Errorf("%s: %d %s", body, w.Code, w.Body)
		}
	}
}
