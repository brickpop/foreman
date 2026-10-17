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

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
	"github.com/brickpop/foreman/internal/wiki"
	"github.com/brickpop/foreman/internal/workspace"
)

var ctx = context.Background()

var atlas = workspace.Team{ID: "atlas", Config: workspace.TeamConfig{
	Team: "atlas", Name: "Atlas", Lead: "mara", Order: []string{"mara", "tove", "ivo"},
	Agents: map[string]workspace.Soul{
		"mara": {Name: "Mara", Role: "team_lead"},
		"tove": {Name: "Tove", Role: "engineer"},
		"ivo":  {Name: "Ivo", Role: "engineer"},
	},
}}

type config struct{}

func (config) Project(id string) (workspace.Project, error) {
	return workspace.Project{ID: id, Repos: []string{"api", "web"}, Config: workspace.ProjectConfig{Main: "main"}}, nil
}

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
}

func (m *messages) Send(_ context.Context, msg store.TeamMessage) (store.TeamMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	msg.Ref = "msg-" + string(rune('0'+len(m.sent)))
	return msg, nil
}

type fixture struct {
	t      *testing.T
	root   string
	w      *Workflow
	m      *messages
	db     *store.DB
	events []events.Event
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
	for _, r := range []string{"api", "web", "project"} {
		dir := filepath.Join(root, "ledger", r)
		if err := os.MkdirAll(filepath.Join(dir, "wiki"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := g.Init(ctx, dir, "main"); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "README"), r+"\n")
		if err := g.Commit(ctx, dir, "first", "README"); err != nil {
			t.Fatal(err)
		}
	}
	for agent, branch := range map[string]string{"mara": "lead/atlas/integration", "tove": "wt/tove/main"} {
		for _, r := range []string{"api", "web"} {
			if err := g.Prepare(ctx, filepath.Join(root, "ledger", r), filepath.Join(root, "ledger", ".sandboxes", agent, r), branch, "main"); err != nil {
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
	if _, err := db.Deployments().Deploy(ctx, "atlas", "ledger", ""); err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, root: root, m: m, db: db}
	f.w = New(Deps{Root: root, Git: g, Config: config{}, Messages: m, Now: now, WorkItems: db.WorkItems(),
		MergeRequests: db.MergeRequests(), Wiki: entries{"backlog/led-44.md": "Export invoices"}, Actions: db.Actions(), Requests: db.Requests(),
		Publish: func(e events.Event) { f.events = append(f.events, e) }})
	return f
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

func (f *fixture) call(agent, tool string, args map[string]string) agentlink.Result {
	f.t.Helper()
	raw, _ := json.Marshal(args)
	c := teamtools.Caller{
		Instance:   store.Instance{ID: agent + "@ledger", AgentID: agent, SandboxPath: filepath.Join(f.root, "ledger", ".sandboxes", agent)},
		Deployment: store.Deployment{ID: "atlas@ledger", TeamID: "atlas", ProjectID: "ledger", Status: store.DeploymentActive},
		Team:       atlas, Lead: agent == "mara",
	}
	return f.w.Call(ctx, c, tool, raw)
}

func (f *fixture) ok(agent, tool string, args map[string]string) string {
	f.t.Helper()
	res := f.call(agent, tool, args)
	if res.Error {
		f.t.Fatalf("%s %s %v: %s", agent, tool, args, res.Text)
	}
	return res.Text
}

func (f *fixture) refused(agent, tool string, args map[string]string, code string) string {
	f.t.Helper()
	res := f.call(agent, tool, args)
	if !res.Error || !strings.HasPrefix(res.Text, code+":") {
		f.t.Errorf("%s %s %v: want %s, got %+v", agent, tool, args, code, res)
	}
	return res.Text
}

func TestSwitchInEveryRepo(t *testing.T) {
	f := newFixture(t)
	text := f.ok("tove", "switch", map[string]string{"branch": "wt/tove/LED-44"})
	if text != "Switched api, web to wt/tove/LED-44. New in api, web, made from main." {
		t.Errorf("%q", text)
	}
	for _, r := range []string{"api", "web"} {
		if b := f.git(f.wt("tove", r), "branch", "--show-current"); b != "wt/tove/LED-44" {
			t.Errorf("%s on %q", r, b)
		}
	}
	if text := f.ok("tove", "switch", map[string]string{"branch": "wt/tove/main"}); strings.Contains(text, "New") {
		t.Errorf("back home: %q", text)
	}
	if text := f.ok("mara", "switch", map[string]string{"branch": "lead/atlas/settlement"}); !strings.Contains(text, "New in api, web") {
		t.Errorf("lead: %q", text)
	}
}

func TestSwitchRefusals(t *testing.T) {
	f := newFixture(t)
	f.refused("tove", "switch", map[string]string{"branch": "wt/mara/x"}, "branch_not_allowed")
	f.refused("tove", "switch", map[string]string{"branch": "lead/atlas/x"}, "branch_not_allowed")
	f.refused("tove", "switch", map[string]string{"branch": "main"}, "branch_not_allowed")
	f.refused("tove", "switch", map[string]string{"branch": "wt/tove/"}, "bad_branch")
	f.refused("tove", "switch", map[string]string{"branch": "wt/tove/a b"}, "bad_branch")
	f.refused("mara", "switch", map[string]string{"branch": "wt/mara/x"}, "branch_not_allowed")
	f.refused("mara", "switch", map[string]string{"branch": "lead/other/x"}, "branch_not_allowed")
	// Changes in one repo stop the switch in all of them: it never stops halfway.
	writeFile(t, filepath.Join(f.wt("tove", "web"), "draft"), "x\n")
	if text := f.refused("tove", "switch", map[string]string{"branch": "wt/tove/x"}, "uncommitted_changes"); !strings.Contains(text, "web") || strings.Contains(text, "api") {
		t.Errorf("%q", text)
	}
	if b := f.git(f.wt("tove", "api"), "branch", "--show-current"); b != "wt/tove/main" {
		t.Errorf("api switched anyway, to %q", b)
	}
}

func TestCommitInEveryRepoAsTheAgent(t *testing.T) {
	f := newFixture(t)
	f.refused("tove", "commit", map[string]string{"message": "x"}, "nothing_to_commit")
	f.refused("tove", "commit", map[string]string{"message": " "}, "empty")
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal\n")
	text := f.ok("tove", "commit", map[string]string{"message": "cal v0"})
	if !strings.HasPrefix(text, "Committed api (wt/tove/main, ") || strings.Contains(text, "web") {
		t.Errorf("%q", text)
	}
	if who := f.git(f.wt("tove", "api"), "log", "-1", "--format=%an <%ae>|%s"); who != "Tove <tove@foreman.invalid>|cal v0" {
		t.Errorf("authored as %q", who)
	}
	// Nothing reached main or the CEO's checkout.
	if f.git(filepath.Join(f.root, "ledger", "api"), "rev-parse", "main") == f.git(f.wt("tove", "api"), "rev-parse", "HEAD") {
		t.Error("main moved")
	}
}

// The lead's commit also takes its edits in the live project repo, on main;
// a member's never does.
func TestTheLeadCommitsTheProjectRepoToo(t *testing.T) {
	f := newFixture(t)
	project := filepath.Join(f.root, "ledger", "project")
	writeFile(t, filepath.Join(project, "wiki", "board.md"), "# Board\n")
	f.refused("tove", "commit", map[string]string{"message": "x"}, "nothing_to_commit")
	text := f.ok("mara", "commit", map[string]string{"message": "the board"})
	if !strings.Contains(text, "project (main, ") {
		t.Errorf("%q", text)
	}
	if who := f.git(project, "log", "-1", "--format=%an|%s"); who != "Mara|the board" {
		t.Errorf("project repo HEAD: %q", who)
	}
}

func TestMergeATeammatesBranch(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal\n")
	f.ok("tove", "commit", map[string]string{"message": "cal"})
	f.refused("mara", "merge", map[string]string{"branch": "wt/tove/nope"}, "branch_not_found")
	f.refused("mara", "merge", map[string]string{"branch": "-x"}, "bad_branch")
	if text := f.ok("mara", "merge", map[string]string{"branch": "wt/tove/main"}); text != "Merged wt/tove/main into api, web." {
		t.Errorf("%q", text)
	}
	if _, err := os.Stat(filepath.Join(f.wt("mara", "api"), "cal.go")); err != nil {
		t.Error("tove's work didn't arrive")
	}
	writeFile(t, filepath.Join(f.wt("mara", "web"), "wip"), "x\n")
	f.refused("mara", "merge", map[string]string{"branch": "main"}, "uncommitted_changes")
}

func TestMergeWithConflictsThenCommit(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("tove", "api"), "README"), "tove\n")
	f.ok("tove", "commit", map[string]string{"message": "tove's readme"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "mara\n")
	f.ok("mara", "commit", map[string]string{"message": "mara's readme"})
	text := f.ok("mara", "merge", map[string]string{"branch": "wt/tove/main"})
	if text != "Merged wt/tove/main into web. Conflicts in api (README): resolve the markers, then commit() to finish the merge, or abort_merge() to back out." {
		t.Errorf("%q", text)
	}
	if text := f.refused("mara", "commit", map[string]string{"message": "merged"}, "conflicts_left"); !strings.Contains(text, "api/README") {
		t.Errorf("%q", text)
	}
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "both\n")
	if text := f.ok("mara", "commit", map[string]string{"message": "merged"}); !strings.Contains(text, "api (lead/atlas/integration") {
		t.Errorf("%q", text)
	}
}

func TestHandoffNamesACommittedBranch(t *testing.T) {
	f := newFixture(t)
	f.ok("tove", "switch", map[string]string{"branch": "wt/tove/LED-44"})
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/tove/LED-44", "summary": "done"}, "nothing_new")
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal\n")
	// Pending work is refused, never committed for the agent: commit() is the one way.
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/tove/LED-44", "summary": "done"}, "uncommitted_changes")
	if len(f.m.sent) != 0 {
		t.Fatalf("a refusal sent %v", f.m.sent)
	}
	f.ok("tove", "commit", map[string]string{"message": "cal v0"})
	text := f.ok("tove", "handoff", map[string]string{"teammate": "Mara", "branch": "wt/tove/LED-44", "summary": "cal v0: all 6 samples match"})
	if text != "msg-1: wt/tove/LED-44 handed to Mara. Your part ends here." {
		t.Errorf("%q", text)
	}
	if subject := f.git(f.wt("tove", "api"), "log", "-1", "--format=%s"); subject != "cal v0" {
		t.Errorf("handoff committed something: %q", subject)
	}
	if len(f.m.sent) != 1 {
		t.Fatalf("sent %v", f.m.sent)
	}
	msg := f.m.sent[0]
	if msg.Kind != "handoff" || msg.ToInstance != "mara@ledger" || msg.FromInstance != "tove@ledger" || msg.ArtifactRef != "wt/tove/LED-44" || msg.Body != "cal v0: all 6 samples match" {
		t.Errorf("message %+v", msg)
	}
}

func TestHandoffRefusals(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("tove", "api"), "draft"), "x\n")
	f.refused("tove", "handoff", map[string]string{"teammate": "rhea", "branch": "wt/tove/main", "summary": "x"}, "unknown_teammate")
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ivo/main", "summary": "x"}, "branch_not_allowed")
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "main", "summary": "x"}, "branch_not_allowed")
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/tove/main", "summary": ""}, "empty")
	// No refusal commits anything.
	if status := f.git(f.wt("tove", "api"), "status", "--porcelain"); !strings.Contains(status, "draft") {
		t.Errorf("a refused hand-off committed: %q", status)
	}
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/tove/main", "summary": "x"}, "uncommitted_changes")
	os.Remove(filepath.Join(f.wt("tove", "api"), "draft"))
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/tove/never", "summary": "x"}, "branch_not_found")
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
	path := "project/wiki/inbox/2026-10-16-tove-flaky-settle-test.md"
	if text != "Filed "+path+"." {
		t.Errorf("%q", text)
	}
	data, err := os.ReadFile(filepath.Join(f.root, "ledger", path))
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
	f.refused("tove", "contribute", map[string]string{"kind": "note", "title": strings.Repeat("x", maxTitle+1), "body": "x"}, "too_long")
	f.refused("tove", "contribute", map[string]string{"kind": "note", "title": "x", "body": strings.Repeat("x", teamtools.MaxBody+1)}, "too_long")
	if entries, _ := os.ReadDir(filepath.Join(f.root, "ledger", "project", "wiki", "inbox")); len(entries) != 0 {
		t.Errorf("a refusal filed %d notes", len(entries))
	}
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
	f.ok("tove", "commit", map[string]string{"message": "tove's readme"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "mara\n")
	f.ok("mara", "commit", map[string]string{"message": "mara's readme"})
	f.ok("mara", "merge", map[string]string{"branch": "wt/tove/main"})
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
	f.ok("tove", "switch", map[string]string{"branch": "wt/tove/LED-44"})
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal\n")
	f.ok("tove", "commit", map[string]string{"message": "cal v0"})
	f.refused("tove", "switch", map[string]string{"branch": "main"}, "branch_not_allowed")
	f.ok("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/tove/LED-44", "summary": "done"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "mara\n")
	f.ok("mara", "commit", map[string]string{"message": "mara's readme"})
	writeFile(t, filepath.Join(f.wt("tove", "api"), "README"), "tove\n")
	f.ok("tove", "commit", map[string]string{"message": "tove's readme"})
	f.ok("mara", "merge", map[string]string{"branch": "wt/tove/LED-44"})
	f.ok("mara", "abort_merge", nil)

	var tools []string
	for _, a := range kept.kept {
		tools = append(tools, a.Tool)
	}
	if strings.Join(tools, " ") != "switch commit switch commit commit merge abort_merge" {
		t.Fatalf("kept %v", tools)
	}
	sw, commit, refused, merge, abort := kept.kept[0], kept.kept[1], kept.kept[2], kept.kept[5], kept.kept[6]
	if sw.Instance != "tove@ledger" || sw.DeploymentID != "atlas@ledger" || sw.Branch != "wt/tove/LED-44" || len(sw.Repos) != 2 || !sw.Repos[0].Created {
		t.Errorf("switch %+v", sw)
	}
	if commit.Message != "cal v0" || len(commit.Repos) != 1 || commit.Repos[0].Repo != "api" || commit.Repos[0].Branch != "wt/tove/LED-44" || len(commit.Repos[0].Commit) != 40 {
		t.Errorf("commit %+v", commit)
	}
	if !strings.HasPrefix(refused.Error, "branch_not_allowed:") || len(refused.Repos) != 0 {
		t.Errorf("refused %+v", refused)
	}
	if merge.Branch != "wt/tove/LED-44" || merge.Error != "" || len(merge.Repos) != 2 {
		t.Fatalf("merge %+v", merge)
	}
	for _, r := range merge.Repos {
		if r.Branch != "lead/atlas/integration" || (r.Repo == "api") != (len(r.Conflicts) == 1) {
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
	for _, r := range repos {
		writeFile(f.t, filepath.Join(f.wt("tove", r), branch[len("wt/tove/"):]+".go"), fmt.Sprintf("package x // %d\n", len(f.m.sent)))
	}
	f.ok("tove", "commit", map[string]string{"message": "work on " + branch})
	f.ok("tove", "handoff", map[string]string{"teammate": "mara", "branch": branch, "summary": "done"})
}

func TestWorkItemsFollowHandoffDeliveryAndIntegration(t *testing.T) {
	f := newFixture(t)
	f.handOff("wt/tove/LED-44", "api", "web")
	w := f.item("wt/tove/LED-44")
	if w.ID != "wi-1" || w.Stage != store.StageHanded || w.AuthorInstance != "tove@ledger" {
		t.Fatalf("after the hand-off: %+v", w)
	}
	if len(f.events) == 0 || f.events[len(f.events)-1].Type != "pipeline.changed" {
		t.Errorf("no pipeline.changed: %v", f.events)
	}

	// Delivered into mara's session: she's reviewing it. Other deliveries move nothing.
	delivered := store.TeamMessage{DeploymentID: "atlas@ledger", Kind: "handoff", ArtifactRef: "wt/tove/LED-44", State: store.MessageDelivered}
	f.w.Delivered(ctx, store.TeamMessage{DeploymentID: "atlas@ledger", Kind: "handoff", ArtifactRef: "wt/tove/LED-44", State: store.MessageQueued})
	f.w.Delivered(ctx, store.TeamMessage{DeploymentID: "atlas@ledger", Kind: "ask", ArtifactRef: "wt/tove/LED-44", State: store.MessageDelivered})
	if f.item("wt/tove/LED-44").Stage != store.StageHanded {
		t.Error("a queued hand-off, or an ask, moved the work item")
	}
	f.w.Delivered(ctx, delivered)
	if f.item("wt/tove/LED-44").Stage != store.StageReviewing {
		t.Error("the delivery didn't move it to reviewing")
	}

	// Tove's own merges never integrate: only the lead's.
	f.ok("tove", "merge", map[string]string{"branch": "main"})
	if f.item("wt/tove/LED-44").Stage != store.StageReviewing {
		t.Error("a member's merge integrated it")
	}
	// Merged into the lead branch in one repo only: not yet.
	f.git(f.wt("mara", "api"), "-c", "user.name=t", "-c", "user.email=t@t", "merge", "-q", "--no-ff", "-m", "api only", "wt/tove/LED-44")
	f.ok("mara", "merge", map[string]string{"branch": "main"}) // any merge of hers checks
	if f.item("wt/tove/LED-44").Stage != store.StageReviewing {
		t.Error("integrated with web still missing")
	}
	f.ok("mara", "merge", map[string]string{"branch": "wt/tove/LED-44"})
	if got := f.item("wt/tove/LED-44"); got.Stage != store.StageIntegrated {
		t.Errorf("after the lead's merge: %+v", got)
	}

	// A new hand-off of the same branch (after a change request) brings it back.
	f.handOff("wt/tove/LED-44", "api")
	if got := f.item("wt/tove/LED-44"); got.ID != "wi-1" || got.Stage != store.StageHanded {
		t.Errorf("handed again: %+v", got)
	}
}

func TestIntegrationIsPerBranchAndInAnyLeadBranch(t *testing.T) {
	f := newFixture(t)
	f.handOff("wt/tove/a", "api")
	f.handOff("wt/tove/b", "web")
	// Mara merges b into another lead branch of hers: b is integrated, a isn't.
	f.ok("mara", "switch", map[string]string{"branch": "lead/atlas/settlement"})
	f.ok("mara", "merge", map[string]string{"branch": "wt/tove/b"})
	if a, b := f.item("wt/tove/a"), f.item("wt/tove/b"); a.Stage != store.StageHanded || b.Stage != store.StageIntegrated {
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

// integrated: tove's wt/tove/LED-44 (api only) handed off and merged by mara.
func (f *fixture) integrated() {
	f.t.Helper()
	f.handOff("wt/tove/LED-44", "api")
	f.ok("mara", "merge", map[string]string{"branch": "wt/tove/LED-44"})
}

func TestRequestMergeRefusals(t *testing.T) {
	f := newFixture(t)
	req := func(branch, entry string) map[string]string {
		return map[string]string{"branch": branch, "entry": entry}
	}
	f.refused("tove", "request_merge", req("wt/tove/main", "backlog/led-44.md"), "not_lead")
	f.refused("mara", "request_merge", req("wt/tove/main", "backlog/led-44.md"), "branch_not_allowed")
	f.refused("mara", "request_merge", req("lead/orion/x", "backlog/led-44.md"), "branch_not_allowed")
	f.refused("mara", "request_merge", req("lead/atlas/integration", ""), "entry_not_found")
	f.refused("mara", "request_merge", req("lead/atlas/integration", "backlog/nope.md"), "entry_not_found")
	f.refused("mara", "request_merge", req("lead/atlas/integration", "backlog/led-44.md"), "nothing_to_merge")
	f.refused("mara", "request_merge", req("lead/atlas/none", "backlog/led-44.md"), "branch_not_found")
	writeFile(t, filepath.Join(f.wt("mara", "web"), "wip"), "x\n")
	f.refused("mara", "request_merge", req("lead/atlas/integration", "backlog/led-44.md"), "uncommitted_changes")
}

func TestFromHandoffToMain(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	req := map[string]string{"branch": "lead/atlas/integration", "entry": "backlog/led-44.md"}
	// One merge request, in the one repo with new commits, carrying the hand-off.
	if text := f.ok("mara", "request_merge", req); text != "Merge request mr-1 (api): waiting for the CEO." {
		t.Fatalf("%q", text)
	}
	if w := f.item("wt/tove/LED-44"); w.MergeRequestID != "mr-1" {
		t.Errorf("work item %+v", w)
	}
	if text := f.ok("mara", "request_merge", req); text != "Merge request mr-1 (api), refreshed: waiting for the CEO." {
		t.Errorf("again: %q", text)
	}
	p, _ := f.w.Pipeline(ctx, "ledger")
	if len(p.MergeRequests) != 1 || p.MergeRequests[0].Entry != "backlog/led-44.md" || p.MergeRequests[0].AuthorInstance != "mara@ledger" {
		t.Fatalf("pipeline %+v", p.MergeRequests)
	}
	if diff, err := f.w.Diff(ctx, "mr-1"); err != nil || !strings.Contains(diff, "LED-44.go") {
		t.Errorf("diff %q %v", diff, err)
	}

	// The CEO's checkout of api is on main: the merge happens there.
	api := filepath.Join(f.root, "ledger", "api")
	if err := f.w.MergeToMain(ctx, "mr-1"); err != nil {
		t.Fatal(err)
	}
	if subject := f.git(api, "log", "-1", "--format=%s", "main"); subject != "Merge Export invoices" {
		t.Errorf("main's head %q", subject)
	}
	if _, err := os.Stat(filepath.Join(api, "LED-44.go")); err != nil {
		t.Error("the CEO's checkout didn't move with main")
	}
	mr, _ := f.w.MergeRequest(ctx, "mr-1")
	if mr.State != store.MergeMerged || mr.MergedAt.IsZero() {
		t.Errorf("merge request %+v", mr)
	}
	if n := f.lastNotice(); n.ToInstance != "mara@ledger" || n.FromInstance != "foreman" || !strings.Contains(n.Body, "lead/atlas/integration was merged into main in api.") {
		t.Errorf("notice %+v", n)
	}
	if err := f.w.MergeToMain(ctx, "mr-1"); !errors.Is(err, ErrNotOpen) {
		t.Errorf("merged twice: %v", err)
	}
	if err := f.w.Reject(ctx, "mr-1", "x"); !errors.Is(err, ErrNotOpen) {
		t.Errorf("rejected once merged: %v", err)
	}
}

func TestMergeToMainConflicts(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "the lead's\n")
	f.ok("mara", "commit", map[string]string{"message": "readme"})
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/integration", "entry": "backlog/led-44.md"})
	api := filepath.Join(f.root, "ledger", "api")
	writeFile(t, filepath.Join(api, "README"), "main's\n")
	f.git(api, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", "main moved")
	head := f.git(api, "rev-parse", "main")
	if err := f.w.MergeToMain(ctx, "mr-1"); !errors.Is(err, ErrConflict) {
		t.Fatalf("err %v", err)
	}
	if f.git(api, "rev-parse", "main") != head {
		t.Error("main moved on a conflict")
	}
	if mr, _ := f.w.MergeRequest(ctx, "mr-1"); !mr.Conflicting || mr.State != store.MergeOpen {
		t.Errorf("merge request %+v", mr)
	}
	if n := f.lastNotice(); !strings.Contains(n.Body, `merge("main")`) {
		t.Errorf("notice %+v", n)
	}
	// The lead merges main in, resolves, asks again: the conflict is gone, and the merge goes through.
	f.ok("mara", "merge", map[string]string{"branch": "main"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "README"), "both\n")
	f.ok("mara", "commit", map[string]string{"message": "main merged in"})
	if text := f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/integration", "entry": "backlog/led-44.md"}); !strings.Contains(text, "mr-1 (api), refreshed") {
		t.Errorf("%q", text)
	}
	if mr, _ := f.w.MergeRequest(ctx, "mr-1"); mr.Conflicting {
		t.Error("still marked conflicting")
	}
	if err := f.w.MergeToMain(ctx, "mr-1"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(api, "README")); string(data) != "both\n" {
		t.Errorf("README %q", data)
	}
}

func TestRejectReturnsTheWork(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/integration", "entry": "backlog/led-44.md"})
	if err := f.w.Reject(ctx, "mr-1", "split it in two"); err != nil {
		t.Fatal(err)
	}
	mr, _ := f.w.MergeRequest(ctx, "mr-1")
	if mr.State != store.MergeRejected || mr.Note != "split it in two" || mr.RejectedAt.IsZero() {
		t.Errorf("merge request %+v", mr)
	}
	if w := f.item("wt/tove/LED-44"); w.MergeRequestID != "" || w.Stage != store.StageInWorktree || w.Note != "rejected: split it in two" {
		t.Errorf("work item %+v", w)
	}
	if n := f.lastNotice(); n.ToInstance != "mara@ledger" || n.FromInstance != "ceo" || !strings.Contains(n.Body, "the merge of lead/atlas/integration in api was rejected: split it in two") {
		t.Errorf("notice %+v", n)
	}
	// The branch stays: the lead asks again, a new merge request.
	if text := f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/integration", "entry": "backlog/led-44.md"}); !strings.HasPrefix(text, "Merge request mr-2 (api)") {
		t.Errorf("%q", text)
	}
}

func TestMergesOutsideForemanAreNoticedAndMainMovedIsTold(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/integration", "entry": "backlog/led-44.md"})
	f.ok("mara", "switch", map[string]string{"branch": "lead/atlas/second"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "second.go"), "package s\n")
	f.ok("mara", "commit", map[string]string{"message": "second"})
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/second", "entry": "backlog/led-44.md"})
	// The CEO merges the first by hand, in their own checkout.
	api := filepath.Join(f.root, "ledger", "api")
	f.git(api, "-c", "user.name=t", "-c", "user.email=t@t", "merge", "-q", "--no-ff", "-m", "by hand", "lead/atlas/integration")
	sent := len(f.m.sent)
	f.w.Reconcile(ctx)
	if mr, _ := f.w.MergeRequest(ctx, "mr-1"); mr.State != store.MergeMerged {
		t.Errorf("not noticed: %+v", mr)
	}
	if mr, _ := f.w.MergeRequest(ctx, "mr-2"); mr.State != store.MergeOpen {
		t.Errorf("the other one moved: %+v", mr)
	}
	// Its lead is told it's merged, and the other open one's that main moved.
	if news := f.m.sent[sent:]; len(news) != 2 || !strings.Contains(news[0].Body, "lead/atlas/integration was merged into main in api.") || !strings.Contains(news[1].Body, "main moved (lead/atlas/integration was merged). Merge it into lead/atlas/second") {
		t.Errorf("notices %+v", news)
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
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/integration", "entry": "backlog/led-44.md"})
	f.refused("mara", "request_merge", map[string]string{"branch": "lead/atlas/integration", "entry": "backlog/nope.md"}, "entry_not_found")
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
	if a := mine[0]; a.Instance != "mara@ledger" || a.Branch != "lead/atlas/integration" || len(a.Repos) != 1 || a.Repos[0].Repo != "api" || a.Repos[0].MergeRequest != "mr-1" {
		t.Errorf("action %+v", a)
	}
	if a := mine[1]; !strings.Contains(a.Error, "entry_not_found") && !strings.Contains(a.Error, "no wiki entry") {
		t.Errorf("refusal %+v", a)
	}
}

// A merge request waits for the CEO as a review request, until it's merged or rejected.
func TestReviewRequests(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	req := map[string]string{"branch": "lead/atlas/integration", "entry": "backlog/led-44.md"}
	f.ok("mara", "request_merge", req)
	f.ok("mara", "request_merge", req) // a refresh raises no second one
	open, _ := f.w.OpenRequests(ctx)
	if len(open) != 1 {
		t.Fatalf("open %+v", open)
	}
	if r := open[0]; r.Kind != "review" || r.MergeRequestID != "mr-1" || r.InstanceID != "mara@ledger" || r.ProjectID != "ledger" ||
		r.Source != "agent" || r.Summary != "Merge lead/atlas/integration into main in api: Export invoices." {
		t.Errorf("review %+v", r)
	}
	if err := f.w.MergeToMain(ctx, "mr-1"); err != nil {
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

	// Rejected: resolved too.
	f.ok("mara", "switch", map[string]string{"branch": "lead/atlas/second"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "second.go"), "package s\n")
	f.ok("mara", "commit", map[string]string{"message": "second"})
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/second", "entry": "backlog/led-44.md"})
	if err := f.w.Reject(ctx, "mr-2", "no"); err != nil {
		t.Fatal(err)
	}
	if open, _ := f.w.OpenRequests(ctx); len(open) != 0 {
		t.Errorf("still open after the rejection: %+v", open)
	}

	// One made before reviews existed gets its review at the next reconcile.
	if _, err := f.db.MergeRequests().Save(ctx, store.MergeRequest{ProjectID: "ledger", DeploymentID: "atlas@ledger", AuthorInstance: "mara@ledger",
		Repo: "api", Branch: "lead/atlas/second", Entry: "backlog/led-44.md"}); err != nil {
		t.Fatal(err)
	}
	f.w.Reconcile(ctx)
	f.w.Reconcile(ctx)
	if open, _ := f.w.OpenRequests(ctx); len(open) != 1 || open[0].MergeRequestID != "mr-3" {
		t.Errorf("backfilled %+v", open)
	}
}
