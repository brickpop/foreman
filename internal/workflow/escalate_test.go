package workflow

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
)

func (f *fixture) open() []store.Request {
	f.t.Helper()
	list, err := f.db.Requests().Open(ctx)
	if err != nil {
		f.t.Fatal(err)
	}
	return list
}

func TestEscalateRaisesARequest(t *testing.T) {
	f := newFixture(t)
	text := f.ok("mara", "escalate", map[string]any{"kind": "approval", "title": "Add decimal.js?"})
	if text != "Raised req-1 for the CEO. Wait for the answer before that step; work on anything else meanwhile. It arrives in this session as a message." {
		t.Errorf("%q", text)
	}
	if text := f.ok("mara", "escalate", map[string]any{"kind": "question", "title": "Which currency first?"}); !strings.HasSuffix(text, "Keep working: the answer arrives in this session as a message.") {
		t.Errorf("a question: %q", text)
	}
	if text := f.ok("mara", "escalate", map[string]any{"kind": "question", "title": "EUR or USD?", "wait": true}); !strings.Contains(text, "Wait for the answer") {
		t.Errorf("a waited question: %q", text)
	}
	f.ok("mara", "escalate", map[string]any{"kind": "finding", "title": "A race in settlement.", "body": "  Two payouts can settle twice:\n\n- see wiki/findings/race.md  ", "options": []string{" Fix it now ", "Add to backlog", "Not an issue"}})
	// At the limit, in characters not bytes.
	f.ok("mara", "escalate", map[string]any{"kind": "question", "title": strings.Repeat("é", teamtools.MaxTitle), "body": strings.Repeat("é", teamtools.MaxBody)})

	open := f.open()
	if len(open) != 5 {
		t.Fatalf("open %+v", open)
	}
	approval, finding := open[0], open[3]
	if approval.Kind != "approval" || approval.Source != "agent" || approval.InstanceID != "mara@ledger" || approval.DeploymentID != "atlas@ledger" ||
		approval.ProjectID != "ledger" || strings.Join(approval.Options, ",") != "Approve,Deny" {
		t.Errorf("approval %+v", approval)
	}
	if finding.Body != "Two payouts can settle twice:\n\n- see wiki/findings/race.md" || approval.Body != "" {
		t.Errorf("bodies: %q, %q", finding.Body, approval.Body)
	}
	if strings.Join(finding.Options, ",") != "Fix it now,Add to backlog,Not an issue" || len(open[1].Options) != 0 {
		t.Errorf("options: finding %v, question %v", finding.Options, open[1].Options)
	}
	created := 0
	for _, e := range f.events {
		if e.Type == "request.created" {
			created++
		}
	}
	if created != 5 {
		t.Errorf("%d request.created", created)
	}
}

func TestEscalateRefusals(t *testing.T) {
	f := newFixture(t)
	f.refused("mara", "escalate", map[string]any{"kind": "review", "title": "x"}, "bad_kind")
	f.refused("mara", "escalate", map[string]any{"kind": "question", "title": " "}, "empty")
	// A title is one short line; the rest goes in body.
	f.refused("mara", "escalate", map[string]any{"kind": "question", "title": strings.Repeat("é", teamtools.MaxTitle+1)}, "too_long")
	f.refused("mara", "escalate", map[string]any{"kind": "question", "title": "Which currency?\nEUR has…"}, "too_long")
	f.refused("mara", "escalate", map[string]any{"kind": "question", "title": "x", "body": strings.Repeat("x", teamtools.MaxBody+1)}, "too_long")
	f.refused("mara", "escalate", map[string]any{"kind": "question", "title": "x", "options": []string{"a", "b", "c", "d", "e", "f", "g"}}, "too_long")
	f.refused("mara", "escalate", map[string]any{"kind": "question", "title": "x", "options": []string{strings.Repeat("o", maxOption+1)}}, "too_long")
	f.refused("mara", "escalate", map[string]any{"kind": "question", "title": "x", "options": []string{"Yes", " Yes"}}, "bad_option")
	f.refused("mara", "escalate", map[string]any{"kind": "question", "title": "x", "options": []string{"Yes", ""}}, "bad_option")
	f.refused("mara", "escalate", map[string]any{"kind": "question", "title": "x", "options": []string{"a\nb"}}, "bad_option")
	if open := f.open(); len(open) != 0 {
		t.Errorf("raised anyway: %+v", open)
	}
}

// The same request again is the one open; another kind, another agent, or
// after the window, is a new one.
func TestEscalateTwiceIsOneRequest(t *testing.T) {
	f := newFixture(t)
	f.ok("mara", "escalate", map[string]any{"kind": "approval", "title": "Add  decimal.js?"})
	if text := f.ok("mara", "escalate", map[string]any{"kind": "approval", "title": "add decimal.js? "}); !strings.HasPrefix(text, "The CEO has it already, as req-1: it's still open.") {
		t.Errorf("%q", text)
	}
	f.ok("mara", "escalate", map[string]any{"kind": "question", "title": "Add decimal.js?"})
	// The same title saying something else in its body is another request.
	if text := f.ok("mara", "escalate", map[string]any{"kind": "approval", "title": "Add decimal.js?", "body": "For rounding."}); !strings.HasPrefix(text, "Raised req-3") {
		t.Errorf("another body: %q", text)
	}
	f.ok("tove", "escalate", map[string]any{"kind": "approval", "title": "Add decimal.js?"})
	if open := f.open(); len(open) != 4 {
		t.Fatalf("open %+v", open)
	}
	old := f.open()[0]
	old.CreatedAt = time.Now().Add(-dedupWindow - time.Minute)
	if _, err := f.db.Requests().Save(ctx, old); err != nil {
		t.Fatal(err)
	}
	if text := f.ok("mara", "escalate", map[string]any{"kind": "approval", "title": "Add decimal.js?"}); !strings.HasPrefix(text, "Raised req-5") {
		t.Errorf("after the window: %q", text)
	}
}

func TestTheCEOsAnswerReachesTheLead(t *testing.T) {
	f := newFixture(t)
	f.ok("mara", "escalate", map[string]any{"kind": "approval", "title": "Add decimal.js?"})
	if _, err := f.w.Answer(ctx, "req-1", " "); !errors.Is(err, ErrInvalid) {
		t.Errorf("no answer: %v", err)
	}
	r, err := f.w.Answer(ctx, "req-1", " Approve, but pin 2.x ")
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != store.RequestResolved || r.Answer != "Approve, but pin 2.x" || r.ResolvedAt.IsZero() || len(f.open()) != 0 {
		t.Errorf("resolved %+v", r)
	}
	if len(f.m.sent) != 1 {
		t.Fatalf("sent %+v", f.m.sent)
	}
	if msg := f.m.sent[0]; msg.FromInstance != "ceo" || msg.ToInstance != "mara@ledger" || msg.Kind != "answer" || msg.InReplyTo != "req-1" ||
		msg.Body != `On your approval "Add decimal.js?": Approve, but pin 2.x` {
		t.Errorf("message %+v", msg)
	}
	// Answered once: a second answer reaches nobody.
	if _, err := f.w.Answer(ctx, "req-1", "Deny"); !errors.Is(err, ErrAnswered) || len(f.m.sent) != 1 {
		t.Errorf("a second answer: %v, sent %d", err, len(f.m.sent))
	}
	if _, err := f.w.Answer(ctx, "req-9", "Deny"); !errors.Is(err, ErrAnswered) {
		t.Errorf("no such request: %v", err)
	}
}

// A review is decided on its page, a stuck agent by restarting it.
func TestOnlyAnAgentsRequestIsAnsweredHere(t *testing.T) {
	f := newFixture(t)
	for _, r := range []store.Request{{Kind: "review", Source: "agent", ReviewID: "rev-1"}, {Kind: "blocked", Source: "foreman"}} {
		saved, err := f.db.Requests().Save(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.w.Answer(ctx, saved.ID, "Merge"); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", r.Kind, err)
		}
	}
	if len(f.open()) != 2 || len(f.m.sent) != 0 {
		t.Error("answered anyway")
	}
}

// Once its team stops, a request has nobody to answer: it expires, and
// nothing is sent.
func TestARequestExpiresWithItsDeployment(t *testing.T) {
	f := newFixture(t)
	f.ok("mara", "escalate", map[string]any{"kind": "question", "title": "EUR or USD?"})
	f.ok("mara", "escalate", map[string]any{"kind": "approval", "title": "Add decimal.js?"})
	review, err := f.db.Requests().Save(ctx, store.Request{Kind: "review", Source: "agent", DeploymentID: "atlas@ledger", ReviewID: "rev-1"})
	if err != nil {
		t.Fatal(err)
	}
	f.teams.running["atlas@ledger"] = false
	// Answered before the stop was heard: it expires then.
	if _, err := f.w.Answer(ctx, "req-1", "EUR"); !errors.Is(err, ErrAnswered) {
		t.Errorf("answered a stopped team's request: %v", err)
	}
	f.w.DeploymentStopped(ctx, "atlas@ledger")
	open := f.open()
	if len(open) != 1 || open[0].ID != review.ID {
		t.Errorf("open %+v: only the review stays", open)
	}
	if len(f.m.sent) != 0 {
		t.Errorf("sent %+v", f.m.sent)
	}
}
