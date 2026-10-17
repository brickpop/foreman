// Package prompt composes an instance's session prompt from its soul, its
// team, its deployment and its project (wiki/product/architecture/session-prompt.md).
// The harness adapter writes it where its harness reads standing
// instructions (Claude Code: CLAUDE.md in its config dir).
package prompt

import (
	"fmt"
	"slices"
	"strings"

	"github.com/brickpop/foreman/internal/workspace"
)

// Input is everything a session prompt is made of.
type Input struct {
	Agent   string               // the agent's id in Team.Agents
	Team    workspace.TeamConfig // souls, roster, playbook
	Project string               // the project's id
	Path    string               // the project folder on the host
	Config  workspace.ProjectConfig
	Goal    string // the deployment's goal
	// Tools are the team tools Foreman offers this agent (Offered): the prompt
	// describes those and no others. Without ask, it describes working alone,
	// the CEO relaying; without commit, git is read-only and the CEO reviews.
	Tools []string
}

// built are the team tools that exist so far (the Messaging and hand-offs
// epic brings them in steps), in the order the prompt lists them.
var built = []string{"ask", "reply", "read_message", "switch", "commit", "merge", "abort_merge", "handoff", "contribute"}

// Offered is the team tools an agent gets: what is built, as its role allows
// (handoff for members, request_merge for the lead). The team-tool router
// serves exactly these.
func Offered(lead bool) []string {
	var tools []string
	for _, t := range built {
		if forRole(t, lead) {
			tools = append(tools, t)
		}
	}
	return tools
}

// forRole: the lead hands nothing off (it works only on lead branches), and
// only the lead requests merges.
func forRole(tool string, lead bool) bool {
	return !(tool == "handoff" && lead) && !(tool == "request_merge" && !lead)
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
	line("Project: %s  (%s)", in.Project, in.Path)
	line("Goal: %s", goal(in.Goal, lead))
	line("Your sandbox: %s/.sandboxes/%s (a git worktree per repo)", in.Project, in.Agent)
	line("Branches: %s", branches(in, lead, has))
	line("Knowledge base: %s/.sandboxes/%s/project/wiki (%s)", in.Project, in.Agent, wikiAccess(lead, has))
	line("It is the project's live wiki, not a copy: the whole team reads the same files, so what is written there is shared at once.")
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

func branches(in Input, lead bool, has func(string) bool) string {
	switching := has("switch")
	switch {
	case !switching && lead:
		return fmt.Sprintf("you work on lead/%s/integration, made for you from %s.", in.Team.Team, in.Config.Main)
	case !switching:
		return fmt.Sprintf("you work on wt/%s/main, made for you from %s.", in.Agent, in.Config.Main)
	case lead && has("request_merge"):
		return fmt.Sprintf("lead/%s/<topic>, as many as you need (lead/%s/integration exists). Switch to one (or make it) with switch(branch). "+
			"Ask for a merge to main with request_merge(branch, entry), one per branch. When told main moved, merge it in with merge(\"main\"), "+
			"resolve any conflicts and commit(), then call request_merge again.", in.Team.Team, in.Team.Team)
	case lead:
		return fmt.Sprintf("lead/%s/<topic>, as many as you need (lead/%s/integration exists). Switch to one (or make it) with switch(branch). "+
			"Merge the hand-offs you accept into it with merge(branch). When main moves, merge it in with merge(\"main\"). "+
			"Tell the CEO which lead branch is ready for review.", in.Team.Team, in.Team.Team)
	default:
		return fmt.Sprintf("wt/%s/<card> only; switch to one (or make it) with switch(branch). No push to main.", in.Agent)
	}
}

func wikiAccess(lead bool, has func(string) bool) string {
	switch {
	case lead && has("commit"):
		return "read and write: edit, then commit()"
	case lead:
		return "read and write"
	case has("contribute"):
		return "read only: use contribute"
	default:
		return "read only"
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

// toolRows are the Tools section's lines, by tool
// (wiki/product/architecture/session-prompt.md#template).
var toolRows = map[string][]string{
	"ask":          {"ask(teammate, request)              Get info or work. Keep going meanwhile."},
	"reply":        {"reply(request_id, output)           Answer a request. Only `output` is returned."},
	"read_message": {"read_message(id)                    Read a message to you in full (a delivery quotes only its start)."},
	"switch":       {"switch(branch)                      Go to one of your branches (made from main if new)."},
	"commit":       {"commit(message)                     Commit your changes, in every repo, on your branch."},
	"merge": {
		"merge(branch)                       Merge a branch (the lead's, main, a teammate's) into yours.",
		"                                    On conflicts: resolve the markers, then commit().",
	},
	"abort_merge": {"abort_merge()                       Back out of an unfinished merge."},
	"handoff":     {"handoff(teammate, branch, summary)  Pass finished, committed work on. Your part ends."},
	"contribute":  {"contribute(kind, title, body)       Add to the knowledge base without writing it. kind: finding | proposal | note."},
	"escalate": {
		"escalate(kind, summary, options)    Ask the CEO. kind: approval | question | finding.",
		"                                    For an approval, wait for the answer before that step.",
	},
}

// tools is the Tools section: a row per tool offered, then how they work.
func tools(in Input, lead bool, has func(string) bool) string {
	var rows []string
	for _, t := range []string{"ask", "reply", "read_message", "switch", "commit", "merge", "abort_merge", "handoff", "contribute", "escalate"} {
		if has(t) {
			rows = append(rows, toolRows[t]...)
		}
	}
	if has("request_merge") {
		rows = append(rows,
			"request_merge(branch, entry)        Ask the CEO to merge one of your lead/"+in.Team.Team+"/* branches into main;",
			"                                    entry: the wiki card that explains the work. Only you can.",
			"                                    Call it again on the same branch to refresh it.",
		)
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
			"left open, and how you checked it. A few lines, never a transcript.\n"
	}
	return "## Tools (from Foreman)\n" + strings.Join(rows, "\n") + "\n" + notes
}

// Who resolves a playbook or escalation `who`
// (wiki/product/models/team-definition.md#playbook-and-escalation).
func Who(t workspace.TeamConfig, who string) string {
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

// leadMerges ends the lead's default job once request_merge exists.
const leadMerges = " You are the only member who requests merges to main (request_merge, with the card that explains the work)."

// instruction is the soul's job description, or its role's default
// (wiki/product/models/agent-soul.md#default-job-descriptions).
func instruction(s workspace.Soul, merges bool) string {
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
func Template(team workspace.TeamConfig, agent string) string {
	return Compose(Input{
		Agent: agent, Team: team, Project: "{project}", Path: "{path}", Goal: "{goal}",
		Config: workspace.ProjectConfig{Main: "{main}", Checks: "{checks}", AlwaysAskCEO: []string{"{gates}"}},
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
