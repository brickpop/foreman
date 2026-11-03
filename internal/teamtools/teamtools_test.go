package teamtools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/messaging"
	"github.com/brickpop/foreman/internal/prompt"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
	"github.com/brickpop/foreman/internal/workflow"
)

var ctx = context.Background()

var atlas = home.Team{ID: "atlas", Config: home.TeamConfig{
	Team: "atlas", Lead: "mara", Agents: map[string]home.Soul{"mara": {Name: "Mara"}, "tove": {Name: "Tove"}},
}}

// config is the home: every project has one code repo, unless bare (a
// research team's, only a wiki).
type config struct{ bare bool }

func (config) Team(string) (home.Team, error) { return atlas, nil }
func (c config) Project(id string) (home.Project, error) {
	p := home.Project{ID: id, Config: home.ProjectConfig{}}
	if !c.bare {
		p.Repos = []home.Repo{{Name: "api", Default: "main"}}
	}
	return p, nil
}
func (config) Projects() []home.Project      { return nil }
func (config) Sandbox(string, string) string { return "" }

// spy is a block that records the calls that reach it.
type spy struct{ calls []string }

func (s *spy) Tools() []agentlink.Tool { return []agentlink.Tool{{Name: "ask"}, {Name: "handoff"}} }
func (s *spy) Call(_ context.Context, c teamtools.Caller, name string, args json.RawMessage) agentlink.Result {
	s.calls = append(s.calls, c.Instance.ID+" "+name+" "+string(args))
	return agentlink.Result{Text: "done"}
}

func newStore(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Deployments().Deploy(ctx, "atlas", "ledger"); err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{"mara", "tove"} {
		if err := db.Instances().Put(ctx, store.Instance{ID: a + "@ledger", DeploymentID: "atlas@ledger", AgentID: a, Status: "idle", SandboxPath: "/s", Harness: "claude-code"}); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// The router serves exactly what the session prompt describes, per role,
// with the real blocks: the two can't drift.
func TestEveryOfferedToolIsServed(t *testing.T) {
	db := newStore(t)
	r := &teamtools.Router{Instances: db.Instances(), Deployments: db.Deployments(), Config: config{}, Blocks: []teamtools.Block{
		messaging.New(messaging.Deps{Messages: db.Messages(), Instances: db.Instances(), Deployments: db.Deployments(), Config: config{}}),
		workflow.New(workflow.Deps{Config: config{}}),
	}}
	for _, id := range []string{"mara@ledger", "tove@ledger"} {
		var names []string
		for _, tool := range r.List(id) {
			names = append(names, tool.Name)
			if !json.Valid(tool.InputSchema) || tool.Description == "" {
				t.Errorf("%s: a bad definition", tool.Name)
			} else if err := strictSchema(tool.InputSchema); err != nil {
				t.Errorf("%s: %v", tool.Name, err)
			}
		}
		if want := prompt.Offered(id == "mara@ledger", true); !slices.Equal(names, want) {
			t.Errorf("%s served %v, the prompt says %v", id, names, want)
		}
	}
	if tools := r.List("nobody@ledger"); tools != nil {
		t.Errorf("an unknown instance got %v", tools)
	}
}

// On a project with no code repo, the git tools aren't offered, nor served.
func TestNoCodeRepoNoGitTools(t *testing.T) {
	db := newStore(t)
	r := &teamtools.Router{Instances: db.Instances(), Deployments: db.Deployments(), Config: config{bare: true}, Blocks: []teamtools.Block{
		messaging.New(messaging.Deps{Messages: db.Messages(), Instances: db.Instances(), Deployments: db.Deployments(), Config: config{}}),
		workflow.New(workflow.Deps{Config: config{}}),
	}}
	for id, want := range map[string][]string{
		"mara@ledger": {"ask", "reply", "read_message", "commit", "contribute", "escalate", "team"},
		"tove@ledger": {"ask", "reply", "read_message", "contribute"},
	} {
		var names []string
		for _, tool := range r.List(id) {
			names = append(names, tool.Name)
		}
		if !slices.Equal(names, want) {
			t.Errorf("%s: %v, want %v", id, names, want)
		}
	}
	if res := r.Call(ctx, "mara@ledger", "switch", nil); !res.Error || !strings.HasPrefix(res.Text, "unknown_tool") {
		t.Errorf("switch served: %+v", res)
	}
}

// strictSchema holds a schema to what strict providers (Moonshot) check, at
// every level: an object's required, when there, is an array of properties it
// declares.
func strictSchema(raw json.RawMessage) error {
	var s map[string]json.RawMessage
	if json.Unmarshal(raw, &s) != nil {
		return nil // not a schema: absent, or a value such as an enum's
	}
	var props map[string]json.RawMessage
	_ = json.Unmarshal(s["properties"], &props)
	if req, ok := s["required"]; ok {
		var names []string
		if err := json.Unmarshal(req, &names); err != nil || names == nil {
			return fmt.Errorf("required must be an array, got %s", req)
		}
		for _, n := range names {
			if _, ok := props[n]; !ok {
				return fmt.Errorf("required %q isn't a property", n)
			}
		}
	}
	for _, sub := range props {
		if err := strictSchema(sub); err != nil {
			return err
		}
	}
	return strictSchema(s["items"])
}

func TestTheRouterChecksBeforeAnyBlock(t *testing.T) {
	db := newStore(t)
	s := &spy{}
	r := &teamtools.Router{Instances: db.Instances(), Deployments: db.Deployments(), Config: config{}, Blocks: []teamtools.Block{s}}
	for _, tc := range []struct{ from, tool, code string }{
		{"nobody@ledger", "ask", "unknown_caller:"},
		{"mara@ledger", "handoff", "unknown_tool:"}, // a lead isn't offered handoff
		{"tove@ledger", "rm_rf", "unknown_tool:"},
	} {
		if res := r.Call(ctx, tc.from, tc.tool, nil); !res.Error || !strings.HasPrefix(res.Text, tc.code) {
			t.Errorf("%s %s: %+v", tc.from, tc.tool, res)
		}
	}
	if res := r.Call(ctx, "tove@ledger", "handoff", nil); res.Error || len(s.calls) != 1 || s.calls[0] != "tove@ledger handoff {}" {
		t.Errorf("a member's handoff: %+v, calls %v", res, s.calls)
	}
	if err := db.Deployments().Stop(ctx, "atlas@ledger", ""); err != nil {
		t.Fatal(err)
	}
	if res := r.Call(ctx, "tove@ledger", "ask", nil); !strings.HasPrefix(res.Text, "deployment_stopped:") || len(s.calls) != 1 {
		t.Errorf("a stopped deployment: %+v", res)
	}
}

func TestTeammate(t *testing.T) {
	c := teamtools.Caller{Instance: store.Instance{ID: "tove@ledger", AgentID: "tove"}, Deployment: store.Deployment{ProjectID: "ledger"}, Team: atlas}
	if to, _, ok := c.Teammate(" MARA "); !ok || to != "mara@ledger" {
		t.Errorf("by name: %q %v", to, ok)
	}
	if _, res, ok := c.Teammate("tove"); ok || !strings.HasPrefix(res.Text, "self:") {
		t.Errorf("self: %+v", res)
	}
	if _, res, ok := c.Teammate("rhea"); ok || res.Text != `unknown_teammate: no "rhea" on this team; your teammates: Mara` {
		t.Errorf("unknown: %+v", res)
	}
	if c.LeadInstance() != "mara@ledger" || c.Name() != "Tove" {
		t.Errorf("lead %q, name %q", c.LeadInstance(), c.Name())
	}
}

// The lead's tools are refused to a member that calls them anyway.
func TestAMemberCantCallTheLeadsTools(t *testing.T) {
	db := newStore(t)
	r := &teamtools.Router{Instances: db.Instances(), Deployments: db.Deployments(), Config: config{}, Blocks: []teamtools.Block{
		workflow.New(workflow.Deps{Config: config{}}),
	}}
	for _, tool := range []string{"escalate", "fetch", "request_merge"} {
		if res := r.Call(ctx, "tove@ledger", tool, json.RawMessage(`{"kind":"approval","summary":"x"}`)); !res.Error || !strings.HasPrefix(res.Text, "unknown_tool") {
			t.Errorf("%s served to a member: %+v", tool, res)
		}
	}
}
