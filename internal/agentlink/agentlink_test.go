package agentlink

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeTeam records what reaches it; only mara@ledger and tove@ledger run.
type fakeTeam struct {
	mu    sync.Mutex
	calls []string
	hooks []string
}

func (f *fakeTeam) List(instance string) []Tool {
	return []Tool{{Name: "whoami", Description: instance, InputSchema: json.RawMessage(`{"type":"object"}`)}}
}

func (f *fakeTeam) Call(_ context.Context, instance, name string, args json.RawMessage) Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, instance+" "+name+" "+string(args))
	if name == "refuse" {
		return Result{Text: "unknown_teammate", Error: true}
	}
	return Result{Text: "ok " + instance}
}

func (f *fakeTeam) Hook(instance, event string, payload []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hooks = append(f.hooks, instance+" "+event+" "+string(payload))
}

func setup(t *testing.T) (*Link, *fakeTeam, *httptest.Server) {
	t.Helper()
	team := &fakeTeam{}
	live := func(instance string) bool { return instance == "mara@ledger" || instance == "tove@ledger" }
	link := New([]byte("secret"), live, team, team)
	srv := httptest.NewServer(link.Handler())
	t.Cleanup(srv.Close)
	return link, team, srv
}

func TestTheRelayCarriesToolsAndHooksAsTheTokensInstance(t *testing.T) {
	link, team, srv := setup(t)
	c := Client{URL: srv.URL, Token: link.Token("mara@ledger")}
	ctx := context.Background()
	tools, err := c.Tools(ctx)
	if err != nil || len(tools) != 1 || tools[0].Description != "mara@ledger" {
		t.Fatalf("tools %+v %v", tools, err)
	}
	res, err := c.Call(ctx, "ask", json.RawMessage(`{"teammate":"tove"}`))
	if err != nil || res != (Result{Text: "ok mara@ledger"}) {
		t.Errorf("call %+v %v", res, err)
	}
	if res, _ := c.Call(ctx, "refuse", nil); !res.Error || res.Text != "unknown_teammate" {
		t.Errorf("a refusal: %+v", res)
	}
	if err := c.Hook(ctx, "Stop", []byte(`{"last_assistant_message":"done"}`)); err != nil {
		t.Fatal(err)
	}
	if strings.Join(team.calls, "|") != `mara@ledger ask {"teammate":"tove"}|mara@ledger refuse ` ||
		strings.Join(team.hooks, "|") != `mara@ledger Stop {"last_assistant_message":"done"}` {
		t.Errorf("calls %q hooks %q", team.calls, team.hooks)
	}
}

// Identity comes only from a genuine token of a running instance.
func TestOnlyGenuineTokensOfRunningInstancesGetThrough(t *testing.T) {
	link, team, srv := setup(t)
	other := New([]byte("another secret"), func(string) bool { return true }, team, team)
	mara := link.Token("mara@ledger")
	name, sig, _ := strings.Cut(mara, ".")
	toveName, _, _ := strings.Cut(link.Token("tove@ledger"), ".")
	for what, token := range map[string]string{
		"no token":                 "",
		"another home's secret":    other.Token("mara@ledger"),
		"mara's signature on tove": toveName + "." + sig,
		"a stopped instance":       link.Token("ivo@ledger"),
		"no signature":             name,
		"garbage":                  "!!!.abc",
	} {
		req, _ := http.NewRequest("GET", srv.URL+"/tools", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: %d", what, resp.StatusCode)
		}
	}
	// The same instance always gets the same token, so a container that kept
	// running across a Foreman restart is still recognised.
	if New([]byte("secret"), nil, nil, nil).Token("mara@ledger") != mara {
		t.Error("the token isn't stable")
	}
}

func TestAnOversizedCallIsRefused(t *testing.T) {
	link, team, srv := setup(t)
	c := Client{URL: srv.URL, Token: link.Token("mara@ledger")}
	big := json.RawMessage(`"` + strings.Repeat("x", maxBody) + `"`)
	if _, err := c.Call(context.Background(), "ask", big); err == nil {
		t.Error("an oversized call went through")
	}
	if len(team.calls) != 0 {
		t.Errorf("it reached the tools: %d", len(team.calls))
	}
}
