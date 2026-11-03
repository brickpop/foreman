package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
)

// The lead's escalate(): a request the CEO answers on the Dashboard and in
// the project's Inbox, the answer coming back as a message
// (wiki/product/architecture/protocol.md#escalate). Nothing is held open: the
// call returns at once, and the lead keeps working (and getting messages)
// on anything but the step it asked about.

// escalations are the kinds of request an escalate() raises.
var escalations = []string{"approval", "question", "finding"}

const (
	maxSummary = 2000 // a request panel's text
	maxOptions = 6
	maxOption  = 80 // a button's label
	// dedupWindow: the same request again within it is the one already open.
	dedupWindow = 10 * time.Minute
)

// approvalOptions are an approval's when the lead names none.
var approvalOptions = []string{"Approve", "Deny"}

func (t tool) escalate(ctx context.Context, kind, summary string, options []string, wait bool) agentlink.Result {
	switch {
	case !slices.Contains(escalations, kind):
		return teamtools.Refusal("bad_kind", "kind is approval, question or finding")
	case summary == "":
		return teamtools.Refusal("empty", "say what you need from the CEO in summary")
	case teamtools.TooLong(summary, maxSummary):
		return teamtools.Refusal("too_long", "a summary is at most %d characters; put the detail in a wiki entry and name it", maxSummary)
	case len(options) > maxOptions:
		return teamtools.Refusal("too_long", "at most %d options", maxOptions)
	}
	var opts []string
	for _, o := range options {
		o = strings.TrimSpace(o)
		switch {
		case o == "" || strings.ContainsAny(o, "\r\n"):
			return teamtools.Refusal("bad_option", "each option is one line of text")
		case teamtools.TooLong(o, maxOption):
			return teamtools.Refusal("too_long", "an option is at most %d characters", maxOption)
		case slices.Contains(opts, o):
			return teamtools.Refusal("bad_option", "%q is there twice", o)
		}
		opts = append(opts, o)
	}
	if kind == "approval" && len(opts) == 0 {
		opts = approvalOptions
	}
	r, fresh, err := t.w.raise(ctx, store.Request{Kind: kind, ProjectID: t.c.Deployment.ProjectID, DeploymentID: t.c.Deployment.ID,
		InstanceID: t.c.Instance.ID, Source: "agent", Summary: summary, Options: opts})
	if err != nil {
		return teamtools.Refusal("internal", "%v", err)
	}
	text := fmt.Sprintf("Raised %s for the CEO.", r.ID)
	if !fresh {
		text = fmt.Sprintf("The CEO has it already, as %s: it's still open.", r.ID)
	}
	if kind == "approval" || wait {
		return agentlink.Result{Text: text + " Wait for the answer before that step; work on anything else meanwhile. It arrives in this session as a message."}
	}
	return agentlink.Result{Text: text + " Keep working: the answer arrives in this session as a message."}
}

// raise saves a request, unless the same one is open already: same agent,
// kind and summary (in case and spacing alike), raised within the window.
// fresh says whether it is a new one.
func (w *Workflow) raise(ctx context.Context, r store.Request) (store.Request, bool, error) {
	open, err := w.Requests.Open(ctx)
	if err != nil {
		return r, false, err
	}
	for _, o := range open {
		if o.InstanceID == r.InstanceID && o.Kind == r.Kind && sameText(o.Summary, r.Summary) && time.Since(o.CreatedAt) < dedupWindow {
			return o, false, nil
		}
	}
	saved, err := w.Requests.Save(ctx, r)
	if err != nil {
		return r, false, err
	}
	w.publishRequest("request.created", saved)
	return saved, true, nil
}

// sameText compares two texts as a reader would: case and spacing aside.
func sameText(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}

// ErrAnswered means the request isn't open any more: answered already, or
// expired with its deployment.
var ErrAnswered = errors.New("workflow: the request isn't open")

// Answer is the CEO's answer to an agent's request: an option or their own
// text. It resolves the request and reaches the agent as a message from the
// CEO; a request whose deployment stopped expires instead. Reviews are
// decided on their page, a stuck agent by restarting it.
func (w *Workflow) Answer(ctx context.Context, id, answer string) (store.Request, error) {
	answer = strings.TrimSpace(answer)
	switch {
	case answer == "":
		return store.Request{}, fmt.Errorf("%w: write an answer", ErrInvalid)
	case teamtools.TooLong(answer, teamtools.MaxBody):
		return store.Request{}, fmt.Errorf("%w: an answer is at most %d characters", ErrInvalid, teamtools.MaxBody)
	}
	// One answer at a time: two at once would both find it open, and both reach the agent.
	w.answering.Lock()
	defer w.answering.Unlock()
	open, err := w.Requests.Open(ctx)
	if err != nil {
		return store.Request{}, err
	}
	i := slices.IndexFunc(open, func(r store.Request) bool { return r.ID == id })
	if i < 0 {
		return store.Request{}, ErrAnswered
	}
	r := open[i]
	if !slices.Contains(escalations, r.Kind) || r.Source != "agent" {
		return store.Request{}, fmt.Errorf("%w: this request isn't answered here", ErrInvalid)
	}
	if active, err := w.Teams.Active(ctx, r.DeploymentID); err != nil {
		return store.Request{}, err
	} else if !active {
		w.expire(ctx, r)
		return store.Request{}, ErrAnswered
	}
	r.Status, r.Answer, r.ResolvedAt = store.RequestResolved, answer, time.Now().UTC()
	if r, err = w.Requests.Save(ctx, r); err != nil {
		return r, err
	}
	w.publishRequest("request.resolved", r)
	if _, err := w.Messages.Send(ctx, store.TeamMessage{DeploymentID: r.DeploymentID, FromInstance: "ceo", ToInstance: r.InstanceID,
		Kind: "answer", InReplyTo: r.ID, Body: fmt.Sprintf("On your %s %q: %s", r.Kind, r.Summary, answer)}); err != nil {
		slog.Warn("delivering an answer", "request", r.ID, "err", err)
	}
	return r, nil
}

// DeploymentStopped expires the agents' requests of a deployment that
// stopped: nobody is left to get the answer.
func (w *Workflow) DeploymentStopped(ctx context.Context, deployment string) {
	open, err := w.Requests.Open(ctx)
	if err != nil {
		slog.Warn("expiring requests", "deployment", deployment, "err", err)
		return
	}
	for _, r := range open {
		if r.DeploymentID == deployment && slices.Contains(escalations, r.Kind) && r.Source == "agent" {
			w.expire(ctx, r)
		}
	}
}

func (w *Workflow) expire(ctx context.Context, r store.Request) {
	r.Status, r.ResolvedAt = store.RequestExpired, time.Now().UTC()
	saved, err := w.Requests.Save(ctx, r)
	if err != nil {
		slog.Warn("expiring a request", "request", r.ID, "err", err)
		return
	}
	w.publishRequest("request.resolved", saved)
}
