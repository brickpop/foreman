package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/cos"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/instances"
)

type fakeCoS struct {
	startErr  error
	openers   []string
	stopped   int
	proposal  *cos.Proposal
	createErr error
	looks     map[string]home.Look
}

func (f *fakeCoS) Start(_ context.Context, opener string) (string, error) {
	f.openers = append(f.openers, opener)
	return cos.Instance, f.startErr
}

func (f *fakeCoS) Stop() error { f.stopped++; return nil }

func (f *fakeCoS) Proposal() (cos.Proposal, bool) {
	if f.proposal == nil {
		return cos.Proposal{}, false
	}
	return *f.proposal, true
}

func (f *fakeCoS) Create(_ context.Context, id int, looks map[string]home.Look) (home.Team, error) {
	if f.createErr != nil {
		return home.Team{}, f.createErr
	}
	if f.proposal == nil || f.proposal.ID != id {
		return home.Team{}, cos.ErrStale
	}
	f.looks = looks
	return home.Team{ID: f.proposal.Team.Team, Config: f.proposal.Team}, nil
}

func TestChiefOfStaff(t *testing.T) {
	f := &fakeCoS{}
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), ChiefOfStaff: f, UI: builtUI})

	var session cosSession
	decodeBody(t, send(h, "POST", "/api/cos", `{"opener":"Hire someone for Atlas."}`), &session)
	if session.Terminal != "cos" || session.Proposal != nil || f.openers[0] != "Hire someone for Atlas." {
		t.Errorf("start: %+v %q", session, f.openers)
	}
	// Coming back to the page finds the proposal still on screen.
	f.proposal = &cos.Proposal{ID: 3, Team: home.TeamConfig{Team: "vesper", Name: "Vesper"}}
	decodeBody(t, send(h, "POST", "/api/cos", `{}`), &session)
	if session.Proposal == nil || session.Proposal.ID != 3 || f.openers[1] != "" {
		t.Errorf("again: %+v", session)
	}

	var team home.Team
	decodeBody(t, send(h, "POST", "/api/cos/proposal", `{"id":3,"looks":{"nyx":{"color":"#8E9AAF"}}}`), &team)
	if team.ID != "vesper" || f.looks["nyx"].Color != "#8E9AAF" {
		t.Errorf("create: %+v %v", team, f.looks)
	}
	for _, c := range []struct {
		body string
		err  error
		code int
		want string
	}{
		{`{"id":2}`, nil, http.StatusConflict, "STALE"},
		{`{"id":3}`, cos.ErrChanged, http.StatusConflict, "CONFLICT"},
		{`{"id":3}`, &home.InvalidError{Problems: []string{"lead: required"}}, http.StatusBadRequest, "INVALID"},
		{`nope`, nil, http.StatusBadRequest, "BAD_REQUEST"},
	} {
		f.createErr = c.err
		if w := send(h, "POST", "/api/cos/proposal", c.body); w.Code != c.code || !strings.Contains(w.Body.String(), c.want) {
			t.Errorf("%s with %v: %d %s", c.body, c.err, w.Code, w.Body)
		}
	}

	for err, want := range map[error]string{
		fmt.Errorf("%w: podman isn't running", instances.ErrNotReady): "NOT_READY",
		harness.ErrNotConnected: "NOT_CONNECTED",
	} {
		f.startErr = err
		if w := send(h, "POST", "/api/cos", `{}`); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), want) {
			t.Errorf("start refused (%v): %d %s", err, w.Code, w.Body)
		}
	}
	if w := send(h, "DELETE", "/api/cos", ""); w.Code != http.StatusNoContent || f.stopped != 1 {
		t.Errorf("stop: %d %d", w.Code, f.stopped)
	}
}
