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
var every = []string{"ask", "reply", "read_message", "switch", "commit", "merge", "abort_merge", "discard", "set_submodule", "handoff", "contribute", "escalate", "request_merge", "create_target", "withdraw_merge"}

func input(agent string, tools []string) Input {
	return Input{
		Agent: agent, Team: atlas, Project: "ledger", Sandbox: "/home/ceo/workspace/ledger/agents/" + agent, ProjectRepo: "/home/ceo/workspace/ledger/project", Tools: tools,
		Repos:  []home.Repo{{Name: "api", Default: "main"}},
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
Goal: the "Current goal" line at the top of /home/ceo/workspace/ledger/project/wiki/backlog/index.md, kept by your lead. It changes over time: read it again before each new piece of work. While there is none yet, your lead tells you what to work on.
Your sandbox: /home/ceo/workspace/ledger/agents/tove (your own git worktree of each code repo)
Branches: you work on wt/ledger/tove/main, made for you from main.
Project repo: /home/ceo/workspace/ledger/project (the project's one shared checkout, the same for every agent)
Knowledge base: /home/ceo/workspace/ledger/project/wiki (read only, by design, not a fault)
It is the project's live wiki, not a copy: the whole team reads the same files, so what is written there is shared at once.
Read /home/ceo/workspace/ledger/project/AGENTS.md first: how this project's wiki and tasks work.

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
		"Break the goal into cards on the board. Assign cards and review every handoff before integrating. You are the only member who requests merges into a target", // the role's default job
		"Branches: lead/ledger/atlas/<topic>, as many as you need (lead/ledger/atlas/integration exists).",
		"(read and write: edit, then commit its repo, project)",
		"- Mara, Team Lead (lead: integrates, requests merges)\n",
		"## Tools (from Foreman)\n",
		"request_merge(branch, entry, into)  Ask the CEO to merge one of your lead/ledger/atlas/* branches into into,",
		"create_target(repo, name, from)     Make a new target in one repo",
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
	if !strings.Contains(member, "Branches: wt/ledger/ilya/<card> only;") || !strings.Contains(member, "(read only, by design, not a fault: add to it with contribute(), also when a card or a teammate says to write a wiki file)") {
		t.Errorf("member branches or wiki access:\n%s", member)
	}
	lead := Compose(input("mara", nil))
	for _, absent := range []string{"## Tools", "request_merge", "commit()", "ask()"} {
		if strings.Contains(lead, absent) {
			t.Errorf("an epic 1 prompt mentions %q:\n%s", absent, lead)
		}
	}
	if !strings.Contains(lead, "Branches: you work on lead/ledger/atlas/integration, made for you from main.") || !strings.Contains(lead, "(read and write)") ||
		!strings.Contains(lead, "Your teammates can't write it: ask them for content, never to write wiki files.") {
		t.Errorf("epic 1 lead:\n%s", lead)
	}
}

func TestOptionalSectionsAreLeftOut(t *testing.T) {
	in := input("ilya", nil)
	in.Team.Playbook = nil
	in.Config = home.ProjectConfig{}
	in.Repos = []home.Repo{{Name: "api", Default: "trunk"}}
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
		got := Compose(input(agent, Offered(agent == "mara", true)))
		for _, want := range []string{
			"Work with them only through the tools below.",
			"ask(teammate, request)", "reply(request_id, output)", "read_message(id)",
			"switch(branch, from)", "commit(commits)", "merge(branch)", "abort_merge()", "discard(repos)", "contribute(kind, title, body)",
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
	lead := Compose(input("mara", Offered(true, true)))
	if !strings.Contains(lead, "request_merge(branch, entry, into)") || !strings.Contains(lead, "Ask for a merge with request_merge(branch, entry, into), one per branch, into the target its work started from.") || !strings.Contains(lead, "create_target(repo, name, from)") ||
		!strings.Contains(lead, "withdraw_merge(branch, reason,      Take back your open request") || !strings.Contains(lead, "reviews()                           Where your reviews stand") ||
		!strings.Contains(lead, "team()                              Where each teammate stands") ||
		!strings.Contains(lead, "requests merges") || strings.Contains(lead, "Tell the CEO which lead branch is ready for review.") || strings.Contains(lead, "handoff(") {
		t.Errorf("lead:\n%s", lead)
	}
	member := Compose(input("ilya", Offered(false, true)))
	if !strings.Contains(member, "handoff(teammate, branch, summary)") || !strings.Contains(member, "Before any handoff, run: npm test") || !strings.Contains(member, "(read only, by design, not a fault: add to it with contribute(), also when a card or a teammate says to write a wiki file)") || strings.Contains(member, "request_merge") || strings.Contains(member, "create_target") || strings.Contains(member, "withdraw_merge") || strings.Contains(member, "team()") {
		t.Errorf("member:\n%s", member)
	}
}

// Worktrees outlive a session: every agent with git tools is told to check
// where it is and start new work from the default branch or another target;
// the lead, to name the topic it hands out and the target it starts from. Without switch there is nothing to say.
func TestWorktreesPersist(t *testing.T) {
	persist := "Your worktrees persist between runs: they may hold earlier work. Before you start, check where you are (git status, git branch). " +
		"New work goes on a new topic: switch(<topic>) makes it from the default branch, switch(<topic>, from) from another target. Earlier work you resume, or commit to its topic before switching. What you don't want: discard()."
	handOut := "Your teammates' worktrees persist too: when you give one new work, name the topic branch it should switch to, and the target it starts from when it isn't the default."
	lead, member := Compose(input("mara", Offered(true, true))), Compose(input("ilya", Offered(false, true)))
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
	lead, member := Offered(true, true), Offered(false, true)
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
	got := Template(atlas, "tove", "")
	for _, want := range []string{"Project: {project}\n", "Your sandbox: {sandbox} (", `Goal: the "Current goal" line at the top of {project repo}/wiki/backlog/index.md`, "Project repo: {project repo} (", "- {gates}", "run: {checks}", "# You are Tove, Engineer on team Atlas"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// The goal is pointed to, never copied: it changes while sessions run. The
// lead keeps it and asks the CEO for it when there is none; a member only
// reads it.
func TestTheGoalIsInTheWiki(t *testing.T) {
	at := `Goal: the "Current goal" line at the top of /home/ceo/workspace/ledger/project/wiki/backlog/index.md, `
	for agent, want := range map[string][]string{
		"mara": {at + "which you keep.", "ask the CEO for it in this session before anything else, then write it there", "update it there and tell your team"},
		"tove": {at + "kept by your lead.", "your lead tells you what to work on"},
	} {
		got := Compose(input(agent, every))
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: missing %q in:\n%s", agent, w, got)
			}
		}
		if agent == "tove" && strings.Contains(got, "ask the CEO for it") {
			t.Errorf("a member asks the CEO for the goal:\n%s", got)
		}
	}
}

// Repos on different default branches are each named; the same one is named once.
func TestTheDefaultBranchOfEachRepo(t *testing.T) {
	for _, tc := range []struct {
		repos []home.Repo
		want  string
	}{
		{[]home.Repo{{Name: "api", Default: "main"}, {Name: "web", Default: "main"}}, "made for you from main."},
		{[]home.Repo{{Name: "poc", Default: "master"}, {Name: "web", Default: "main"}}, "made for you from master in poc, main in web."},
	} {
		in := input("tove", nil)
		in.Repos = tc.repos
		if got := Compose(in); !strings.Contains(got, tc.want) {
			t.Errorf("%v: no %q in\n%s", tc.repos, tc.want, got)
		}
	}
}

// A project with no code repo (a research team's) leaves out what only code
// needs: branches, worktrees, git rules and the git tools. The lead still
// commits the project repo, by name.
func TestAProjectWithNoCodeRepo(t *testing.T) {
	for _, agent := range []string{"mara", "ilya"} {
		lead := agent == "mara"
		in := input(agent, Offered(lead, false))
		in.Repos = nil
		// From the assignment on: a built-in role's job text is the team's to choose
		full := Compose(in)
		got := full[strings.Index(full, "## This assignment"):]
		for _, gone := range []string{"Branches:", "worktree", "## Git", "switch(", "handoff(", "request_merge(", "targets(", "Tests bound for a target"} {
			if strings.Contains(got, gone) {
				t.Errorf("%s: %q in\n%s", agent, gone, got)
			}
		}
		if !strings.Contains(got, "This project has no code repo: its work is its knowledge base.") {
			t.Errorf("%s: not told there is no code repo:\n%s", agent, got)
		}
		if commits := strings.Contains(got, "commit the project\nrepo only through commit(), naming project."); commits != lead {
			t.Errorf("%s: commit note %v, want %v", agent, commits, lead)
		}
	}
}

// Targets are never in the prompt, so making or removing one leaves every
// agent's prompt as it was: targets() tells them, to the lead and members alike.
func TestTargetsAreAskedForNotListed(t *testing.T) {
	for _, agent := range []string{"ilya", "mara"} {
		in := input(agent, Offered(agent == "mara", true))
		in.Repos = []home.Repo{{Name: "api", Default: "main", Targets: []home.Target{{Name: "main", Default: true}}}}
		before := Compose(in)
		in.Repos[0].Targets = append(in.Repos[0].Targets, home.Target{Name: "release/2.0"})
		if after := Compose(in); after != before || strings.Contains(after, "release/2.0") {
			t.Errorf("%s: a new target changed the prompt", agent)
		}
		if !strings.Contains(before, "\ntargets()  ") {
			t.Errorf("%s: no targets():\n%s", agent, before)
		}
	}
}

// A lead's account of what went wrong (2026-10-23) became rules, each
// following the tool it is about: reviews filed in the wiki and linked, briefs
// the CEO decides from, few merge requests, one card or topic per branch,
// and no proof tests in main.
func TestTheLeadsLessons(t *testing.T) {
	lead, member := Compose(input("mara", Offered(true, true))), Compose(input("ilya", Offered(false, true)))
	brief := "When the CEO must decide, bring a brief that stands alone: the question in plain words, who it affects, the options with their consequences, and your recommendation."
	for _, want := range []string{
		"by its author through contribute(), or by you the moment it arrives. To pass one on, link its entry: never retype it, and never present a summary as the full text.",
		"## The CEO\nThe CEO talks to you. Break requests down and delegate with ask().\nAgree the scope with the CEO before building anything it didn't ask for.\n" + brief,
		"then call request_merge again. A lead branch holds one topic: merge in only that topic's hand-offs, never another topic's branch. Keep at most two merge requests open,",
		"Withdraw a request before you replace it.\n",
		"Tests bound for a target assert correct behaviour only. A test that passes while a bug exists (a proof, a repro) stays on its own branch, and lands with the fix, as the fix's regression test.\n",
	} {
		if !strings.Contains(lead, want) {
			t.Errorf("lead: missing %q in:\n%s", want, lead)
		}
	}
	for _, want := range []string{
		"goes in through contribute(): then point to the entry it files, never retype it.\n",
		"Never push to a target. A card's branch starts from the target the lead names (the default branch unless told) and holds only that card: never merge in another card's branch (an audit's, a proof's). To reuse a test from one, copy it in (git show <branch>:<file>).\n",
		"Tests bound for a target assert correct behaviour only.",
	} {
		if !strings.Contains(member, want) {
			t.Errorf("member: missing %q in:\n%s", want, member)
		}
	}
	for _, absent := range []string{brief, "merge requests open", "A lead branch holds", "or by you the moment"} {
		if strings.Contains(member, absent) {
			t.Errorf("member is told %q:\n%s", absent, member)
		}
	}
	// Without the tools a rule is about, it isn't said; the brief needs none.
	bare := Compose(input("mara", nil))
	for _, absent := range []string{"merge requests open", "A lead branch holds", "Tests bound for main", "contribute()"} {
		if strings.Contains(bare, absent) {
			t.Errorf("a lead without tools is told %q:\n%s", absent, bare)
		}
	}
	if !strings.Contains(bare, "## The CEO\nThe CEO talks to you.\nAgree the scope") {
		t.Errorf("a lead without tools:\n%s", bare)
	}
}

// The CEO's instructions come last, after Foreman's own rules, which win;
// none (or only blank lines) adds no section.
func TestTheCEOsInstructionsComeLast(t *testing.T) {
	in := input("tove", every)
	in.Instructions = "\n  Write commit messages in English.\n# Never use `sudo`\n\n"
	got := Compose(in)
	want := "\n## The CEO's standing instructions\nThe CEO's own rules for every agent. Where one conflicts with what Foreman says above, Foreman's rules win.\n\nWrite commit messages in English.\n# Never use `sudo`\n"
	if !strings.HasSuffix(got, want) {
		t.Errorf("not last, or not as written:\n%s", got)
	}
	for _, blank := range []string{"", " \n\t\n"} {
		in.Instructions = blank
		if got := Compose(in); strings.Contains(got, "standing instructions") {
			t.Errorf("%q adds a section:\n%s", blank, got)
		}
	}
	if got := Template(atlas, "tove", "Be brief."); !strings.HasSuffix(got, "\nBe brief.\n") {
		t.Errorf("the template leaves them out:\n%s", got)
	}
}
