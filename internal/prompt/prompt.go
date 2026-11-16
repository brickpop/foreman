// Package prompt composes an instance's session prompt from its soul, its
// team, its deployment and its project (wiki/product/architecture/session-prompt.md).
// The harness adapter writes it where its harness reads standing
// instructions (Claude Code: CLAUDE.md in its config dir).
package prompt

import (
	"fmt"
	"slices"
	"strings"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
)

// Input is everything a session prompt is made of.
type Input struct {
	Agent   string          // the agent's id in Team.Agents
	Team    home.TeamConfig // souls, roster, playbook
	Project string          // the project's id
	Sandbox string          // the agent's sandbox, at the same path on the host and in its container
	HQ      string          // the project's HQ, at the same path on the host and in its container
	Files   string          // the project's files (inbox/, shared/), at the same path on the host and in its container
	Config  home.ProjectConfig
	Repos   []home.Repo // the project's code repos, each with its default branch
	// Tools are the team tools Foreman offers this agent (Offered): the prompt
	// describes those and no others. Without ask, it describes working alone,
	// the CEO relaying; without commit, git is read-only and the CEO reviews.
	Tools []string
	// Instructions are the CEO's standing instructions (Settings), last.
	Instructions string
}

// Who may be offered a team tool.
type role int

const (
	anyone role = iota
	leadOnly
	membersOnly
	nobody
)

// teamTool is a team tool as the prompt knows it: who gets it, who of them
// needs a code repo for it (a project may have none: a research team works
// only on the wiki), whether it exists yet (the Messaging and hand-offs epic
// brings them in steps), and its lines in the Tools section ({lead}: what the
// lead's branches start with, lead/<project>/<team>/;
// wiki/product/architecture/session-prompt.md#template).
type teamTool struct {
	name  string
	role  role
	code  role
	built bool
	rows  []string
}

// teamTools are every team tool, in the order the prompt lists them.
var teamTools = []teamTool{
	{"ask", anyone, nobody, true, []string{"ask(teammate, request)              Get info or work. Keep going meanwhile."}},
	{"reply", anyone, nobody, true, []string{"reply(request_id, output)           Answer a request. Only `output` is returned."}},
	{"read_message", anyone, nobody, true, []string{"read_message(id)                    Read a message to you in full (a delivery quotes only its start)."}},
	{"open", anyone, anyone, true, []string{
		"open(repo)                          Check a code repo out in your sandbox, the first time you need its",
		"                                    files. switch, merge and set_submodule open what they need.",
	}},
	{"switch", anyone, anyone, true, []string{
		"switch(branch, from)                Go to one of your branches; a new one starts from from, any branch",
		"                                    or commit (each repo's default branch when left out).",
	}},
	{"commit", anyone, membersOnly, true, []string{
		"commit(commits)                     Commit the repos you name, each with its own message:",
		"                                    [{repo, message}], repo being its folder's name.",
	}},
	{"merge", anyone, anyone, true, []string{
		"merge(branch)                       Merge a branch (the lead's, the CEO's, a teammate's) into yours.",
		"                                    On conflicts: resolve the markers, then commit those repos.",
	}},
	{"abort_merge", anyone, anyone, true, []string{"abort_merge()                       Back out of an unfinished merge."}},
	{"discard", anyone, anyone, true, []string{"discard(repos)                      Throw away what you didn't commit (all repos if none named)."}},
	{"set_submodule", anyone, anyone, true, []string{
		"set_submodule(repo, path, ref, url) Pin a submodule (e.g. lib/forge-std) to a branch, tag or commit;",
		"                                    url adds a new one. Switch and merge keep submodules in step.",
	}},
	// The lead hands nothing off: it works only on lead branches.
	{"handoff", membersOnly, anyone, true, []string{"handoff(teammate, branch, summary)  Pass finished, committed work on. Your part ends."}},
	{"contribute", anyone, nobody, true, []string{"contribute(kind, title, body)       Add to the knowledge base without writing it. kind: finding | proposal | note."}},
	// Members take what needs the CEO to their lead, who decides what reaches them.
	{"escalate", leadOnly, nobody, true, []string{
		"escalate(kind, title, body, options)",
		"                                    Ask the CEO. kind: approval | question | finding. A short",
		"                                    one-line title; the detail, if any, in body.",
		"                                    For an approval, wait for the answer before that step.",
	}},
	{"fetch", leadOnly, anyone, true, []string{
		"fetch()                             Fetch the repos' remotes: what was pushed upstream shows as",
		"                                    origin/*. To sync with upstream (say origin/develop): fetch(), then",
		"                                    merge(\"origin/develop\"). The CEO's branches don't move.",
	}},
	{"prune", anyone, anyone, true, []string{
		"prune(branches)                     Delete your merged or superseded branches (the lead: its teammates'",
		"                                    too); one of yours named in branches goes even unmerged. Clean up after a merge.",
	}},
	{"request_merge", leadOnly, anyone, true, []string{
		"request_merge(branch, title, body, entry, into, land)",
		"                                    Ask the CEO to merge one of your {lead}* branches into into,",
		"                                    a branch of the CEO's (each repo's default when left out). title: what it",
		"                                    brings, one short line; body, if any: what to know to review it;",
		"                                    entry: the wiki card that explains the work. Only you can.",
		"                                    land, if any: a new branch it lands on instead, made from into",
		"                                    when the CEO merges ({landings}); the CEO may rename it.",
		"                                    Call it again on the same branch to refresh it.",
	}},
	{"withdraw_merge", leadOnly, anyone, true, []string{
		"withdraw_merge(branch, reason,      Take back your open request to merge a branch, before the CEO",
		"  superseded_by)                    decides it: superseded_by, another of your branches under review",
		"                                    that holds all of it. When the CEO merges a branch that holds",
		"                                    all of another under review, Foreman withdraws that one itself.",
	}},
	{"reviews", leadOnly, anyone, true, []string{
		"reviews()                           Where your reviews stand, with the CEO's comments on changes",
		"                                    requested. Read it again whenever you're unsure.",
	}},
	{"team", leadOnly, nobody, true, []string{
		"team()                              Where each teammate stands: status, last report, branches.",
		"                                    Check it before you ask a teammate how it's going.",
	}},
}

// Offered is the team tools an agent gets: those built, as its role allows,
// leaving out those it would need a code repo for on a project with none
// (code: the project has one). The team-tool router serves exactly these.
func Offered(lead, code bool) []string {
	var tools []string
	for _, t := range teamTools {
		if t.built && t.role.covers(lead) && (code || !t.code.covers(lead)) {
			tools = append(tools, t.name)
		}
	}
	return tools
}

// covers says whether the role includes the lead (lead) or a member.
func (r role) covers(lead bool) bool {
	switch r {
	case anyone:
		return true
	case leadOnly:
		return lead
	case membersOnly:
		return !lead
	}
	return false
}

// forRole: whether a role may have the tool named.
func forRole(name string, lead bool) bool {
	i := slices.IndexFunc(teamTools, func(t teamTool) bool { return t.name == name })
	return i >= 0 && teamTools[i].role.covers(lead)
}

// Compose returns the session prompt, as Markdown.
func Compose(in Input) string {
	soul := in.Team.Agents[in.Agent]
	lead := in.Agent == in.Team.Lead
	code := len(in.Repos) > 0 // a research team's project may have none: the wiki is all its work
	has := func(tool string) bool { return forRole(tool, lead) && slices.Contains(in.Tools, tool) }
	var b strings.Builder
	line := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }

	line("# You are %s, %s on team %s", soul.Name, RoleLabel(soul.Role), in.Team.Name)
	if soul.Personality != "" {
		line("%s", strings.TrimSpace(soul.Personality))
	}
	line("")
	line("## Your job")
	line("%s", instruction(soul, has("request_merge")))
	line("")
	line("## This assignment")
	line("Project: %s", in.Project)
	line("Goal: %s", goal(in.HQ, lead))
	if code {
		if has("open") {
			line("Your sandbox: %s. A code repo is checked out there, as <sandbox>/<repo>, only once you open it: open(repo), or switch, merge or set_submodule opening what they need. Before working in a repo, open it.", in.Sandbox)
			line("Code repos: %s. Read one you haven't opened through its git, no checkout needed: git -C <its path> log %s, show %s:<file>, grep -n <text> %s.", repoList(in.Repos), first(in.Repos), first(in.Repos), first(in.Repos))
		} else {
			line("Your sandbox: %s (your own git worktree of each code repo)", in.Sandbox)
		}
		line("Branches: %s", branches(in, lead, has))
	} else {
		line("Your sandbox: %s, your working folder. This project has no code repo: its work is its knowledge base.", in.Sandbox)
	}
	if has("switch") {
		line("%s", persisting(lead && len(in.Team.Roster()) > 1, has))
	}
	if has("commit") && code {
		line("%s", provingTests)
	}
	line("The project's HQ: %s (its one shared checkout, the same for every agent)", in.HQ)
	line("Knowledge base: %s/wiki (%s)", in.HQ, wikiAccess(lead, has))
	line("It is the project's live wiki, not a copy: the whole team reads the same files, so what is written there is shared at once.")
	if lead && len(in.Team.Roster()) > 1 {
		// Without this, a lead writes cards that tell members to write wiki
		// files, and a member takes its read-only mount for a fault.
		if has("contribute") {
			line("Your teammates can't write it: what they add arrives through contribute() in wiki/inbox/, for you to file. Ask them for content, never to write wiki files or commit hq.")
			line("%s", leadReviews)
		} else {
			line("Your teammates can't write it: ask them for content, never to write wiki files.")
		}
	}
	if !lead && has("contribute") {
		line("%s", memberReviews)
	}
	line("%s", files(in.Files))
	line("Read %s/AGENTS.md first: how this project's wiki and tasks work.", in.HQ)
	line("")
	line("## Your team on %s", in.Project)
	for _, id := range in.Team.Roster() {
		s := in.Team.Agents[id]
		switch {
		case id == in.Team.Lead && has("request_merge"):
			line("- %s, %s (lead: integrates, requests merges)", s.Name, RoleLabel(s.Role))
		case id == in.Team.Lead:
			line("- %s, %s (lead)", s.Name, RoleLabel(s.Role))
		default:
			line("- %s, %s", s.Name, RoleLabel(s.Role))
		}
	}
	switch {
	case has("ask") && code:
		line("You can't see their worktrees, only the branches they name. Work with them only through the tools below.")
	case has("ask"):
		line("Work with them only through the tools below.")
	case code:
		line("You can't see their worktrees, and there is no way to reach them yet: the CEO relays what is needed.")
	default:
		line("There is no way to reach them yet: the CEO relays what is needed.")
	}
	line("They are separate sessions that Foreman runs: your harness's own agent or messaging tools don't reach them.")
	if len(in.Tools) > 0 {
		line("")
		b.WriteString(tools(in, lead, has))
	}
	if !has("commit") && code {
		line("")
		line("## Git")
		line("Git is read-only for you (status, diff, log, blame work). Don't commit, branch or merge:")
		line("the CEO reviews the work you leave uncommitted in your worktrees.")
	}
	line("")
	line("## The CEO")
	line("%s", ceoSection(in, lead, has("ask")))
	if len(in.Team.Playbook) > 0 {
		line("")
		line("## Playbook")
		for i, step := range in.Team.Playbook {
			for who, text := range step {
				line("%d. %s: %s", i+1, Who(in.Team, who), text)
			}
		}
	}
	if len(in.Config.AlwaysAskCEO) > 0 {
		line("")
		if !lead && has("ask") {
			line("## Always take to your lead first, for the CEO") // it has no escalate
		} else {
			line("## Always escalate to the CEO")
		}
		for _, gate := range in.Config.AlwaysAskCEO {
			line("- %s", gate)
		}
	}
	if in.Config.Checks != "" {
		switch {
		case has("handoff"):
			line("Before any handoff, run: %s", in.Config.Checks)
		case has("request_merge"):
			line("Before you request a merge, run: %s", in.Config.Checks)
		default:
			line("Before you tell the CEO you're done, run: %s", in.Config.Checks)
		}
	}
	// Last, and below Foreman's: a personal note can't override the team
	// tools or the git policy.
	if text := strings.TrimSpace(in.Instructions); text != "" {
		line("")
		line("## The CEO's standing instructions")
		line("The CEO's own rules for every agent. Where one conflicts with what Foreman says above, Foreman's rules win.")
		line("")
		line("%s", text)
	}
	return b.String()
}

// repoList names each code repo with its path and default branch.
func repoList(repos []home.Repo) string {
	var each []string
	for _, r := range repos {
		each = append(each, fmt.Sprintf("%s at %s (default %s)", r.Name, r.Path, r.Default))
	}
	return strings.Join(each, "; ")
}

// first is the first repo's default branch, for an example.
func first(repos []home.Repo) string {
	if len(repos) == 0 {
		return "main"
	}
	return repos[0].Default
}

// defaults names the branch the agent's branches are made from: one name when
// every repo has the same default, else each repo's ("master in api, main in web").
func defaults(repos []home.Repo) string {
	var each []string
	same := true
	for _, r := range repos {
		each = append(each, r.Default+" in "+r.Name)
		same = same && r.Default == repos[0].Default
	}
	switch {
	case len(repos) == 0:
		return "main"
	case same:
		return repos[0].Default
	}
	return strings.Join(each, ", ")
}

func branches(in Input, lead bool, has func(string) bool) string {
	switching := has("switch")
	// Their names carry the project: the same team may work on another
	// project sharing a repo.
	mine := git.BranchPrefix(in.Project, in.Agent, in.Team.Team, false)
	ours := git.BranchPrefix(in.Project, in.Agent, in.Team.Team, true)
	idle := " Between pieces of work your repos sit on no branch, at their default branch: switch to a topic before you commit or merge (edits made meanwhile come along to a new topic)."
	switch {
	case !switching:
		return fmt.Sprintf("none of your own: your worktrees sit on no branch, at %s.", defaults(in.Repos))
	case lead && has("request_merge"):
		return fmt.Sprintf("%s<topic>, as many as you need. Switch to one (or make it) with switch(branch, from)."+idle+" "+
			"Ask for a merge with request_merge(branch, title, body, entry, into, land), one per branch, usually into the branch its work started from. "+
			"To stack them (main < f/a < f/b), request the first with land f/a, and the next into f/a once the CEO merged it. When told the branch it goes into moved, merge that in with merge(<that branch>), "+
			"resolve any conflicts and commit(), then call request_merge again.%s %s", ours, oneTopic(has), fewRequests)
	case lead:
		return fmt.Sprintf("%s<topic>, as many as you need. Switch to one (or make it) with switch(branch, from)."+idle+" "+
			"Merge the hand-offs you accept into it with merge(branch). When told a branch your work goes into moved, merge that in with merge(<that branch>). "+
			"Tell the CEO which lead branch is ready for review.%s", ours, oneTopic(has))
	default:
		return fmt.Sprintf("%s<card> only; switch to one (or make it) with switch(branch, from)."+idle+" Never push.%s", mine, oneCard(has))
	}
}

// The rules below came from a lead's account of what went wrong
// (2026-10-23): reviews passed on as summaries, the CEO asked to decide from
// documents, seven merge requests at once, proof tests that pass while the
// bug exists reaching main, and an audit branch merged into fix branches that
// then had to be rebuilt twice. Since 2026-11-06 they are advice, not rules
// (the CEO): a card may span several merge requests and the other way round,
// and more merge requests are fine when the work justifies them.

// oneTopic keeps a lead branch to its topic (with a leading space, to follow
// a sentence).
func oneTopic(has func(string) bool) string {
	if !has("merge") {
		return ""
	}
	return " Best, a lead branch holds one topic: merge in that topic's hand-offs, and another topic's branch only when the work needs it (its review then shows both)."
}

// oneCard keeps a member's branch to its card (with a leading space).
func oneCard(has func(string) bool) string {
	if !has("merge") {
		return ""
	}
	return " Best, a card's branch starts from where the lead says its work goes (the default branch unless told) and holds that card's work: merge in another card's branch (an audit's, a proof's) only when the work needs it. " +
		"To reuse just a test from one, copy it in (git show <branch>:<file>)."
}

// fewRequests keeps the CEO's reviews to what fits in a sitting, without
// holding work back unsaid.
const fewRequests = "Prefer few merge requests open at once, on topics that don't overlap, each one the CEO can review in one sitting, and see them through before you propose more. " +
	"When the work justifies more (separate pieces, something urgent), send them; when you hold finished work back instead, tell the CEO what waits. Withdraw a request before you replace it."

// provingTests keeps merged tests passing only when the code is right.
const provingTests = "Tests bound for a merge assert correct behaviour only. A test that passes while a bug exists (a proof, a repro) stays on its own branch, " +
	"and lands with the fix, as the fix's regression test."

// leadReviews and memberReviews keep anything up for review or a decision
// whole, in one place everyone can read.
const (
	leadReviews = "Anything up for review or a decision (a design, a gate review) is filed in the wiki: by its author through contribute(), or by you the moment it arrives. " +
		"To pass one on, link its entry: never retype it, and never present a summary as the full text."
	memberReviews = "What you put up for review or a decision (a design, a gate review) goes in through contribute(): then point to the entry it files, never retype it."
)

// persisting says that worktrees outlive a session: a deployment resumes
// (wiki/product/architecture/sandboxes-and-git.md#branches), so an agent may
// start on earlier work, and a lead hands out work on fresh topics.
func persisting(leadOfTeam bool, has func(string) bool) string {
	s := "Your worktrees persist between runs: they may hold earlier work. Before you start, check what you have open (ls your sandbox) and where each is (git status, git branch). " +
		"New work goes on a new topic: switch(<topic>) makes it from the default branch, switch(<topic>, from) from another branch. Earlier work you resume, or commit to its topic before switching."
	if has("discard") {
		s += " What you don't want: discard()."
	}
	if leadOfTeam {
		s += " Your teammates' worktrees persist too: when you give one new work, name the topic branch it should switch to, and the branch it starts from when it isn't the default."
	}
	return s
}

func wikiAccess(lead bool, has func(string) bool) string {
	switch {
	case lead && has("commit"):
		return "read and write: edit, then commit its repo, hq"
	case lead:
		return "read and write"
	case has("contribute"):
		return "read only, by design, not a fault: add to it with contribute(), also when a card or a teammate says to write a wiki file"
	default:
		return "read only, by design, not a fault"
	}
}

// ceoBrief: the CEO decides from the lead's words, and builds only what it
// agreed to.
const ceoBrief = "Agree the scope with the CEO before building anything it didn't ask for.\n" +
	"When the CEO must decide, bring a brief that stands alone: the question in plain words, who it affects, " +
	"the options with their consequences, and your recommendation. Documents are optional depth, never required reading."

func ceoSection(in Input, lead, asking bool) string {
	if lead {
		s := "The CEO talks to you."
		if asking {
			s += " Break requests down and delegate with ask()."
		}
		return s + "\n" + ceoBrief
	}
	leadName := in.Team.Agents[in.Team.Lead].Name
	return fmt.Sprintf("The CEO normally talks to %s. If the CEO instructs you directly, do it, and tell %s if it's relevant to the team.", leadName, leadName)
}

// tools is the Tools section: a row per tool offered, then how they work.
func tools(in Input, lead bool, has func(string) bool) string {
	var rows []string
	for _, t := range teamTools {
		if has(t.name) {
			for _, row := range t.rows {
				rows = append(rows, strings.NewReplacer("{lead}", git.BranchPrefix(in.Project, in.Agent, in.Team.Team, true), "{landings}", landings(in.Config)).Replace(row))
			}
		}
	}
	notes := ""
	switch {
	case has("commit") && has("switch"):
		notes += "Git is read-only for you (status, diff, log, blame work); branch, commit\n" +
			"and merge only through switch(), commit() and merge().\n"
	case has("commit"):
		notes += "Git is read-only for you (status, diff, log, blame work); commit the project's\n" +
			"HQ only through commit(), naming hq.\n"
	}
	if has("read_message") {
		notes += "Messages arrive in this session as one line, \"Message msg-N from <name> (<kind>): …\".\n" +
			"Read the whole message with read_message(\"msg-N\"). After a reply, resume the task\n"
		if has("escalate") {
			notes += "that asked for it; after an approval, wait for the CEO's answer, then resume.\n"
		} else {
			notes += "that asked for it.\n"
		}
	}
	if has("reply") {
		notes += "A reply (or a handoff) is its outcome: the result, the decisions you took, what is\n" +
			"left open, and how you checked it. A few lines, never a transcript.\n" +
			"A reply ends its ask: don't answer a reply, and don't ask() just to thank, confirm or\n" +
			"close a loop, since every ask() waits for a reply. Ask only when you need something back.\n"
	}
	return "## Tools (from Foreman)\n" + strings.Join(rows, "\n") + "\n" + notes
}

// Who resolves a playbook or escalation `who`
// (wiki/product/models/team-definition.md#playbook-and-escalation).
func Who(t home.TeamConfig, who string) string {
	switch who {
	case "lead":
		return t.Agents[t.Lead].Name
	case "members":
		return "Members"
	case "ceo":
		return "CEO"
	}
	if s, ok := t.Agents[who]; ok {
		return s.Name
	}
	var names []string
	for _, id := range t.Roster() {
		if t.Agents[id].Role == who {
			names = append(names, t.Agents[id].Name)
		}
	}
	if len(names) > 0 {
		return strings.Join(names, ", ")
	}
	return who // shown as written; the Team page flags it
}

var roleLabels = map[string]string{
	"team_lead": "Team Lead", "architect": "Architect", "engineer": "Engineer", "qa": "QA / Tester", "reviewer": "Reviewer",
}

// RoleLabel is how a role reads: the built-in labels, or a custom snake_case
// role with spaces and a capital ("security_auditor" → "Security auditor").
func RoleLabel(role string) string {
	if label, ok := roleLabels[role]; ok {
		return label
	}
	words := strings.ReplaceAll(role, "_", " ")
	if words == "" {
		return ""
	}
	return strings.ToUpper(words[:1]) + words[1:]
}

var defaultInstructions = map[string]string{
	"team_lead": "Break the goal into cards on the board. Assign cards and review every handoff before integrating.",
	"engineer":  "Work only in your own worktree. Write the test first. Hand off to the lead with a summary and test results.",
	"qa":        "Try to break whatever is handed to you: property tests, fuzzing, races. File every finding with a repro. Never fix the code yourself.",
	"architect": "Propose designs as ADRs with contribute (kind proposal). Back every claim with a spike and numbers.",
	"reviewer":  "Review diffs and ADRs for correctness and clarity. Approve or request changes. Never push code.",
}

// Roles are the built-in roles, as they are offered.
var Roles = []string{"team_lead", "engineer", "qa", "architect", "reviewer"}

// DefaultJob is a built-in role's default job description, as a session gets
// it; "" for any other role.
func DefaultJob(role string) string {
	return instruction(home.Soul{Role: role}, true)
}

// leadMerges ends the lead's default job once request_merge exists.
const leadMerges = " You are the only member who requests merges into the CEO's branches (request_merge, with the card that explains the work)."

// instruction is the soul's job description, or its role's default
// (wiki/product/models/agent-soul.md#default-job-descriptions).
func instruction(s home.Soul, merges bool) string {
	if text := strings.TrimSpace(s.Instruction); text != "" {
		return text
	}
	if s.Role == "team_lead" && merges {
		return defaultInstructions[s.Role] + leadMerges
	}
	return defaultInstructions[s.Role]
}

// Template is an agent's prompt before any deployment: the deployment's and
// the project's parts stay as {placeholders}, so the whole shape shows. The
// CEO's instructions are already known, so they show as they are.
func Template(team home.TeamConfig, agent, instructions string) string {
	return Compose(Input{
		Agent: agent, Team: team, Project: "{project}", Sandbox: "{sandbox}", HQ: "{hq}", Files: "{files}",
		Repos:        []home.Repo{{Name: "{repo}", Path: "{repo path}", Default: "{default}"}},
		Config:       home.ProjectConfig{Checks: "{checks}", AlwaysAskCEO: []string{"{gates}"}},
		Tools:        Offered(agent == team.Lead, true),
		Instructions: instructions,
	})
}

// goal says where the project's goal is: the "Current goal" line of its
// wiki's backlog index, which the lead keeps. Pointed to, not copied, since
// it changes over time while a session runs (the CEO, 2026-10-31).
func goal(hq string, lead bool) string {
	at := fmt.Sprintf("the \"Current goal\" line at the top of %s/wiki/backlog/index.md", hq)
	if lead {
		return at + ", which you keep. It changes over time: read it again before planning new work. When it is missing or says none yet, ask the CEO for it in this session before anything else, then write it there. When the CEO changes it, update it there and tell your team."
	}
	return at + ", kept by your lead. It changes over time: read it again before each new piece of work. While there is none yet, your lead tells you what to work on."
}

// files says where the project's files are: the CEO's for the team, and the
// team's own, which outlast a task and a team
// (wiki/product/models/project-config.md#the-projects-folder).
func files(dir string) string {
	return fmt.Sprintf("Files from the CEO: %s/%s (read only, by design: what the CEO gives the team to work with).\n", dir, home.FilesInbox) +
		fmt.Sprintf("Shared files: %s/%s (every agent of the project reads and writes it, and it outlasts your tasks and this team: keep there the data later work needs, such as downloads, datasets or generated assets. Knowledge goes in the wiki, code in your branches.)", dir, home.FilesShared)
}

// landings says where a lead may name a branch its merge lands on: the
// project's landing prefixes.
func landings(c home.ProjectConfig) string {
	if len(c.LandingPrefixes) == 0 {
		return "this project takes none: leave it out"
	}
	return "under " + strings.Join(c.LandingPrefixes, ", ")
}
