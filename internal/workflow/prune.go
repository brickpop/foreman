package workflow

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
)

// Deleting branches (wiki/product/architecture/sandboxes-and-git.md#cleaning-up-branches):
// the agents' prune() and the CEO's branch cleanup on the Repos page share
// one rule. A branch someone has checked out (WIP, or the branch they're on),
// one under review, and one an open review goes into never go; everything
// that goes is kept under refs/foreman/cleaned/ for 30 days first.

// Where a branch's work stands: deleting a merged or superseded one loses no
// commit.
const (
	// Merged: all of it is in a branch of the CEO's, or upstream (a remote's
	// branch, but its own copy there), or merging it where work lands would
	// change nothing (squash-aware).
	Merged = "merged"
	// Superseded: not merged, but all of it is in another agents' branch.
	Superseded = "superseded"
	// Unmerged: some of it is nowhere else.
	Unmerged = "unmerged"
)

// CleanupBranch is one branch of one of a project's code repos, as the CEO's
// branch cleanup lists it.
type CleanupBranch struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	// Agents: an agents' branch (wt/*, lead/*); else the CEO's.
	Agents bool `json:"agents"`
	// State is Merged, Superseded or Unmerged.
	State string `json:"state"`
	// SupersededBy are the agents' branches holding all of a superseded one.
	SupersededBy []string `json:"supersededBy,omitempty"`
	// Legacy: an agents' branch named before branches carried their project
	// (wt/<agent>/…), or carrying a project Foreman no longer has.
	Legacy bool `json:"legacy"`
	// Stays says why it can't be deleted: checked out, under review, an open
	// review's target. "" when it can.
	Stays string `json:"stays,omitempty"`
	// Stray is the folder of the stray worktree that has it checked out (no
	// project's agent's, not the CEO's checkout): picked, that worktree is
	// removed first, unless it holds changes not committed.
	Stray string `json:"stray,omitempty"`
	// Free is the agent of the project, not at work, whose worktree has it
	// checked out: picked with its siblings (the rows of the same agent and
	// branch in other repos), each worktree is detached first, at the same
	// commit (freeable). Set only when Stays is "".
	Free string `json:"free,omitempty"`

	agent, tree, reviewed string // the project's agent that has it checked out, where, and what review keeps it
}

// standing is where branch's work stands (Merged, Superseded, Unmerged), with
// the agents' branches holding it when superseded. Unknown (a git error)
// counts as Unmerged.
func (w *Workflow) standing(ctx context.Context, project home.Project, name, dir, branch string) (string, []string) {
	local, upstream, err := w.Git.Holders(ctx, dir, branch)
	if err != nil {
		return Unmerged, nil
	}
	var agents []string
	for _, h := range local {
		if !git.AgentBranch(h) {
			return Merged, nil
		}
		agents = append(agents, h)
	}
	for _, h := range upstream {
		_, base, _ := strings.Cut(h, "/") // origin/<base>
		switch {
		case base == branch: // its own copy upstream holds nothing more
		case git.AgentBranch(base):
			agents = append(agents, h)
		default:
			return Merged, nil
		}
	}
	// In a review the CEO merged: the lead branch has moved on since (a squash
	// moves it to the merged commit), but its head then is kept. Its history
	// holding branch isn't enough: resolving a conflict, the lead may have
	// dropped some of it, which the review never showed; so every line branch
	// changed must still be there (the CEO, 2026-11-11).
	if kept, err := w.Git.KeptHolding(ctx, dir, "refs/foreman/decided/", branch); err == nil {
		for _, id := range kept {
			mr, err := w.MergeRequests.Get(ctx, id)
			if err != nil || mr.State != store.MergeMerged {
				continue
			}
			if ok, _ := w.Git.Survives(ctx, dir, branch, mr.Target, "refs/foreman/decided/"+id); ok {
				return Merged, nil
			}
		}
	}
	// Where work lands: each project's default for the repo, and each target
	// one of its merge requests went into.
	var landings []string
	land := func(b string) {
		if b != "" && !slices.Contains(landings, b) {
			landings = append(landings, b)
		}
	}
	for _, s := range w.sharing(project, name) {
		land(s.project.Default(s.repo.ID))
		if mrs, err := w.MergeRequests.ForProject(ctx, s.project.ID); err == nil {
			for _, mr := range mrs {
				if mr.Repo == s.repo.ID {
					land(mr.Target)
					land(mr.Landing)
				}
			}
		}
	}
	for _, l := range landings {
		if l == branch {
			continue
		}
		if merged, err := w.Git.Merged(ctx, dir, l, branch); err == nil && merged {
			return Merged, nil
		}
	}
	if len(agents) > 0 {
		return Superseded, agents
	}
	return Unmerged, nil
}

// branchRepo is one code repo of a project, with what deleting needs to know
// about its branches.
type branchRepo struct {
	project home.Project
	repo    home.Repo
	trees   map[string]string // branch → the worktree that has it checked out
	review  map[string]bool   // branches under an open review, in any project with this repo
	target  map[string]bool   // branches an open review goes into, the same
}

func (w *Workflow) branchRepo(ctx context.Context, p home.Project, r home.Repo) (branchRepo, error) {
	trees, err := w.Git.Worktrees(ctx, r.Path)
	if err != nil {
		return branchRepo{}, err
	}
	b := branchRepo{project: p, repo: r, trees: trees, review: map[string]bool{}, target: map[string]bool{}}
	for _, s := range w.sharing(p, r.ID) {
		mrs, err := w.MergeRequests.ForProject(ctx, s.project.ID)
		if err != nil {
			return b, err
		}
		for _, mr := range mrs {
			if mr.Repo == s.repo.ID && mr.State == store.MergeOpen {
				b.review[mr.Branch], b.target[mr.Target] = true, true
			}
		}
	}
	return b, nil
}

// shared is one project's name for a git repo.
type shared struct {
	project home.Project
	repo    home.Repo
}

// sharing is project's repo name and every other project's that is the same
// repo (the same folder): its reviews, targets and default read it as well.
// Project's own comes first.
func (w *Workflow) sharing(project home.Project, name string) []shared {
	i := slices.IndexFunc(project.Repos, func(r home.Repo) bool { return r.ID == name })
	if i < 0 {
		return []shared{{project, home.Repo{ID: name}}}
	}
	all := []shared{{project, project.Repos[i]}}
	for _, q := range w.Config.Projects() {
		if q.ID == project.ID {
			continue
		}
		for _, qr := range q.Repos {
			if sameDir(qr.Path, project.Repos[i].Path) {
				all = append(all, shared{q, qr})
			}
		}
	}
	return all
}

// checkedOut says who has branch checked out: an agent ("tove", or
// "tove@suite" on another project), the CEO's own checkout ("you"), or a
// stray worktree, one in no project's agents' sandboxes (moved by hand, a
// folder since deleted), named by its folder. "" when none has it.
func (w *Workflow) checkedOut(b branchRepo, branch string) (who, stray string) {
	dir, ok := b.trees[branch]
	switch {
	case !ok:
		return "", ""
	case sameDir(dir, b.repo.Path):
		return "you", ""
	}
	if agent := home.WorktreeAgent(dir, func(a string) string { return w.Config.Sandbox(b.project.ID, a) }); agent != "" {
		return agent, ""
	}
	for _, q := range w.Config.Projects() {
		if agent := home.WorktreeAgent(dir, func(a string) string { return w.Config.Sandbox(q.ID, a) }); agent != "" {
			return agent + "@" + q.ID, ""
		}
	}
	return "", dir
}

// sameDir compares folders as the file system sees them (git reports
// worktrees with symlinks resolved).
func sameDir(a, b string) bool {
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	return resolve(a) == resolve(b)
}

// stays says why branch can't be deleted, "" when it can.
func (w *Workflow) stays(b branchRepo, branch string) string {
	switch who, stray := w.checkedOut(b, branch); {
	case who == "you":
		return "checked out in your checkout"
	case who != "":
		return "checked out by " + who
	case stray != "":
		return "checked out in " + stray
	}
	return reviewed(b, branch)
}

// reviewed says why a review keeps branch, "" when none does.
func reviewed(b branchRepo, branch string) string {
	switch {
	case b.review[branch]:
		return "its review is open"
	case b.target[branch]:
		return "an open review merges into it"
	}
	return ""
}

// delete deletes branch, kept first; "" when it went, else why it stayed.
func (w *Workflow) delete(ctx context.Context, b branchRepo, branch string) string {
	if why := w.stays(b, branch); why != "" {
		return why
	}
	switch err := w.Git.DeleteBranch(ctx, b.repo.Path, branch, git.BackupRef("cleaned", w.Now(), branch)); {
	case errors.Is(err, git.ErrCheckedOut):
		return "checked out"
	case err != nil:
		return err.Error()
	}
	return ""
}

// ofProject says whether an agents' branch is the project's to clean: named
// with it, or legacy (with no project Foreman has, so nobody else's).
func (w *Workflow) ofProject(p home.Project, branch string) (ok, legacy bool) {
	if !git.AgentBranch(branch) {
		return false, false
	}
	second, _, _ := strings.Cut(strings.SplitN(branch, "/", 2)[1], "/")
	if second == p.ID {
		return true, false
	}
	for _, other := range w.Config.Projects() {
		if other.ID == second {
			return false, false
		}
	}
	return true, true
}

// offered says whether the CEO's cleanup offers branch, and why not: an
// agents' branch of the project, or legacy, whatever its state; one of the
// CEO's, but the repo's default, only merged or superseded (unmerged, it may
// be the CEO's own work in progress, which Foreman can't see).
func (w *Workflow) offered(p home.Project, r home.Repo, branch, state string) (legacy bool, why string) {
	switch {
	case git.AgentBranch(branch):
		ok, legacy := w.ofProject(p, branch)
		if !ok {
			return false, "another project's"
		}
		return legacy, ""
	case slices.ContainsFunc(w.sharing(p, r.ID), func(s shared) bool { return s.repo.Default == branch }):
		return false, "the default branch" // of any project with this repo
	case slices.Contains(longLived, branch):
		return false, "a long-lived branch"
	case state == Unmerged:
		return false, "not merged"
	}
	return false, ""
}

// longLived are the names a repo's lasting branches go by: never offered,
// merged or not (the CEO, 2026-11-11), whatever the repo's default is.
var longLived = []string{"main", "master", "trunk", "develop", "development", "dev", "staging", "production", "prod"}

// CleanupBranches lists what the CEO's branch cleanup offers in each of the
// project's code repos (offered; a missing repo has none), each with where its
// work stands and what keeps it: by repo, in the project's order, then by
// branch. The repos are read side by side: where each branch stands takes a
// moment on a repo with many.
func (w *Workflow) CleanupBranches(ctx context.Context, projectID string) ([]CleanupBranch, error) {
	p, err := w.Config.Project(projectID)
	if err != nil {
		return nil, err
	}
	lists := make([][]CleanupBranch, len(p.Repos))
	errs := make([]error, len(p.Repos))
	var wg sync.WaitGroup
	for i, r := range p.Repos {
		if r.Ready() {
			wg.Go(func() { lists[i], errs[i] = w.cleanupIn(ctx, p, r) })
		}
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	list := append([]CleanupBranch{}, slices.Concat(lists...)...)
	return list, w.freeable(ctx, p, list)
}

// freeable sets, for each branch an agent of the project has checked out,
// Free when the cleanup can free it, else Stays with why not: the agent is at
// work, or in one of the repos where it has the branch checked out (its
// siblings, in the project's order) the repo has a problem, the branch isn't
// merged nor superseded, a review keeps it, or a merge is under way.
func (w *Workflow) freeable(ctx context.Context, p home.Project, list []CleanupBranch) error {
	for i := range list {
		if list[i].agent == "" || list[i].Free != "" || list[i].Stays != "" {
			continue // not the project's agent's, or its group done
		}
		agent, branch := list[i].agent, list[i].Branch
		group := map[string]*CleanupBranch{}
		for j := range list {
			if list[j].agent == agent && list[j].Branch == branch {
				group[list[j].Repo] = &list[j]
			}
		}
		why, err := w.whyNotFree(ctx, p, agent, branch, group)
		if err != nil {
			return err
		}
		for _, row := range group {
			row.Free, row.Stays = agent, ""
			if why != "" {
				row.Free, row.Stays = "", "checked out by "+agent+", "+why
			}
		}
	}
	return nil
}

func (w *Workflow) whyNotFree(ctx context.Context, p home.Project, agent, branch string, group map[string]*CleanupBranch) (string, error) {
	status, at, err := w.atWork(ctx, p.ID, agent)
	switch {
	case err != nil:
		return "", err
	case at:
		return "who is " + status, nil
	}
	for _, r := range p.Repos {
		row, ok := group[r.ID]
		switch {
		case !r.Ready():
			if w.holds(ctx, p, agent, r, branch) {
				return r.ID + " has a problem", nil
			}
		case !ok:
		case row.State != Merged && row.State != Superseded:
			return "not merged in " + r.ID, nil
		case row.reviewed != "":
			return "a review keeps it in " + r.ID, nil
		default:
			merging, err := w.Git.Merging(ctx, row.tree)
			if err != nil {
				return "", err
			}
			if merging {
				return "a merge under way in " + r.ID, nil
			}
		}
	}
	return "", nil
}

// projectAgent says whether checkedOut named an agent of the project itself
// (tove, not tove@suite, you, or a stray worktree).
func projectAgent(who string) bool {
	return who != "" && who != "you" && !strings.Contains(who, "@")
}

// atWork is the status of the agent's instance on the project, and whether it
// is at work (instances.AtWork): "" and false when no active deployment of
// the project has it.
func (w *Workflow) atWork(ctx context.Context, project, agent string) (string, bool, error) {
	insts, err := w.Teams.List(ctx)
	if err != nil {
		return "", false, err
	}
	for _, inst := range insts {
		if inst.ID == agent+"@"+project {
			return inst.Status, instances.AtWork(inst.Status), nil
		}
	}
	return "", false, nil
}

// holds says whether the agent's worktree of r, a repo with a problem, may
// have branch checked out: on it, or unreadable (its repo missing).
func (w *Workflow) holds(ctx context.Context, p home.Project, agent string, r home.Repo, branch string) bool {
	dir := filepath.Join(w.Config.Sandbox(p.ID, agent), r.ID)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return false
	}
	on, err := w.Git.Branch(ctx, dir)
	return err != nil || on == branch
}

func (w *Workflow) cleanupIn(ctx context.Context, p home.Project, r home.Repo) ([]CleanupBranch, error) {
	b, err := w.branchRepo(ctx, p, r)
	if err != nil {
		return nil, err
	}
	names, err := w.Git.Branches(ctx, r.Path, "")
	if err != nil {
		return nil, err
	}
	var list []CleanupBranch
	for _, name := range names {
		state, by := w.standing(ctx, p, r.ID, r.Path, name)
		legacy, why := w.offered(p, r, name, state)
		if why != "" {
			continue
		}
		who, stray := w.checkedOut(b, name)
		row := CleanupBranch{Repo: r.ID, Branch: name, Agents: git.AgentBranch(name), State: state,
			SupersededBy: by, Legacy: legacy, Stray: stray}
		if projectAgent(who) {
			row.agent, row.tree, row.reviewed = who, b.trees[name], reviewed(b, name)
		} else {
			row.Stays = w.stays(b, name)
		}
		list = append(list, row)
	}
	return list, nil
}

// BranchPick is a branch of one of a project's code repos, picked to delete.
type BranchPick struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
}

// DeleteBranches deletes the CEO's pick of what CleanupBranches offers, each
// checked again as it goes: what stays, stays, and one of the CEO's no longer
// merged nor superseded (its only holder deleted just before) is kept. A pick
// in a repo the project hasn't (or one missing) is kept, with why.
func (w *Workflow) DeleteBranches(ctx context.Context, projectID string, picks []BranchPick) (Cleanup, error) {
	p, err := w.Config.Project(projectID)
	if err != nil {
		return Cleanup{}, err
	}
	var out Cleanup
	if picks, err = w.freePicks(ctx, p, picks, &out); err != nil {
		return out, err
	}
	for _, r := range p.Repos {
		var names []string
		for _, pick := range picks {
			if pick.Repo == r.ID {
				names = append(names, pick.Branch)
			}
		}
		if len(names) == 0 {
			continue
		}
		if !r.Ready() {
			for _, name := range names {
				out.Kept = append(out.Kept, Cleaned{Repo: r.ID, What: "branch", Branch: name, Why: "its repo is " + r.Problem})
			}
			continue
		}
		if err := w.deleteIn(ctx, p, r, names, &out); err != nil {
			return out, err
		}
	}
	for _, pick := range picks {
		if !slices.ContainsFunc(p.Repos, func(r home.Repo) bool { return r.ID == pick.Repo }) {
			out.Kept = append(out.Kept, Cleaned{Repo: pick.Repo, What: "branch", Branch: pick.Branch, Why: "no such repo"})
		}
	}
	return out, nil
}

func (w *Workflow) deleteIn(ctx context.Context, p home.Project, r home.Repo, branches []string, out *Cleanup) error {
	b, err := w.branchRepo(ctx, p, r)
	if err != nil {
		return err
	}
	removed := len(out.Removed)
	for _, name := range slices.Compact(slices.Sorted(slices.Values(branches))) {
		c := Cleaned{Repo: r.ID, What: "branch", Branch: name}
		if !git.ValidBranch(ctx, name) || !w.Git.HasBranch(ctx, r.Path, name) {
			c.Why = "no such branch"
		} else {
			state, _ := w.standing(ctx, p, r.ID, r.Path, name)
			if _, c.Why = w.offered(p, r, name, state); c.Why == "" {
				c.Why = w.unstray(ctx, b, name, out)
			}
			if c.Why == "" {
				c.Why = w.delete(ctx, b, name)
			}
		}
		if c.Why == "" {
			out.Removed = append(out.Removed, c)
		} else {
			out.Kept = append(out.Kept, c)
		}
	}
	if len(out.Removed) > removed {
		w.pruneBackups(ctx, r.Path)
	}
	return nil
}

// freePicks frees the picks an agent of the project has checked out
// (freeHeld), by agent and branch, and returns the others.
func (w *Workflow) freePicks(ctx context.Context, p home.Project, picks []BranchPick, out *Cleanup) ([]BranchPick, error) {
	repos := map[string]branchRepo{}
	for _, r := range p.Repos {
		if r.Ready() {
			b, err := w.branchRepo(ctx, p, r)
			if err != nil {
				return nil, err
			}
			repos[r.ID] = b
		}
	}
	type held struct{ agent, branch string }
	picked := map[held]map[string]bool{} // the repos picked, by agent and branch
	var order []held
	var rest []BranchPick
	for _, pick := range picks {
		who := ""
		if b, ok := repos[pick.Repo]; ok {
			who, _ = w.checkedOut(b, pick.Branch)
		}
		if !projectAgent(who) {
			rest = append(rest, pick)
			continue
		}
		k := held{who, pick.Branch}
		if picked[k] == nil {
			picked[k] = map[string]bool{}
			order = append(order, k)
		}
		picked[k][pick.Repo] = true
	}
	for _, k := range order {
		w.freeHeld(ctx, p, repos, k.agent, k.branch, picked[k], out)
	}
	return rest, nil
}

// errStarted is still's refusal: the agent is at work again.
var errStarted = errors.New("started meanwhile")

// freeHeld detaches the agent's worktrees from branch and deletes it, in every
// repo where the agent has it checked out (its siblings) or in none: each
// checked again, then all at once under the repos' locks, the agent checked
// not at work there (git.FreeBranch). Each worktree left idle is in
// out.Removed next to its branch; when it can't, every sibling is kept with
// one why.
func (w *Workflow) freeHeld(ctx context.Context, p home.Project, repos map[string]branchRepo, agent, branch string, picked map[string]bool, out *Cleanup) {
	var siblings []home.Repo
	for _, r := range p.Repos {
		b, ok := repos[r.ID]
		if who, _ := w.checkedOut(b, branch); (ok && who == agent) || (!ok && w.holds(ctx, p, agent, r, branch)) {
			siblings = append(siblings, r)
		}
	}
	keep := func(why string) {
		for _, r := range siblings {
			out.Kept = append(out.Kept, Cleaned{Repo: r.ID, What: "branch", Branch: branch, Why: why})
		}
	}
	var ids []string
	for _, r := range siblings {
		ids = append(ids, r.ID)
	}
	var frees []git.Free
	for _, r := range siblings {
		if !r.Ready() {
			keep(r.ID + " has a problem")
			return
		}
	}
	for _, r := range siblings {
		if !picked[r.ID] {
			keep("pick it in every repo " + agent + " has it: " + strings.Join(ids, ", "))
			return
		}
	}
	for _, r := range siblings {
		b := repos[r.ID]
		state, _ := w.standing(ctx, p, r.ID, r.Path, branch)
		if _, why := w.offered(p, r, branch, state); why != "" {
			keep(why)
			return
		}
		if state != Merged && state != Superseded {
			keep("no longer merged in " + r.ID)
			return
		}
		if reviewed(b, branch) != "" {
			keep("a review keeps it in " + r.ID)
			return
		}
		frees = append(frees, git.Free{Repo: r.Path, Tree: b.trees[branch]})
	}
	still := func() error {
		_, at, err := w.atWork(ctx, p.ID, agent)
		if err == nil && at {
			err = errStarted
		}
		return err
	}
	err := w.Git.FreeBranch(ctx, frees, branch, git.BackupRef("cleaned", w.Now(), branch), still)
	var tree *git.TreeError
	switch {
	case errors.Is(err, errStarted):
		keep(agent + " started meanwhile")
	case errors.As(err, &tree):
		i := slices.IndexFunc(frees, func(f git.Free) bool { return f.Tree == tree.Tree })
		repo := siblings[max(i, 0)].ID
		switch {
		case errors.Is(tree.Err, git.ErrOtherBranch):
			keep(agent + "'s worktree of " + repo + " is on another branch now")
		case errors.Is(tree.Err, git.ErrMerging):
			keep("a merge is under way in " + agent + "'s worktree of " + repo)
		default:
			keep(tree.Err.Error())
		}
	case err != nil:
		keep(err.Error())
	default:
		for _, r := range siblings {
			out.Removed = append(out.Removed,
				Cleaned{Repo: r.ID, What: "branch", Branch: branch},
				Cleaned{Agent: agent, Repo: r.ID, What: "detached", Branch: branch})
			w.pruneBackups(ctx, r.Path)
		}
	}
}

// unstray removes the stray worktree that has branch checked out, if one does
// (checkedOut), so the branch can go: "" when it did, or there was none; else
// why it stays. Not when a review keeps the branch anyway, nor with changes
// not committed in it, a merge under way included (git refuses those): what
// git ignores in it (build output, a .env) goes with its folder.
func (w *Workflow) unstray(ctx context.Context, b branchRepo, branch string, out *Cleanup) string {
	_, stray := w.checkedOut(b, branch)
	if stray == "" {
		return ""
	}
	if why := reviewed(b, branch); why != "" {
		return why
	}
	if merging, err := w.Git.Merging(ctx, stray); err == nil && merging {
		return "a merge is under way in its worktree, " + stray
	}
	switch err := w.Git.RemoveWorktree(ctx, b.repo.Path, stray); {
	case errors.Is(err, git.ErrUncommitted):
		return "its worktree holds changes not committed, " + stray
	case err != nil:
		return err.Error()
	}
	delete(b.trees, branch)
	out.Removed = append(out.Removed, Cleaned{Repo: b.repo.ID, What: "worktree", Branch: branch, Why: stray})
	return ""
}

// prune is an agent's prune(named): in each code repo, its merged and
// superseded branches go, and a named one of its own even unmerged; for the
// lead, its teammates' merged ones too, and their superseded ones when
// another branch of that same teammate holds them. Never a home branch, one
// checked out (a teammate's WIP, the branch anyone is on) or one under review:
// a teammate's work held nowhere but by others is only its owner's to delete.
// Each branch's standing is read as it goes, so of two at the same commit one
// stays.
func (t tool) prune(ctx context.Context, named []string) agentlink.Result {
	ownPrefix := git.BranchPrefix(t.project.ID, t.c.Instance.AgentID, t.c.Team.ID, t.c.Lead)
	prefixes := []string{ownPrefix}
	if t.c.Lead {
		for _, agent := range slices.Sorted(maps.Keys(t.c.Team.Config.Agents)) {
			if agent != t.c.Team.Config.Lead {
				prefixes = append(prefixes, git.BranchPrefix(t.project.ID, agent, "", false))
			}
		}
	}
	asked := map[string]bool{}
	for _, n := range named {
		n = strings.TrimSpace(n)
		if !slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(n, p) && n != p }) {
			mine := ownPrefix + "<topic>"
			if t.c.Lead {
				mine += " or a teammate's wt/" + t.project.ID + "/<agent>/<topic>"
			}
			return teamtools.Refusal("not_yours", "%s isn't one of your branches: %s", n, mine)
		}
		asked[n] = true
	}
	found := map[string]bool{}
	var lines []string
	deleted := false
	for _, r := range t.project.Repos {
		b, err := t.w.branchRepo(ctx, t.project, r)
		if err != nil {
			return gitError(err)
		}
		var gone, kept []string
		for _, prefix := range prefixes {
			branches, err := t.w.Git.Branches(ctx, r.Path, prefix)
			if err != nil {
				return gitError(err)
			}
			own := prefix == ownPrefix
			for _, name := range branches {
				found[name] = true
				why := ""
				switch state, by := t.w.standing(ctx, t.project, r.ID, r.Path, name); {
				case state == Merged:
				case own && (state == Superseded || asked[name]):
				case own:
					why = "not merged: name it to delete it anyway"
				case state == Superseded && slices.ContainsFunc(by, func(h string) bool { return strings.HasPrefix(h, prefix) }):
				default:
					why = "not merged: only its owner deletes unmerged work"
				}
				if why == "" {
					why = t.w.delete(ctx, b, name)
				}
				if why == "" {
					gone = append(gone, name)
				} else {
					kept = append(kept, name+" ("+why+")")
				}
			}
		}
		deleted = deleted || len(gone) > 0
		var each []string
		if len(gone) > 0 {
			each = append(each, "deleted "+strings.Join(gone, ", "))
			t.w.pruneBackups(ctx, r.Path)
		}
		if len(kept) > 0 {
			each = append(each, "kept "+strings.Join(kept, ", "))
		}
		if len(each) == 0 {
			each = append(each, "nothing to delete")
		}
		lines = append(lines, r.ID+": "+strings.Join(each, "; "))
	}
	var missing []string
	for n := range asked {
		if !found[n] {
			missing = append(missing, n+": no such branch")
		}
	}
	slices.Sort(missing)
	text := strings.Join(append(lines, missing...), "\n")
	if deleted {
		text += "\nForeman keeps a copy of each for 30 days."
	}
	return agentlink.Result{Text: text}
}
