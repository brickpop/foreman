// Package prompt composes an instance's session prompt from its soul, its
// team, its deployment and its project (wiki/product/architecture/session-prompt.md).
// The harness adapter writes it where its harness reads standing
// instructions (Claude Code: CLAUDE.md in its config dir).
package prompt

import (
	"fmt"
	"sort"
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
	// TeamTools says whether the agent link and its tools exist (epic 2). Without
	// them the prompt describes working alone, the CEO relaying.
	TeamTools bool
}

// Compose returns the session prompt, as Markdown.
func Compose(in Input) string {
	soul := in.Team.Agents[in.Agent]
	lead := in.Agent == in.Team.Lead
	var b strings.Builder
	line := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }

	line("# You are %s, %s on team %s", soul.Name, RoleLabel(soul.Role), in.Team.Name)
	if soul.Personality != "" {
		line("%s", strings.TrimSpace(soul.Personality))
	}
	line("")
	line("## Your job")
	line("%s", instruction(soul, in.TeamTools))
	line("")
	line("## This assignment")
	line("Project: %s  (%s)", in.Project, in.Path)
	line("Goal: %s", strings.TrimSpace(in.Goal))
	line("Your sandbox: %s/.sandboxes/%s (a git worktree per repo)", in.Project, in.Agent)
	line("Branches: %s", branches(in, lead))
	line("Knowledge base: %s/.sandboxes/%s/project/wiki (%s)", in.Project, in.Agent, wikiAccess(in, lead))
	line("")
	line("## Your team on %s", in.Project)
	for _, id := range roster(in.Team) {
		s := in.Team.Agents[id]
		switch {
		case id == in.Team.Lead && in.TeamTools:
			line("- %s, %s (lead: integrates, requests merges to main)", s.Name, RoleLabel(s.Role))
		case id == in.Team.Lead:
			line("- %s, %s (lead)", s.Name, RoleLabel(s.Role))
		default:
			line("- %s, %s", s.Name, RoleLabel(s.Role))
		}
	}
	if in.TeamTools {
		line("You can't see their worktrees, only the branches they name. Work with them only through the tools below.")
		line("")
		b.WriteString(tools(in, lead))
	} else {
		line("You can't see their worktrees, and there is no way to reach them yet: the CEO relays what is needed.")
		line("")
		line("## Git")
		line("Git is read-only for you (status, diff, log, blame work). Don't commit, branch or merge:")
		line("the CEO reviews the work you leave uncommitted in your worktrees.")
	}
	line("")
	line("## The CEO")
	line("%s", ceoSection(in, lead))
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
		if in.TeamTools {
			line("Before any handoff, run: %s", in.Config.Checks)
		} else {
			line("Before you tell the CEO you're done, run: %s", in.Config.Checks)
		}
	}
	return b.String()
}

func branches(in Input, lead bool) string {
	switch {
	case !in.TeamTools && lead:
		return fmt.Sprintf("you work on lead/%s/integration, made for you from %s.", in.Team.Team, in.Config.Main)
	case !in.TeamTools:
		return fmt.Sprintf("you work on wt/%s/main, made for you from %s.", in.Agent, in.Config.Main)
	case lead:
		return fmt.Sprintf("lead/%s/<topic>, as many as you need (lead/%s/integration exists). Switch to one (or make it) with switch(branch). "+
			"Ask for a merge to main with request_merge(branch, entry), one per branch. When told main moved, merge it in with update(\"main\"), "+
			"resolve any conflicts and commit(), then call request_merge again.", in.Team.Team, in.Team.Team)
	default:
		return fmt.Sprintf("wt/%s/<card> only; switch to one (or make it) with switch(branch). No push to main.", in.Agent)
	}
}

func wikiAccess(in Input, lead bool) string {
	switch {
	case lead && in.TeamTools:
		return "read and write: edit, then commit()"
	case lead:
		return "read and write"
	case in.TeamTools:
		return "read only: use contribute"
	default:
		return "read only"
	}
}

func ceoSection(in Input, lead bool) string {
	if lead {
		if in.TeamTools {
			return "The CEO talks to you. Break requests down and delegate with ask() or handoff()."
		}
		return "The CEO talks to you."
	}
	leadName := in.Team.Agents[in.Team.Lead].Name
	return fmt.Sprintf("The CEO normally talks to %s. If the CEO instructs you directly, do it, and tell %s if it's relevant to the team.", leadName, leadName)
}

// tools is the Tools section (wiki/product/architecture/session-prompt.md#template).
func tools(in Input, lead bool) string {
	rows := []string{
		"ask(teammate, request)              Get info or work. Keep going meanwhile.",
		"reply(request_id, output)           Answer a request. Only `output` is returned.",
		"switch(branch)                      Go to one of your branches (made from main if new).",
		"commit(message)                     Commit your changes, in every repo, on your branch.",
		"update(branch)                      Merge a branch (the lead's, main, a teammate's) into yours.",
		"                                    On conflicts: resolve the markers, then commit().",
	}
	if !lead {
		rows = append(rows, "handoff(teammate, branch, summary)  Pass finished work on (commits first). Your part ends.")
	}
	rows = append(rows,
		"contribute(kind, title, body)       Add to the knowledge base without writing it. kind: finding | proposal | note.",
		"escalate(kind, summary, options)    Ask the CEO. kind: approval | question | finding.",
		"                                    A question doesn't pause you unless you pass wait: true.",
	)
	if lead {
		rows = append(rows,
			"request_merge(branch, entry)        Ask the CEO to merge one of your lead/"+in.Team.Team+"/* branches into main;",
			"                                    entry: the wiki card that explains the work. Only you can.",
			"                                    Call it again on the same branch to refresh it.",
		)
	}
	return "## Tools (from Foreman)\n" + strings.Join(rows, "\n") + "\n" +
		"Git is read-only for you (status, diff, log, blame work); branch, commit\n" +
		"and merge only through switch(), commit() and update().\n" +
		"Replies arrive in this session as \"⇄ <name> replied to req-N\".\n" +
		"Resume the task that asked for it. After escalate you resume\n" +
		"from the same point with the CEO's answer.\n"
}

// roster lists the team's agent ids, the lead first, then by id.
func roster(t workspace.TeamConfig) []string {
	var ids []string
	for id := range t.Agents {
		if id != t.Lead {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if _, ok := t.Agents[t.Lead]; ok {
		ids = append([]string{t.Lead}, ids...)
	}
	return ids
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
	for _, id := range roster(t) {
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
	"architect": "Propose designs as ADRs in wiki/adr/. Back every claim with a spike and numbers.",
	"reviewer":  "Review diffs and ADRs for correctness and clarity. Approve or request changes. Never push code.",
}

// leadMerges ends the lead's default job once request_merge exists.
const leadMerges = " You are the only member who requests merges to main (request_merge, with the card that explains the work)."

// instruction is the soul's job description, or its role's default
// (wiki/product/models/agent-soul.md#default-job-descriptions).
func instruction(s workspace.Soul, teamTools bool) string {
	if text := strings.TrimSpace(s.Instruction); text != "" {
		return text
	}
	if s.Role == "team_lead" && teamTools {
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
	})
}
