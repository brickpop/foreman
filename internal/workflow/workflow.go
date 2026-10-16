// Package workflow is the Workflow block (wiki/product/architecture/building-blocks.md#7-workflow-workflow).
// Today it holds the team tools that write git and the knowledge base for an
// agent: switch, commit, merge and abort_merge in its own worktrees, handoff
// (a message naming a committed branch, which Foreman records) and contribute
// (a note in the project's wiki inbox).
// Agents can't write git themselves: each repo's .git is read-only for them
// (wiki/product/architecture/sandboxes-and-git.md#agents-and-git).
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
	"github.com/brickpop/foreman/internal/workspace"
)

// Git is what Workflow needs of the Git block.
type Git interface {
	Clean(ctx context.Context, dir string) (bool, error)
	Branch(ctx context.Context, dir string) (string, error)
	HasBranch(ctx context.Context, dir, branch string) bool
	Ahead(ctx context.Context, dir, branch, base string) (int, error)
	Switch(ctx context.Context, dir, branch, base string) (bool, error)
	CommitAll(ctx context.Context, dir, message string, by git.Author) (string, error)
	CommitAs(ctx context.Context, dir, message string, by git.Author, files ...string) error
	Merge(ctx context.Context, dir, branch string, by git.Author) ([]string, error)
	Merging(ctx context.Context, dir string) (bool, error)
	AbortMerge(ctx context.Context, dir string) error
}

// Config is the projects, as loaded.
type Config interface {
	Project(id string) (workspace.Project, error)
}

// Messages sends a message to a teammate (Messaging).
type Messages interface {
	Send(ctx context.Context, msg store.TeamMessage) (store.TeamMessage, error)
}

// Deps are the blocks Workflow is assembled from.
type Deps struct {
	Root     string // the workspace folder: projects are its subfolders
	Git      Git
	Config   Config
	Messages Messages
	Now      func() time.Time // the contributions' dates; time.Now when nil
}

// Workflow serves its team tools.
type Workflow struct{ Deps }

// New returns the Workflow block.
func New(d Deps) *Workflow {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Workflow{Deps: d}
}

var _ teamtools.Block = (*Workflow)(nil)

func object(props string, required ...string) json.RawMessage {
	req, _ := json.Marshal(required)
	return json.RawMessage(`{"type":"object","properties":{` + props + `},"required":` + string(req) + `}`)
}

var definitions = []agentlink.Tool{
	{Name: "switch", Description: "Switch your worktrees, in every repo, to one of your branches, making it from main if it is new. Commit first: a switch never carries changes along.",
		InputSchema: object(`"branch":{"type":"string","description":"One of your branches: wt/<you>/<topic> for a member, lead/<team>/<topic> for the lead"}`, "branch")},
	{Name: "commit", Description: "Commit everything you changed, in every repo, on the branch checked out there (git is read-only for you). After an update with conflicts, it concludes the merge once no conflict markers are left.",
		InputSchema: object(`"message":{"type":"string","description":"The commit message"}`, "message")},
	{Name: "merge", Description: "Merge a branch (main, the lead's, a teammate's) into yours, in every repo that has it. Conflict markers stay in the files: resolve them, then commit() to finish the merge, or abort_merge() to back out.",
		InputSchema: object(`"branch":{"type":"string","description":"The branch to merge in, e.g. main"}`, "branch")},
	{Name: "abort_merge", Description: "Back out of an unfinished merge, in every repo where one is under way: your branch and files are as they were before it.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
	{Name: "handoff", Description: "Pass finished, committed work on a branch of yours to a teammate (usually the lead): they get your summary and the branch, and Foreman records the hand-off. Commit first. Your part ends.",
		InputSchema: object(`"teammate":{"type":"string","description":"A teammate's name or id"},`+
			`"branch":{"type":"string","description":"wt/<you>/<topic>"},`+
			`"summary":{"type":"string","description":"The outcome: what was done, decisions, what is left open, how it was checked"}`, "teammate", "branch", "summary")},
	{Name: "contribute", Description: "Add a finding, a proposal or a note to the project's knowledge base without writing it yourself: Foreman files it in the wiki's inbox and tells the lead.",
		InputSchema: object(`"kind":{"type":"string","enum":["finding","proposal","note"]},`+
			`"title":{"type":"string","description":"A short title"},`+
			`"body":{"type":"string","description":"The contribution, in Markdown"}`, "kind", "title", "body")},
}

// Tools implements teamtools.Block.
func (w *Workflow) Tools() []agentlink.Tool { return definitions }

// Call implements teamtools.Block.
func (w *Workflow) Call(ctx context.Context, c teamtools.Caller, name string, args json.RawMessage) agentlink.Result {
	var a struct {
		Branch   string `json:"branch"`
		Message  string `json:"message"`
		Teammate string `json:"teammate"`
		Summary  string `json:"summary"`
		Kind     string `json:"kind"`
		Title    string `json:"title"`
		Body     string `json:"body"`
	}
	if res, ok := teamtools.Args(args, &a); !ok {
		return res
	}
	project, err := w.Config.Project(c.Deployment.ProjectID)
	if err != nil {
		return teamtools.Refusal("git_error", "the project: %v", err)
	}
	t := tool{w: w, c: c, project: project}
	switch name {
	case "switch":
		return t.switchTo(ctx, strings.TrimSpace(a.Branch))
	case "commit":
		return t.commit(ctx, strings.TrimSpace(a.Message))
	case "merge":
		return t.merge(ctx, strings.TrimSpace(a.Branch))
	case "abort_merge":
		return t.abortMerge(ctx)
	case "handoff":
		return t.handoff(ctx, a.Teammate, strings.TrimSpace(a.Branch), strings.TrimSpace(a.Summary))
	default: // contribute
		return t.contribute(ctx, strings.TrimSpace(a.Kind), strings.TrimSpace(a.Title), strings.TrimSpace(a.Body))
	}
}

// tool is one call: the caller and its project.
type tool struct {
	w       *Workflow
	c       teamtools.Caller
	project workspace.Project
}

// repo is one of the caller's worktrees, or the live project repo.
type repo struct{ name, dir string }

// worktrees are the caller's worktrees, one per code repo of the project
// that it has (a repo added since its deploy has none yet).
func (t tool) worktrees() []repo {
	var repos []repo
	for _, name := range t.project.Repos {
		dir := filepath.Join(t.c.Instance.SandboxPath, name)
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			repos = append(repos, repo{name, dir})
		}
	}
	return repos
}

func (t tool) projectRepo() repo {
	return repo{"project", filepath.Join(t.w.Root, t.project.ID, "project")}
}

// author is the caller, as its commits name it.
func (t tool) author() git.Author {
	return git.Author{Name: t.c.Name(), Email: t.c.Instance.AgentID + "@foreman.invalid"}
}

// own says whether branch is one of the caller's: wt/<agent>/<topic> for a
// member, lead/<team>/<topic> for the lead.
func (t tool) own(branch string) (string, bool) {
	prefix := "wt/" + t.c.Instance.AgentID + "/"
	if t.c.Lead {
		prefix = "lead/" + t.c.Team.ID + "/"
	}
	topic, ok := strings.CutPrefix(branch, prefix)
	return prefix + "<topic>", ok && topic != ""
}

func gitError(err error) agentlink.Result {
	return teamtools.Refusal("git_error", "%v", err)
}

// dirty are the repos with changes or an unfinished merge.
func (t tool) dirty(ctx context.Context, repos []repo) ([]string, error) {
	var names []string
	for _, r := range repos {
		clean, err := t.w.Git.Clean(ctx, r.dir)
		if err != nil {
			return nil, err
		}
		if !clean {
			names = append(names, r.name)
		}
	}
	return names, nil
}

func names(repos []repo) string {
	var n []string
	for _, r := range repos {
		n = append(n, r.name)
	}
	return strings.Join(n, ", ")
}

func (t tool) switchTo(ctx context.Context, branch string) agentlink.Result {
	if !git.ValidBranch(ctx, branch) {
		return teamtools.Refusal("bad_branch", "%q isn't a valid branch name", branch)
	}
	if pattern, ok := t.own(branch); !ok {
		return teamtools.Refusal("branch_not_allowed", "%s isn't one of your branches: yours are %s", branch, pattern)
	}
	repos := t.worktrees()
	if len(repos) == 0 {
		return teamtools.Refusal("no_repos", "you have no worktree in this project")
	}
	// Checked in every repo first, so a switch never stops halfway.
	if dirty, err := t.dirty(ctx, repos); err != nil {
		return gitError(err)
	} else if len(dirty) > 0 {
		return teamtools.Refusal("uncommitted_changes", "commit first, in %s: a switch never carries changes along", strings.Join(dirty, ", "))
	}
	var created []string
	for _, r := range repos {
		made, err := t.w.Git.Switch(ctx, r.dir, branch, t.project.Config.Main)
		if err != nil {
			return gitError(fmt.Errorf("%s: %w", r.name, err))
		}
		if made {
			created = append(created, r.name)
		}
	}
	text := fmt.Sprintf("Switched %s to %s.", names(repos), branch)
	if len(created) > 0 {
		text += fmt.Sprintf(" New in %s, made from %s.", strings.Join(created, ", "), t.project.Config.Main)
	}
	return agentlink.Result{Text: text}
}

// commitAll commits every repo of the caller with changes; for the lead, the
// live project repo too.
func (t tool) commitAll(ctx context.Context, message string) (done []string, conflicts []string, err error) {
	repos := t.worktrees()
	if t.c.Lead {
		repos = append(repos, t.projectRepo())
	}
	for _, r := range repos {
		sha, err := t.w.Git.CommitAll(ctx, r.dir, message, t.author())
		var left *git.ConflictsError
		switch {
		case errors.Is(err, git.ErrNothingToCommit):
		case errors.As(err, &left):
			for _, f := range left.Files {
				conflicts = append(conflicts, r.name+"/"+f)
			}
		case err != nil:
			return done, conflicts, fmt.Errorf("%s: %w", r.name, err)
		default:
			branch, _ := t.w.Git.Branch(ctx, r.dir)
			done = append(done, fmt.Sprintf("%s (%s, %.8s)", r.name, branch, sha))
		}
	}
	return done, conflicts, nil
}

func (t tool) commit(ctx context.Context, message string) agentlink.Result {
	if message == "" {
		return teamtools.Refusal("empty", "give a commit message")
	}
	done, conflicts, err := t.commitAll(ctx, message)
	switch {
	case err != nil:
		return gitError(err)
	case len(conflicts) > 0:
		text := "conflict markers are still in " + strings.Join(conflicts, ", ") + ": resolve them, then commit again"
		if len(done) > 0 {
			text += ". Committed meanwhile: " + strings.Join(done, ", ")
		}
		return teamtools.Refusal("conflicts_left", "%s", text)
	case len(done) == 0:
		return teamtools.Refusal("nothing_to_commit", "nothing changed since your last commit")
	}
	return agentlink.Result{Text: "Committed " + strings.Join(done, ", ") + "."}
}

func (t tool) merge(ctx context.Context, branch string) agentlink.Result {
	if !git.ValidBranch(ctx, branch) {
		return teamtools.Refusal("bad_branch", "%q isn't a valid branch name", branch)
	}
	var repos []repo
	for _, r := range t.worktrees() {
		if t.w.Git.HasBranch(ctx, r.dir, branch) {
			repos = append(repos, r)
		}
	}
	if len(repos) == 0 {
		return teamtools.Refusal("branch_not_found", "no branch %q in your repos", branch)
	}
	if dirty, err := t.dirty(ctx, repos); err != nil {
		return gitError(err)
	} else if len(dirty) > 0 {
		return teamtools.Refusal("uncommitted_changes", "commit first, in %s: a merge never mixes with unsaved work", strings.Join(dirty, ", "))
	}
	var merged, conflicted []string
	for _, r := range repos {
		files, err := t.w.Git.Merge(ctx, r.dir, branch, t.author())
		if err != nil {
			return gitError(fmt.Errorf("%s: %w", r.name, err))
		}
		if len(files) > 0 {
			conflicted = append(conflicted, fmt.Sprintf("%s (%s)", r.name, strings.Join(files, ", ")))
		} else {
			merged = append(merged, r.name)
		}
	}
	var text []string
	if len(merged) > 0 {
		text = append(text, fmt.Sprintf("Merged %s into %s.", branch, strings.Join(merged, ", ")))
	}
	if len(conflicted) > 0 {
		text = append(text, fmt.Sprintf("Conflicts in %s: resolve the markers, then commit() to finish the merge, or abort_merge() to back out.", strings.Join(conflicted, "; ")))
	}
	return agentlink.Result{Text: strings.Join(text, " ")}
}

func (t tool) handoff(ctx context.Context, teammate, branch, summary string) agentlink.Result {
	to, res, ok := t.c.Teammate(teammate)
	if !ok {
		return res
	}
	if pattern, own := t.own(branch); !own {
		return teamtools.Refusal("branch_not_allowed", "you hand off only your own branches, %s", pattern)
	}
	if summary == "" {
		return teamtools.Refusal("empty", "give the outcome in summary")
	}
	if len(summary) > teamtools.MaxBody {
		return teamtools.Refusal("too_long", "a summary is at most %d characters; put the rest in the branch or a wiki entry", teamtools.MaxBody)
	}
	// Nothing pending: what is handed off is exactly what is committed.
	if dirty, err := t.dirty(ctx, t.worktrees()); err != nil {
		return gitError(err)
	} else if len(dirty) > 0 {
		return teamtools.Refusal("uncommitted_changes", "commit first, in %s: a hand-off is your committed work", strings.Join(dirty, ", "))
	}
	found, ahead := false, false
	for _, r := range t.worktrees() {
		if !t.w.Git.HasBranch(ctx, r.dir, branch) {
			continue
		}
		found = true
		n, err := t.w.Git.Ahead(ctx, r.dir, branch, t.project.Config.Main)
		if err != nil {
			return gitError(err)
		}
		ahead = ahead || n > 0
	}
	switch {
	case !found:
		return teamtools.Refusal("branch_not_found", "no branch %q in your repos", branch)
	case !ahead:
		return teamtools.Refusal("nothing_new", "%s has no commits beyond %s", branch, t.project.Config.Main)
	}
	msg, err := t.w.Messages.Send(ctx, store.TeamMessage{DeploymentID: t.c.Deployment.ID, FromInstance: t.c.Instance.ID,
		ToInstance: to, Kind: "handoff", Body: summary, ArtifactRef: branch})
	if err != nil {
		return teamtools.Refusal("internal", "%v", err)
	}
	name := t.c.Team.Config.Agents[strings.Split(to, "@")[0]].Name
	return agentlink.Result{Text: fmt.Sprintf("%s: %s handed to %s. Your part ends here.", msg.Ref, branch, name)}
}

// maxTitle caps a contribution's title.
const maxTitle = 120

var kinds = []string{"finding", "proposal", "note"}

func (t tool) contribute(ctx context.Context, kind, title, body string) agentlink.Result {
	switch {
	case !slices.Contains(kinds, kind):
		return teamtools.Refusal("bad_kind", "kind is finding, proposal or note")
	case title == "" || body == "":
		return teamtools.Refusal("empty", "give a title and a body")
	case len(title) > maxTitle || strings.ContainsAny(title, "\r\n"):
		return teamtools.Refusal("too_long", "a title is one line of at most %d characters", maxTitle)
	case len(body) > teamtools.MaxBody:
		return teamtools.Refusal("too_long", "a body is at most %d characters", teamtools.MaxBody)
	}
	project := t.projectRepo()
	inbox := filepath.Join(project.dir, "wiki", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		return gitError(err)
	}
	content := fmt.Sprintf("---\ntype: note\nkind: %s\nauthor: %s\ndeployment: %s\n---\n\n# %s\n\n%s\n",
		kind, t.c.Instance.AgentID, t.c.Deployment.ID, title, body)
	file, err := create(inbox, fmt.Sprintf("%s-%s-%s", t.w.Now().Format("2006-01-02"), t.c.Instance.AgentID, slug(title)), content)
	if err != nil {
		return gitError(err)
	}
	rel := filepath.ToSlash(filepath.Join("wiki", "inbox", file))
	if err := t.w.Git.CommitAs(ctx, project.dir, "Contribution: "+title, t.author(), rel); err != nil {
		return gitError(err)
	}
	path := "project/" + rel
	if !t.c.Lead {
		if _, err := t.w.Messages.Send(ctx, store.TeamMessage{DeploymentID: t.c.Deployment.ID, FromInstance: t.c.Instance.ID,
			ToInstance: t.c.LeadInstance(), Kind: "contribute", Body: title + ", in " + path + ".", ArtifactRef: path}); err != nil {
			return teamtools.Refusal("internal", "%v", err)
		}
	}
	return agentlink.Result{Text: "Filed " + path + "."}
}

// create writes a new file named base.md in dir (base-2.md, … when taken),
// never over another, and returns its name.
func create(dir, base, content string) (string, error) {
	for n := 1; n < 100; n++ {
		name := base + ".md"
		if n > 1 {
			name = fmt.Sprintf("%s-%d.md", base, n)
		}
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, err = f.WriteString(content)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return name, err
	}
	return "", fmt.Errorf("too many contributions named %s", base)
}

// slug is a title as a file name: lowercase letters and digits, hyphens
// between words, at most 60 characters, cut between words.
func slug(title string) string {
	words := strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	s := strings.Join(words, "-")
	if len(s) > 60 {
		s = s[:60]
		if i := strings.LastIndexByte(s, '-'); i > 0 {
			s = s[:i]
		}
	}
	if s == "" {
		return "contribution"
	}
	return s
}

func (t tool) abortMerge(ctx context.Context) agentlink.Result {
	var aborted []string
	for _, r := range t.worktrees() {
		merging, err := t.w.Git.Merging(ctx, r.dir)
		if err != nil {
			return gitError(err)
		}
		if !merging {
			continue
		}
		if err := t.w.Git.AbortMerge(ctx, r.dir); err != nil {
			return gitError(fmt.Errorf("%s: %w", r.name, err))
		}
		aborted = append(aborted, r.name)
	}
	if len(aborted) == 0 {
		return teamtools.Refusal("no_merge", "no merge is under way in your repos")
	}
	return agentlink.Result{Text: fmt.Sprintf("Merge aborted in %s: your branch is as it was before it.", strings.Join(aborted, ", "))}
}
