package workflow

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
)

// instance is a fixture agent's instance on atlas@ledger.
func (f *fixture) instance(agent, status string) store.Instance {
	return store.Instance{ID: agent + "@ledger", DeploymentID: "atlas@ledger", AgentID: agent, Status: status,
		SandboxPath: filepath.Join(f.root, "ledger", ".sandboxes", agent)}
}

// The lead reads where each teammate stands, from what Foreman knows: its
// status, when its harness last reported, and each worktree's branch with
// what it holds not committed. The lead itself, and other deployments'
// instances, aren't listed.
func TestTeamTellsTheLeadWhereItsTeammatesStand(t *testing.T) {
	f := newFixture(t)
	f.ok("tove", "switch", map[string]string{"branch": "wt/tove/led-44"})
	writeFile(t, filepath.Join(f.wt("tove", "api"), "draft.go"), "package api\n")
	tove := f.instance("tove", "running")
	tove.Doing = &store.Doing{Line: "Editing draft.go", At: f.w.Now().Add(-3 * time.Minute)}
	ivo := f.instance("ivo", "asleep") // no worktrees yet: its first start makes them
	ivo.Doing = &store.Doing{Line: "from its last run", At: f.w.Now().Add(-time.Hour)}
	other := f.instance("tove", "stopped")
	other.ID, other.DeploymentID = "tove@billing", "atlas@billing"
	f.teams.instances = []store.Instance{f.instance("mara", "running"), other, ivo, tove}

	text := f.ok("mara", "team", map[string]string{})
	want := "Your teammates now:\n\n" +
		"Tove, Engineer: running, last report 3m ago.\n" +
		"  api on wt/tove/led-44, uncommitted changes · web on wt/tove/led-44\n" +
		"Ivo, Engineer: asleep."
	if text != want {
		t.Errorf("got\n%s\nwant\n%s", text, want)
	}
	if strings.Contains(text, "Editing") {
		t.Error("the activity line crossed sandboxes")
	}
}

// A merge under way says so rather than passing for changes; a teammate
// Foreman stopped says why; one whose harness hasn't reported since a
// restart shows no time; a member added since the deploy has no instance.
func TestTeamStatesAndMerges(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.root, "ledger", "web", "README"), "trunk\n")
	f.git(filepath.Join(f.root, "ledger", "web"), "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "trunk moves")
	writeFile(t, filepath.Join(f.wt("tove", "web"), "README"), "tove\n")
	f.ok("tove", "commit", commits("web", "mine"))
	f.call("tove", "merge", map[string]string{"branch": "trunk"}) // conflicts in web
	f.teams.instances = []store.Instance{f.instance("tove", "idle")}
	ivo := f.instance("ivo", "stopped")
	ivo.StopReason = "it exited twice in a row (code 1)"
	f.teams.instances = append(f.teams.instances, ivo)

	want := "Your teammates now:\n\n" +
		"Tove, Engineer: idle.\n" +
		"  api on wt/tove/main · web on wt/tove/main, merge under way\n" +
		"Ivo, Engineer: stopped (it exited twice in a row (code 1))."
	if text := f.ok("mara", "team", map[string]string{}); text != want {
		t.Errorf("got\n%s\nwant\n%s", text, want)
	}

	f.teams.instances = f.teams.instances[:1]
	if text := f.ok("mara", "team", map[string]string{}); !strings.Contains(text, "Ivo, Engineer: not in this deployment (joined the team since it was deployed).") {
		t.Errorf("%s", text)
	}
}

func TestTeamRefusalsAndALeadAlone(t *testing.T) {
	f := newFixture(t)
	f.refused("tove", "team", map[string]string{}, "not_lead")

	solo := home.Team{ID: "solo", Config: home.TeamConfig{Team: "solo", Name: "Solo", Lead: "mara",
		Agents: map[string]home.Soul{"mara": {Name: "Mara", Role: "team_lead"}}}}
	c := teamtools.Caller{Instance: f.instance("mara", "running"), Team: solo, Lead: true,
		Deployment: store.Deployment{ID: "solo@ledger", TeamID: "solo", ProjectID: "ledger", Status: store.DeploymentActive}}
	if res := f.w.Call(ctx, c, "team", json.RawMessage(`{}`)); res.Error || res.Text != "You have no teammates." {
		t.Errorf("%+v", res)
	}
}

func TestAgo(t *testing.T) {
	for d, want := range map[time.Duration]string{
		-time.Second:                    "0s ago", // the clock went back
		0:                               "0s ago",
		59 * time.Second:                "59s ago",
		time.Minute:                     "1m ago",
		59*time.Minute + 59*time.Second: "59m ago",
		time.Hour:                       "1h ago",
		50 * time.Hour:                  "50h ago",
	} {
		if got := ago(d); got != want {
			t.Errorf("ago(%v) = %q, want %q", d, got, want)
		}
	}
}
