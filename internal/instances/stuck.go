package instances

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/brickpop/foreman/internal/store"
)

// heardFrom notes activity from the instance's run: its stuck card, if any,
// resolves.
func (m *Manager) heardFrom(ctx context.Context, id string, r *run) {
	m.mu.Lock()
	r.heard = time.Now()
	stuck := r.stuck
	r.stuck = nil
	m.mu.Unlock()
	m.resolveCard(ctx, stuck, "active again")
}

// resolveCard resolves a card about the run (stuck, waiting), if there is
// one, with why. One still being saved (no id yet) is left to its raiser.
func (m *Manager) resolveCard(ctx context.Context, card *store.Request, why string) {
	if card == nil || card.ID == "" {
		return
	}
	r := *card
	r.Status, r.Answer, r.ResolvedAt = store.RequestResolved, why, time.Now().UTC()
	if _, err := m.Requests.Save(ctx, r); err != nil {
		slog.Warn("resolving a card", "instance", r.InstanceID, "err", err)
		return
	}
	m.publish("request.resolved", r)
}

// CheckStuck raises a card for each agent mid-turn (running) from which no
// hook has come for the stuck_after_minutes setting: hooks fire on every
// prompt, tool call and turn end, so a working agent is never that quiet. The
// agent keeps running, and the CEO decides (wiki/product/architecture/agent-runtime.md#stuck-agents).
func (m *Manager) CheckStuck(ctx context.Context, now time.Time) {
	if m.Requests == nil {
		return
	}
	limit := time.Duration(m.Config.Settings().Settings.StuckAfterMinutes) * time.Minute
	quiet := map[string]*run{}
	m.mu.Lock()
	for id, r := range m.runs {
		if r.term != nil && r.stuck == nil && now.Sub(r.heard) >= limit {
			quiet[id] = r
		}
	}
	m.mu.Unlock()
	for id, r := range quiet {
		inst, err := m.Instances.Get(ctx, id)
		if err != nil || inst.Status != Running {
			continue
		}
		dep, err := m.Deployments.Get(ctx, inst.DeploymentID)
		if err != nil {
			continue
		}
		card, err := m.Requests.Save(ctx, store.Request{Kind: "blocked", Source: "foreman",
			ProjectID: dep.ProjectID, DeploymentID: dep.ID, InstanceID: id, Options: []string{"Restart"},
			Summary: fmt.Sprintf("%s may be stuck: no activity for %d min.", m.agentName(dep, inst.AgentID), int(limit.Minutes()))})
		if err != nil {
			slog.Warn("raising a stuck card", "instance", id, "err", err)
			continue
		}
		m.publish("request.created", card)
		// Kept by its run; one that ended meanwhile (or stirred) has it resolved at once.
		m.mu.Lock()
		kept := m.runs[id] == r && r.stuck == nil && now.Sub(r.heard) >= limit
		if kept {
			r.stuck = &card
		}
		m.mu.Unlock()
		if !kept {
			m.resolveCard(ctx, &card, "active again")
		}
	}
}

// agentName is the instance's agent as its team names it, else its id.
func (m *Manager) agentName(dep store.Deployment, agent string) string {
	if team, err := m.Config.Team(dep.TeamID); err == nil {
		if a, ok := team.Config.Agents[agent]; ok && a.Name != "" {
			return a.Name
		}
	}
	return agent
}

// resolveOldStuck resolves the stuck cards a previous Foreman left open
// (Foreman's blocked cards about an agent): the watch starts over.
func (m *Manager) resolveOldStuck(ctx context.Context) {
	if m.Requests == nil {
		return
	}
	open, err := m.Requests.Open(ctx)
	if err != nil {
		slog.Warn("listing stuck cards", "err", err)
		return
	}
	for _, r := range open {
		if r.Kind == "blocked" && r.Source == "foreman" && r.InstanceID != "" {
			m.resolveCard(ctx, &r, "Foreman restarted")
		}
	}
}
