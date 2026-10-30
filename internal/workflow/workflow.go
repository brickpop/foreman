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
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
	"github.com/brickpop/foreman/internal/wiki"
)

// Git is what Workflow needs of the Git block.
type Git interface {
	Clean(ctx context.Context, dir string) (bool, error)
	Branch(ctx context.Context, dir string) (string, error)
	HasBranch(ctx context.Context, dir, branch string) bool
	Switch(ctx context.Context, dir, branch, base string) (bool, error)
	CommitAll(ctx context.Context, dir, message string, by git.Author) (string, error)
	CommitAs(ctx context.Context, dir, message string, by git.Author, files ...string) error
	Merge(ctx context.Context, dir, branch string, by git.Author) ([]string, error)
	Merging(ctx context.Context, dir string) (bool, error)
	AbortMerge(ctx context.Context, dir string) error
	Branches(ctx context.Context, dir, prefix string) ([]string, error)
	Contains(ctx context.Context, dir, into, branch string) (bool, error)
	Merged(ctx context.Context, dir, into, branch string) (bool, error)
	Squash(ctx context.Context, dir, target, branch, head, message string) error
	MoveBranch(ctx context.Context, dir, branch, from, to string) error
	DropBranch(ctx context.Context, dir, branch, backup, main string) error
	Discard(ctx context.Context, worktree, backup string) (bool, error)
	Freshen(ctx context.Context, repo, branch, main string) (bool, error)
	PruneBackups(ctx context.Context, dir string, before time.Time) error
	Diff(ctx context.Context, dir, target, branch string) (string, error)
	Compare(ctx context.Context, dir, from, to string, lines int, paths ...string) (string, error)
	Resolve(ctx context.Context, dir, ref string) (string, error)
	SyncSubmodules(ctx context.Context, dir string, recursive bool) error
	DirtySubmodules(ctx context.Context, dir string) ([]string, error)
	SetSubmodule(ctx context.Context, dir, path, ref, url, message string, by git.Author) (string, error)
}

// Config is the projects, as loaded.
type Config interface {
	Project(id string) (home.Project, error)
	Projects() []home.Project
}

// Wiki reads a project's wiki entries (the Wiki block).
type Wiki interface {
	Entry(project, path string) (wiki.Entry, error)
}

// Messages sends a message to a teammate (Messaging).
type Messages interface {
	Send(ctx context.Context, msg store.TeamMessage) (store.TeamMessage, error)
}

// Teams are the deployments a dismissed review's team runs in (the instance
// manager): the CEO may stop it with the close.
type Teams interface {
	// Active reports whether the deployment still runs.
	Active(ctx context.Context, deployment string) (bool, error)
	// List returns the instances of every active deployment, each with when its
	// harness last reported (team()).
	List(ctx context.Context) ([]store.Instance, error)
	Stop(ctx context.Context, deployment string) error
}

// Deps are the blocks Workflow is assembled from.
type Deps struct {
	Git      Git
	Config   Config
	Messages Messages
	Teams    Teams
	// Actions keeps each git tool call and what it did (the Team traffic
	// panel); Publish announces it (team.action). Either may be nil.
	Actions store.Actions
	Publish func(events.Event)
	// WorkItems tracks hand-offs on their way to the lead branch (pipeline.changed).
	WorkItems store.WorkItems
	// MergeRequests are the leads' requests to merge into main; Wiki names them
	// (the title of the entry that explains the work).
	MergeRequests store.MergeRequests
	Wiki          Wiki
	// Requests are what waits for the CEO: a review waiting on them (request.created, request.resolved).
	Requests store.Requests
	// Reviews group a lead branch's merge requests for the CEO; ReviewComments
	// are the CEO's comments on them.
	Reviews        store.Reviews
	ReviewComments store.ReviewComments
	Now            func() time.Time // the contributions' dates; time.Now when nil
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

// object is a tool's input schema. With nothing required it leaves required
// out: JSON Schema wants an array there, and strict providers (Moonshot)
// refuse the null a nil slice marshals to.
func object(props string, required ...string) json.RawMessage {
	if len(required) == 0 {
		return json.RawMessage(`{"type":"object","properties":{` + props + `}}`)
	}
	req, _ := json.Marshal(required)
	return json.RawMessage(`{"type":"object","properties":{` + props + `},"required":` + string(req) + `}`)
}

var definitions = []agentlink.Tool{
	{Name: "switch", Description: "Switch your worktrees, in every repo, to one of your branches, making it from main if it is new. Commit first: a switch never carries changes along.",
		InputSchema: object(`"branch":{"type":"string","description":"One of your branches: wt/<you>/<topic> for a member, lead/<team>/<topic> for the lead"}`, "branch")},
	{Name: "commit", Description: "Commit everything you changed in each repo you name, on the branch checked out there, with that repo's message (git is read-only for you); repos you don't name are left as they are. After a merge with conflicts, committing a repo concludes its merge once no conflict markers are left.",
		InputSchema: object(`"commits":{"type":"array","minItems":1,"description":"One commit per repo","items":{"type":"object","properties":{`+
			`"repo":{"type":"string","description":"The repo's folder in your sandbox, e.g. api; the lead's project repo is project"},`+
			`"message":{"type":"string","description":"The commit message"}},"required":["repo","message"]}}`, "commits")},
	{Name: "merge", Description: "Merge a branch (main, the lead's, a teammate's) into yours, in every repo that has it. Conflict markers stay in the files: resolve them, then commit those repos to finish the merge, or abort_merge() to back out.",
		InputSchema: object(`"branch":{"type":"string","description":"The branch to merge in, e.g. main"}`, "branch")},
	{Name: "discard", Description: "Throw away what you changed and didn't commit, in each repo you name (all of them if none): its files go back to the branch as committed, new files removed (ignored ones stay). Foreman keeps a copy. After a merge, abort_merge() first.",
		InputSchema: object(`"repos":{"type":"array","items":{"type":"string"},"description":"Your repos' folders, e.g. api; all of them when left out"}`)},
	{Name: "abort_merge", Description: "Back out of an unfinished merge, in every repo where one is under way: your branch and files are as they were before it.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
	{Name: "set_submodule", Description: "Put a submodule of one of your repos (e.g. lib/forge-std) on a branch, tag or commit of its own repo, and commit that on your branch; with url, add the submodule first. What was changed in its files is discarded. During a merge it settles a submodule conflict instead, and committing the repo finishes the merge. Foreman fetches over https without credentials: a private repo needs the CEO.",
		InputSchema: object(`"repo":{"type":"string","description":"The repo's folder in your sandbox, e.g. contracts"},`+
			`"path":{"type":"string","description":"The submodule's path in that repo, e.g. lib/forge-std"},`+
			`"ref":{"type":"string","description":"A branch, tag or commit of the submodule's repo, e.g. v1.9.0"},`+
			`"url":{"type":"string","description":"Only to add a new submodule: its https URL"}`, "repo", "path", "ref")},
	{Name: "handoff", Description: "Pass finished, committed work on a branch of yours to a teammate (usually the lead): they get your summary and the branch, and Foreman records the hand-off. Commit first. Your part ends.",
		InputSchema: object(`"teammate":{"type":"string","description":"A teammate's name or id"},`+
			`"branch":{"type":"string","description":"wt/<you>/<topic>"},`+
			`"summary":{"type":"string","description":"The outcome: what was done, decisions, what is left open, how it was checked"}`, "teammate", "branch", "summary")},
	{Name: "contribute", Description: "Add a finding, a proposal or a note to the project's knowledge base without writing it yourself: Foreman files it in the wiki's inbox and tells the lead.",
		InputSchema: object(`"kind":{"type":"string","enum":["finding","proposal","note"]},`+
			`"title":{"type":"string","description":"A short title"},`+
			`"body":{"type":"string","description":"The contribution, in Markdown"}`, "kind", "title", "body")},
	{Name: "request_merge", Description: "Ask the CEO to merge one of your lead branches into main, in every repo where it has new commits: one merge request each, carrying the hand-offs merged into it. Commit first. Call it again on the same branch to refresh it (after merging main in, say).",
		InputSchema: object(`"branch":{"type":"string","description":"lead/<team>/<topic>"},`+
			`"entry":{"type":"string","description":"The wiki entry explaining the work, usually its task card, e.g. backlog/export-csv.md"}`, "branch", "entry")},
	{Name: "withdraw_merge", Description: "Withdraw your open request to merge a lead branch, in every repo, before the CEO decides it: the work is superseded by another of your branches, or no longer wanted. The branch stays. The CEO's draft comments on it are lost.",
		InputSchema: object(`"branch":{"type":"string","description":"lead/<team>/<topic>, with a review open"},`+
			`"reason":{"type":"string","description":"Why, for the CEO"},`+
			`"superseded_by":{"type":"string","description":"Only when another of your lead branches, with its own review open, holds all of this one: that branch"}`, "branch", "reason")},
	{Name: "reviews", Description: "Where your reviews stand: the open ones and those decided in the last 7 days, each repo's state, and while changes are requested, the CEO's note and comments, each on its line now. Read-only.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
	{Name: "team", Description: "Where each teammate stands: its status, when its harness last reported, and each repo's branch with whether it holds uncommitted changes or a merge under way. Read-only; wakes nobody.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
}

// Tools implements teamtools.Block.
func (w *Workflow) Tools() []agentlink.Tool { return definitions }

// Call implements teamtools.Block.
func (w *Workflow) Call(ctx context.Context, c teamtools.Caller, name string, args json.RawMessage) agentlink.Result {
	var a struct {
		Branch   string   `json:"branch"`
		Teammate string   `json:"teammate"`
		Entry    string   `json:"entry"`
		Summary  string   `json:"summary"`
		Kind     string   `json:"kind"`
		Title    string   `json:"title"`
		Body     string   `json:"body"`
		Repo     string   `json:"repo"`
		Path     string   `json:"path"`
		Ref      string   `json:"ref"`
		URL      string   `json:"url"`
		Repos    []string `json:"repos"`
		Reason   string   `json:"reason"`
		By       string   `json:"superseded_by"`
		Commits  []struct {
			Repo    string `json:"repo"`
			Message string `json:"message"`
		} `json:"commits"`
	}
	if res, ok := teamtools.Args(args, &a); !ok {
		return res
	}
	project, err := w.Config.Project(c.Deployment.ProjectID)
	if err != nil {
		return teamtools.Refusal("git_error", "the project: %v", err)
	}
	act := &store.TeamAction{DeploymentID: c.Deployment.ID, Instance: c.Instance.ID, Tool: name}
	t := tool{w: w, c: c, project: project, act: act}
	var res agentlink.Result
	switch name {
	case "switch":
		act.Branch = strings.TrimSpace(a.Branch)
		res = t.switchTo(ctx, act.Branch)
	case "commit":
		for _, c := range a.Commits {
			act.Repos = append(act.Repos, store.ActionRepo{Repo: strings.TrimSpace(c.Repo), Message: strings.TrimSpace(c.Message)})
		}
		res = t.commit(ctx)
	case "merge":
		act.Branch = strings.TrimSpace(a.Branch)
		res = t.merge(ctx, act.Branch)
	case "abort_merge":
		res = t.abortMerge(ctx)
	case "discard":
		res = t.discard(ctx, a.Repos)
	case "set_submodule":
		res = t.setSubmodule(ctx, strings.TrimSpace(a.Repo), strings.TrimSpace(a.Path), strings.TrimSpace(a.Ref), strings.TrimSpace(a.URL))
	case "handoff":
		return t.handoff(ctx, a.Teammate, strings.TrimSpace(a.Branch), strings.TrimSpace(a.Summary))
	case "request_merge":
		act.Branch = strings.TrimSpace(a.Branch)
		res = t.requestMerge(ctx, act.Branch, strings.TrimSpace(a.Entry))
	case "reviews":
		return t.reviews(ctx) // read-only: not an action
	case "team":
		return t.team(ctx) // read-only: not an action
	case "withdraw_merge":
		act.Branch = strings.TrimSpace(a.Branch)
		res = t.withdrawMerge(ctx, act.Branch, strings.TrimSpace(a.Reason), strings.TrimSpace(a.By))
	default: // contribute
		return t.contribute(ctx, strings.TrimSpace(a.Kind), strings.TrimSpace(a.Title), strings.TrimSpace(a.Body))
	}
	// The git tools, request_merge and withdraw_merge are kept as actions, refusals included (a hand-off and a
	// contribution are messages already).
	if res.Error {
		act.Error = res.Text
	}
	w.record(ctx, *act)
	// A merge into a lead branch (or the commit that finishes one) may
	// integrate hand-offs: checked right then.
	if (name == "merge" || name == "commit") && c.Lead && !res.Error {
		t.integrate(ctx)
	}
	return res
}

// record keeps an action and announces it.
func (w *Workflow) record(ctx context.Context, a store.TeamAction) {
	if w.Actions == nil {
		return
	}
	saved, err := w.Actions.Add(ctx, a)
	if err != nil {
		slog.Warn("keeping an action", "instance", a.Instance, "tool", a.Tool, "err", err)
		return
	}
	if w.Publish != nil {
		w.Publish(events.Event{Type: "team.action", Data: saved})
	}
}

// tool is one call: the caller and its project, and, for a git tool, the
// action it records.
type tool struct {
	w       *Workflow
	c       teamtools.Caller
	project home.Project
	act     *store.TeamAction
}

// repo is one of the caller's worktrees, or the live project repo.
type repo struct{ name, dir string }

// worktrees are the caller's worktrees.
func (t tool) worktrees() []repo { return worktreesIn(t.project, t.c.Instance.SandboxPath) }

// worktreesIn are an agent's worktrees under its sandbox, one per code repo of
// the project that it has (a repo added since its deploy has none yet).
func worktreesIn(project home.Project, sandbox string) []repo {
	var repos []repo
	for _, r := range project.Repos {
		dir := filepath.Join(sandbox, r.Name)
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			repos = append(repos, repo{r.Name, dir})
		}
	}
	return repos
}

func (t tool) projectRepo() repo {
	return repo{"project", t.project.Path}
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

// unsettled refuses (ok false) when a repo has what the action would carry
// along or trip on: first files changed in a submodule, which no commit
// records, then uncommitted changes; why says what the action needs.
func (t tool) unsettled(ctx context.Context, repos []repo, why string) (agentlink.Result, bool) {
	var subs []string
	for _, r := range repos {
		paths, err := t.w.Git.DirtySubmodules(ctx, r.dir)
		if err != nil {
			return gitError(err), false
		}
		for _, p := range paths {
			subs = append(subs, r.name+"/"+p)
		}
	}
	if len(subs) > 0 {
		return teamtools.Refusal("submodule_changed", "files changed in the submodule %s, which no commit records: restore them, or put it on a commit with set_submodule(repo, path, ref), which discards them", strings.Join(subs, ", ")), false
	}
	if dirty, err := t.dirty(ctx, repos); err != nil {
		return gitError(err), false
	} else if len(dirty) > 0 {
		return teamtools.Refusal("uncommitted_changes", "commit first, in %s: %s", strings.Join(dirty, ", "), why), false
	}
	return agentlink.Result{}, true
}

// synced puts the repos' submodules at the commits their branch pins, after
// an action that moved it, and says where that failed: the action itself went
// through.
func (t tool) synced(ctx context.Context, repos []repo) string {
	var failed []string
	for _, r := range repos {
		if err := t.w.Git.SyncSubmodules(ctx, r.dir, t.project.Config.RecursiveSubmodules); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", r.name, err))
		}
	}
	if len(failed) == 0 {
		return ""
	}
	return " Its submodules couldn't be updated in " + strings.Join(failed, "; ") + ": tell the CEO if you need them."
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
	if res, ok := t.unsettled(ctx, repos, "a switch never carries changes along"); !ok {
		return res
	}
	var created []string
	for _, r := range repos {
		main := t.project.Main(r.name)
		made, err := t.w.Git.Switch(ctx, r.dir, branch, main)
		if err != nil {
			return gitError(fmt.Errorf("%s: %w", r.name, err))
		}
		t.act.Repos = append(t.act.Repos, store.ActionRepo{Repo: r.name, Branch: branch, Created: made})
		if made {
			created = append(created, fmt.Sprintf("%s (from %s)", r.name, main))
		}
	}
	text := fmt.Sprintf("Switched %s to %s.", names(repos), branch)
	if len(created) > 0 {
		text += fmt.Sprintf(" New in %s.", strings.Join(created, ", "))
	}
	return agentlink.Result{Text: text + t.synced(ctx, repos)}
}

// committable are the repos the caller may commit, by name: its worktrees,
// and for the lead the live project repo.
func (t tool) committable() map[string]repo {
	repos := t.worktrees()
	if t.c.Lead {
		repos = append(repos, t.projectRepo())
	}
	byName := map[string]repo{}
	for _, r := range repos {
		byName[r.name] = r
	}
	return byName
}

// commit commits each repo the caller names (t.act.Repos), with its own
// message, and fills in what happened there. Every entry is checked before
// anything is committed.
func (t tool) commit(ctx context.Context) agentlink.Result {
	asked := t.act.Repos
	if len(asked) == 0 {
		return teamtools.Refusal("empty", "name at least one repo to commit, with its message")
	}
	repos := t.committable()
	seen := map[string]bool{}
	for i := range asked {
		a := &asked[i]
		a.Message = withoutAttribution(a.Message)
		if _, ok := repos[a.Repo]; !ok {
			names := slices.Sorted(maps.Keys(repos))
			return teamtools.Refusal("unknown_repo", "%q isn't one of your repos: yours are %s", a.Repo, strings.Join(names, ", "))
		}
		if seen[a.Repo] {
			return teamtools.Refusal("repeated_repo", "%s is named twice: one commit per repo", a.Repo)
		}
		seen[a.Repo] = true
		if a.Message == "" {
			return teamtools.Refusal("empty", "give %s a commit message", a.Repo)
		}
	}
	var done, unchanged, conflicts, unsettled, nested []string
	for i := range asked {
		a := &asked[i]
		r := repos[a.Repo]
		sha, err := t.w.Git.CommitAll(ctx, r.dir, a.Message, t.author())
		var left *git.ConflictsError
		var sub *git.SubmoduleConflictError
		var inside *git.NestedReposError
		switch {
		case errors.Is(err, git.ErrNothingToCommit):
			unchanged = append(unchanged, r.name)
		case errors.As(err, &left):
			a.Conflicts = left.Files
			for _, f := range left.Files {
				conflicts = append(conflicts, r.name+"/"+f)
			}
		case errors.As(err, &sub):
			a.Conflicts = sub.Paths
			for _, p := range sub.Paths {
				unsettled = append(unsettled, r.name+"/"+p)
			}
		case errors.As(err, &inside):
			for _, p := range inside.Paths {
				nested = append(nested, r.name+"/"+p)
			}
		case err != nil:
			return gitError(fmt.Errorf("%s: %w", r.name, err))
		default:
			a.Branch, _ = t.w.Git.Branch(ctx, r.dir)
			a.Commit = sha
			done = append(done, fmt.Sprintf("%s (%s, %.8s)", r.name, a.Branch, sha))
		}
	}
	meanwhile := ""
	if len(done) > 0 {
		meanwhile = ". Committed meanwhile: " + strings.Join(done, ", ")
	}
	switch {
	case len(unsettled) > 0:
		return teamtools.Refusal("submodule_conflict", "the merge left the submodule %s on no commit: pick one with set_submodule(repo, path, ref), then commit again%s", strings.Join(unsettled, ", "), meanwhile)
	case len(nested) > 0:
		return teamtools.Refusal("nested_repo", "%s is a git repo inside your worktree, which a commit can't hold: delete it, or add it as a submodule with set_submodule(repo, path, ref, url)%s", strings.Join(nested, ", "), meanwhile)
	case len(conflicts) > 0:
		return teamtools.Refusal("conflicts_left", "conflict markers are still in %s: resolve them, then commit again%s", strings.Join(conflicts, ", "), meanwhile)
	case len(done) == 0:
		return teamtools.Refusal("nothing_to_commit", "nothing changed in %s since your last commit", strings.Join(unchanged, ", "))
	}
	text := "Committed " + strings.Join(done, ", ") + "."
	if len(unchanged) > 0 {
		text += " Nothing changed in " + strings.Join(unchanged, ", ") + "."
	}
	return agentlink.Result{Text: text}
}

// withoutAttribution is an agent's commit message without the attribution a
// harness may add on its own: its Co-Authored-By lines, and Claude Code's
// "🤖 Generated with [Claude Code](…)". A commit carries its message alone
// (the CEO, 2026-10-29).
func withoutAttribution(message string) string {
	var kept []string
	for _, line := range strings.Split(message, "\n") {
		l := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "🤖")))
		if !strings.HasPrefix(l, "co-authored-by:") && !strings.HasPrefix(l, "generated with [claude code]") {
			kept = append(kept, line)
		}
	}
	return strings.TrimRight(strings.Join(kept, "\n"), " \t\r\n")
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
	if res, ok := t.unsettled(ctx, repos, "a merge never mixes with unsaved work"); !ok {
		return res
	}
	var merged, conflicted []string
	for _, r := range repos {
		files, err := t.w.Git.Merge(ctx, r.dir, branch, t.author())
		if err != nil {
			return gitError(fmt.Errorf("%s: %w", r.name, err))
		}
		into, _ := t.w.Git.Branch(ctx, r.dir)
		t.act.Repos = append(t.act.Repos, store.ActionRepo{Repo: r.name, Branch: into, Conflicts: files})
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
		text = append(text, fmt.Sprintf("Conflicts in %s: resolve the markers (a submodule: pick its commit with set_submodule), then commit those repos to finish the merge, or abort_merge() to back out.", strings.Join(conflicted, "; ")))
	}
	return agentlink.Result{Text: strings.Join(text, " ") + t.synced(ctx, repos)}
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
	if teamtools.TooLong(summary, teamtools.MaxBody) {
		return teamtools.Refusal("too_long", "a summary is at most %d characters; put the rest in the branch or a wiki entry", teamtools.MaxBody)
	}
	// Nothing pending: what is handed off is exactly what is committed.
	if res, ok := t.unsettled(ctx, t.worktrees(), "a hand-off is your committed work"); !ok {
		return res
	}
	found, news := false, false
	for _, r := range t.worktrees() {
		if !t.w.Git.HasBranch(ctx, r.dir, branch) {
			continue
		}
		found = true
		merged, err := t.w.Git.Merged(ctx, r.dir, t.project.Main(r.name), branch)
		if err != nil {
			return gitError(err)
		}
		news = news || !merged
	}
	switch {
	case !found:
		return teamtools.Refusal("branch_not_found", "no branch %q in your repos", branch)
	case !news:
		return teamtools.Refusal("nothing_new", "%s changes nothing in its repo's main branch", branch)
	}
	msg, err := t.w.Messages.Send(ctx, store.TeamMessage{DeploymentID: t.c.Deployment.ID, FromInstance: t.c.Instance.ID,
		ToInstance: to, Kind: "handoff", Body: summary, ArtifactRef: branch})
	if err != nil {
		return teamtools.Refusal("internal", "%v", err)
	}
	t.handed(ctx, branch)
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
	case teamtools.TooLong(title, maxTitle) || strings.ContainsAny(title, "\r\n"):
		return teamtools.Refusal("too_long", "a title is one line of at most %d characters", maxTitle)
	case teamtools.TooLong(body, teamtools.MaxContribution):
		return teamtools.Refusal("too_long", "a body is at most %d characters: split it into separate contributions, one subject each", teamtools.MaxContribution)
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
		into, _ := t.w.Git.Branch(ctx, r.dir)
		t.act.Repos = append(t.act.Repos, store.ActionRepo{Repo: r.name, Branch: into})
	}
	if len(aborted) == 0 {
		return teamtools.Refusal("no_merge", "no merge is under way in your repos")
	}
	return agentlink.Result{Text: fmt.Sprintf("Merge aborted in %s: your branch is as it was before it.", strings.Join(aborted, ", ")) + t.synced(ctx, t.worktrees())}
}

// discard puts the caller's worktrees named (all when none) back to their
// branch as committed; what it throws away is kept under
// refs/foreman/discarded/ (wiki/product/architecture/sandboxes-and-git.md#backups).
func (t tool) discard(ctx context.Context, asked []string) agentlink.Result {
	repos := t.worktrees()
	if len(repos) == 0 {
		return teamtools.Refusal("no_repos", "you have no worktree in this project")
	}
	if len(asked) > 0 {
		var chosen []repo
		for _, name := range asked {
			name = strings.TrimSpace(name)
			i := slices.IndexFunc(repos, func(r repo) bool { return r.name == name })
			if i < 0 {
				return teamtools.Refusal("unknown_repo", "%q isn't one of your repos: yours are %s", name, names(repos))
			}
			if slices.Contains(chosen, repos[i]) {
				return teamtools.Refusal("repeated_repo", "%s is named twice", name)
			}
			chosen = append(chosen, repos[i])
		}
		repos = chosen
	}
	// Checked in every repo first, so a discard never stops halfway.
	var merging []string
	for _, r := range repos {
		if m, err := t.w.Git.Merging(ctx, r.dir); err != nil {
			return gitError(err)
		} else if m {
			merging = append(merging, r.name)
		}
	}
	if len(merging) > 0 {
		return teamtools.Refusal("merge_under_way", "a merge is under way in %s: abort_merge() first", strings.Join(merging, ", "))
	}
	at := t.w.Now()
	var done []string
	for _, r := range repos {
		branch, _ := t.w.Git.Branch(ctx, r.dir)
		saved, err := t.w.Git.Discard(ctx, r.dir, git.BackupRef("discarded", at, t.c.Instance.AgentID+"/"+r.name))
		if err != nil {
			return gitError(fmt.Errorf("%s: %w", r.name, err))
		}
		if saved {
			done = append(done, r.name)
			t.act.Repos = append(t.act.Repos, store.ActionRepo{Repo: r.name, Branch: branch})
			t.w.pruneBackups(ctx, r.dir)
		}
	}
	if len(done) == 0 {
		return agentlink.Result{Text: fmt.Sprintf("Nothing to discard in %s: it's as committed.", names(repos))}
	}
	return agentlink.Result{Text: fmt.Sprintf("Discarded what wasn't committed in %s: back to the branch as committed. Foreman kept a copy.", strings.Join(done, ", ")) + t.synced(ctx, repos)}
}

// setSubmodule puts a submodule of one of the caller's worktrees on ref
// (adding it from url first), committed on its branch, or staged during a
// merge (wiki/product/architecture/sandboxes-and-git.md#submodules).
func (t tool) setSubmodule(ctx context.Context, name, path, ref, url string) agentlink.Result {
	var r repo
	repos := t.worktrees()
	for _, w := range repos {
		if w.name == name {
			r = w
		}
	}
	switch {
	case r.dir == "":
		return teamtools.Refusal("unknown_repo", "%q isn't one of your code repos: yours are %s", name, names(repos))
	case path == "" || ref == "":
		return teamtools.Refusal("empty", "give the submodule's path and the ref to put it on")
	}
	message := fmt.Sprintf("Move submodule %s to %s", path, ref)
	if url != "" {
		message = fmt.Sprintf("Add submodule %s at %s (%s)", path, ref, url)
	}
	sha, err := t.w.Git.SetSubmodule(ctx, r.dir, path, ref, url, message, t.author())
	if errors.Is(err, git.ErrNothingToCommit) {
		return agentlink.Result{Text: fmt.Sprintf("%s was on %s already in %s: what was changed in its files is discarded, nothing to commit.", path, ref, r.name)}
	}
	switch {
	case errors.Is(err, git.ErrBadPath):
		return teamtools.Refusal("bad_path", "%q isn't a path inside %s", path, r.name)
	case errors.Is(err, git.ErrNotSubmodule):
		return teamtools.Refusal("not_a_submodule", "%s isn't a submodule of %s: give its url to add one", path, r.name)
	case errors.Is(err, git.ErrSubmoduleExists):
		return teamtools.Refusal("path_taken", "%s already holds something in %s: leave out url to move a submodule there", path, r.name)
	case errors.Is(err, git.ErrBadURL):
		return teamtools.Refusal("bad_url", "%v", err)
	case errors.Is(err, git.ErrUnknownRef):
		return teamtools.Refusal("unknown_ref", "%s's repo has no branch, tag or commit %q", path, ref)
	case errors.Is(err, git.ErrNeedsCredentials):
		return teamtools.Refusal("needs_credentials", "%s needs credentials, and Foreman fetches without the CEO's: ask the CEO to add it", path)
	case err != nil:
		return gitError(fmt.Errorf("%s: %w", r.name, err))
	}
	branch, _ := t.w.Git.Branch(ctx, r.dir)
	t.act.Branch = branch
	t.act.Repos = append(t.act.Repos, store.ActionRepo{Repo: r.name, Branch: branch, Message: message, Commit: sha})
	text := fmt.Sprintf("Committed in %s (%s, %.8s): %s.", r.name, branch, sha, message)
	if sha == "" {
		text = fmt.Sprintf("%s is on %s in %s: commit %s to finish the merge.", path, ref, r.name, r.name)
	}
	// Its own submodules, when the project wants them.
	return agentlink.Result{Text: text + t.synced(ctx, []repo{r})}
}
