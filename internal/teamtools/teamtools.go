// Package teamtools is the team-tools router
// (wiki/product/architecture/team-tools.md): it serves each instance exactly
// the tools its role is offered (prompt.Offered, which the session prompt
// describes too), knows the caller only from its token, and hands each call
// to the block that owns the tool (Messaging, Workflow).
package teamtools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/prompt"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/workspace"
)

// Caller is who calls a tool: its instance, deployment and team.
type Caller struct {
	Instance   store.Instance
	Deployment store.Deployment
	Team       workspace.Team
	Lead       bool
}

// Block serves some of the team tools.
type Block interface {
	// Tools are its tools, as the harness sees them.
	Tools() []agentlink.Tool
	// Call runs one of its tools for a caller of an active deployment that
	// is offered it.
	Call(ctx context.Context, c Caller, name string, args json.RawMessage) agentlink.Result
}

// Config is the team files, as loaded.
type Config interface {
	Team(id string) (workspace.Team, error)
}

// Router serves the team tools of every block.
type Router struct {
	Instances   store.Instances
	Deployments store.Deployments
	Config      Config
	Blocks      []Block
}

var _ agentlink.Tools = (*Router)(nil)

// MaxBody caps any text an agent sends (a request, an output, a summary, a
// contribution): longer work belongs in a branch or a wiki entry, named.
const MaxBody = 8000

// Refusal is a tool error the agent reads: a code, then what to do.
func Refusal(code, format string, a ...any) agentlink.Result {
	return agentlink.Result{Text: code + ": " + fmt.Sprintf(format, a...), Error: true}
}

// List implements agentlink.Tools: the tools the instance's role is offered,
// in prompt.Offered's order.
func (r *Router) List(instance string) []agentlink.Tool {
	c, err := r.caller(context.Background(), instance)
	if err != nil {
		return nil
	}
	var tools []agentlink.Tool
	for _, name := range prompt.Offered(c.Lead) {
		if t, _, ok := r.find(name); ok {
			tools = append(tools, t)
		}
	}
	return tools
}

// Call implements agentlink.Tools.
func (r *Router) Call(ctx context.Context, instance, name string, args json.RawMessage) agentlink.Result {
	c, err := r.caller(ctx, instance)
	if err != nil {
		return Refusal("unknown_caller", "%v", err)
	}
	_, block, ok := r.find(name)
	if !ok || !slices.Contains(prompt.Offered(c.Lead), name) {
		return Refusal("unknown_tool", "%q isn't one of your tools", name)
	}
	if c.Deployment.Status != store.DeploymentActive {
		return Refusal("deployment_stopped", "this deployment was stopped")
	}
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	return block.Call(ctx, c, name, args)
}

func (r *Router) find(name string) (agentlink.Tool, Block, bool) {
	for _, b := range r.Blocks {
		for _, t := range b.Tools() {
			if t.Name == name {
				return t, b, true
			}
		}
	}
	return agentlink.Tool{}, nil, false
}

func (r *Router) caller(ctx context.Context, instance string) (Caller, error) {
	inst, err := r.Instances.Get(ctx, instance)
	if err != nil {
		return Caller{}, err
	}
	dep, err := r.Deployments.Get(ctx, inst.DeploymentID)
	if err != nil {
		return Caller{}, err
	}
	team, err := r.Config.Team(dep.TeamID)
	if err != nil {
		return Caller{}, err
	}
	return Caller{Instance: inst, Deployment: dep, Team: team, Lead: inst.AgentID == team.Config.Lead}, nil
}

// Args decodes a tool's arguments, or says why it can't.
func Args(args json.RawMessage, into any) (agentlink.Result, bool) {
	if err := json.Unmarshal(args, into); err != nil {
		return Refusal("bad_arguments", "%v", err), false
	}
	return agentlink.Result{}, true
}

// Teammate resolves a teammate, by id or name, to its instance in the
// caller's deployment, or says why it can't.
func (c Caller) Teammate(teammate string) (string, agentlink.Result, bool) {
	want := strings.TrimSpace(teammate)
	var names []string
	for _, id := range c.Team.Config.Roster() {
		soul := c.Team.Config.Agents[id]
		if strings.EqualFold(want, id) || strings.EqualFold(want, soul.Name) {
			if id == c.Instance.AgentID {
				return "", Refusal("self", "that is you"), false
			}
			return id + "@" + c.Deployment.ProjectID, agentlink.Result{}, true
		}
		if id != c.Instance.AgentID {
			names = append(names, soul.Name)
		}
	}
	return "", Refusal("unknown_teammate", "no %q on this team; your teammates: %s", want, strings.Join(names, ", ")), false
}

// Name is how the caller is named to its teammates.
func (c Caller) Name() string { return c.Team.Config.Agents[c.Instance.AgentID].Name }

// LeadInstance is the instance of the caller's lead.
func (c Caller) LeadInstance() string { return c.Team.Config.Lead + "@" + c.Deployment.ProjectID }
