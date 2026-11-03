package instances

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/brickpop/foreman/internal/store"
)

// An agent can stop at its harness's own question or permission dialog
// without calling any team tool: a card (source harness) then tells the CEO,
// on the Dashboard and the project's Inbox, and they answer in its session.
// One card per wait, however many hooks repeat it; the session's next
// activity, or its end, resolves it
// (wiki/product/architecture/protocol.md#waiting-for-the-ceo).

// raiseWaiting raises the run's waiting card, unless it has one.
func (m *Manager) raiseWaiting(ctx context.Context, id string, r *run, asks string) {
	if m.Requests == nil {
		return
	}
	// Taken before the save, so a hook arriving meanwhile raises no second.
	held := &store.Request{}
	m.mu.Lock()
	if r.waiting != nil {
		m.mu.Unlock()
		return
	}
	r.waiting = held
	m.mu.Unlock()
	card, err := m.saveWaiting(ctx, id, asks)
	m.mu.Lock()
	kept := err == nil && r.waiting == held && m.current(id, r)
	if r.waiting == held {
		r.waiting = nil
		if kept {
			r.waiting = &card
		}
	}
	m.mu.Unlock()
	if err != nil {
		slog.Warn("raising a waiting card", "instance", id, "err", err)
		return
	}
	m.publish("request.created", card)
	if !kept { // answered, or ended, while it was saved
		m.resolveCard(ctx, &card, "answered in its session")
	}
}

func (m *Manager) saveWaiting(ctx context.Context, id, asks string) (store.Request, error) {
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return store.Request{}, err
	}
	dep, err := m.Deployments.Get(ctx, inst.DeploymentID)
	if err != nil {
		return store.Request{}, err
	}
	summary := asks
	if summary == "" {
		summary = fmt.Sprintf("%s is waiting for your input.", m.agentName(dep, inst.AgentID))
	}
	return m.Requests.Save(ctx, store.Request{Kind: "question", Source: "harness",
		ProjectID: dep.ProjectID, DeploymentID: dep.ID, InstanceID: id, Summary: summary})
}

// answeredInSession resolves the run's waiting card: the harness works, or
// is at its prompt, again.
func (m *Manager) answeredInSession(ctx context.Context, r *run) {
	m.mu.Lock()
	card := r.waiting
	if card != nil && card.ID == "" {
		card = nil // still being saved: raiseWaiting resolves it
	}
	r.waiting = nil
	m.mu.Unlock()
	m.resolveCard(ctx, card, "answered in its session")
}

// settleOldWaiting follows the waiting cards a previous Foreman left open,
// once the runs are re-attached: an agent still awaiting in its run keeps
// its card; any other's resolves.
func (m *Manager) settleOldWaiting(ctx context.Context) {
	if m.Requests == nil {
		return
	}
	open, err := m.Requests.Open(ctx)
	if err != nil {
		slog.Warn("listing waiting cards", "err", err)
		return
	}
	for _, card := range open {
		if card.Source != "harness" {
			continue
		}
		inst, err := m.Instances.Get(ctx, card.InstanceID)
		m.mu.Lock()
		r := m.runs[card.InstanceID]
		adopted := err == nil && inst.Status == Awaiting && r != nil && m.current(card.InstanceID, r) && r.waiting == nil
		if adopted {
			c := card
			r.waiting = &c
		}
		m.mu.Unlock()
		if !adopted {
			m.resolveCard(ctx, &card, "Foreman restarted")
		}
	}
}
