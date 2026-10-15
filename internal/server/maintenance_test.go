package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/runner"
)

type fakeMaintenance struct{ rebuilt chan struct{} }

func (fakeMaintenance) Status(context.Context) runner.Maintenance {
	return runner.Maintenance{Engine: "Podman 5.7.1 · running", Images: []runner.ImageStatus{{Name: "localhost/foreman/harness", State: "ready"}}}
}

func (f fakeMaintenance) Rebuild(context.Context) error {
	close(f.rebuilt)
	return nil
}

func TestMaintenance(t *testing.T) {
	f := fakeMaintenance{rebuilt: make(chan struct{})}
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Maintenance: f, UI: builtUI})
	var got map[string]any
	decodeBody(t, get(t, h, "/api/maintenance"), &got)
	if got["engine"] != "Podman 5.7.1 · running" || len(got["images"].([]any)) != 1 {
		t.Errorf("status %v", got)
	}
	if w := send(h, "POST", "/api/maintenance/rebuild", ""); w.Code != http.StatusAccepted {
		t.Errorf("rebuild: %d", w.Code)
	}
	select {
	case <-f.rebuilt:
	case <-time.After(3 * time.Second):
		t.Error("the rebuild never ran")
	}
}
