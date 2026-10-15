// Package messaging is the Messaging block (wiki/product/architecture/building-blocks.md#6-messaging-messaging):
// the team tools that carry messages between the instances of one deployment
// (ask, reply, read_message), and their delivery: one plain line typed into
// the recipient's session when it is idle, the whole message read with
// read_message (wiki/product/architecture/protocol.md#messages).
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/prompt"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/workspace"
)

// Config is the team files, as loaded.
type Config interface {
	Team(id string) (workspace.Team, error)
}

// Sessions is what Messaging needs of the instance manager.
type Sessions interface {
	// Wake starts an asleep instance; any other is left as it is.
	Wake(ctx context.Context, id string) error
	// Type writes into an instance's terminal, as if typed.
	Type(id string, p []byte) error
}

// Deps are the blocks Messaging is assembled from.
type Deps struct {
	Messages    store.Messages
	Instances   store.Instances
	Deployments store.Deployments
	Config      Config
	Sessions    Sessions
	Publish     func(events.Event)
	// Pause is how long to wait between typing a delivery and pressing Enter,
	// so the harness doesn't take the Enter as part of a paste.
	Pause time.Duration
}

// Messenger serves the messaging tools and delivers their messages.
type Messenger struct {
	Deps
	delivering sync.Mutex // one delivery at a time: a line is never typed into another
}

// New returns a messenger.
func New(d Deps) *Messenger { return &Messenger{Deps: d} }

var _ agentlink.Tools = (*Messenger)(nil)

// maxBody caps a request or an output: longer work belongs in a branch or a
// wiki entry, named as the artifact.
const maxBody = 8000

// previewLength is how much of a message its notice quotes.
const previewLength = 120

var definitions = map[string]agentlink.Tool{
	"ask": {Name: "ask", Description: "Send a request to a teammate in this deployment: a question, or work to do. You get its msg id at once and keep working; their reply arrives in your session.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` +
			`"teammate":{"type":"string","description":"A teammate's name or id"},` +
			`"request":{"type":"string","description":"What you need, complete enough to act on alone"}},` +
			`"required":["teammate","request"]}`)},
	"reply": {Name: "reply", Description: "Answer an ask that was sent to you. Only the output (and the artifact) reaches the asker: give the outcome, the decisions you took, what is left open and how you checked it, in a few lines, never a transcript.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` +
			`"request_id":{"type":"string","description":"The ask's id, e.g. msg-41"},` +
			`"output":{"type":"string","description":"The outcome"},` +
			`"artifact":{"type":"string","description":"Optional: a branch or a wiki path with the work"}},` +
			`"required":["request_id","output"]}`)},
	"read_message": {Name: "read_message", Description: "Read a message to you in full: a delivery in your session quotes only its start.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` +
			`"id":{"type":"string","description":"The message's id, e.g. msg-41"}},"required":["id"]}`)},
}

// List implements agentlink.Tools: the tools the instance's role is offered.
func (m *Messenger) List(instance string) []agentlink.Tool {
	c, err := m.caller(context.Background(), instance)
	if err != nil {
		return nil
	}
	var tools []agentlink.Tool
	for _, name := range prompt.Offered(c.lead) {
		if d, ok := definitions[name]; ok {
			tools = append(tools, d)
		}
	}
	return tools
}

// caller is who calls a tool: its instance, deployment and team.
type caller struct {
	inst store.Instance
	dep  store.Deployment
	team workspace.Team
	lead bool
}

func (m *Messenger) caller(ctx context.Context, instance string) (caller, error) {
	inst, err := m.Instances.Get(ctx, instance)
	if err != nil {
		return caller{}, err
	}
	dep, err := m.Deployments.Get(ctx, inst.DeploymentID)
	if err != nil {
		return caller{}, err
	}
	team, err := m.Config.Team(dep.TeamID)
	if err != nil {
		return caller{}, err
	}
	return caller{inst: inst, dep: dep, team: team, lead: inst.AgentID == team.Config.Lead}, nil
}

// refusal is a tool error the agent reads: a code, then what to do.
func refusal(code, format string, a ...any) agentlink.Result {
	return agentlink.Result{Text: code + ": " + fmt.Sprintf(format, a...), Error: true}
}

// Call implements agentlink.Tools.
func (m *Messenger) Call(ctx context.Context, instance, name string, args json.RawMessage) agentlink.Result {
	c, err := m.caller(ctx, instance)
	if err != nil {
		return refusal("unknown_caller", "%v", err)
	}
	if !slices.Contains(prompt.Offered(c.lead), name) {
		return refusal("unknown_tool", "%q isn't one of your tools", name)
	}
	if c.dep.Status != store.DeploymentActive {
		return refusal("deployment_stopped", "this deployment was stopped")
	}
	var a struct {
		Teammate  string `json:"teammate"`
		Request   string `json:"request"`
		RequestID string `json:"request_id"`
		Output    string `json:"output"`
		Artifact  string `json:"artifact"`
		ID        string `json:"id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &a); err != nil {
			return refusal("bad_arguments", "%v", err)
		}
	}
	switch name {
	case "ask":
		return m.ask(ctx, c, a.Teammate, a.Request)
	case "reply":
		return m.reply(ctx, c, strings.TrimSpace(a.RequestID), a.Output, strings.TrimSpace(a.Artifact))
	default: // read_message
		return m.read(ctx, c, strings.TrimSpace(a.ID))
	}
}

func (m *Messenger) ask(ctx context.Context, c caller, teammate, request string) agentlink.Result {
	to, res, ok := teammateOf(c, teammate)
	if !ok {
		return res
	}
	if strings.TrimSpace(request) == "" {
		return refusal("empty", "say what you need in request")
	}
	if len(request) > maxBody {
		return refusal("too_long", "a request is at most %d characters; put the rest in a wiki entry or a branch and name it", maxBody)
	}
	msg, err := m.send(ctx, store.TeamMessage{DeploymentID: c.dep.ID, FromInstance: c.inst.ID, ToInstance: to, Kind: "ask", Body: request})
	if err != nil {
		return refusal("internal", "%v", err)
	}
	return agentlink.Result{Text: fmt.Sprintf("%s sent to %s. Their reply will arrive in this session.", msg.Ref, nameOf(c.team, to))}
}

func (m *Messenger) reply(ctx context.Context, c caller, ref, output, artifact string) agentlink.Result {
	ask, err := m.Messages.Get(ctx, c.dep.ID, ref)
	if err != nil || ask.Kind != "ask" {
		return refusal("unknown_request", "no ask %q in this deployment", ref)
	}
	if ask.ToInstance != c.inst.ID {
		return refusal("not_addressee", "%s wasn't sent to you", ref)
	}
	if _, err := m.Messages.ReplyTo(ctx, c.dep.ID, ref); err == nil {
		return refusal("already_replied", "%s has its reply; for more, ask again", ref)
	}
	if strings.TrimSpace(output) == "" {
		return refusal("empty", "give the outcome in output")
	}
	if len(output) > maxBody {
		return refusal("too_long", "an output is at most %d characters: give the outcome, and put the rest in a branch or a wiki entry named as the artifact", maxBody)
	}
	if artifact != "" && (len(artifact) > 200 || strings.ContainsAny(artifact, " \t\n") || strings.Contains(artifact, "..")) {
		return refusal("bad_artifact", "an artifact is a branch name or a wiki path")
	}
	msg, err := m.send(ctx, store.TeamMessage{DeploymentID: c.dep.ID, FromInstance: c.inst.ID, ToInstance: ask.FromInstance,
		Kind: "reply", InReplyTo: ref, Body: output, ArtifactRef: artifact})
	if err != nil {
		return refusal("internal", "%v", err)
	}
	return agentlink.Result{Text: fmt.Sprintf("%s sent to %s.", msg.Ref, nameOf(c.team, ask.FromInstance))}
}

func (m *Messenger) read(ctx context.Context, c caller, ref string) agentlink.Result {
	msg, err := m.Messages.Get(ctx, c.dep.ID, ref)
	if err != nil || msg.ToInstance != c.inst.ID {
		// The same answer either way, so no one can probe others' messages.
		return refusal("unknown_message", "no message %q to you", ref)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s from %s (%s). Its text, as they wrote it:\n\n%s\n", msg.Ref, nameOf(c.team, msg.FromInstance), kindText(msg), msg.Body)
	if msg.ArtifactRef != "" {
		fmt.Fprintf(&b, "\nArtifact: %s\n", msg.ArtifactRef)
	}
	if msg.Kind == "ask" {
		fmt.Fprintf(&b, "\nAnswer it with reply(%q, output).\n", msg.Ref)
	}
	return agentlink.Result{Text: b.String()}
}

// teammateOf resolves a teammate, by id or name, to its instance in the
// caller's deployment.
func teammateOf(c caller, teammate string) (string, agentlink.Result, bool) {
	want := strings.TrimSpace(teammate)
	var names []string
	for _, id := range c.team.Config.Roster() {
		soul := c.team.Config.Agents[id]
		if strings.EqualFold(want, id) || strings.EqualFold(want, soul.Name) {
			if id == c.inst.AgentID {
				return "", refusal("self", "that is you"), false
			}
			return id + "@" + c.dep.ProjectID, agentlink.Result{}, true
		}
		if id != c.inst.AgentID {
			names = append(names, soul.Name)
		}
	}
	return "", refusal("unknown_teammate", "no %q on this team; your teammates: %s", want, strings.Join(names, ", ")), false
}

// send stores a message and tries to deliver it.
func (m *Messenger) send(ctx context.Context, msg store.TeamMessage) (store.TeamMessage, error) {
	msg, err := m.Messages.Add(ctx, msg)
	if err != nil {
		return msg, err
	}
	m.publish(msg)
	go m.Deliver(msg.ToInstance)
	return msg, nil
}

// Deliver gives an instance what waits for it, if it is idle: everything
// queued, as one line, in one turn. An asleep instance is woken first; its
// messages follow once it is idle.
func (m *Messenger) Deliver(id string) {
	m.delivering.Lock()
	defer m.delivering.Unlock()
	ctx := context.Background()
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return
	}
	queued, err := m.Messages.Queued(ctx, id)
	if err != nil || len(queued) == 0 {
		return
	}
	switch inst.Status {
	case "asleep":
		if err := m.Sessions.Wake(ctx, id); err != nil {
			slog.Warn("waking for a message", "instance", id, "err", err)
		}
		return
	case "idle":
	default:
		return // its next idle brings them
	}
	dep, err := m.Deployments.Get(ctx, inst.DeploymentID)
	if err != nil {
		return
	}
	team, err := m.Config.Team(dep.TeamID)
	if err != nil {
		return
	}
	ids := make([]int64, len(queued))
	for i, msg := range queued {
		ids[i] = msg.ID
	}
	// Marked first, so a second idle while this is typed finds nothing to send.
	if err := m.Messages.SetState(ctx, ids, store.MessageDelivered, ""); err != nil {
		return
	}
	if err := m.typeLine(id, Notice(team, queued)); err != nil {
		slog.Warn("delivering", "instance", id, "err", err)
		_ = m.Messages.SetState(ctx, ids, store.MessageQueued, "")
		return
	}
	for _, msg := range queued {
		if got, err := m.Messages.Get(ctx, msg.DeploymentID, msg.Ref); err == nil {
			m.publish(got)
		}
	}
}

func (m *Messenger) typeLine(id, line string) error {
	if err := m.Sessions.Type(id, []byte(line)); err != nil {
		return err
	}
	time.Sleep(m.Pause)
	return m.Sessions.Type(id, []byte("\r"))
}

// StatusChanged hears every instance status: an idle one gets its messages.
func (m *Messenger) StatusChanged(id, status string) {
	if status == "idle" {
		go m.Deliver(id)
	}
}

// DeploymentStopped expires what still waits in it.
func (m *Messenger) DeploymentStopped(deployment string) {
	expired, err := m.Messages.Expire(context.Background(), deployment, "deployment_stopped")
	if err != nil {
		slog.Warn("expiring messages", "deployment", deployment, "err", err)
	}
	for _, msg := range expired {
		m.publish(msg)
	}
}

// Sweep delivers to every idle instance with messages waiting: at startup,
// after the instances are restored.
func (m *Messenger) Sweep(ctx context.Context) error {
	deps, err := m.Deployments.List(ctx)
	if err != nil {
		return err
	}
	for _, dep := range deps {
		if dep.Status != store.DeploymentActive {
			continue
		}
		insts, err := m.Instances.List(ctx, dep.ID)
		if err != nil {
			return err
		}
		for _, inst := range insts {
			m.Deliver(inst.ID)
		}
	}
	return nil
}

// Traffic is a deployment's messages, oldest first (the Messages drawer).
func (m *Messenger) Traffic(ctx context.Context, deployment string) ([]store.TeamMessage, error) {
	return m.Messages.List(ctx, deployment)
}

func (m *Messenger) publish(msg store.TeamMessage) {
	if m.Publish != nil {
		m.Publish(events.Event{Type: "team.message", Data: msg})
	}
}

// Notice is the one line that delivers messages: one message's notice, or a
// line naming each of a burst (wiki/product/architecture/protocol.md#messages).
// Plain words only: no symbols, no line breaks.
func Notice(team workspace.Team, msgs []store.TeamMessage) string {
	if len(msgs) == 1 {
		msg := msgs[0]
		head := fmt.Sprintf("Message %s from %s (%s): %s", msg.Ref, nameOf(team, msg.FromInstance), kindText(msg), preview(msg.Body))
		switch msg.Kind {
		case "ask":
			return fmt.Sprintf("%s Read it with read_message(%q); answer with reply(%q, output).", head, msg.Ref, msg.Ref)
		case "reply":
			return fmt.Sprintf("%s Read it with read_message(%q), then resume the task that asked for it.", head, msg.Ref)
		default:
			return fmt.Sprintf("%s Read it with read_message(%q).", head, msg.Ref)
		}
	}
	parts := make([]string, len(msgs))
	for i, msg := range msgs {
		parts[i] = fmt.Sprintf("%s from %s (%s)", msg.Ref, nameOf(team, msg.FromInstance), kindText(msg))
	}
	return fmt.Sprintf("%d new messages: %s. Read each with read_message.", len(msgs), strings.Join(parts, ", "))
}

func kindText(msg store.TeamMessage) string {
	switch {
	case msg.Kind == "reply":
		return "reply to " + msg.InReplyTo
	case msg.Kind == "handoff" && msg.ArtifactRef != "":
		return "handoff of " + msg.ArtifactRef
	case msg.Kind == "contribute":
		return "contribution"
	}
	return msg.Kind
}

// preview is the start of a body on one line, cut on a rune.
func preview(body string) string {
	flat := strings.Join(strings.Fields(body), " ")
	if utf8.RuneCountInString(flat) <= previewLength {
		return flat
	}
	return string([]rune(flat)[:previewLength]) + "..."
}

// nameOf is how an instance or a sender is named to agents.
func nameOf(team workspace.Team, instance string) string {
	switch instance {
	case "foreman":
		return "Foreman"
	case "ceo":
		return "the CEO"
	}
	agent, _, _ := strings.Cut(instance, "@")
	if soul, ok := team.Config.Agents[agent]; ok {
		return soul.Name
	}
	return agent
}
