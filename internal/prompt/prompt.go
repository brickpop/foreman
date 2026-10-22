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
	Repos   []home.Repo // the project's code repos, each with its main branch
	Goal    string      // the deployment's goal
	// Tools are the team tools Foreman offers this agent (Offered): the prompt
	// describes those and no others. Without ask, it describes working alone,
	// the CEO relaying; without commit, git is read-only and the CEO reviews.
	Tools []string
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
	{"switch", anyone, true, []string{"switch(branch)                      Go to one of your branches (made from each repo's main if new)."}},
	{"commit", anyone, true, []string{
		"commit(commits)                     Commit the repos you name, each with its own message:",
		"                                    [{repo, message}], repo being its folder in your sandbox.",
	}},
	{"merge", anyone, true, []string{
		"merge(branch)                       Merge a branch (the lead's, main, a teammate's) into yours.",
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
		"request_merge(branch, entry)        Ask the CEO to merge one of your lead/{team}/* branches into main;",
		"                                    entry: the wiki card that explains the work. Only you can.",
		"                                    Call it again on the same branch to refresh it.",
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
	line("Goal: %s", goal(in.Goal, lead))
	line("Your sandbox: %s (your own git worktree of each code repo, plus project/: the project's one shared checkout, the same for every agent)", in.Sandbox)
	line("Branches: %s", branches(in, lead, has))
	if has("switch") {
		line("%s", persisting(lead && len(in.Team.Roster()) > 1, has))
	}
	line("Knowledge base: %s/project/wiki (%s)", in.Sandbox, wikiAccess(lead, has))
	line("It is the project's live wiki, not a copy: the whole team reads the same files, so what is written there is shared at once.")
	if lead && len(in.Team.Roster()) > 1 {
		// Without this, a lead writes cards that tell members to write wiki
		// files, and a member takes its read-only mount for a fault.
		if has("contribute") {
			line("Your teammates can't write it: what they add arrives through contribute() in wiki/inbox/, for you to file. Ask them for content, never to write wiki files or commit project.")
		} else {
			line("Your teammates can't write it: ask them for content, never to write wiki files.")
		}
	}
	line("Read %s/project/AGENTS.md first: how this project's wiki and tasks work.", in.Sandbox)
	line("")
	line("## Your team on %s", in.Project)
	for _, id := range in.Team.Roster() {
		s := in.Team.Agents[id]
		switch {
		case id == in.Team.Lead && has("request_merge"):
			line("- %s, %s (lead: integrates, requests merges to main)", s.Name, RoleLabel(s.Role))
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
	return b.String()
}

// mains names the branch the agent's branches are made from: one name when
// every repo has the same main, else each repo's ("master in api, main in web").
func mains(repos []home.Repo) string {
	var each []string
	same := true
	for _, r := range repos {
		each = append(each, r.Main+" in "+r.Name)
		same = same && r.Main == repos[0].Main
	}
	switch {
	case len(repos) == 0:
		return "main"
	case same:
		return repos[0].Main
	}
	return strings.Join(each, ", ")
}

func branches(in Input, lead bool, has func(string) bool) string {
	switching := has("switch")
	switch {
	case !switching && lead:
		return fmt.Sprintf("you work on lead/%s/integration, made for you from %s.", in.Team.Team, mains(in.Repos))
	case !switching:
		return fmt.Sprintf("you work on wt/%s/main, made for you from %s.", in.Agent, mains(in.Repos))
	case lead && has("request_merge"):
		return fmt.Sprintf("lead/%s/<topic>, as many as you need (lead/%s/integration exists). Switch to one (or make it) with switch(branch). "+
			"Ask for a merge to main with request_merge(branch, entry), one per branch. When told a repo's main moved, merge it in with merge(<its main>), "+
			"resolve any conflicts and commit(), then call request_merge again.", in.Team.Team, in.Team.Team)
	case lead:
		return fmt.Sprintf("lead/%s/<topic>, as many as you need (lead/%s/integration exists). Switch to one (or make it) with switch(branch). "+
			"Merge the hand-offs you accept into it with merge(branch). When told a repo's main moved, merge it in with merge(<its main>). "+
			"Tell the CEO which lead branch is ready for review.", in.Team.Team, in.Team.Team)
	default:
		return fmt.Sprintf("wt/%s/<card> only; switch to one (or make it) with switch(branch). No push to main.", in.Agent)
	}
}

// persisting says that worktrees outlive a session: a deployment resumes
// (wiki/product/architecture/sandboxes-and-git.md#branches), so an agent may
// start on earlier work, and a lead hands out work on fresh topics.
func persisting(leadOfTeam bool, has func(string) bool) string {
	s := "Your worktrees persist between runs: they may hold earlier work. Before you start, check where you are (git status, git branch). " +
		"New work goes on a new topic: switch(<topic>) makes it from main. Earlier work you resume, or commit to its topic before switching."
	if has("discard") {
		s += " What you don't want: discard()."
	}
	if leadOfTeam {
		s += " Your teammates' worktrees persist too: when you give one new work, name the topic branch it should switch to."
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

func ceoSection(in Input, lead, asking bool) string {
	if lead {
		if asking {
			return "The CEO talks to you. Break requests down and delegate with ask()."
		}
		return "The CEO talks to you."
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
const leadMerges = " You are the only member who requests merges to main (request_merge, with the card that explains the work)."

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
// the project's parts stay as {placeholders}, so the whole shape shows.
func Template(team home.TeamConfig, agent string) string {
	return Compose(Input{
		Agent: agent, Team: team, Project: "{project}", Sandbox: "{sandbox}", Goal: "{goal}",
		Repos:  []home.Repo{{Name: "{repo}", Main: "{main}"}},
		Config: home.ProjectConfig{Checks: "{checks}", AlwaysAskCEO: []string{"{gates}"}},
		Tools:  Offered(agent == team.Lead),
	})
}

// goal is the assignment's goal, or who gives it when the CEO deployed
// without one (to give it in the lead's session).
func goal(g string, lead bool) string {
	switch {
	case strings.TrimSpace(g) != "":
		return strings.TrimSpace(g)
	case lead:
		return "none set yet: the CEO gives it in this session. Wait for it."
	default:
		return "none set yet: the CEO gives it to your lead, who tells you what to work on."
	}
}
