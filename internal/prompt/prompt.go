// Package prompt composes an instance's session prompt from its soul, its
// team, its deployment and its project (wiki/product/architecture/session-prompt.md).
// The harness adapter writes it where its harness reads standing
// instructions (Claude Code: CLAUDE.md in its config dir).
package prompt

import (
	"fmt"
	"slices"
	"strings"

	"github.com/brickpop/foreman/internal/home"
)

// Input is everything a session prompt is made of.
type Input struct {
	Agent   string          // the agent's id in Team.Agents
	Team    home.TeamConfig // souls, roster, playbook
	Project string          // the project's id
	Sandbox string          // the agent's sandbox, at the same path on the host and in its container
	Config  home.ProjectConfig
	Repos   []home.Repo // the project's code repos, each with its targets
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
)

// teamTool is a team tool as the prompt knows it: who gets it, whether it
// exists yet (the Messaging and hand-offs epic brings them in steps), and its
// lines in the Tools section ({team}: the team's id;
// wiki/product/architecture/session-prompt.md#template).
type teamTool struct {
	name  string
	role  role
	built bool
	rows  []string
}

// teamTools are every team tool, in the order the prompt lists them.
var teamTools = []teamTool{
	{"ask", anyone, true, []string{"ask(teammate, request)              Get info or work. Keep going meanwhile."}},
	{"reply", anyone, true, []string{"reply(request_id, output)           Answer a request. Only `output` is returned."}},
	{"read_message", anyone, true, []string{"read_message(id)                    Read a message to you in full (a delivery quotes only its start)."}},
	{"switch", anyone, true, []string{
		"switch(branch, from)                Go to one of your branches; a new one starts from from, a target",
		"                                    (each repo's default branch when left out).",
	}},
	{"commit", anyone, true, []string{
		"commit(commits)                     Commit the repos you name, each with its own message:",
		"                                    [{repo, message}], repo being its folder in your sandbox.",
	}},
	{"merge", anyone, true, []string{
		"merge(branch)                       Merge a branch (the lead's, a target, a teammate's) into yours.",
		"                                    On conflicts: resolve the markers, then commit those repos.",
	}},
	{"abort_merge", anyone, true, []string{"abort_merge()                       Back out of an unfinished merge."}},
	{"discard", anyone, true, []string{"discard(repos)                      Throw away what you didn't commit (all repos if none named)."}},
	{"set_submodule", anyone, true, []string{
		"set_submodule(repo, path, ref, url) Pin a submodule (e.g. lib/forge-std) to a branch, tag or commit;",
		"                                    url adds a new one. Switch and merge keep submodules in step.",
	}},
	// The lead hands nothing off: it works only on lead branches.
	{"handoff", membersOnly, true, []string{"handoff(teammate, branch, summary)  Pass finished, committed work on. Your part ends."}},
	{"contribute", anyone, true, []string{"contribute(kind, title, body)       Add to the knowledge base without writing it. kind: finding | proposal | note."}},
	{"escalate", anyone, false, []string{
		"escalate(kind, summary, options)    Ask the CEO. kind: approval | question | finding.",
		"                                    For an approval, wait for the answer before that step.",
	}},
	{"request_merge", leadOnly, true, []string{
		"request_merge(branch, entry, into)  Ask the CEO to merge one of your lead/{team}/* branches into into,",
		"                                    a target (each repo's default branch when left out);",
		"                                    entry: the wiki card that explains the work. Only you can.",
		"                                    Call it again on the same branch to refresh it.",
	}},
	{"create_target", leadOnly, true, []string{
		"create_target(repo, name, from)     Make a new target in one repo: branch name at from (a commit or",
		"                                    a branch). Work for it starts there; only the CEO merges into it.",
	}},
	{"withdraw_merge", leadOnly, true, []string{
		"withdraw_merge(branch, reason,      Take back your open request to merge a branch, before the CEO",
		"  superseded_by)                    decides it: superseded_by, another of your branches under review",
		"                                    that holds all of it. When the CEO merges a branch that holds",
		"                                    all of another under review, Foreman withdraws that one itself.",
	}},
	{"reviews", leadOnly, true, []string{
		"reviews()                           Where your reviews stand, with the CEO's comments on changes",
		"                                    requested. Read it again whenever you're unsure.",
	}},
	{"team", leadOnly, true, []string{
		"team()                              Where each teammate stands: status, last report, branches.",
		"                                    Check it before you ask a teammate how it's going.",
	}},
}

// Offered is the team tools an agent gets: those built, as its role allows.
// The team-tool router serves exactly these.
func Offered(lead bool) []string {
	var tools []string
	for _, t := range teamTools {
		if t.built && t.forRole(lead) {
			tools = append(tools, t.name)
		}
	}
	return tools
}

func (t teamTool) forRole(lead bool) bool {
	return t.role == anyone || (t.role == leadOnly) == lead
}

// forRole: whether a role may have the tool named.
func forRole(name string, lead bool) bool {
	i := slices.IndexFunc(teamTools, func(t teamTool) bool { return t.name == name })
	return i >= 0 && teamTools[i].forRole(lead)
}

// Compose returns the session prompt, as Markdown.
func Compose(in Input) string {
	soul := in.Team.Agents[in.Agent]
	lead := in.Agent == in.Team.Lead
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
	line("Goal: %s", goal(in.Sandbox, lead))
	line("Your sandbox: %s (your own git worktree of each code repo, plus project/: the project's one shared checkout, the same for every agent)", in.Sandbox)
	line("Branches: %s", branches(in, lead, has))
	if has("switch") {
		line("%s", persisting(lead && len(in.Team.Roster()) > 1, has))
	}
	if has("commit") {
		line("%s", provingTests)
	}
	line("Knowledge base: %s/project/wiki (%s)", in.Sandbox, wikiAccess(lead, has))
	line("It is the project's live wiki, not a copy: the whole team reads the same files, so what is written there is shared at once.")
	if lead && len(in.Team.Roster()) > 1 {
		// Without this, a lead writes cards that tell members to write wiki
		// files, and a member takes its read-only mount for a fault.
		if has("contribute") {
			line("Your teammates can't write it: what they add arrives through contribute() in wiki/inbox/, for you to file. Ask them for content, never to write wiki files or commit project.")
			line("%s", leadReviews)
		} else {
			line("Your teammates can't write it: ask them for content, never to write wiki files.")
		}
	}
	if !lead && has("contribute") {
		line("%s", memberReviews)
	}
	line("Read %s/project/AGENTS.md first: how this project's wiki and tasks work.", in.Sandbox)
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
	if has("ask") {
		line("You can't see their worktrees, only the branches they name. Work with them only through the tools below.")
	} else {
		line("You can't see their worktrees, and there is no way to reach them yet: the CEO relays what is needed.")
	}
	line("They are separate sessions that Foreman runs: your harness's own agent or messaging tools don't reach them.")
	if len(in.Tools) > 0 {
		line("")
		b.WriteString(tools(in, lead, has))
	}
	if !has("commit") {
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
		line("## Always escalate to the CEO")
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

// targets lists each repo's targets, its default first, when a repo has more
// than its default (with a leading space, to follow a sentence); "" otherwise.
func targets(repos []home.Repo) string {
	var each []string
	more := false
	for _, r := range repos {
		names := []string{r.Default + " (default)"}
		for _, t := range r.Targets {
			names = append(names, t.Name)
		}
		more = more || len(r.Targets) > 0
		each = append(each, r.Name+": "+strings.Join(names, ", "))
	}
	if !more {
		return ""
	}
	return " Targets, which only the CEO's merges write: " + strings.Join(each, "; ") + "."
}

func branches(in Input, lead bool, has func(string) bool) string {
	switching := has("switch")
	switch {
	case !switching && lead:
		return fmt.Sprintf("you work on lead/%s/integration, made for you from %s.", in.Team.Team, defaults(in.Repos))
	case !switching:
		return fmt.Sprintf("you work on wt/%s/main, made for you from %s.", in.Agent, defaults(in.Repos))
	case lead && has("request_merge"):
		return fmt.Sprintf("lead/%s/<topic>, as many as you need (lead/%s/integration exists). Switch to one (or make it) with switch(branch, from). "+
			"Ask for a merge with request_merge(branch, entry, into), one per branch, into the target its work started from. When told a target moved, merge it in with merge(<that target>), "+
			"resolve any conflicts and commit(), then call request_merge again.%s %s%s", in.Team.Team, in.Team.Team, oneTopic(has), fewRequests, targets(in.Repos))
	case lead:
		return fmt.Sprintf("lead/%s/<topic>, as many as you need (lead/%s/integration exists). Switch to one (or make it) with switch(branch, from). "+
			"Merge the hand-offs you accept into it with merge(branch). When told a target moved, merge it in with merge(<that target>). "+
			"Tell the CEO which lead branch is ready for review.%s%s", in.Team.Team, in.Team.Team, oneTopic(has), targets(in.Repos))
	default:
		return fmt.Sprintf("wt/%s/<card> only; switch to one (or make it) with switch(branch, from). Never push to a target.%s%s", in.Agent, oneCard(has), targets(in.Repos))
	}
}

// The rules below came from a lead's account of what went wrong
// (2026-10-23): reviews passed on as summaries, the CEO asked to decide from
// documents, seven merge requests at once, proof tests that pass while the
// bug exists reaching main, and an audit branch merged into fix branches that
// then had to be rebuilt twice.

// oneTopic keeps a lead branch to its topic (with a leading space, to follow
// a sentence).
func oneTopic(has func(string) bool) string {
	if !has("merge") {
		return ""
	}
	return " A lead branch holds one topic: merge in only that topic's hand-offs, never another topic's branch."
}

// oneCard keeps a member's branch to its card (with a leading space).
func oneCard(has func(string) bool) string {
	if !has("merge") {
		return ""
	}
	return " A card's branch starts from the target the lead names (the default branch unless told) and holds only that card: never merge in another card's branch (an audit's, a proof's). " +
		"To reuse a test from one, copy it in (git show <branch>:<file>)."
}

// fewRequests keeps the CEO's reviews to what fits in a sitting.
const fewRequests = "Keep at most two merge requests open, on topics that don't overlap, each one topic the CEO can review in one sitting. " +
	"See them through to their merge before you propose the next. Withdraw a request before you replace it."

// provingTests keeps a target's tests passing only when the code is right.
const provingTests = "Tests bound for a target assert correct behaviour only. A test that passes while a bug exists (a proof, a repro) stays on its own branch, " +
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
	s := "Your worktrees persist between runs: they may hold earlier work. Before you start, check where you are (git status, git branch). " +
		"New work goes on a new topic: switch(<topic>) makes it from the default branch, switch(<topic>, from) from another target. Earlier work you resume, or commit to its topic before switching."
	if has("discard") {
		s += " What you don't want: discard()."
	}
	if leadOfTeam {
		s += " Your teammates' worktrees persist too: when you give one new work, name the topic branch it should switch to, and the target it starts from when it isn't the default."
	}
	return s
}

func wikiAccess(lead bool, has func(string) bool) string {
	switch {
	case lead && has("commit"):
		return "read and write: edit, then commit its repo, project"
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
				rows = append(rows, strings.ReplaceAll(row, "{team}", in.Team.Team))
			}
		}
	}
	notes := ""
	if has("commit") {
		notes += "Git is read-only for you (status, diff, log, blame work); branch, commit\n" +
			"and merge only through switch(), commit() and merge().\n"
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
const leadMerges = " You are the only member who requests merges into a target (request_merge, with the card that explains the work)."

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
		Agent: agent, Team: team, Project: "{project}", Sandbox: "{sandbox}",
		Repos:        []home.Repo{{Name: "{repo}", Default: "{default}"}},
		Config:       home.ProjectConfig{Checks: "{checks}", AlwaysAskCEO: []string{"{gates}"}},
		Tools:        Offered(agent == team.Lead),
		Instructions: instructions,
	})
}

// goal says where the project's goal is: the "Current goal" line of its
// wiki's backlog index, which the lead keeps. Pointed to, not copied, since
// it changes over time while a session runs (the CEO, 2026-10-31).
func goal(sandbox string, lead bool) string {
	at := fmt.Sprintf("the \"Current goal\" line at the top of %s/project/wiki/backlog/index.md", sandbox)
	if lead {
		return at + ", which you keep. It changes over time: read it again before planning new work. When it is missing or says none yet, ask the CEO for it in this session before anything else, then write it there. When the CEO changes it, update it there and tell your team."
	}
	return at + ", kept by your lead. It changes over time: read it again before each new piece of work. While there is none yet, your lead tells you what to work on."
}
