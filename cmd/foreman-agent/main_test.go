package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// team is Foreman's side: one tool, and what reached it.
type team struct {
	mu    sync.Mutex
	calls []string
	hooks []string
}

func (f *team) List(string) []agentlink.Tool {
	return []agentlink.Tool{{Name: "ask", Description: "Ask a teammate.", InputSchema: json.RawMessage(
		`{"type":"object","properties":{"teammate":{"type":"string"}},"required":["teammate"]}`)}}
}

func (f *team) Call(_ context.Context, instance, name string, args json.RawMessage) agentlink.Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, instance+" "+name+" "+string(args))
	if strings.Contains(string(args), "nobody") {
		return agentlink.Result{Text: "unknown_teammate: nobody", Error: true}
	}
	return agentlink.Result{Text: "msg-1"}
}

func (f *team) Hook(instance, event string, payload []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hooks = append(f.hooks, instance+" "+event+" "+string(payload))
}

func foreman(t *testing.T) (*team, func(string) string) {
	t.Helper()
	tm := &team{}
	link := agentlink.New([]byte("s"), func(i string) bool { return i == "mara@ledger" }, tm, tm)
	srv := httptest.NewServer(link.Handler())
	t.Cleanup(srv.Close)
	env := map[string]string{"FOREMAN_AGENT_URL": srv.URL, "FOREMAN_AGENT_TOKEN": link.Token("mara@ledger")}
	return tm, func(k string) string { return env[k] }
}

// A harness sees Foreman's tools through the MCP server, and its calls reach
// Foreman as the token's instance.
func TestMCPRelaysTheTeamTools(t *testing.T) {
	tm, getenv := foreman(t)
	ctx := context.Background()
	serverSide, clientSide := mcp.NewInMemoryTransports()
	link := agentlink.Client{URL: getenv("FOREMAN_AGENT_URL"), Token: getenv("FOREMAN_AGENT_TOKEN")}
	done := make(chan error, 1)
	go func() { done <- serve(ctx, link, serverSide) }()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "harness"}, nil).Connect(ctx, clientSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	list, err := session.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 1 || list.Tools[0].Name != "ask" {
		t.Fatalf("tools %+v %v", list, err)
	}
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "ask", Arguments: map[string]any{"teammate": "tove"}})
	if err != nil || res.IsError || res.Content[0].(*mcp.TextContent).Text != "msg-1" {
		t.Errorf("call %+v %v", res, err)
	}
	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "ask", Arguments: map[string]any{"teammate": "nobody"}})
	if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "unknown_teammate") {
		t.Errorf("a refusal reaches the agent as a tool error: %+v %v", res, err)
	}
	if len(tm.calls) != 2 || tm.calls[0] != `mara@ledger ask {"teammate":"tove"}` {
		t.Errorf("calls %q", tm.calls)
	}
	session.Close()
	if err := <-done; err != nil {
		t.Errorf("closing: %v", err)
	}
}

func TestHookForwardsAndNeverFails(t *testing.T) {
	tm, getenv := foreman(t)
	var stderr bytes.Buffer
	if code := run([]string{"hook", "Stop"}, strings.NewReader(`{"x":1}`), &bytes.Buffer{}, &stderr, getenv); code != 0 || stderr.Len() != 0 {
		t.Errorf("hook: %d %q", code, stderr.String())
	}
	if len(tm.hooks) != 1 || tm.hooks[0] != `mara@ledger Stop {"x":1}` {
		t.Errorf("hooks %q", tm.hooks)
	}
	// Foreman unreachable: said on stderr, and still a success for the harness.
	unreachable := func(k string) string {
		return map[string]string{"FOREMAN_AGENT_URL": "http://127.0.0.1:1", "FOREMAN_AGENT_TOKEN": "x"}[k]
	}
	stderr.Reset()
	if code := run([]string{"hook", "Stop"}, strings.NewReader("{}"), &bytes.Buffer{}, &stderr, unreachable); code != 0 || stderr.Len() == 0 {
		t.Errorf("unreachable: %d %q", code, stderr.String())
	}
	if code := run([]string{"nope"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, getenv); code != 2 {
		t.Errorf("usage: %d", code)
	}
}

// Without Foreman the MCP server doesn't start, and says why.
func TestMCPNeedsForeman(t *testing.T) {
	var stderr bytes.Buffer
	unreachable := func(k string) string {
		return map[string]string{"FOREMAN_AGENT_URL": "http://127.0.0.1:1"}[k]
	}
	if code := run([]string{"mcp"}, strings.NewReader(""), &bytes.Buffer{}, &stderr, unreachable); code != 1 ||
		!strings.Contains(stderr.String(), "listing the team tools") {
		t.Errorf("%d %q", code, stderr.String())
	}
}
