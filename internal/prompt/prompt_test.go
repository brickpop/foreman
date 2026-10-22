package prompt

import (
	"slices"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/home"
)

var atlas = home.TeamConfig{
	Team: "atlas", Name: "Atlas", Lead: "mara",
	Playbook: []home.PlaybookStep{
		{"lead": "Turns the goal into cards"},
		{"members": "Build, then hand off"},
		{"engineer": "Write the tests first"},
		{"tove": "Reviews the UI"},
		{"ceo": "Merges"},
		{"designer": "Draws"},
	},
	Agents: map[string]home.Soul{
		"mara": {Name: "Mara", Role: "team_lead", Personality: "Calm, decisive."},
		"tove": {Name: "Tove", Role: "engineer", Instruction: "Own the UI."},
		"ilya": {Name: "Ilya", Role: "engineer"},
		"sam":  {Name: "Sam", Role: "security_auditor"},
	},
}

// every team tool, as the finished epic will offer them
var every = []string{"ask", "reply", "read_message", "switch", "commit", "merge", "abort_merge", "discard", "set_submodule", "handoff", "contribute", "escalate", "request_merge", "withdraw_merge"}

func input(agent string, tools []string) Input {
	return Input{
		Agent: agent, Team: atlas, Project: "ledger", Sandbox: "/home/ceo/.foreman/sandboxes/ledger/" + agent, Goal: "Ship invoices.\n", Tools: tools,
		Repos:  []home.Repo{{Name: "api", Main: "main"}},
		Config: home.ProjectConfig{Checks: "npm test", AlwaysAskCEO: []string{"new dependency"}},
	}
}

// The whole prompt of a member in epic 1 (no team tools), exactly.
func TestMemberPromptWithoutTeamTools(t *testing.T) {
	want := `# You are Tove, Engineer on team Atlas

## Your job
Own the UI.

## This assignment
Project: ledger
Goal: Ship invoices.
Your sandbox: /home/ceo/.foreman/sandboxes/ledger/tove (your own git worktree of each code repo, plus project/: the project's one shared checkout, the same for every agent)
Branches: you work on wt/tove/main, made for you from main.
Knowledge base: /home/ceo/.foreman/sandboxes/ledger/tove/project/wiki (read only, by design, not a fault)
It is the project's live wiki, not a copy: the whole team reads the same files, so what is written there is shared at once.
Read /home/ceo/.foreman/sandboxes/ledger/tove/project/AGENTS.md first: how this project's wiki and tasks work.

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
		// so the lead never asks a member for what its mount refuses
		"Your teammates can't write it: what they add arrives through contribute() in wiki/inbox/, for you to file. Ask them for content, never to write wiki files or commit project.",
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
	if !strings.Contains(member, "Branches: wt/ilya/<card> only;") || !strings.Contains(member, "(read only, by design, not a fault: add to it with contribute(), also when a card or a teammate says to write a wiki file)") {
		t.Errorf("member branches or wiki access:\n%s", member)
	}
	lead := Compose(input("mara", nil))
	for _, absent := range []string{"## Tools", "request_merge", "commit()", "ask()"} {
		if strings.Contains(lead, absent) {
			t.Errorf("an epic 1 prompt mentions %q:\n%s", absent, lead)
		}
	}
	if !strings.Contains(lead, "Branches: you work on lead/atlas/integration, made for you from main.") || !strings.Contains(lead, "(read and write)") ||
		!strings.Contains(lead, "Your teammates can't write it: ask them for content, never to write wiki files.") {
		t.Errorf("epic 1 lead:\n%s", lead)
	}
}

func TestOptionalSectionsAreLeftOut(t *testing.T) {
	in := input("ilya", nil)
	in.Team.Playbook = nil
	in.Config = home.ProjectConfig{}
	in.Repos = []home.Repo{{Name: "api", Main: "trunk"}}
	got := Compose(in)
	for _, absent := range []string{"## Playbook", "## Always escalate", "run:"} {
		if strings.Contains(got, absent) {
			t.Errorf("%q with nothing to say:\n%s", absent, got)
		}
	}
	if !strings.Contains(got, "made for you from trunk.") {
		t.Error("the repo's main branch name isn't used")
	}
	solo := input("mara", every)
	solo.Team.Agents = map[string]home.Soul{"mara": atlas.Agents["mara"]}
	if got := Compose(solo); strings.Contains(got, "Your teammates") {
		t.Errorf("a lead alone is told about teammates:\n%s", got)
	}
}

// Today's step: messaging, the git tools, request_merge and withdraw_merge (the lead's); no escalate yet.
func TestTodaysTools(t *testing.T) {
	for _, agent := range []string{"mara", "ilya"} {
		got := Compose(input(agent, Offered(agent == "mara")))
		for _, want := range []string{
			"Work with them only through the tools below.",
			"ask(teammate, request)", "reply(request_id, output)", "read_message(id)",
			"switch(branch)", "commit(commits)", "merge(branch)", "abort_merge()", "discard(repos)", "contribute(kind, title, body)",
			"set_submodule(repo, path, ref, url) Pin a submodule",
			"branch, commit\nand merge only through switch(), commit() and merge().",
			"Messages arrive in this session as one line",
			"resume the task\nthat asked for it.\n",
			"A reply (or a handoff) is its outcome: the result, the decisions you took",
			"A reply ends its ask: don't answer a reply,",
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
		!strings.Contains(lead, "withdraw_merge(branch, reason,      Take back your open request") || !strings.Contains(lead, "reviews()                           Where your reviews stand") ||
		!strings.Contains(lead, "requests merges") || strings.Contains(lead, "Tell the CEO which lead branch is ready for review.") || strings.Contains(lead, "handoff(") {
		t.Errorf("lead:\n%s", lead)
	}
	member := Compose(input("ilya", Offered(false)))
	if !strings.Contains(member, "handoff(teammate, branch, summary)") || !strings.Contains(member, "Before any handoff, run: npm test") || !strings.Contains(member, "(read only, by design, not a fault: add to it with contribute(), also when a card or a teammate says to write a wiki file)") || strings.Contains(member, "request_merge") || strings.Contains(member, "withdraw_merge") {
		t.Errorf("member:\n%s", member)
	}
}

// Worktrees outlive a session: every agent with git tools is told to check
// where it is and start new work from main; the lead, to name the topic it
// hands out. Without switch there is nothing to say.
func TestWorktreesPersist(t *testing.T) {
	persist := "Your worktrees persist between runs: they may hold earlier work. Before you start, check where you are (git status, git branch). " +
		"New work goes on a new topic: switch(<topic>) makes it from main. Earlier work you resume, or commit to its topic before switching. What you don't want: discard()."
	handOut := "Your teammates' worktrees persist too: when you give one new work, name the topic branch it should switch to."
	lead, member := Compose(input("mara", Offered(true))), Compose(input("ilya", Offered(false)))
	if !strings.Contains(lead, persist+" "+handOut) {
		t.Errorf("lead:\n%s", lead)
	}
	if !strings.Contains(member, persist+"\n") || strings.Contains(member, handOut) {
		t.Errorf("member:\n%s", member)
	}
	if got := Compose(input("ilya", nil)); strings.Contains(got, "persist between runs") {
		t.Errorf("without git tools:\n%s", got)
	}
}

// The role decides: handoff for members, request_merge and withdraw_merge for
// the lead; a tool not built yet goes to nobody.
func TestOfferedFollowsTheRole(t *testing.T) {
	lead, member := Offered(true), Offered(false)
	for _, name := range []string{"handoff", "escalate"} {
		if slices.Contains(lead, name) {
			t.Errorf("the lead gets %s: %v", name, lead)
		}
	}
	for _, name := range []string{"request_merge", "withdraw_merge", "reviews", "escalate"} {
		if slices.Contains(member, name) {
			t.Errorf("a member gets %s: %v", name, member)
		}
	}
	for _, name := range []string{"ask", "commit", "discard", "request_merge", "withdraw_merge", "reviews"} {
		if !slices.Contains(lead, name) {
			t.Errorf("the lead lacks %s: %v", name, lead)
		}
	}
	if !slices.Contains(member, "handoff") || !slices.Contains(member, "ask") {
		t.Errorf("member %v", member)
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
	for _, want := range []string{"Project: {project}\n", "Your sandbox: {sandbox} (", "Goal: {goal}", "- {gates}", "run: {checks}", "# You are Tove, Engineer on team Atlas"} {
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

// Repos on different main branches are each named; the same one is named once.
func TestTheMainBranchOfEachRepo(t *testing.T) {
	for _, tc := range []struct {
		repos []home.Repo
		want  string
	}{
		{[]home.Repo{{Name: "api", Main: "main"}, {Name: "web", Main: "main"}}, "made for you from main."},
		{[]home.Repo{{Name: "poc", Main: "master"}, {Name: "web", Main: "main"}}, "made for you from master in poc, main in web."},
		{nil, "made for you from main."},
	} {
		in := input("tove", nil)
		in.Repos = tc.repos
		if got := Compose(in); !strings.Contains(got, tc.want) {
			t.Errorf("%v: no %q in\n%s", tc.repos, tc.want, got)
		}
	}
}
