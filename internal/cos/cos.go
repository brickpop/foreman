// Package cos is the Chief of Staff's side of Foreman
// (wiki/product/features/chief-of-staff.md): its one tool, propose_team, and
// the proposal the CEO creates from the page. It proposes; only the CEO's
// click writes a team file. The session itself is an ad-hoc terminal, like a
// profile's login (cmd/foreman).
package cos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/teamtools"
)

// Instance is the Chief of Staff's id: its terminal's, and its caller's on
// the agent port.
const Instance = "cos"

// Teams is the team files as the Chief of Staff works on them.
type Teams interface {
	Teams() []home.Team
	Team(id string) (home.Team, error)
	// CheckTeam is what a save would refuse.
	CheckTeam(c home.TeamConfig) []string
	SaveTeam(ctx context.Context, c home.TeamConfig, version string) (home.Team, error)
}

// Deps are what the Chief of Staff is assembled from.
type Deps struct {
	Teams Teams
	// Deployed reports whether a team has an active deployment: its members
	// can't be removed then.
	Deployed func(ctx context.Context, team string) bool
	Publish  func(events.Event)
	// Tell types a line into the Chief of Staff's session, as the CEO would.
	Tell func(text string)
}

// CoS holds the proposal on screen.
type CoS struct {
	Deps
	mu       sync.Mutex
	last     int       // the last proposal's id
	proposal *Proposal // the one on screen, nil when none
	version  string    // the team file's version it was drafted from ("" for a new team)
}

// New returns the Chief of Staff's side.
func New(d Deps) *CoS { return &CoS{Deps: d} }

var (
	// ErrStale means the proposal isn't the one on screen any more.
	ErrStale = errors.New("cos: a newer proposal replaced it, or none is on screen")
	// ErrChanged means the team file changed since the proposal was drafted:
	// nothing was written, and the Chief of Staff was asked for a new one.
	ErrChanged = errors.New("cos: the team changed since it was proposed")
)

// ProposeTeam is the tool's name.
const ProposeTeam = "propose_team"

var _ agentlink.Tools = (*CoS)(nil)

// List implements agentlink.Tools: the one tool.
func (c *CoS) List(string) []agentlink.Tool {
	return []agentlink.Tool{{
		Name: ProposeTeam,
		Description: "Show the CEO a team as it should be, to create or change it: a new team, or an existing one " +
			"with a member added (a hire) or changed (a refinement). Give the whole team, every member included. " +
			"Foreman checks it and opens it as a preview; the CEO creates it from there, or tells you what to change. " +
			"Nothing is written until they do. A newer proposal replaces the one on screen.",
		InputSchema: teamSchema,
	}}
}

// Call implements agentlink.Tools.
func (c *CoS) Call(ctx context.Context, _ string, name string, args json.RawMessage) agentlink.Result {
	if name != ProposeTeam {
		return teamtools.Refusal("unknown_tool", "%q isn't one of your tools", name)
	}
	var team home.TeamConfig
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&team); err != nil {
		return teamtools.Refusal("invalid", "the team isn't in the expected shape: %v", err)
	}
	current, version := c.current(team.Team)
	if current != nil && len(team.Order) == 0 {
		team.Order = current.Order // the file's order, then the new members
	}
	if problems := c.Teams.CheckTeam(team); len(problems) > 0 {
		return teamtools.Refusal("invalid", "fix these, then propose again:\n- %s", strings.Join(problems, "\n- "))
	}
	if current != nil && c.Deployed(ctx, team.Team) {
		for id := range current.Agents {
			if _, kept := team.Agents[id]; !kept {
				return teamtools.Refusal("deployed", "%s is deployed: its members can't be removed (%s is missing); stop its deployments first", team.Team, id)
			}
		}
	}
	c.mu.Lock()
	c.last++
	c.proposal, c.version = &Proposal{ID: c.last, Team: team, Current: current}, version
	p := *c.proposal
	c.mu.Unlock()
	c.Publish(events.Event{Type: "cos.proposal", Data: p})
	return agentlink.Result{Text: "The CEO sees your proposal now. Wait for them: they create it, or tell you what to change."}
}

// current is the team as it is now and its version; nil for a new team.
func (c *CoS) current(id string) (*home.TeamConfig, string) {
	t, err := c.Teams.Team(id)
	if err != nil {
		return nil, ""
	}
	return &t.Config, t.Version
}

// Proposal is the one on screen.
func (c *CoS) Proposal() (Proposal, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.proposal == nil {
		return Proposal{}, false
	}
	return *c.proposal, true
}

// Create writes proposal id, the one on screen, as the CEO created it. Once
// written, or once the team changed meanwhile, the proposal is gone and the
// Chief of Staff is told.
func (c *CoS) Create(ctx context.Context, id int) (home.Team, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.proposal == nil || c.proposal.ID != id {
		return home.Team{}, ErrStale
	}
	team := c.proposal.Team
	team.Agents = cloneAgents(team.Agents)
	saved, err := c.Teams.SaveTeam(ctx, team, c.version)
	switch {
	case errors.Is(err, home.ErrConflict):
		c.clear()
		c.Tell(fmt.Sprintf("From Foreman: %s changed since your proposal, so nothing was written. Read its file again and propose it again.", team.Name))
		return home.Team{}, ErrChanged
	case err != nil:
		return home.Team{}, err
	}
	c.clear()
	verb := "changed"
	if c.version == "" {
		verb = "created"
	}
	c.Tell(fmt.Sprintf("From Foreman: the CEO %s %s; it is saved.", verb, team.Name))
	return saved, nil
}

// Drop forgets the proposal on screen: the session ended.
func (c *CoS) Drop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.proposal != nil {
		c.clear()
	}
}

// clear forgets the proposal and tells the page. Called with c.mu held.
func (c *CoS) clear() {
	c.proposal, c.version = nil, ""
	c.Publish(events.Event{Type: "cos.proposal", Data: nil})
}

// cloneAgents copies the proposal's members, so the save shares nothing with it.
func cloneAgents(agents map[string]home.Soul) map[string]home.Soul {
	out := make(map[string]home.Soul, len(agents))
	for id, s := range agents {
		s.Skills = slices.Clone(s.Skills)
		out[id] = s
	}
	return out
}
