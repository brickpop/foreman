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
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
)

// Config is the team files, as loaded.
type Config interface {
	Team(id string) (home.Team, error)
}

// Sessions is what Messaging needs of the instance manager.
type Sessions interface {
	// Wake starts an asleep instance; any other is left as it is.
	Wake(ctx context.Context, id string) error
	// Type writes into an instance's terminal, as if typed.
	Type(id string, p []byte) error
	// Newline is the keystroke that starts a new line in the instance's
	// harness without sending.
	Newline(id string) string
}

// Deps are the blocks Messaging is assembled from.
type Deps struct {
	Messages    store.Messages
	Instances   store.Instances
	Deployments store.Deployments
	Config      Config
	Sessions    Sessions
	Publish     func(events.Event)
	// Delivered hears each message typed into its recipient's session
	// (Workflow: a hand-off's work item moves to "lead reviewing"). May be nil.
	Delivered func(ctx context.Context, msg store.TeamMessage)
	// Pause is how long to wait between typing a delivery and pressing Enter,
	// so the harness doesn't take the Enter as part of a paste.
	Pause time.Duration
	// Hold is how long messages wait on the CEO's draft, counted only while
	// the CEO isn't typing (Quiet since the last key); Tick is the countdown's
	// step. Zero means the defaults: 60 s, 5 s (so the CEO isn't rushed), 250 ms.
	Hold, Quiet, Tick time.Duration
}

// Messenger serves the messaging tools and delivers their messages.
type Messenger struct {
	Deps
	delivering sync.Mutex // one delivery at a time: a line is never typed into another
	mu         sync.Mutex // guards drafts
	drafts     map[string]*draft
}

// New returns a messenger.
func New(d Deps) *Messenger {
	if d.Hold == 0 {
		d.Hold = time.Minute
	}
	if d.Quiet == 0 {
		d.Quiet = 5 * time.Second
	}
	if d.Tick == 0 {
		d.Tick = 250 * time.Millisecond
	}
	return &Messenger{Deps: d, drafts: map[string]*draft{}}
}

func (m *Messenger) now() time.Time { return time.Now() }

var _ teamtools.Block = (*Messenger)(nil)

// previewLength is how much of a message its notice quotes.
const previewLength = 120

var definitions = []agentlink.Tool{
	{Name: "ask", Description: "Send a request to a teammate in this deployment: a question, or work to do. You get its msg id at once and keep working; their reply arrives in your session.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` +
			`"teammate":{"type":"string","description":"A teammate's name or id"},` +
			`"request":{"type":"string","description":"What you need, complete enough to act on alone"}},` +
			`"required":["teammate","request"]}`)},
	{Name: "reply", Description: "Answer an ask that was sent to you. Only the output (and the artifact) reaches the asker: give the outcome, the decisions you took, what is left open and how you checked it, in a few lines, never a transcript.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` +
			`"request_id":{"type":"string","description":"The ask's id, e.g. msg-41"},` +
			`"output":{"type":"string","description":"The outcome"},` +
			`"artifact":{"type":"string","description":"Optional: a branch or a wiki path with the work"}},` +
			`"required":["request_id","output"]}`)},
	{Name: "read_message", Description: "Read a message to you in full: a delivery in your session quotes only its start.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` +
			`"id":{"type":"string","description":"The message's id, e.g. msg-41"}},"required":["id"]}`)},
}

// Tools implements teamtools.Block.
func (m *Messenger) Tools() []agentlink.Tool { return definitions }

// Call implements teamtools.Block.
func (m *Messenger) Call(ctx context.Context, c teamtools.Caller, name string, args json.RawMessage) agentlink.Result {
	var a struct {
		Teammate  string `json:"teammate"`
		Request   string `json:"request"`
		RequestID string `json:"request_id"`
		Output    string `json:"output"`
		Artifact  string `json:"artifact"`
		ID        string `json:"id"`
	}
	if res, ok := teamtools.Args(args, &a); !ok {
		return res
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

func (m *Messenger) ask(ctx context.Context, c teamtools.Caller, teammate, request string) agentlink.Result {
	to, res, ok := c.Teammate(teammate)
	if !ok {
		return res
	}
	if strings.TrimSpace(request) == "" {
		return teamtools.Refusal("empty", "say what you need in request")
	}
	if teamtools.TooLong(request, teamtools.MaxBody) {
		return teamtools.Refusal("too_long", "a request is at most %d characters; put the rest in a wiki entry or a branch and name it", teamtools.MaxBody)
	}
	msg, err := m.Send(ctx, store.TeamMessage{DeploymentID: c.Deployment.ID, FromInstance: c.Instance.ID, ToInstance: to, Kind: "ask", Body: request})
	if err != nil {
		return teamtools.Refusal("internal", "%v", err)
	}
	return agentlink.Result{Text: fmt.Sprintf("%s sent to %s. Their reply will arrive in this session.", msg.Ref, nameOf(c.Team, to))}
}

func (m *Messenger) reply(ctx context.Context, c teamtools.Caller, ref, output, artifact string) agentlink.Result {
	ask, err := m.Messages.Get(ctx, c.Deployment.ID, ref)
	if err != nil || ask.Kind != "ask" {
		return teamtools.Refusal("unknown_request", "no ask %q in this deployment", ref)
	}
	if ask.ToInstance != c.Instance.ID {
		// Seen 2026-10-19: a lead "replying" to its own ask to answer the
		// teammate's reply, over and over. Say what to do instead.
		if ask.FromInstance == c.Instance.ID {
			return teamtools.Refusal("not_addressee", "%s is your own ask to %s; a reply comes from them and ends it. "+
				"Nothing to acknowledge: to follow up, ask() again, only when you need something back", ref, nameOf(c.Team, ask.ToInstance))
		}
		return teamtools.Refusal("not_addressee", "%s wasn't sent to you", ref)
	}
	if _, err := m.Messages.ReplyTo(ctx, c.Deployment.ID, ref); err == nil {
		return teamtools.Refusal("already_replied", "%s has its reply; for more, ask again", ref)
	}
	if strings.TrimSpace(output) == "" {
		return teamtools.Refusal("empty", "give the outcome in output")
	}
	if teamtools.TooLong(output, teamtools.MaxBody) {
		return teamtools.Refusal("too_long", "an output is at most %d characters: give the outcome, and put the rest in a branch or a wiki entry named as the artifact", teamtools.MaxBody)
	}
	if artifact != "" && (len(artifact) > 200 || strings.ContainsAny(artifact, " \t\n") || strings.Contains(artifact, "..")) {
		return teamtools.Refusal("bad_artifact", "an artifact is a branch name or a wiki path")
	}
	msg, err := m.Send(ctx, store.TeamMessage{DeploymentID: c.Deployment.ID, FromInstance: c.Instance.ID, ToInstance: ask.FromInstance,
		Kind: "reply", InReplyTo: ref, Body: output, ArtifactRef: artifact})
	if err != nil {
		return teamtools.Refusal("internal", "%v", err)
	}
	return agentlink.Result{Text: fmt.Sprintf("%s sent to %s.", msg.Ref, nameOf(c.Team, ask.FromInstance))}
}

func (m *Messenger) read(ctx context.Context, c teamtools.Caller, ref string) agentlink.Result {
	msg, err := m.Messages.Get(ctx, c.Deployment.ID, ref)
	if err != nil || msg.ToInstance != c.Instance.ID {
		// The same answer either way, so no one can probe others' messages.
		return teamtools.Refusal("unknown_message", "no message %q to you", ref)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s from %s (%s). Its text, as they wrote it:\n\n%s\n", msg.Ref, nameOf(c.Team, msg.FromInstance), kindText(msg), msg.Body)
	if msg.ArtifactRef != "" {
		fmt.Fprintf(&b, "\nArtifact: %s\n", msg.ArtifactRef)
	}
	if msg.Kind == "ask" {
		fmt.Fprintf(&b, "\nAnswer it with reply(%q, output).\n", msg.Ref)
	}
	return agentlink.Result{Text: b.String()}
}

// Send stores a message and delivers it when its recipient is idle.
func (m *Messenger) Send(ctx context.Context, msg store.TeamMessage) (store.TeamMessage, error) {
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
	if err != nil {
		return
	}
	if len(queued) == 0 {
		m.recount(id, 0) // they went another way (expired): the notice goes
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
		m.recount(id, len(queued))
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
	// The CEO is writing in its prompt: the messages wait, unless the
	// countdown ran out, when they go after the draft, marked as unfinished.
	wait, after := m.hold(id, len(queued))
	if wait {
		return
	}
	lines := []string{Notice(team, queued)}
	if after {
		lines = []string{"", "", "---", "", draftNote, "", lines[0]}
	}
	ids := make([]int64, len(queued))
	for i, msg := range queued {
		ids[i] = msg.ID
	}
	// Marked first, so a second idle while this is typed finds nothing to send.
	if err := m.Messages.SetState(ctx, ids, store.MessageDelivered, ""); err != nil {
		return
	}
	if err := m.typeLines(id, lines); err != nil {
		slog.Warn("delivering", "instance", id, "err", err)
		_ = m.Messages.SetState(ctx, ids, store.MessageQueued, "")
		return
	}
	m.forget(id) // the prompt was sent: a draft before the delivery went with it
	for _, msg := range queued {
		if got, err := m.Messages.Get(ctx, msg.DeploymentID, msg.Ref); err == nil {
			m.publish(got)
			if m.Delivered != nil {
				m.Delivered(ctx, got)
			}
		}
	}
}

// typeLines types lines into an instance's prompt, joined by its harness's
// newline keystroke, then sends them with Enter. Each newline and the Enter
// come after a pause, typed on their own, so the harness takes them as keys,
// not as part of a paste.
func (m *Messenger) typeLines(id string, lines []string) error {
	for i, line := range lines {
		if line != "" {
			if err := m.Sessions.Type(id, []byte(line)); err != nil {
				return err
			}
		}
		if i < len(lines)-1 {
			time.Sleep(m.Pause)
			if err := m.Sessions.Type(id, []byte(m.Sessions.Newline(id))); err != nil {
				return err
			}
		}
	}
	time.Sleep(m.Pause)
	return m.Sessions.Type(id, []byte("\r"))
}

// splitLines is text as lines, whatever its line endings.
func splitLines(text string) []string {
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}

// StatusChanged hears every instance status: an idle one gets its messages,
// and one that went to sleep (a clean exit) with messages waiting is woken
// for them. A stopped one isn't: its messages wait for the CEO's Start again.
func (m *Messenger) StatusChanged(id, status string) {
	switch status {
	case "starting", "stopped", "asleep", "queued":
		m.forget(id) // a new session starts with an empty prompt
	}
	if status == "idle" || status == "asleep" {
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

// Traffic is a deployment's messages, oldest first (the Sessions activity panel).
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
func Notice(team home.Team, msgs []store.TeamMessage) string {
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
func nameOf(team home.Team, instance string) string {
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
