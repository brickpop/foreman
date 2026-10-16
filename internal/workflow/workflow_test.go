package workflow

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
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
	t    *testing.T
	root string
	w    *Workflow
	m    *messages
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
	return &fixture{t: t, root: root, m: m, w: New(Deps{Root: root, Git: g, Config: config{}, Messages: m, Now: now})}
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
