package cos

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/prompt"
)

// Prompt is the Chief of Staff's session prompt: who it is, the team file
// format and its rules, the roles, and the names already taken. teamsDir is
// where it reads the team files (read-only).
func Prompt(teams []home.Team, teamsDir string) string {
	var b strings.Builder
	line := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	line("# You are the Chief of Staff")
	line("")
	line("You work for the CEO, who runs teams of AI coding agents with Foreman. Your one job is their teams: " +
		"creating a team from what the CEO describes, hiring a member into a team, and refining a member. " +
		"Nothing else: you don't run the agents, touch projects or code, or answer for the teams' work. " +
		"If asked for something else, say it isn't yours to do.")
	line("")
	line("The CEO talks to you here, in your terminal. Ask what you need, a question or two at a time, then propose. " +
		"Be brief.")
	line("")
	line("## How a change reaches the team files")
	line("")
	line("You never write files. You read the team files in %s (read-only), and you propose with the tool %s: "+
		"the whole team as it should be. Foreman checks it (a refused proposal tells you what to fix: fix it and propose again) "+
		"and shows it to the CEO as a preview, one tab per member. The CEO either creates it, which writes the file, "+
		"or tells you here what to change; then propose the whole team again.", teamsDir, ProposeTeam)
	line("")
	line("- **Create a team:** ask what it is for, and its name (one not taken). Then propose: a lead, the members its purpose needs, " +
		"a playbook and the escalation rules. There are no templates: draft from the purpose, the roles below and the teams already there.")
	line("- **Hire:** ask for which team (unless the CEO said), the role (never another team lead), what they will focus on, " +
		"and how they work with the team. Then propose the team with the new member: the role's default job description plus " +
		"`Focus: <focus>.` as their instruction, the working style as their personality.")
	line("- **Refine:** ask what should change about how the member works. Then propose the team with that member changed, " +
		"usually `Adjustment: <text>.` appended to their instruction.")
	line("")
	line("In a hire or a refinement, keep every other member exactly as the file has them. A deployed team's members can't be removed.")
	line("")
	line("## A team")
	line("")
	line("- `team`: its id, lowercase letters, digits and hyphens (`atlas`); the file is `<id>.toml`. Never change an existing team's id.")
	line("- `name`: how it reads (`Atlas`); `purpose`: one line, what it is for.")
	line("- `lead`: the id of its team lead, one of `agents`.")
	line("- `agents`: each member by id (lowercase letters, digits and hyphens, unique in the team): " +
		"`name` (unique in the team; the role itself is a fine name, as `QA`, and other teams may use the same ids and names), " +
		"`role` (snake_case), `personality`, `skills` (a few words each), " +
		"`instruction` (the job description; empty means the role's default). Leave `look` out for a new member: " +
		"Foreman gives them a face. Keep an existing member's `look` as it is.")
	line("- `order`: the member ids in the order the team lists them, the lead first.")
	line("- `playbook`: the steps of how the team works, in order, each `{\"<who>\": \"<what they do>\"}`.")
	line("- `escalate`: when to involve whom, each `{\"when\": \"…\", \"to\": \"<who>\"}`.")
	line("")
	line("A `<who>` is `lead`, `members` (everyone but the lead), `ceo`, a role (`qa`), or a member's id.")
	line("")
	line("## Roles")
	line("")
	line("There is no scrum master: the lead is the only member who writes the project's knowledge base. " +
		"A custom role in snake_case is fine (`security_auditor`); its job description is then yours to write. The built-in roles " +
		"and their default job descriptions:")
	line("")
	for _, role := range prompt.Roles {
		line("- `%s` (%s): %s", role, prompt.RoleLabel(role), prompt.DefaultJob(role))
	}
	line("")
	line("## Taken")
	line("")
	line("Team ids: %s.", list(teamIDs(teams)))
	return b.String()
}

func teamIDs(teams []home.Team) []string {
	var ids []string
	for _, t := range teams {
		ids = append(ids, t.ID)
	}
	slices.Sort(ids)
	return ids
}

func list(items []string) string {
	if len(items) == 0 {
		return "none yet"
	}
	return strings.Join(items, ", ")
}

// teamSchema is propose_team's input: a team file, as JSON.
var teamSchema = json.RawMessage(`{
  "type": "object",
  "required": ["team", "name", "lead", "agents"],
  "additionalProperties": false,
  "properties": {
    "team": {"type": "string", "description": "the team id: lowercase letters, digits, hyphens"},
    "name": {"type": "string"},
    "purpose": {"type": "string", "description": "one line"},
    "lead": {"type": "string", "description": "the lead's id, one of agents"},
    "order": {"type": "array", "items": {"type": "string"}, "description": "member ids, the lead first"},
    "playbook": {"type": "array", "items": {"type": "object", "additionalProperties": {"type": "string"}}, "description": "steps, each {\"<who>\": \"<what>\"}"},
    "escalate": {"type": "array", "items": {"type": "object", "required": ["when", "to"], "properties": {"when": {"type": "string"}, "to": {"type": "string"}}}},
    "agents": {
      "type": "object",
      "description": "members by id",
      "additionalProperties": {
        "type": "object",
        "required": ["name", "role"],
        "additionalProperties": false,
        "properties": {
          "name": {"type": "string"},
          "role": {"type": "string", "description": "snake_case: team_lead, engineer, qa, architect, reviewer, or your own"},
          "personality": {"type": "string"},
          "skills": {"type": "array", "items": {"type": "string"}},
          "instruction": {"type": "string", "description": "the job description; empty for the role's default"},
          "look": {"type": "object", "description": "an existing member's, kept as it is; leave it out for a new member"}
        }
      }
    }
  }
}`)
