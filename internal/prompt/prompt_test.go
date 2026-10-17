package prompt

import (
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/workspace"
)

var atlas = workspace.TeamConfig{
	Team: "atlas", Name: "Atlas", Lead: "mara",
	Playbook: []workspace.PlaybookStep{
		{"lead": "Turns the goal into cards"},
		{"members": "Build, then hand off"},
		{"engineer": "Write the tests first"},
		{"tove": "Reviews the UI"},
		{"ceo": "Merges"},
		{"designer": "Draws"},
	},
	Agents: map[string]workspace.Soul{
		"mara": {Name: "Mara", Role: "team_lead", Personality: "Calm, decisive."},
		"tove": {Name: "Tove", Role: "engineer", Instruction: "Own the UI."},
		"ilya": {Name: "Ilya", Role: "engineer"},
		"sam":  {Name: "Sam", Role: "security_auditor"},
	},
}

// every team tool, as the finished epic will offer them
var every = []string{"ask", "reply", "read_message", "switch", "commit", "merge", "abort_merge", "handoff", "contribute", "escalate", "request_merge"}

func input(agent string, tools []string) Input {
	return Input{
		Agent: agent, Team: atlas, Project: "ledger", Path: "/ws/ledger", Goal: "Ship invoices.\n", Tools: tools,
		Config: workspace.ProjectConfig{Main: "main", Checks: "npm test", AlwaysAskCEO: []string{"new dependency"}},
	}
}

// The whole prompt of a member in epic 1 (no team tools), exactly.
func TestMemberPromptWithoutTeamTools(t *testing.T) {
	want := `# You are Tove, Engineer on team Atlas

## Your job
Own the UI.

## This assignment
Project: ledger  (/ws/ledger)
Goal: Ship invoices.
Your sandbox: ledger/.sandboxes/tove (a git worktree per repo)
Branches: you work on wt/tove/main, made for you from main.
Knowledge base: ledger/.sandboxes/tove/project/wiki (read only)
It is the project's live wiki, not a copy: the whole team reads the same files, so what is written there is shared at once.
Read ledger/.sandboxes/tove/project/AGENTS.md first: how this project's wiki and tasks work.

## Your team on ledger
- Mara, Team Lead (lead)
- Ilya, Engineer
- Sam, Security auditor
- Tove, Engineer
You can't see their worktrees, and there is no way to reach them yet: the CEO relays what is needed.
They are separate sessions that Foreman runs: your harness's own agent or messaging tools don't reach them.

## Git
Git is read-only for you (status, diff, log, blame work). Don't commit, branch or merge:
the CEO reviews the work you leave uncommitted in your worktrees.

## The CEO
The CEO normally talks to Mara. If the CEO instructs you directly, do it, and tell Mara if it's relevant to the team.

## Playbook
1. Mara: Turns the goal into cards
2. Members: Build, then hand off
3. Ilya, Tove: Write the tests first
4. Tove: Reviews the UI
5. CEO: Merges
6. designer: Draws

## Always escalate to the CEO
- new dependency
Before you tell the CEO you're done, run: npm test
`
	if got := Compose(input("tove", nil)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestLeadPromptWithTeamTools(t *testing.T) {
	got := Compose(input("mara", every))
	for _, want := range []string{
		"# You are Mara, Team Lead on team Atlas\nCalm, decisive.\n",
		"Break the goal into cards on the board. Assign cards and review every handoff before integrating. You are the only member who requests merges to main", // the role's default job
		"Branches: lead/atlas/<topic>, as many as you need (lead/atlas/integration exists).",
		"(read and write: edit, then commit its repo, project)",
		"- Mara, Team Lead (lead: integrates, requests merges to main)\n",
		"## Tools (from Foreman)\n",
		"request_merge(branch, entry)        Ask the CEO to merge one of your lead/atlas/* branches into main;",
		"The CEO talks to you. Break requests down and delegate with ask().",
		"Before you request a merge, run: npm test",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "handoff(teammate") {
		t.Error("the lead was offered handoff")
	}
}

func TestMemberToolsAndLeadWithoutTools(t *testing.T) {
	member := Compose(input("ilya", every))
	if !strings.Contains(member, "Before any handoff, run: npm test") {
		t.Errorf("member checks line:\n%s", member)
	}
	if !strings.Contains(member, "handoff(teammate, branch, summary)") || strings.Contains(member, "request_merge(") {
		t.Errorf("member tools:\n%s", member)
	}
	if !strings.Contains(member, "Branches: wt/ilya/<card> only;") || !strings.Contains(member, "(read only: use contribute)") {
		t.Errorf("member branches or wiki access:\n%s", member)
	}
	lead := Compose(input("mara", nil))
	for _, absent := range []string{"## Tools", "request_merge", "commit()", "ask()"} {
		if strings.Contains(lead, absent) {
			t.Errorf("an epic 1 prompt mentions %q:\n%s", absent, lead)
		}
	}
	if !strings.Contains(lead, "Branches: you work on lead/atlas/integration, made for you from main.") || !strings.Contains(lead, "(read and write)") {
		t.Errorf("epic 1 lead:\n%s", lead)
	}
}

func TestOptionalSectionsAreLeftOut(t *testing.T) {
	in := input("ilya", nil)
	in.Team.Playbook = nil
	in.Config = workspace.ProjectConfig{Main: "trunk"}
	got := Compose(in)
	for _, absent := range []string{"## Playbook", "## Always escalate", "run:"} {
		if strings.Contains(got, absent) {
			t.Errorf("%q with nothing to say:\n%s", absent, got)
		}
	}
	if !strings.Contains(got, "made for you from trunk.") {
		t.Error("the project's main branch name isn't used")
	}
}

// Today's step: messaging, the git tools and request_merge (the lead's); no escalate yet.
func TestTodaysTools(t *testing.T) {
	for _, agent := range []string{"mara", "ilya"} {
		got := Compose(input(agent, Offered(agent == "mara")))
		for _, want := range []string{
			"Work with them only through the tools below.",
			"ask(teammate, request)", "reply(request_id, output)", "read_message(id)",
			"switch(branch)", "commit(commits)", "merge(branch)", "abort_merge()", "contribute(kind, title, body)",
			"branch, commit\nand merge only through switch(), commit() and merge().",
			"Messages arrive in this session as one line",
			"resume the task\nthat asked for it.\n",
			"A reply (or a handoff) is its outcome: the result, the decisions you took",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: missing %q in:\n%s", agent, want, got)
			}
		}
		for _, absent := range []string{"## Git\n", "escalate(", "approval", "the CEO relays", "made for you"} {
			if strings.Contains(got, absent) {
				t.Errorf("%s: mentions %q, which isn't built:\n%s", agent, absent, got)
			}
		}
	}
	lead := Compose(input("mara", Offered(true)))
	if !strings.Contains(lead, "request_merge(branch, entry)") || !strings.Contains(lead, "Ask for a merge to main with request_merge(branch, entry)") ||
		!strings.Contains(lead, "requests merges") || strings.Contains(lead, "Tell the CEO which lead branch is ready for review.") || strings.Contains(lead, "handoff(") {
		t.Errorf("lead:\n%s", lead)
	}
	member := Compose(input("ilya", Offered(false)))
	if !strings.Contains(member, "handoff(teammate, branch, summary)") || !strings.Contains(member, "Before any handoff, run: npm test") || !strings.Contains(member, "(read only: use contribute)") || strings.Contains(member, "request_merge") {
		t.Errorf("member:\n%s", member)
	}
}

// The role decides: handoff for members, request_merge for the lead.
func TestOfferedFollowsTheRole(t *testing.T) {
	saved := built
	t.Cleanup(func() { built = saved })
	built = every
	lead, member := strings.Join(Offered(true), ","), strings.Join(Offered(false), ",")
	if strings.Contains(lead, "handoff") || !strings.Contains(lead, "request_merge") ||
		!strings.Contains(member, "handoff") || strings.Contains(member, "request_merge") {
		t.Errorf("lead %s; member %s", lead, member)
	}
}

func TestRoleLabels(t *testing.T) {
	for role, want := range map[string]string{"team_lead": "Team Lead", "qa": "QA / Tester", "security_auditor": "Security auditor", "x": "X", "": ""} {
		if got := RoleLabel(role); got != want {
			t.Errorf("RoleLabel(%q) = %q, want %q", role, got, want)
		}
	}
}

// Before any deployment the prompt keeps the deployment's parts as placeholders.
func TestTemplateShowsThePlaceholders(t *testing.T) {
	got := Template(atlas, "tove")
	for _, want := range []string{"Project: {project}  ({path})", "Goal: {goal}", "- {gates}", "run: {checks}", "# You are Tove, Engineer on team Atlas"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// Deployed without a goal: the lead waits for the CEO's, a member for the
// lead's word; neither is told an empty goal.
func TestNoGoalSaysWhoGivesIt(t *testing.T) {
	for agent, want := range map[string]string{
		"mara": "Goal: none set yet: the CEO gives it in this session. Wait for it.\n",
		"tove": "Goal: none set yet: the CEO gives it to your lead, who tells you what to work on.\n",
	} {
		in := input(agent, nil)
		in.Goal = "  "
		if got := Compose(in); !strings.Contains(got, want) {
			t.Errorf("%s:\n%s", agent, got)
		}
	}
}
