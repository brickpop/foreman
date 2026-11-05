package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
	"github.com/brickpop/foreman/internal/wiki"
)

var ctx = context.Background()

var atlas = home.Team{ID: "atlas", Config: home.TeamConfig{
	Team: "atlas", Name: "Atlas", Lead: "mara", Order: []string{"mara", "tove", "ivo"},
	Agents: map[string]home.Soul{
		"mara": {Name: "Mara", Role: "team_lead"},
		"tove": {Name: "Tove", Role: "engineer"},
		"ivo":  {Name: "Ivo", Role: "engineer"},
	},
}}

// config is the home: projects in root/<id>, each with its repos api and web
// and its project repo beside them, and the targets made. web's default
// branch is trunk, so every test also checks that each repo's own is used.
type config struct {
	root    string
	targets *targets
}

func (c config) Project(id string) (home.Project, error) {
	dir := filepath.Join(c.root, id)
	repos := []home.Repo{{Name: "api", Path: filepath.Join(dir, "api"), Default: "main"}, {Name: "web", Path: filepath.Join(dir, "web"), Default: "trunk"}}
	for i := range repos {
		repos[i].Targets = append([]home.Target{{Name: repos[i].Default, Default: true}}, c.targets.made[repos[i].Name]...)
	}
	return home.Project{ID: id, Path: filepath.Join(dir, "project"), Repos: repos}, nil
}

// targets fakes the home's: each repo's targets, made as branches at from.
// "taken" is a name the repo has already.
type targets struct {
	root string
	made map[string][]home.Target
}

func (tg *targets) CreateTarget(ctx context.Context, project, repo, name, from, by string) (home.Project, error) {
	if repo != "api" && repo != "web" {
		return home.Project{}, home.ErrNotFound
	}
	if name == "taken" {
		return home.Project{}, fmt.Errorf("%w: %s has a branch taken already", home.ErrExists, repo)
	}
	dir := filepath.Join(tg.root, project, repo)
	commit, err := git.New().Resolve(ctx, dir, from)
	if err != nil {
		return home.Project{}, fmt.Errorf("%w: %s has no branch or commit %q", home.ErrInvalid, repo, from)
	}
	if err := git.New().CreateBranch(ctx, dir, name, commit); err != nil {
		return home.Project{}, fmt.Errorf("%w: %s: %v", home.ErrInvalid, name, err)
	}
	tg.made[repo] = append(tg.made[repo], home.Target{Name: name, MadeBy: by, From: commit})
	return config{tg.root, tg}.Project(project)
}

func (c config) Sandbox(project, agent string) string {
	return filepath.Join(c.root, project, ".sandboxes", agent)
}

func (c config) Projects() []home.Project {
	p, _ := c.Project("ledger")
	return []home.Project{p}
}

// dismissedRef is a dismissed review's backup, as made at the fixture's time.
const dismissedRef = "refs/foreman/dismissed/2026-10-16T09-00-00Z/"

// entries is a wiki: path to title.
type entries map[string]string

func (e entries) Entry(_, path string) (wiki.Entry, error) {
	if title, ok := e[path]; ok {
		return wiki.Entry{Path: path, Title: title}, nil
	}
	return wiki.Entry{}, wiki.ErrNotFound
}

// messages records what Workflow sends.
type messages struct {
	mu   sync.Mutex
	sent []store.TeamMessage
	fail error // a message that can't be kept
}

func (m *messages) Send(_ context.Context, msg store.TeamMessage) (store.TeamMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return msg, m.fail
	}
	m.sent = append(m.sent, msg)
	msg.Ref = "msg-" + string(rune('0'+len(m.sent)))
	return msg, nil
}

// teams fakes the instance manager: the deployments running, those stopped,
// stopping failing with fail, and the instances List returns.
type teams struct {
	running   map[string]bool
	instances []store.Instance
	stopped   []string
	fail      error
}

func (t *teams) Active(_ context.Context, deployment string) (bool, error) {
	return t.running[deployment], nil
}

func (t *teams) List(context.Context) ([]store.Instance, error) { return t.instances, nil }

func (t *teams) Stop(_ context.Context, deployment string) error {
	if t.fail != nil {
		return t.fail
	}
	t.stopped = append(t.stopped, deployment)
	t.running[deployment] = false
	return nil
}

type fixture struct {
	t       *testing.T
	root    string
	targets *targets
	w       *Workflow
	m       *messages
	teams   *teams
	db      *store.DB
	events  []events.Event
}

// newFixture: project ledger with code repos api and web and the project
// repo; mara (the lead) and tove deployed, each with a worktree per repo on
// its home branch, as a deploy makes them.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	g := git.New()
	g.FetchProtocols = []string{"file"} // the tests' submodules are local repos
	for _, r := range []string{"api", "web", "project"} {
		dir := filepath.Join(root, "ledger", r)
		if err := os.MkdirAll(filepath.Join(dir, "wiki"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := g.Init(ctx, dir, mainOf(r)); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "README"), r+"\n")
		if err := g.Commit(ctx, dir, "first", "README"); err != nil {
			t.Fatal(err)
		}
	}
	for agent, branch := range map[string]string{"mara": "lead/ledger/atlas/integration", "tove": "wt/ledger/tove/main"} {
		for _, r := range []string{"api", "web"} {
			if err := g.Prepare(ctx, filepath.Join(root, "ledger", r), filepath.Join(root, "ledger", ".sandboxes", agent, r), branch, mainOf(r)); err != nil {
				t.Fatal(err)
			}
		}
	}
	m := &messages{}
	now := func() time.Time { return time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC) }
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Deployments().Deploy(ctx, "atlas", "ledger"); err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, root: root, m: m, db: db, teams: &teams{running: map[string]bool{"atlas@ledger": true}}}
	f.targets = &targets{root: root, made: map[string][]home.Target{}}
	f.w = New(Deps{Git: g, Config: config{root, f.targets}, Targets: f.targets, Messages: m, Teams: f.teams, Now: now, WorkItems: db.WorkItems(),
		MergeRequests: db.MergeRequests(), Wiki: entries{"backlog/led-44.md": "Export invoices"}, Actions: db.Actions(), Requests: db.Requests(),
		Reviews: db.Reviews(), ReviewComments: db.ReviewComments(),
		Publish: func(e events.Event) { f.events = append(f.events, e) }})
	return f
}

// mainOf is a fixture repo's main branch.
func mainOf(repo string) string {
	if repo == "web" {
		return "trunk"
	}
	return "main"
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) wt(agent, repo string) string {
	return filepath.Join(f.root, "ledger", ".sandboxes", agent, repo)
}

func (f *fixture) git(dir string, args ...string) string {
	f.t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *fixture) call(agent, tool string, args any) agentlink.Result {
	f.t.Helper()
	raw, _ := json.Marshal(args)
	c := teamtools.Caller{
		Instance:   store.Instance{ID: agent + "@ledger", AgentID: agent, SandboxPath: filepath.Join(f.root, "ledger", ".sandboxes", agent)},
		Deployment: store.Deployment{ID: "atlas@ledger", TeamID: "atlas", ProjectID: "ledger", Status: store.DeploymentActive},
		Team:       atlas, Lead: agent == "mara",
	}
	return f.w.Call(ctx, c, tool, raw)
}

// commits are commit's arguments: repo, message, repo, message…
func commits(pairs ...string) map[string]any {
	var list []map[string]string
	for i := 0; i+1 < len(pairs); i += 2 {
		list = append(list, map[string]string{"repo": pairs[i], "message": pairs[i+1]})
	}
	return map[string]any{"commits": list}
}

func (f *fixture) ok(agent, tool string, args any) string {
	f.t.Helper()
	res := f.call(agent, tool, args)
	if res.Error {
		f.t.Fatalf("%s %s %v: %s", agent, tool, args, res.Text)
	}
	return res.Text
}

func (f *fixture) refused(agent, tool string, args any, code string) string {
	f.t.Helper()
	res := f.call(agent, tool, args)
	if !res.Error || !strings.HasPrefix(res.Text, code+":") {
		f.t.Errorf("%s %s %v: want %s, got %+v", agent, tool, args, code, res)
	}
	return res.Text
}

func TestSwitchInEveryRepo(t *testing.T) {
	f := newFixture(t)
	text := f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/LED-44"})
	if text != "Switched api, web to wt/ledger/tove/LED-44. New in api (from main), web (from trunk)." {
		t.Errorf("%q", text)
	}
	for _, r := range []string{"api", "web"} {
		if b := f.git(f.wt("tove", r), "branch", "--show-current"); b != "wt/ledger/tove/LED-44" {
			t.Errorf("%s on %q", r, b)
		}
	}
	if text := f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/main"}); strings.Contains(text, "New") {
		t.Errorf("back home: %q", text)
	}
	if text := f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/settlement"}); !strings.Contains(text, "New in api (from main), web (from trunk)") {
		t.Errorf("lead: %q", text)
	}
}

func TestSwitchRefusals(t *testing.T) {
	f := newFixture(t)
	f.refused("tove", "switch", map[string]string{"branch": "wt/ledger/mara/x"}, "branch_not_allowed")
	f.refused("tove", "switch", map[string]string{"branch": "lead/ledger/atlas/x"}, "branch_not_allowed")
	f.refused("tove", "switch", map[string]string{"branch": "main"}, "branch_not_allowed")
	f.refused("tove", "switch", map[string]string{"branch": "wt/ledger/tove/"}, "bad_branch")
	f.refused("tove", "switch", map[string]string{"branch": "wt/ledger/tove/a b"}, "bad_branch")
	f.refused("mara", "switch", map[string]string{"branch": "wt/ledger/mara/x"}, "branch_not_allowed")
	f.refused("mara", "switch", map[string]string{"branch": "lead/ledger/other/x"}, "branch_not_allowed")
	// Another project's branches, and the names from before branches carried one.
	f.refused("tove", "switch", map[string]string{"branch": "wt/suite/tove/x"}, "branch_not_allowed")
	f.refused("mara", "switch", map[string]string{"branch": "lead/suite/atlas/x"}, "branch_not_allowed")
	f.refused("tove", "switch", map[string]string{"branch": "wt/tove/x"}, "branch_not_allowed")
	f.refused("mara", "switch", map[string]string{"branch": "lead/atlas/x"}, "branch_not_allowed")
	// Changes in one repo stop the switch in all of them: it never stops halfway.
	writeFile(t, filepath.Join(f.wt("tove", "web"), "draft"), "x\n")
	if text := f.refused("tove", "switch", map[string]string{"branch": "wt/ledger/tove/x"}, "uncommitted_changes"); !strings.Contains(text, "web") || strings.Contains(text, "api") {
		t.Errorf("%q", text)
	}
	if b := f.git(f.wt("tove", "api"), "branch", "--show-current"); b != "wt/ledger/tove/main" {
		t.Errorf("api switched anyway, to %q", b)
	}
}

// Each repo named is committed with its own message, as the agent; the rest
// are left alone, changes and all.
func TestCommitTheReposNamedAsTheAgent(t *testing.T) {
	f := newFixture(t)
	f.refused("tove", "commit", commits("api", "x"), "nothing_to_commit")
	f.refused("tove", "commit", commits("api", " "), "empty")
	f.refused("tove", "commit", commits(), "empty")
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal\n")
	writeFile(t, filepath.Join(f.wt("tove", "web"), "draft.ts"), "x\n")
	text := f.ok("tove", "commit", commits("api", "cal v0"))
	if !strings.HasPrefix(text, "Committed api (wt/ledger/tove/main, ") || strings.Contains(text, "web") {
		t.Errorf("%q", text)
	}
	if who := f.git(f.wt("tove", "api"), "log", "-1", "--format=%an <%ae>|%s"); who != "Tove <tove@foreman.invalid>|cal v0" {
		t.Errorf("authored as %q", who)
	}
	if status := f.git(f.wt("tove", "web"), "status", "--porcelain"); status != "?? draft.ts" {
		t.Errorf("web, not named, was touched: %q", status)
	}
	// Nothing reached main or the CEO's checkout.
	if f.git(filepath.Join(f.root, "ledger", "api"), "rev-parse", "main") == f.git(f.wt("tove", "api"), "rev-parse", "HEAD") {
		t.Error("main moved")
	}
	// Two repos, two messages; one with nothing new is said, not refused.
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal // v1\n")
	text = f.ok("tove", "commit", commits("api", "cal v1", "web", "the draft"))
	if !strings.Contains(text, "api (wt/ledger/tove/main, ") || !strings.Contains(text, "web (wt/ledger/tove/main, ") {
		t.Errorf("%q", text)
	}
	if api, web := f.git(f.wt("tove", "api"), "log", "-1", "--format=%s"), f.git(f.wt("tove", "web"), "log", "-1", "--format=%s"); api != "cal v1" || web != "the draft" {
		t.Errorf("messages %q, %q", api, web)
	}
	writeFile(t, filepath.Join(f.wt("tove", "web"), "draft.ts"), "y\n")
	if text := f.ok("tove", "commit", commits("api", "nothing", "web", "y")); !strings.HasSuffix(text, "Nothing changed in api.") {
		t.Errorf("%q", text)
	}
}

// A commit carries its message alone: the attribution a harness adds
// (Co-Authored-By lines, Claude Code's "Generated with") goes, in any case and
// wherever it is; a message of nothing else is none.
func TestCommitDropsAttribution(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal\n")
	f.refused("tove", "commit", commits("api", "Co-Authored-By: Claude <noreply@anthropic.com>"), "empty")
	f.refused("tove", "commit", commits("api", "\n\nco-authored-by: Claude\n"), "empty")
	f.ok("tove", "commit", commits("api", "cal v0\n\nThe calendar.\n  co-authored-by: A <a@x>\nSigned-off-by: Tove\nCO-AUTHORED-BY: B <b@x>\n\n"))
	if msg := f.git(f.wt("tove", "api"), "log", "-1", "--format=%B"); msg != "cal v0\n\nThe calendar.\nSigned-off-by: Tove" {
		t.Errorf("message %q", msg)
	}
	// Only a line that starts with it: one that mentions it stays.
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal // v1\n")
	f.ok("tove", "commit", commits("api", "Explain the Co-Authored-By: trailer"))
	if msg := f.git(f.wt("tove", "api"), "log", "-1", "--format=%B"); msg != "Explain the Co-Authored-By: trailer" {
		t.Errorf("message %q", msg)
	}
	// Claude Code's whole block, its "Generated with" line with or without the
	// robot; a message of only that is none either.
	f.refused("tove", "commit", commits("api", "🤖 Generated with [Claude Code](https://claude.com/claude-code)"), "empty")
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal // v2\n")
	f.ok("tove", "commit", commits("api", "cal v2\n\n🤖 Generated with [Claude Code](https://claude.com/claude-code)\n\nCo-Authored-By: Claude <noreply@anthropic.com>\nGenerated with [Claude Code]\n"))
	if msg := f.git(f.wt("tove", "api"), "log", "-1", "--format=%B"); msg != "cal v2" {
		t.Errorf("message %q", msg)
	}
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal // v3\n")
	f.ok("tove", "commit", commits("api", "Generated with care"))
	if msg := f.git(f.wt("tove", "api"), "log", "-1", "--format=%B"); msg != "Generated with care" {
		t.Errorf("message %q", msg)
	}
}

// Every entry is checked before anything is committed.
func TestACommitNamingABadRepoCommitsNothing(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal\n")
	for _, c := range []struct {
		args map[string]any
		code string
	}{
		{commits("api", "cal", "nope", "x"), "unknown_repo"},
		{commits("api", "cal", "project", "x"), "unknown_repo"}, // a member's project repo is read-only
		{commits("api", "cal", "../api", "x"), "unknown_repo"},
		{commits("api", "cal", "api", "again"), "repeated_repo"},
		{commits("api", "cal", "web", ""), "empty"},
	} {
		f.refused("tove", "commit", c.args, c.code)
	}
	if text := f.refused("tove", "commit", commits("nope", "x"), "unknown_repo"); !strings.Contains(text, "yours are api, web") {
		t.Errorf("%q", text)
	}
	if status := f.git(f.wt("tove", "api"), "status", "--porcelain"); status != "?? cal.go" {
		t.Errorf("api committed anyway: %q", status)
	}
}

// The lead commits the live project repo on main when it names it, and only
// then: its code worktrees keep their changes.
func TestTheLeadCommitsTheProjectRepoByName(t *testing.T) {
	f := newFixture(t)
	project := filepath.Join(f.root, "ledger", "project")
	writeFile(t, filepath.Join(project, "wiki", "board.md"), "# Board\n")
	writeFile(t, filepath.Join(f.wt("mara", "api"), "half.go"), "package half\n")
	text := f.ok("mara", "commit", commits("project", "the board"))
	if text != "Committed project (main, "+f.git(project, "rev-parse", "HEAD")[:8]+")." {
		t.Errorf("%q", text)
	}
	if who := f.git(project, "log", "-1", "--format=%an|%s"); who != "Mara|the board" {
		t.Errorf("project repo HEAD: %q", who)
	}
	if status := f.git(f.wt("mara", "api"), "status", "--porcelain"); status != "?? half.go" {
		t.Errorf("api, not named, was committed: %q", status)
	}
}

func TestMergeATeammatesBranch(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal\n")
	f.ok("tove", "commit", commits("api", "cal"))
	f.refused("mara", "merge", map[string]string{"branch": "wt/ledger/tove/nope"}, "branch_not_found")
	f.refused("mara", "merge", map[string]string{"branch": "-x"}, "bad_branch")
	// A tag of the same name, on older work, is never what gets merged.
	f.git(filepath.Join(f.root, "ledger", "api"), "tag", "wt/ledger/tove/main", "main")
	if text := f.ok("mara", "merge", map[string]string{"branch": "wt/ledger/tove/main"}); text != "Merged wt/ledger/tove/main into api, web." {
		t.Errorf("%q", text)
	}
	if _, err := os.Stat(filepath.Join(f.wt("mara", "api"), "cal.go")); err != nil {
		t.Error("tove's work didn't arrive")
	}
	writeFile(t, filepath.Join(f.wt("mara", "web"), "wip"), "x\n")
	f.refused("mara", "merge", map[string]string{"branch": "trunk"}, "uncommitted_changes")
}

func TestMergeWithConflictsThenCommit(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("tove", "api"), "README"), "tove\n")
	f.ok("tove", "commit", commits("api", "tove's readme"))
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "mara\n")
	f.ok("mara", "commit", commits("api", "mara's readme"))
	text := f.ok("mara", "merge", map[string]string{"branch": "wt/ledger/tove/main"})
	if text != "Merged wt/ledger/tove/main into web. Conflicts in api (README): resolve the markers (a submodule: pick its commit with set_submodule), then commit those repos to finish the merge, or abort_merge() to back out." {
		t.Errorf("%q", text)
	}
	if text := f.refused("mara", "commit", commits("api", "merged"), "conflicts_left"); !strings.Contains(text, "api/README") {
		t.Errorf("%q", text)
	}
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "both\n")
	if text := f.ok("mara", "commit", commits("api", "merged")); !strings.Contains(text, "api (lead/ledger/atlas/integration") {
		t.Errorf("%q", text)
	}
}

func TestHandoffNamesACommittedBranch(t *testing.T) {
	f := newFixture(t)
	f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/LED-44"})
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/LED-44", "summary": "done"}, "nothing_new")
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal\n")
	// Pending work is refused, never committed for the agent: commit() is the one way.
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/LED-44", "summary": "done"}, "uncommitted_changes")
	if len(f.m.sent) != 0 {
		t.Fatalf("a refusal sent %v", f.m.sent)
	}
	f.ok("tove", "commit", commits("api", "cal v0"))
	text := f.ok("tove", "handoff", map[string]string{"teammate": "Mara", "branch": "wt/ledger/tove/LED-44", "summary": "cal v0: all 6 samples match"})
	if text != "msg-1: wt/ledger/tove/LED-44 handed to Mara. Your part ends here." {
		t.Errorf("%q", text)
	}
	if subject := f.git(f.wt("tove", "api"), "log", "-1", "--format=%s"); subject != "cal v0" {
		t.Errorf("handoff committed something: %q", subject)
	}
	if len(f.m.sent) != 1 {
		t.Fatalf("sent %v", f.m.sent)
	}
	msg := f.m.sent[0]
	if msg.Kind != "handoff" || msg.ToInstance != "mara@ledger" || msg.FromInstance != "tove@ledger" || msg.ArtifactRef != "wt/ledger/tove/LED-44" || msg.Body != "cal v0: all 6 samples match" {
		t.Errorf("message %+v", msg)
	}
}

func TestHandoffRefusals(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("tove", "api"), "draft"), "x\n")
	f.refused("tove", "handoff", map[string]string{"teammate": "rhea", "branch": "wt/ledger/tove/main", "summary": "x"}, "unknown_teammate")
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/ivo/main", "summary": "x"}, "branch_not_allowed")
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "main", "summary": "x"}, "branch_not_allowed")
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/main", "summary": ""}, "empty")
	// No refusal commits anything.
	if status := f.git(f.wt("tove", "api"), "status", "--porcelain"); !strings.Contains(status, "draft") {
		t.Errorf("a refused hand-off committed: %q", status)
	}
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/main", "summary": "x"}, "uncommitted_changes")
	os.Remove(filepath.Join(f.wt("tove", "api"), "draft"))
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/never", "summary": "x"}, "branch_not_found")
	if len(f.m.sent) != 0 {
		t.Errorf("a refusal sent %v", f.m.sent)
	}
}

func TestContributeFilesANoteAndTellsTheLead(t *testing.T) {
	f := newFixture(t)
	project := filepath.Join(f.root, "ledger", "project")
	// The lead's unfinished edit in the same checkout stays out of the commit.
	writeFile(t, filepath.Join(project, "wiki", "board.md"), "unfinished\n")
	text := f.ok("tove", "contribute", map[string]string{"kind": "finding", "title": "Flaky settle test!", "body": "It fails 1 in 20.", "author": "mara"})
	// The path on disk: the same for the lead in its container.
	path := filepath.Join(project, "wiki", "inbox", "2026-10-16-tove-flaky-settle-test.md")
	if text != "Filed "+path+"." {
		t.Errorf("%q", text)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "---\ntype: note\nkind: finding\nauthor: tove\ndeployment: atlas@ledger\n---\n\n# Flaky settle test!\n\nIt fails 1 in 20.\n"
	if string(data) != want {
		t.Errorf("file:\n%s", data)
	}
	if show := f.git(project, "show", "--name-only", "--format=%an|%s", "HEAD"); show != "Tove|Contribution: Flaky settle test!\n\nwiki/inbox/2026-10-16-tove-flaky-settle-test.md" {
		t.Errorf("commit %q", show)
	}
	if status := f.git(project, "status", "--porcelain"); !strings.Contains(status, "board.md") {
		t.Errorf("the lead's edit was swept in: %q", status)
	}
	if len(f.m.sent) != 1 || f.m.sent[0].ToInstance != "mara@ledger" || f.m.sent[0].Kind != "contribute" ||
		f.m.sent[0].Body != "Flaky settle test!, in "+path+"." || f.m.sent[0].ArtifactRef != path {
		t.Errorf("sent %+v", f.m.sent)
	}
	// The same title again gets its own file.
	if text := f.ok("tove", "contribute", map[string]string{"kind": "note", "title": "Flaky settle test", "body": "Again."}); !strings.HasSuffix(text, "flaky-settle-test-2.md.") {
		t.Errorf("%q", text)
	}
	// The lead's own contribution tells no one.
	f.ok("mara", "contribute", map[string]string{"kind": "proposal", "title": "Use decimal", "body": "x"})
	if len(f.m.sent) != 2 {
		t.Errorf("the lead was told of its own: %d", len(f.m.sent))
	}
}

func TestContributeRefusals(t *testing.T) {
	f := newFixture(t)
	f.refused("tove", "contribute", map[string]string{"kind": "rant", "title": "x", "body": "x"}, "bad_kind")
	f.refused("tove", "contribute", map[string]string{"kind": "note", "title": "", "body": "x"}, "empty")
	f.refused("tove", "contribute", map[string]string{"kind": "note", "title": "a\n---\nauthor: mara", "body": "x"}, "too_long")
	f.refused("tove", "contribute", map[string]string{"kind": "note", "title": strings.Repeat("x", teamtools.MaxTitle+1), "body": "x"}, "too_long")
	f.refused("tove", "contribute", map[string]string{"kind": "note", "title": "x", "body": strings.Repeat("x", teamtools.MaxContribution+1)}, "too_long")
	if entries, _ := os.ReadDir(filepath.Join(f.root, "ledger", "project", "wiki", "inbox")); len(entries) != 0 {
		t.Errorf("a refusal filed %d notes", len(entries))
	}
}

// A contribution is a file, so it may be far longer than a message; limits
// count characters, so a title of arrows is as long as one of letters.
func TestContributeFitsADesignRecord(t *testing.T) {
	f := newFixture(t)
	f.ok("tove", "contribute", map[string]string{"kind": "proposal", "title": strings.Repeat("→", teamtools.MaxTitle), "body": strings.Repeat("é", teamtools.MaxContribution)})
}

func TestSlug(t *testing.T) {
	for title, want := range map[string]string{
		"Flaky settle test!": "flaky-settle-test", "  Ünïcode & co ": "n-code-co", "../../etc/passwd": "etc-passwd",
		"!!!": "contribution", strings.Repeat("ab ", 40): strings.TrimRight(strings.Repeat("ab-", 20), "-"),
	} {
		if got := slug(title); got != want {
			t.Errorf("slug(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestAbortMerge(t *testing.T) {
	f := newFixture(t)
	f.refused("mara", "abort_merge", nil, "no_merge")
	writeFile(t, filepath.Join(f.wt("tove", "api"), "README"), "tove\n")
	f.ok("tove", "commit", commits("api", "tove's readme"))
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "mara\n")
	f.ok("mara", "commit", commits("api", "mara's readme"))
	f.ok("mara", "merge", map[string]string{"branch": "wt/ledger/tove/main"})
	if text := f.ok("mara", "abort_merge", nil); text != "Merge aborted in api: your branch is as it was before it." {
		t.Errorf("%q", text)
	}
	if status := f.git(f.wt("mara", "api"), "status", "--porcelain"); status != "" {
		t.Errorf("not clean: %q", status)
	}
	// web merged cleanly and keeps it: only an unfinished merge is backed out.
	if _, err := os.Stat(filepath.Join(f.wt("mara", "web"), "README")); err != nil {
		t.Error(err)
	}
	f.refused("mara", "abort_merge", nil, "no_merge")
}

// An agent drops its uncommitted work without editing it out by hand; Foreman
// keeps a copy.
func TestDiscard(t *testing.T) {
	f := newFixture(t)
	api, web := f.wt("tove", "api"), f.wt("tove", "web")
	if text := f.ok("tove", "discard", nil); text != "Nothing to discard in api, web: it's as committed." {
		t.Errorf("%q", text)
	}
	writeFile(t, filepath.Join(api, "README"), "rewritten\n")
	writeFile(t, filepath.Join(api, "scratch.go"), "package scratch\n")
	writeFile(t, filepath.Join(web, "keep.txt"), "web's, kept\n")
	f.refused("tove", "discard", map[string]any{"repos": []string{"docs"}}, "unknown_repo")
	f.refused("tove", "discard", map[string]any{"repos": []string{"api", "api"}}, "repeated_repo")
	// Only the repo named.
	if text := f.ok("tove", "discard", map[string]any{"repos": []string{"api"}}); text != "Discarded what wasn't committed in api: back to the branch as committed. Foreman kept a copy." {
		t.Errorf("%q", text)
	}
	if status := f.git(api, "status", "--porcelain"); status != "" {
		t.Errorf("api not clean: %q", status)
	}
	if _, err := os.Stat(filepath.Join(web, "keep.txt")); err != nil {
		t.Error("web was discarded too")
	}
	saved := "refs/foreman/discarded/2026-10-16T09-00-00Z/tove/api"
	if f.git(api, "show", saved+":README") != "rewritten" || f.git(api, "show", saved+":scratch.go") != "package scratch" {
		t.Error("no copy kept")
	}
	// A merge under way: abort it first, nothing discarded.
	writeFile(t, filepath.Join(f.wt("mara", "web"), "README"), "mara\n")
	f.ok("mara", "commit", commits("web", "mara's readme"))
	f.git(web, "checkout", "--", ".")
	writeFile(t, filepath.Join(web, "README"), "tove\n")
	f.ok("tove", "commit", commits("web", "tove's readme"))
	f.call("tove", "merge", map[string]string{"branch": "lead/ledger/atlas/integration"})
	writeFile(t, filepath.Join(api, "more.go"), "package more\n")
	f.refused("tove", "discard", nil, "merge_under_way")
	if _, err := os.Stat(filepath.Join(api, "more.go")); err != nil {
		t.Error("discarded api despite the merge in web")
	}
}

// An agent starting fresh hears what its worktrees hold from before, once;
// nothing when they hold nothing.
func TestAResumedAgentIsToldWhatItLeft(t *testing.T) {
	f := newFixture(t)
	instance := func(agent string) store.Instance {
		return store.Instance{ID: agent + "@ledger", DeploymentID: "atlas@ledger", AgentID: agent,
			SandboxPath: filepath.Join(f.root, "ledger", ".sandboxes", agent)}
	}
	sent := len(f.m.sent)
	f.w.Resumed(ctx, instance("tove"), false)
	if len(f.m.sent) != sent {
		t.Fatalf("told of nothing: %+v", f.m.sent[sent:])
	}
	f.integrated()
	writeFile(t, filepath.Join(f.wt("tove", "web"), "draft.txt"), "half done\n")
	f.w.Resumed(ctx, instance("tove"), false)
	if n := f.lastNotice(); n.ToInstance != "tove@ledger" || n.Body != "From Foreman: your worktrees hold earlier work. "+
		"Branches with work no target has: wt/ledger/tove/LED-44 (api). Changes not committed in web (on wt/ledger/tove/LED-44). "+
		"Resume what is still wanted; new work goes on a new topic, made from the default branch with switch(<topic>), or from another target with switch(<topic>, from). Changes you don't want: discard()." {
		t.Errorf("tove: %+v", n)
	}
	// The lead also hears of its reviews still open.
	f.ok("mara", "request_merge", integration)
	f.w.Resumed(ctx, instance("mara"), true)
	if n := f.lastNotice(); n.ToInstance != "mara@ledger" || n.Body != "From Foreman: your worktrees hold earlier work. "+
		"Branches with work no target has: lead/ledger/atlas/integration (api). "+
		"Your reviews still open: lead/ledger/atlas/integration (waiting for the CEO). "+
		"Resume what is still wanted; new work goes on a new topic, made from the default branch with switch(<topic>), or from another target with switch(<topic>, from)." {
		t.Errorf("mara: %+v", n)
	}
}

// actions records what Workflow keeps.
type actions struct{ kept []store.TeamAction }

func (a *actions) Add(_ context.Context, x store.TeamAction) (store.TeamAction, error) {
	a.kept = append(a.kept, x)
	return x, nil
}
func (a *actions) List(context.Context, string) ([]store.TeamAction, error) { return a.kept, nil }

// Each git tool call is kept with what it did in each repo, refusals
// included; a hand-off is a message, not an action.
func TestGitToolsAreKeptAsActions(t *testing.T) {
	f := newFixture(t)
	kept := &actions{}
	f.w.Actions = kept
	f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/LED-44"})
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal\n")
	f.ok("tove", "commit", commits("api", "cal v0"))
	f.refused("tove", "switch", map[string]string{"branch": "main"}, "branch_not_allowed")
	f.ok("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/LED-44", "summary": "done"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "mara\n")
	f.ok("mara", "commit", commits("api", "mara's readme"))
	writeFile(t, filepath.Join(f.wt("tove", "api"), "README"), "tove\n")
	f.ok("tove", "commit", commits("api", "tove's readme"))
	f.ok("mara", "merge", map[string]string{"branch": "wt/ledger/tove/LED-44"})
	f.ok("mara", "abort_merge", nil)

	var tools []string
	for _, a := range kept.kept {
		tools = append(tools, a.Tool)
	}
	if strings.Join(tools, " ") != "switch commit switch commit commit merge abort_merge" {
		t.Fatalf("kept %v", tools)
	}
	sw, commit, refused, merge, abort := kept.kept[0], kept.kept[1], kept.kept[2], kept.kept[5], kept.kept[6]
	if sw.Instance != "tove@ledger" || sw.DeploymentID != "atlas@ledger" || sw.Branch != "wt/ledger/tove/LED-44" || len(sw.Repos) != 2 || !sw.Repos[0].Created {
		t.Errorf("switch %+v", sw)
	}
	if len(commit.Repos) != 1 || commit.Repos[0].Message != "cal v0" || commit.Repos[0].Repo != "api" || commit.Repos[0].Branch != "wt/ledger/tove/LED-44" || len(commit.Repos[0].Commit) != 40 {
		t.Errorf("commit %+v", commit)
	}
	if !strings.HasPrefix(refused.Error, "branch_not_allowed:") || len(refused.Repos) != 0 {
		t.Errorf("refused %+v", refused)
	}
	if merge.Branch != "wt/ledger/tove/LED-44" || merge.Error != "" || len(merge.Repos) != 2 {
		t.Fatalf("merge %+v", merge)
	}
	for _, r := range merge.Repos {
		if r.Branch != "lead/ledger/atlas/integration" || (r.Repo == "api") != (len(r.Conflicts) == 1) {
			t.Errorf("merge in %+v", r)
		}
	}
	if len(abort.Repos) != 1 || abort.Repos[0].Repo != "api" {
		t.Errorf("abort %+v", abort)
	}
}

// item is the work item of a branch, as stored.
func (f *fixture) item(branch string) store.WorkItem {
	f.t.Helper()
	w, err := f.db.WorkItems().Find(ctx, "atlas@ledger", branch)
	if err != nil {
		f.t.Fatalf("work item of %s: %v", branch, err)
	}
	return w
}

// handOff commits a file on tove's branch, in the repos given, and hands it to mara.
func (f *fixture) handOff(branch string, repos ...string) {
	f.t.Helper()
	f.ok("tove", "switch", map[string]string{"branch": branch})
	var each []string
	for _, r := range repos {
		writeFile(f.t, filepath.Join(f.wt("tove", r), branch[len("wt/ledger/tove/"):]+".go"), fmt.Sprintf("package x // %d\n", len(f.m.sent)))
		each = append(each, r, "work on "+branch)
	}
	f.ok("tove", "commit", commits(each...))
	f.ok("tove", "handoff", map[string]string{"teammate": "mara", "branch": branch, "summary": "done"})
}

func TestWorkItemsFollowHandoffDeliveryAndIntegration(t *testing.T) {
	f := newFixture(t)
	f.handOff("wt/ledger/tove/LED-44", "api", "web")
	w := f.item("wt/ledger/tove/LED-44")
	if w.ID != "wi-1" || w.Stage != store.StageHanded || w.AuthorInstance != "tove@ledger" {
		t.Fatalf("after the hand-off: %+v", w)
	}
	if len(f.events) == 0 || f.events[len(f.events)-1].Type != "pipeline.changed" {
		t.Errorf("no pipeline.changed: %v", f.events)
	}

	// Delivered into mara's session: she's reviewing it. Other deliveries move nothing.
	delivered := store.TeamMessage{DeploymentID: "atlas@ledger", Kind: "handoff", ArtifactRef: "wt/ledger/tove/LED-44", State: store.MessageDelivered}
	f.w.Delivered(ctx, store.TeamMessage{DeploymentID: "atlas@ledger", Kind: "handoff", ArtifactRef: "wt/ledger/tove/LED-44", State: store.MessageQueued})
	f.w.Delivered(ctx, store.TeamMessage{DeploymentID: "atlas@ledger", Kind: "ask", ArtifactRef: "wt/ledger/tove/LED-44", State: store.MessageDelivered})
	if f.item("wt/ledger/tove/LED-44").Stage != store.StageHanded {
		t.Error("a queued hand-off, or an ask, moved the work item")
	}
	f.w.Delivered(ctx, delivered)
	if f.item("wt/ledger/tove/LED-44").Stage != store.StageReviewing {
		t.Error("the delivery didn't move it to reviewing")
	}

	// Tove's own merges never integrate: only the lead's.
	f.ok("tove", "merge", map[string]string{"branch": "main"})
	if f.item("wt/ledger/tove/LED-44").Stage != store.StageReviewing {
		t.Error("a member's merge integrated it")
	}
	// Merged into the lead branch in one repo only: not yet.
	f.git(f.wt("mara", "api"), "-c", "user.name=t", "-c", "user.email=t@t", "merge", "-q", "--no-ff", "-m", "api only", "wt/ledger/tove/LED-44")
	f.ok("mara", "merge", map[string]string{"branch": "main"}) // any merge of hers checks
	if f.item("wt/ledger/tove/LED-44").Stage != store.StageReviewing {
		t.Error("integrated with web still missing")
	}
	f.ok("mara", "merge", map[string]string{"branch": "wt/ledger/tove/LED-44"})
	if got := f.item("wt/ledger/tove/LED-44"); got.Stage != store.StageIntegrated {
		t.Errorf("after the lead's merge: %+v", got)
	}

	// A new hand-off of the same branch (after a change request) brings it back.
	f.handOff("wt/ledger/tove/LED-44", "api")
	if got := f.item("wt/ledger/tove/LED-44"); got.ID != "wi-1" || got.Stage != store.StageHanded {
		t.Errorf("handed again: %+v", got)
	}
}

func TestIntegrationIsPerBranchAndInAnyLeadBranch(t *testing.T) {
	f := newFixture(t)
	f.handOff("wt/ledger/tove/a", "api")
	f.handOff("wt/ledger/tove/b", "web")
	// Mara merges b into another lead branch of hers: b is integrated, a isn't.
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/settlement"})
	f.ok("mara", "merge", map[string]string{"branch": "wt/ledger/tove/b"})
	if a, b := f.item("wt/ledger/tove/a"), f.item("wt/ledger/tove/b"); a.Stage != store.StageHanded || b.Stage != store.StageIntegrated {
		t.Errorf("a %+v, b %+v", a, b)
	}
	// The pipeline lists the project's work items.
	p, err := f.w.Pipeline(ctx, "ledger")
	if err != nil || len(p.WorkItems) != 2 || p.MergeRequests == nil {
		t.Errorf("pipeline %+v %v", p, err)
	}
}

// lastNotice is the latest notice Foreman sent, and to whom.
func (f *fixture) lastNotice() store.TeamMessage {
	f.t.Helper()
	for i := len(f.m.sent) - 1; i >= 0; i-- {
		if f.m.sent[i].Kind == "notice" {
			return f.m.sent[i]
		}
	}
	f.t.Fatal("no notice sent")
	return store.TeamMessage{}
}

// integrated: tove's wt/ledger/tove/LED-44 (api only) handed off and merged by mara.
func (f *fixture) integrated() {
	f.t.Helper()
	f.handOff("wt/ledger/tove/LED-44", "api")
	f.ok("mara", "merge", map[string]string{"branch": "wt/ledger/tove/LED-44"})
}

func TestRequestMergeRefusals(t *testing.T) {
	f := newFixture(t)
	req := func(branch, entry string) map[string]string {
		return map[string]string{"branch": branch, "title": "Invoices as CSV", "entry": entry}
	}
	f.refused("tove", "request_merge", req("wt/ledger/tove/main", "backlog/led-44.md"), "not_lead")
	f.refused("mara", "request_merge", req("wt/ledger/tove/main", "backlog/led-44.md"), "branch_not_allowed")
	f.refused("mara", "request_merge", req("lead/ledger/orion/x", "backlog/led-44.md"), "branch_not_allowed")
	f.refused("mara", "request_merge", req("lead/ledger/atlas/integration", ""), "entry_not_found")
	// A title is one short line; the rest is the body's.
	titled := func(title, body string) map[string]string {
		return map[string]string{"branch": "lead/ledger/atlas/integration", "title": title, "body": body, "entry": "backlog/led-44.md"}
	}
	f.refused("mara", "request_merge", titled("  ", ""), "empty")
	f.refused("mara", "request_merge", titled("Invoices\nas CSV", ""), "too_long")
	f.refused("mara", "request_merge", titled(strings.Repeat("é", teamtools.MaxTitle+1), ""), "too_long")
	f.refused("mara", "request_merge", titled("Invoices as CSV", strings.Repeat("x", teamtools.MaxBody+1)), "too_long")
	f.refused("mara", "request_merge", req("lead/ledger/atlas/integration", "backlog/nope.md"), "entry_not_found")
	f.refused("mara", "request_merge", req("lead/ledger/atlas/integration", "backlog/led-44.md"), "nothing_to_merge")
	f.refused("mara", "request_merge", req("lead/ledger/atlas/none", "backlog/led-44.md"), "branch_not_found")
	writeFile(t, filepath.Join(f.wt("mara", "web"), "wip"), "x\n")
	f.refused("mara", "request_merge", req("lead/ledger/atlas/integration", "backlog/led-44.md"), "uncommitted_changes")
}

// mr is a merge request as kept.
func (f *fixture) mr(id string) store.MergeRequest {
	f.t.Helper()
	mr, err := f.db.MergeRequests().Get(ctx, id)
	if err != nil {
		f.t.Fatalf("merge request %s: %v", id, err)
	}
	return mr
}

// review is a review as the CEO reads it.
func (f *fixture) review(id string) Review {
	f.t.Helper()
	v, err := f.w.Review(ctx, id)
	if err != nil {
		f.t.Fatalf("review %s: %v", id, err)
	}
	return v
}

// shown is what the CEO's Merge sends for those repos: the head each was
// shown at, as the review page has it from the diff.
func (f *fixture) shown(id string, repos ...string) map[string]string {
	f.t.Helper()
	diffs, err := f.w.ReviewDiff(ctx, id, false)
	if err != nil {
		f.t.Fatalf("diff of %s: %v", id, err)
	}
	heads := map[string]string{}
	for _, repo := range repos {
		heads[repo] = ""
		for _, d := range diffs {
			if d.Repo == repo {
				heads[repo] = d.Head
			}
		}
	}
	return heads
}

var integration = map[string]string{"branch": "lead/ledger/atlas/integration", "title": "Invoices as CSV", "entry": "backlog/led-44.md"}

func TestFromHandoffToMain(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	// One merge request, in the one repo with new commits, carrying the hand-off, in the branch's review.
	if text := f.ok("mara", "request_merge", integration); text != "Review rev-1, api (mr-1) into main: waiting for the CEO." {
		t.Fatalf("%q", text)
	}
	if w := f.item("wt/ledger/tove/LED-44"); w.MergeRequestID != "mr-1" {
		t.Errorf("work item %+v", w)
	}
	if v := f.review("rev-1"); v.Title != "Invoices as CSV" || v.Body != "" {
		t.Errorf("title %q, body %q", v.Title, v.Body)
	}
	// A refresh takes the new title and body: the CEO reads what the lead says now.
	again := map[string]string{"branch": integration["branch"], "title": "Invoices as CSV and JSON", "body": "JSON too.\n\nRead export.go first.", "entry": integration["entry"]}
	if text := f.ok("mara", "request_merge", again); text != "Review rev-1, api (mr-1) into main, refreshed: waiting for the CEO." {
		t.Errorf("again: %q", text)
	}
	if v := f.review("rev-1"); v.Title != "Invoices as CSV and JSON" || v.Body != "JSON too.\n\nRead export.go first." {
		t.Errorf("refreshed: title %q, body %q", v.Title, v.Body)
	}
	f.ok("mara", "request_merge", integration) // and no body leaves none
	if v := f.review("rev-1"); v.Title != "Invoices as CSV" || v.Body != "" {
		t.Errorf("back: title %q, body %q", v.Title, v.Body)
	}
	p, _ := f.w.Pipeline(ctx, "ledger")
	if len(p.MergeRequests) != 1 || p.MergeRequests[0].Entry != "backlog/led-44.md" || p.MergeRequests[0].ReviewID != "rev-1" {
		t.Fatalf("pipeline %+v", p.MergeRequests)
	}
	v := f.review("rev-1")
	if v.Status != ReviewNeedsYou || v.Title != "Invoices as CSV" || v.EntryTitle != "Export invoices" || v.Lead != "mara@ledger" || v.Branch != "lead/ledger/atlas/integration" || len(v.MergeRequests) != 1 {
		t.Errorf("review %+v", v)
	}
	diffs, err := f.w.ReviewDiff(ctx, "rev-1", false)
	if err != nil || len(diffs) != 1 || diffs[0].Repo != "api" || diffs[0].Target != "main" || !strings.Contains(diffs[0].Diff, "LED-44.go") ||
		diffs[0].Head != f.git(f.wt("mara", "api"), "rev-parse", "HEAD") {
		t.Errorf("diff %+v %v", diffs, err)
	}
	// Never reviewed yet: "since" is against main too.
	if since, _ := f.w.ReviewDiff(ctx, "rev-1", true); len(since) != 1 || since[0].Since != "" || since[0].Diff != diffs[0].Diff {
		t.Errorf("since %+v", since)
	}

	// The CEO's checkout of api is on main: the merge happens there.
	api := filepath.Join(f.root, "ledger", "api")
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api")); err != nil {
		t.Fatal(err)
	}
	// One commit, its message the title alone.
	if msg := f.git(api, "log", "-1", "--format=%B", "main"); msg != "Export invoices" {
		t.Errorf("main's head %q", msg)
	}
	if parents := f.git(api, "log", "-1", "--format=%P", "main"); strings.Contains(parents, " ") {
		t.Errorf("a merge commit: %q", parents)
	}
	if _, err := os.Stat(filepath.Join(api, "LED-44.go")); err != nil {
		t.Error("the CEO's checkout didn't move with main")
	}
	// The lead's branch, checked out in its worktree, starts from the new main.
	if head, main := f.git(f.wt("mara", "api"), "rev-parse", "HEAD"), f.git(api, "rev-parse", "main"); head != main {
		t.Errorf("the lead's branch is at %s, main at %s", head, main)
	}
	if mr := f.mr("mr-1"); mr.State != store.MergeMerged || mr.MergedAt.IsZero() {
		t.Errorf("merge request %+v", mr)
	}
	if v := f.review("rev-1"); v.Status != ReviewMerged {
		t.Errorf("status %q", v.Status)
	}
	if n := f.lastNotice(); n.ToInstance != "mara@ledger" || n.FromInstance != "foreman" || n.Body != "From Foreman: lead/ledger/atlas/integration was merged into main in api, as one commit. lead/ledger/atlas/integration now starts from main." {
		t.Errorf("notice %+v", n)
	}
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api")); !errors.Is(err, ErrNotOpen) {
		t.Errorf("merged twice: %v", err)
	}
	if _, err := f.w.DismissReview(ctx, "rev-1", Dismiss{Note: "x"}); !errors.Is(err, ErrNotOpen) {
		t.Errorf("closed once merged: %v", err)
	}
	if _, err := f.w.Review(ctx, "rev-9"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown review: %v", err)
	}
	// Asked again once everything merged: a new review.
	writeFile(t, filepath.Join(f.wt("mara", "api"), "next.go"), "package n\n")
	f.ok("mara", "commit", commits("api", "next"))
	if text := f.ok("mara", "request_merge", integration); text != "Review rev-2, api (mr-2) into main: waiting for the CEO." {
		t.Errorf("next: %q", text)
	}
}

// A branch that changed several repos is one review: the CEO merges the repos
// they pick, the rest stay open in it, and its request stays until none is.
func TestOneReviewForEveryRepoOfABranch(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("mara", "api"), "a.go"), "package a\n")
	writeFile(t, filepath.Join(f.wt("mara", "web"), "page.html"), "<p>\n")
	f.ok("mara", "commit", commits("api", "a", "web", "page"))
	if text := f.ok("mara", "request_merge", integration); text != "Review rev-1, api (mr-1) into main, web (mr-2) into trunk: waiting for the CEO." {
		t.Fatalf("%q", text)
	}
	open, _ := f.w.OpenRequests(ctx)
	if len(open) != 1 || open[0].ReviewID != "rev-1" {
		t.Fatalf("one request for the review: %+v", open)
	}
	if err := f.w.MergeReview(ctx, "rev-1", nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("no repo picked: %v", err)
	}
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api", "docs")); !errors.Is(err, ErrNotOpen) || f.mr("mr-1").State != store.MergeOpen {
		t.Errorf("a repo not in it merges nothing: %v", err)
	}
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api")); err != nil {
		t.Fatal(err)
	}
	if f.mr("mr-1").State != store.MergeMerged || f.mr("mr-2").State != store.MergeOpen {
		t.Errorf("api merged, web open: %+v %+v", f.mr("mr-1"), f.mr("mr-2"))
	}
	if open, _ := f.w.OpenRequests(ctx); len(open) != 1 {
		t.Errorf("still waiting on the CEO for web: %+v", open)
	}
	if v := f.review("rev-1"); v.Status != ReviewNeedsYou {
		t.Errorf("status %q", v.Status)
	}
	// web left open: the lead's next request on the branch is the same review.
	writeFile(t, filepath.Join(f.wt("mara", "web"), "page.html"), "<p>2\n")
	f.ok("mara", "commit", commits("web", "page 2"))
	if text := f.ok("mara", "request_merge", integration); text != "Review rev-1, web (mr-2) into trunk, refreshed: waiting for the CEO." {
		t.Errorf("%q", text)
	}
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "web")); err != nil {
		t.Fatal(err)
	}
	web := filepath.Join(f.root, "ledger", "web")
	if subject := f.git(web, "log", "-1", "--format=%s", "trunk"); subject != "Export invoices" {
		t.Errorf("trunk's head %q", subject)
	}
	if open, _ := f.w.OpenRequests(ctx); len(open) != 0 {
		t.Errorf("resolved once nothing is open: %+v", open)
	}
	if v := f.review("rev-1"); v.Status != ReviewMerged {
		t.Errorf("status %q", v.Status)
	}
	list, err := f.w.ListReviews(ctx)
	if err != nil || len(list) != 1 || list[0].Comments != nil || list[0].Title != "Invoices as CSV" {
		t.Errorf("list %+v %v", list, err)
	}
}

func TestMergeToTargetConflicts(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "the lead's\n")
	f.ok("mara", "commit", commits("api", "readme"))
	f.ok("mara", "request_merge", integration)
	api := filepath.Join(f.root, "ledger", "api")
	writeFile(t, filepath.Join(api, "README"), "main's\n")
	f.git(api, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "main moved")
	head := f.git(api, "rev-parse", "main")
	err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api"))
	if !errors.Is(err, ErrConflict) || !strings.HasPrefix(err.Error(), "api: ") {
		t.Fatalf("err %v", err)
	}
	if f.git(api, "rev-parse", "main") != head {
		t.Error("main moved on a conflict")
	}
	if mr := f.mr("mr-1"); !mr.Conflicting || mr.State != store.MergeOpen {
		t.Errorf("merge request %+v", mr)
	}
	if n := f.lastNotice(); !strings.Contains(n.Body, `merge("main")`) {
		t.Errorf("notice %+v", n)
	}
	// The lead merges main in, resolves, asks again: the conflict is gone, and the merge goes through.
	f.ok("mara", "merge", map[string]string{"branch": "main"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "both\n")
	f.ok("mara", "commit", commits("api", "main merged in"))
	if text := f.ok("mara", "request_merge", integration); !strings.Contains(text, "api (mr-1) into main, refreshed") {
		t.Errorf("%q", text)
	}
	if f.mr("mr-1").Conflicting {
		t.Error("still marked conflicting")
	}
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api")); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(api, "README")); string(data) != "both\n" {
		t.Errorf("README %q", data)
	}
}

func TestDismissKeepingTheBranches(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	head := f.git(f.wt("mara", "api"), "rev-parse", "HEAD")
	if dismissed, err := f.w.DismissReview(ctx, "rev-1", Dismiss{Note: "  not this quarter "}); err != nil || len(dismissed.Kept) != 0 {
		t.Fatal(dismissed, err)
	}
	if len(f.teams.stopped) != 0 {
		t.Errorf("stopped %v", f.teams.stopped)
	}
	if mr := f.mr("mr-1"); mr.State != store.MergeRejected || mr.Note != "not this quarter" || mr.RejectedAt.IsZero() {
		t.Errorf("merge request %+v", mr)
	}
	// The work item stays with the dismissed merge request: it leaves In flight with it.
	if w := f.item("wt/ledger/tove/LED-44"); w.MergeRequestID != "mr-1" || w.Stage != store.StageIntegrated {
		t.Errorf("work item %+v", w)
	}
	if n := f.lastNotice(); n.ToInstance != "mara@ledger" || n.FromInstance != "foreman" ||
		n.Body != "From Foreman: the CEO dismissed the review of lead/ledger/atlas/integration without merging (api). Stop work on it, and don't request it again. The CEO's note: not this quarter" {
		t.Errorf("notice %+v", n)
	}
	if v := f.review("rev-1"); v.Status != ReviewDismissed {
		t.Errorf("status %q", v.Status)
	}
	if open, _ := f.w.OpenRequests(ctx); len(open) != 0 {
		t.Errorf("still open after closing: %+v", open)
	}
	if f.git(f.wt("mara", "api"), "rev-parse", "HEAD") != head {
		t.Error("the branch moved")
	}
	// Kept, it can be asked again (the CEO changed their mind): a new review
	// carrying the work item again.
	if text := f.ok("mara", "request_merge", integration); !strings.HasPrefix(text, "Review rev-2, api (mr-2)") {
		t.Errorf("%q", text)
	}
	if w := f.item("wt/ledger/tove/LED-44"); w.MergeRequestID != "mr-2" {
		t.Errorf("work item %+v", w)
	}
}

func TestDismissDroppingTheBranches(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	api, lead, member := filepath.Join(f.root, "ledger", "api"), f.wt("mara", "api"), f.wt("tove", "api")
	leadHead, memberHead := f.git(lead, "rev-parse", "HEAD"), f.git(api, "rev-parse", "wt/ledger/tove/LED-44")
	sent := len(f.m.sent)
	dismissed, err := f.w.DismissReview(ctx, "rev-1", Dismiss{Note: "nobody hears this", DropBranches: true, StopTeam: true})
	if err != nil || dismissed.Kept == nil || len(dismissed.Kept) != 0 {
		t.Fatal(dismissed, err)
	}
	if strings.Join(f.teams.stopped, ",") != "atlas@ledger" {
		t.Errorf("stopped %v", f.teams.stopped)
	}
	// A team being stopped isn't told anything.
	if len(f.m.sent) != sent {
		t.Errorf("told a stopped team: %+v", f.m.sent[sent:])
	}
	if mr := f.mr("mr-1"); mr.State != store.MergeRejected || mr.Note != "nobody hears this" {
		t.Errorf("merge request %+v", mr)
	}
	// Checked out in the lead's worktree: it starts from main, nothing of the work left.
	if f.git(lead, "rev-parse", "HEAD") != f.git(api, "rev-parse", "main") {
		t.Error("the lead branch still holds the work")
	}
	if _, err := os.Stat(filepath.Join(lead, "LED-44.go")); err == nil {
		t.Error("the work is still in the lead's worktree")
	}
	// Tove's branch: gone, or at main when still checked out.
	if f.git(member, "branch", "--show-current") == "wt/ledger/tove/LED-44" {
		if f.git(member, "rev-parse", "HEAD") != f.git(api, "rev-parse", "main") {
			t.Error("tove's branch still holds the work")
		}
	} else if out := f.git(api, "branch", "--list", "wt/ledger/tove/LED-44"); out != "" {
		t.Errorf("tove's branch is still there: %q", out)
	}
	// Each kept aside, out of the branch list.
	if f.git(api, "rev-parse", dismissedRef+"rev-1/lead/ledger/atlas/integration") != leadHead ||
		f.git(api, "rev-parse", dismissedRef+"rev-1/wt/ledger/tove/LED-44") != memberHead {
		t.Error("no backups")
	}
}

// Its agents work in its branches: they go only with the team.
func TestDroppingTheBranchesOfATeamLeftRunningIsRefused(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	head := f.git(f.wt("mara", "api"), "rev-parse", "HEAD")
	if _, err := f.w.DismissReview(ctx, "rev-1", Dismiss{DropBranches: true}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("dropped under a running team: %v", err)
	}
	if mr := f.mr("mr-1"); mr.State != store.MergeOpen || f.git(f.wt("mara", "api"), "rev-parse", "HEAD") != head {
		t.Errorf("changed anyway: %+v", mr)
	}
}

// A team that can't be stopped leaves the review as it was: the CEO tries again.
func TestDismissWhenTheTeamCantStopDismissesNothing(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	f.teams.fail = errors.New("podman is gone")
	if _, err := f.w.DismissReview(ctx, "rev-1", Dismiss{StopTeam: true}); err == nil || !strings.Contains(err.Error(), "podman is gone") {
		t.Fatalf("dismissed: %v", err)
	}
	if mr := f.mr("mr-1"); mr.State != store.MergeOpen {
		t.Errorf("dismissed anyway: %+v", mr)
	}
}

// A team no longer deployed: nobody to stop or tell, and its branches can go.
func TestDismissAfterTheTeamStopped(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	f.teams.running["atlas@ledger"] = false
	sent := len(f.m.sent)
	if _, err := f.w.DismissReview(ctx, "rev-1", Dismiss{Note: "kept on the review", DropBranches: true, StopTeam: true}); err != nil {
		t.Fatal(err)
	}
	if len(f.teams.stopped) != 0 || len(f.m.sent) != sent {
		t.Errorf("stopped %v, told %+v", f.teams.stopped, f.m.sent[sent:])
	}
	api := filepath.Join(f.root, "ledger", "api")
	if f.git(f.wt("mara", "api"), "rev-parse", "HEAD") != f.git(api, "rev-parse", "main") {
		t.Error("the lead branch wasn't dropped")
	}
}

// What the lead's idle worktree held not committed goes with the branch,
// kept aside: nothing stays in the way, nothing is lost.
func TestDismissDropsABranchWithChangesNotCommitted(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	api, lead := filepath.Join(f.root, "ledger", "api"), f.wt("mara", "api")
	head := f.git(lead, "rev-parse", "HEAD")
	writeFile(t, filepath.Join(lead, "LED-44.go"), "package cal // the lead's, uncommitted\n")
	dismissed, err := f.w.DismissReview(ctx, "rev-1", Dismiss{DropBranches: true, StopTeam: true})
	if err != nil || len(dismissed.Kept) != 0 {
		t.Fatal(dismissed, err)
	}
	if f.git(lead, "rev-parse", "HEAD") != f.git(api, "rev-parse", "main") || f.git(lead, "status", "--porcelain") != "" {
		t.Error("the lead's worktree isn't clean at main")
	}
	saved := dismissedRef + "rev-1/lead/ledger/atlas/integration@uncommitted"
	if f.git(api, "rev-parse", saved+"^") != head || f.git(api, "show", saved+":LED-44.go") != "package cal // the lead's, uncommitted" {
		t.Error("the change not committed isn't kept")
	}
}

// Backups older than 30 days go, in every project's repos.
func TestOldBackupsArePruned(t *testing.T) {
	f := newFixture(t)
	api := filepath.Join(f.root, "ledger", "api")
	old := "refs/foreman/dismissed/2026-09-01T00-00-00Z/rev-0/lead/ledger/atlas/x"
	recent := "refs/foreman/discarded/2026-10-01T00-00-00Z/tove/api"
	f.git(api, "update-ref", old, "main")
	f.git(api, "update-ref", recent, "main")
	f.w.PruneBackups(ctx)
	if out := f.git(api, "for-each-ref", "--format=%(refname)", "refs/foreman/"); out != recent {
		t.Errorf("backups left: %q", out)
	}
}

// After a merge, idle desks follow the new main; one with work stays.
func TestAMergeBringsIdleDesksUpToMain(t *testing.T) {
	f := newFixture(t)
	f.integrated() // tove now on wt/ledger/tove/LED-44: wt/ledger/tove/main idles, not checked out
	api := filepath.Join(f.root, "ledger", "api")
	f.ok("mara", "request_merge", integration)
	before := f.git(api, "rev-parse", "wt/ledger/tove/main")
	// Another project sharing the repo has its own default branch: its desks stay.
	f.git(api, "branch", "wt/suite/tove/main", before)
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api")); err != nil {
		t.Fatal(err)
	}
	if main := f.git(api, "rev-parse", "main"); main == before || f.git(api, "rev-parse", "wt/ledger/tove/main") != main {
		t.Error("tove's idle desk didn't follow main")
	}
	if f.git(api, "rev-parse", "wt/suite/tove/main") != before {
		t.Error("another project's desk followed this project's main")
	}
	if f.git(f.wt("tove", "api"), "branch", "--show-current") != "wt/ledger/tove/LED-44" {
		t.Error("tove's work moved")
	}
}

func TestAHandOffAfterADismissalIsNewWork(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	if _, err := f.w.DismissReview(ctx, "rev-1", Dismiss{}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.wt("tove", "api"), "more.go"), "package cal\n")
	f.ok("tove", "commit", commits("api", "more"))
	f.ok("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/LED-44", "summary": "again"})
	if w := f.item("wt/ledger/tove/LED-44"); w.MergeRequestID != "" || w.Stage != store.StageHanded {
		t.Errorf("work item %+v", w)
	}
}

// The CEO comments on lines and in general, then requests changes: the lead
// gets them in one message, the review waits on the lead until it asks again,
// and the CEO then reads only what changed since, with the comments following
// their lines.
func TestRequestChangesSendsTheDraftAndWaitsOnTheLead(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("mara", "api"), "calc.go"), "package calc\n\nfunc One() int { return 1 }\n\nfunc Two() int { return 3 }\n")
	f.ok("mara", "commit", commits("api", "calc"))
	f.ok("mara", "request_merge", integration)
	head := f.git(f.wt("mara", "api"), "rev-parse", "HEAD")
	line := func(n int, snippet, body string) store.ReviewComment {
		return store.ReviewComment{Repo: "api", Path: "calc.go", Side: "new", Line: n, Commit: head, Snippet: snippet, Body: body}
	}
	if err := f.w.RequestChanges(ctx, "rev-1", " "); !errors.Is(err, ErrInvalid) {
		t.Errorf("nothing to send: %v", err)
	}
	for _, bad := range []store.ReviewComment{
		line(3, "", "  "),
		{Repo: "docs", Path: "x", Side: "new", Line: 1, Commit: head, Body: "b"},
		{Repo: "api", Path: "calc.go", Side: "new", Line: 1, Body: "no commit"},
		{Repo: "api", Path: "calc.go", Side: "left", Line: 1, Commit: head, Body: "b"},
		{Repo: "api", Path: "calc.go", Side: "new", Line: 0, Commit: head, Body: "b"},
		{Path: "calc.go", Body: "general, with a file"},
	} {
		if _, err := f.w.AddComment(ctx, "rev-1", bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
	two, err := f.w.AddComment(ctx, "rev-1", line(5, "func Two() int { return 3 }", "Two is 2"))
	if err != nil || two.ID != "com-1" || two.State != store.CommentDraft {
		t.Fatalf("%+v %v", two, err)
	}
	one, _ := f.w.AddComment(ctx, "rev-1", line(3, "func One() int { return 1 }", "fine, but name it"))
	gone, _ := f.w.AddComment(ctx, "rev-1", store.ReviewComment{Body: "drop me"})
	general, _ := f.w.AddComment(ctx, "rev-1", store.ReviewComment{Body: "Add a test"})
	if _, err := f.w.EditComment(ctx, "rev-1", one.ID, "One is fine"); err != nil {
		t.Fatal(err)
	}
	if err := f.w.DeleteComment(ctx, "rev-1", gone.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.w.DeleteComment(ctx, "rev-2", one.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("another review's comment: %v", err)
	}

	sent := len(f.m.sent)
	if err := f.w.RequestChanges(ctx, "rev-1", "Nearly there."); err != nil {
		t.Fatal(err)
	}
	if news := f.m.sent[sent:]; len(news) != 1 || news[0].FromInstance != "ceo" || news[0].ToInstance != "mara@ledger" ||
		news[0].Body != "From the CEO: changes requested on lead/ledger/atlas/integration (backlog/led-44.md).\n\nNearly there.\n\n"+
			"- api calc.go:5 `func Two() int { return 3 }`: Two is 2\n\n- api calc.go:3 `func One() int { return 1 }`: One is fine\n\n- Add a test\n\n"+
			"Fix them, commit(), then call request_merge again on the same branch." {
		t.Errorf("message %+v", news)
	}
	v := f.review("rev-1")
	if v.Status != ReviewChangesRequested || len(v.Comments) != 3 || v.Comments[0].State != store.CommentSent || v.Comments[0].SentAt.IsZero() {
		t.Errorf("review %+v", v)
	}
	if f.mr("mr-1").ReviewedHead != head {
		t.Errorf("reviewed head %q", f.mr("mr-1").ReviewedHead)
	}
	if _, err := f.w.EditComment(ctx, "rev-1", general.ID, "x"); !errors.Is(err, ErrSent) {
		t.Errorf("edit a sent one: %v", err)
	}
	f.w.Reconcile(ctx)
	if open, _ := f.w.OpenRequests(ctx); len(open) != 0 {
		t.Errorf("waiting on the lead, not the CEO: %+v", open)
	}

	// The lead adds two lines above One and fixes Two, then asks again.
	writeFile(t, filepath.Join(f.wt("mara", "api"), "calc.go"), "package calc\n\n// One is one.\n// Really.\nfunc One() int { return 1 }\n\nfunc Two() int { return 2 }\n")
	f.ok("mara", "commit", commits("api", "fix Two"))
	f.ok("mara", "request_merge", integration)
	if open, _ := f.w.OpenRequests(ctx); len(open) != 1 || open[0].ReviewID != "rev-1" {
		t.Errorf("waiting on the CEO again: %+v", open)
	}
	v = f.review("rev-1")
	if v.Status != ReviewNeedsYou {
		t.Errorf("status %q", v.Status)
	}
	byID := map[string]Comment{}
	for _, c := range v.Comments {
		byID[c.ID] = c
	}
	if c := byID[one.ID]; c.Now != 5 || c.Outdated {
		t.Errorf("One's comment follows its line: %+v", c)
	}
	if c := byID[two.ID]; !c.Outdated {
		t.Errorf("Two's line changed: %+v", c)
	}
	if c := byID[general.ID]; c.Outdated || c.Now != 0 {
		t.Errorf("a general one stays: %+v", c)
	}
	since, err := f.w.ReviewDiff(ctx, "rev-1", true)
	if err != nil || len(since) != 1 || since[0].Since != head || !strings.Contains(since[0].Diff, "+// Really.") || strings.Contains(since[0].Diff, "+package calc") {
		t.Errorf("since %+v %v", since, err)
	}
}

func TestFollowLine(t *testing.T) {
	for _, c := range []struct {
		name     string
		diff     string
		line     int
		want     int
		outdated bool
	}{
		{"no change", "", 10, 10, false},
		{"lines added above", "@@ -2,0 +3,2 @@\n+a\n+b", 10, 12, false},
		{"lines removed above", "@@ -2,3 +1,0 @@\n-a\n-b\n-c", 10, 7, false},
		{"its line changed", "@@ -10 +10 @@\n-x\n+y", 10, 10, true},
		{"its line removed", "@@ -9,2 +8,0 @@\n-x\n-y", 10, 10, true},
		{"changed below", "@@ -11,2 +11,5 @@\n-x", 10, 10, false},
		{"added right after it", "@@ -10,0 +11 @@\n+z", 10, 10, false},
		{"added right before it", "@@ -9,0 +10 @@\n+z", 10, 11, false},
		{"several hunks", "@@ -1,0 +2 @@\n+a\n@@ -4,2 +5,0 @@\n-b\n-c\n@@ -20 +19 @@", 10, 9, false},
		{"a header it can't read", "@@ nonsense @@", 10, 10, true},
	} {
		if got, outdated := followLine(c.diff, c.line); got != c.want || outdated != c.outdated {
			t.Errorf("%s: %d %v, want %d %v", c.name, got, outdated, c.want, c.outdated)
		}
	}
}

func TestMergeOnlyWhatTheCEORead(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	read := f.shown("rev-1", "api")
	api := filepath.Join(f.root, "ledger", "api")
	main := f.git(api, "rev-parse", "main")
	// The lead commits after the CEO loaded the review.
	writeFile(t, filepath.Join(f.wt("mara", "api"), "late.go"), "package late\n")
	f.ok("mara", "commit", commits("api", "late"))
	if err := f.w.MergeReview(ctx, "rev-1", read); !errors.Is(err, ErrBranchMoved) || !strings.HasPrefix(err.Error(), "api: ") {
		t.Fatalf("err %v, want api: ErrBranchMoved", err)
	}
	if mr := f.mr("mr-1"); mr.State != store.MergeOpen || f.git(api, "rev-parse", "main") != main {
		t.Errorf("merged anyway: %+v", mr)
	}
	if err := f.w.MergeReview(ctx, "rev-1", map[string]string{"api": ""}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no commit named: %v", err)
	}
	// Read again, it merges, late commit included.
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(api, "late.go")); err != nil {
		t.Error("the late commit wasn't merged")
	}
}

func TestAMergedBranchThatCantMoveIsLeftToTheLead(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	// main changes README meanwhile, and the lead has an uncommitted edit there.
	api, lead := filepath.Join(f.root, "ledger", "api"), f.wt("mara", "api")
	writeFile(t, filepath.Join(api, "README"), "the CEO's\n")
	f.git(api, "add", "README")
	f.git(api, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "readme")
	head := f.git(lead, "rev-parse", "HEAD")
	writeFile(t, filepath.Join(lead, "README"), "the lead's\n")
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api")); err != nil {
		t.Fatal(err)
	}
	if mr := f.mr("mr-1"); mr.State != store.MergeMerged {
		t.Errorf("merge request %+v", mr)
	}
	if f.git(lead, "rev-parse", "HEAD") != head {
		t.Error("the lead's branch moved over its uncommitted change")
	}
	if data, _ := os.ReadFile(filepath.Join(lead, "README")); string(data) != "the lead's\n" {
		t.Errorf("the lead's change was lost: %q", data)
	}
	if n := f.lastNotice(); !strings.HasSuffix(n.Body, `so it still holds what was merged: merge main into it with merge("main") before you go on.`) {
		t.Errorf("notice %q", n.Body)
	}
}

func TestMergesOutsideForemanAreNoticedAndMainMovedIsTold(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/second"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "second.go"), "package s\n")
	f.ok("mara", "commit", commits("api", "second"))
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/ledger/atlas/second", "title": "Invoices as CSV", "entry": "backlog/led-44.md"})
	// The CEO squashes the first by hand, in their own checkout, as a forge
	// would: its commits never reach main.
	api := filepath.Join(f.root, "ledger", "api")
	f.git(api, "merge", "-q", "--squash", "lead/ledger/atlas/integration")
	f.git(api, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "by hand")
	sent := len(f.m.sent)
	f.w.Reconcile(ctx)
	if mr := f.mr("mr-1"); mr.State != store.MergeMerged {
		t.Errorf("not noticed: %+v", mr)
	}
	if mr := f.mr("mr-2"); mr.State != store.MergeOpen || mr.ReviewID != "rev-2" {
		t.Errorf("the other one moved: %+v", mr)
	}
	// Its lead is told it's merged, and the other open one's that main moved.
	if news := f.m.sent[sent:]; len(news) != 2 || !strings.Contains(news[0].Body, "lead/ledger/atlas/integration was merged into main in api, as one commit. lead/ledger/atlas/integration now starts from main.") || !strings.Contains(news[1].Body, "main moved in api (lead/ledger/atlas/integration was merged). Merge it into lead/ledger/atlas/second") {
		t.Errorf("notices %+v", news)
	}
	// Not checked out anywhere, it moved too.
	if f.git(api, "rev-parse", "lead/ledger/atlas/integration") != f.git(api, "rev-parse", "main") {
		t.Error("the merged branch didn't move to main")
	}
	f.w.Reconcile(ctx)
	if len(f.m.sent) != sent+2 {
		t.Error("noticed twice")
	}
}

// A merge request shows in the team's activity: the lead's request_merge is
// kept as an action, naming the merge request of each repo.
func TestRequestMergeIsKeptAsAnAction(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	f.refused("mara", "request_merge", map[string]string{"branch": "lead/ledger/atlas/integration", "title": "Invoices as CSV", "entry": "backlog/nope.md"}, "entry_not_found")
	list, err := f.db.Actions().List(ctx, "atlas@ledger")
	if err != nil {
		t.Fatal(err)
	}
	var mine []store.TeamAction
	for _, a := range list {
		if a.Tool == "request_merge" {
			mine = append(mine, a)
		}
	}
	if len(mine) != 2 {
		t.Fatalf("actions %+v", mine)
	}
	if a := mine[0]; a.Instance != "mara@ledger" || a.Branch != "lead/ledger/atlas/integration" || len(a.Repos) != 1 || a.Repos[0].Repo != "api" || a.Repos[0].MergeRequest != "mr-1" {
		t.Errorf("action %+v", a)
	}
	if a := mine[1]; !strings.Contains(a.Error, "entry_not_found") && !strings.Contains(a.Error, "no wiki entry") {
		t.Errorf("refusal %+v", a)
	}
}

// A review waits for the CEO as one review request, until nothing in it is open.
func TestReviewRequests(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	f.ok("mara", "request_merge", integration) // a refresh raises no second one
	open, _ := f.w.OpenRequests(ctx)
	if len(open) != 1 {
		t.Fatalf("open %+v", open)
	}
	if r := open[0]; r.Kind != "review" || r.ReviewID != "rev-1" || r.InstanceID != "mara@ledger" || r.ProjectID != "ledger" ||
		r.Source != "agent" || r.Summary != "Merge lead/ledger/atlas/integration: Invoices as CSV" {
		t.Errorf("review %+v", r)
	}
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api")); err != nil {
		t.Fatal(err)
	}
	if open, _ := f.w.OpenRequests(ctx); len(open) != 0 {
		t.Errorf("still open after the merge: %+v", open)
	}
	var kinds []string
	for _, e := range f.events {
		if strings.HasPrefix(e.Type, "request.") {
			kinds = append(kinds, e.Type)
		}
	}
	if strings.Join(kinds, ",") != "request.created,request.resolved" {
		t.Errorf("events %v", kinds)
	}

	// One whose request couldn't be saved gets it at the next reconcile, once.
	rev, err := f.db.Reviews().Save(ctx, store.Review{ProjectID: "ledger", Branch: "lead/ledger/atlas/second"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.MergeRequests().Save(ctx, store.MergeRequest{ProjectID: "ledger", DeploymentID: "atlas@ledger", AuthorInstance: "mara@ledger",
		Repo: "api", Branch: "lead/ledger/atlas/second", Entry: "backlog/led-44.md", ReviewID: rev.ID}); err != nil {
		t.Fatal(err)
	}
	f.w.Reconcile(ctx)
	f.w.Reconcile(ctx)
	if open, _ := f.w.OpenRequests(ctx); len(open) != 1 || open[0].ReviewID != rev.ID {
		t.Errorf("backfilled %+v", open)
	}
}

// Each repo's merge request goes to that repo's own main: web's is trunk.
func TestEachRepoMergesIntoItsOwnMain(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("mara", "web"), "page.html"), "<p>\n")
	f.ok("mara", "commit", commits("web", "a page"))
	if text := f.ok("mara", "request_merge", integration); text != "Review rev-1, web (mr-1) into trunk: waiting for the CEO." {
		t.Fatalf("%q", text)
	}
	if diffs, _ := f.w.ReviewDiff(ctx, "rev-1", false); len(diffs) != 1 || diffs[0].Target != "trunk" || !strings.Contains(diffs[0].Diff, "page.html") {
		t.Errorf("diff against trunk %+v", diffs)
	}
	web := filepath.Join(f.root, "ledger", "web")
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "web")); err != nil {
		t.Fatal(err)
	}
	if subject := f.git(web, "log", "-1", "--format=%s", "trunk"); subject != "Export invoices" {
		t.Errorf("trunk's head %q", subject)
	}
	if n := f.lastNotice(); !strings.Contains(n.Body, "lead/ledger/atlas/integration was merged into trunk in web, as one commit. lead/ledger/atlas/integration now starts from trunk.") {
		t.Errorf("notice %+v", n)
	}
	// A teammate's hand-off is new against web's trunk, not a main it hasn't.
	writeFile(t, filepath.Join(f.wt("tove", "web"), "style.css"), "p {}\n")
	f.ok("tove", "commit", commits("web", "style"))
	f.ok("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/main", "summary": "style"})
}

// A comment on a very long line (minified code, a long string) is taken: the
// line's text it keeps, only to show where it was, is cut.
func TestACommentOnALongLineKeepsItsStart(t *testing.T) {
	f := newFixture(t)
	long := strings.Repeat("é", maxSnippet+100)
	writeFile(t, filepath.Join(f.wt("mara", "api"), "min.js"), long+"\n")
	f.ok("mara", "commit", commits("api", "min"))
	f.ok("mara", "request_merge", integration)
	head := f.git(f.wt("mara", "api"), "rev-parse", "HEAD")
	c, err := f.w.AddComment(ctx, "rev-1", store.ReviewComment{Repo: "api", Path: "min.js", Side: "new", Line: 1, Commit: head, Snippet: long, Body: "Not minified"})
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Repeat("é", maxSnippet-1) + "…"; c.Snippet != want || c.Body != "Not minified" {
		t.Errorf("snippet of %d characters, body %q", utf8.RuneCountInString(c.Snippet), c.Body)
	}
}

// A line comment follows its file when the lead renames it (the CEO,
// 2026-11-04): to its line there, outdated only when the line itself changed;
// a file deleted outdates its comments. The lead reads the new path.
func TestACommentFollowsARenamedFile(t *testing.T) {
	f := newFixture(t)
	lead := f.wt("mara", "api")
	writeFile(t, filepath.Join(lead, "calc.go"), "package calc\n\nfunc One() int { return 1 }\n\nfunc Two() int { return 3 }\n")
	writeFile(t, filepath.Join(lead, "keep.go"), "package calc\n\nconst Kept = 1\n")
	writeFile(t, filepath.Join(lead, "gone.go"), "package calc\n\nconst Gone = 1\n")
	f.ok("mara", "commit", commits("api", "calc"))
	f.ok("mara", "request_merge", integration)
	head := f.git(lead, "rev-parse", "HEAD")
	line := func(path string, n int, body string) Comment {
		t.Helper()
		c, err := f.w.AddComment(ctx, "rev-1", store.ReviewComment{Repo: "api", Path: path, Side: "new", Line: n, Commit: head, Body: body})
		if err != nil {
			t.Fatal(err)
		}
		return Comment{ReviewComment: c}
	}
	one, two := line("calc.go", 3, "name it"), line("calc.go", 5, "Two is 2")
	kept, gone := line("keep.go", 3, "kept as is"), line("gone.go", 3, "why?")

	// calc.go becomes math.go, gaining a line above One and fixing Two; keep.go
	// is renamed twice, unchanged; gone.go is deleted.
	f.git(lead, "mv", "calc.go", "math.go")
	writeFile(t, filepath.Join(lead, "math.go"), "package calc\n\n// One is one.\nfunc One() int { return 1 }\n\nfunc Two() int { return 2 }\n")
	f.git(lead, "mv", "keep.go", "kept.go")
	f.ok("mara", "commit", commits("api", "rename"))
	f.git(lead, "mv", "kept.go", "still kept.go")
	f.git(lead, "rm", "-q", "gone.go")
	f.ok("mara", "commit", commits("api", "rename again"))
	f.ok("mara", "request_merge", integration)

	byID := map[string]Comment{}
	for _, c := range f.review("rev-1").Comments {
		byID[c.ID] = c
	}
	for _, want := range []struct {
		c        Comment
		path     string
		now      int
		outdated bool
	}{
		{one, "math.go", 4, false},        // renamed with an edit above: the line moves
		{two, "math.go", 5, true},         // renamed with its line changed
		{kept, "still kept.go", 3, false}, // renamed twice, a space in the name: git quotes nothing, finds it
		{gone, "gone.go", 3, true},        // deleted
	} {
		got := byID[want.c.ID]
		if got.NowPath != want.path || got.Outdated != want.outdated || (!want.outdated && got.Now != want.now) {
			t.Errorf("%s: %+v, want %s:%d outdated %v", want.c.Body, got, want.path, want.now, want.outdated)
		}
	}
	sent := len(f.m.sent)
	if err := f.w.RequestChanges(ctx, "rev-1", ""); err != nil {
		t.Fatal(err)
	}
	if body := f.m.sent[sent].Body; !strings.Contains(body, "- api math.go:4: name it") || !strings.Contains(body, "- api still kept.go:3: kept as is") {
		t.Errorf("the lead reads the new paths: %s", body)
	}
}

func TestRenamed(t *testing.T) {
	diff := "diff --git a/x.go b/x.go\nindex 1..2 100644\n--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-rename from a.go\n+b\n" +
		"diff --git a/a.go b/b.go\nsimilarity index 90%\nrename from a.go\nrename to b.go\n@@ -1 +1,2 @@\n+x\n" +
		"diff --git \"a/caf\\303\\251.go\" \"b/new caf\\303\\251.go\"\nsimilarity index 100%\nrename from \"caf\\303\\251.go\"\nrename to \"new caf\\303\\251.go\"\n"
	if to, section, ok := renamed(diff, "a.go"); !ok || to != "b.go" || !strings.HasSuffix(section, "@@ -1 +1,2 @@\n+x") {
		t.Errorf("a.go: %q %q %v", to, section, ok)
	}
	if to, _, ok := renamed(diff, "café.go"); !ok || to != "new café.go" {
		t.Errorf("quoted: %q %v", to, ok)
	}
	// A hunk's text that reads like a header isn't one.
	if to, _, ok := renamed(diff, "x.go"); ok {
		t.Errorf("x.go isn't renamed: %q", to)
	}
}
