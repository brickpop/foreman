package home

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// hasProblem reports whether one of the problems mentions every fragment.
func hasProblem(problems []string, fragments ...string) bool {
	for _, p := range problems {
		all := true
		for _, f := range fragments {
			all = all && strings.Contains(p, f)
		}
		if all {
			return true
		}
	}
	return false
}

const projectExample = `
description = "Settlement and reconciliation service."
checks = "npm test"
always_ask_ceo = ["new dependency"]
`

func TestProjectExampleParsesWithDefaults(t *testing.T) {
	c, p := ParseProject([]byte(projectExample))
	if len(p) > 0 {
		t.Fatalf("problems: %v", p)
	}
	if c.Checks != "npm test" || len(c.AlwaysAskCEO) != 1 {
		t.Errorf("parsed %+v", c)
	}
	minimal, p := ParseProject(nil)
	if len(p) > 0 || minimal.Checks != "" {
		t.Errorf("minimal: %+v %v", minimal, p)
	}
	// Landing prefixes: the default without the key, none with [], else the list.
	if !slices.Equal(minimal.LandingPrefixes, DefaultLandingPrefixes) {
		t.Errorf("default landing prefixes: %v", minimal.LandingPrefixes)
	}
	if c, p := ParseProject([]byte("landing_prefixes = []")); len(p) > 0 || len(c.LandingPrefixes) != 0 {
		t.Errorf("none: %v %v", c.LandingPrefixes, p)
	}
	if c, p := ParseProject([]byte(`landing_prefixes = ["release/"]`)); len(p) > 0 || !slices.Equal(c.LandingPrefixes, []string{"release/"}) {
		t.Errorf("listed: %v %v", c.LandingPrefixes, p)
	}
}

func TestProjectProblems(t *testing.T) {
	for name, tc := range map[string]struct{ toml, want string }{
		"a forge (removed 2026-11-15)": {"[forge]\nkind = \"none\"", "forge.kind: not a field Foreman knows"},
		"its name, the record's":       {`project = "ledger"`, "project: not a field Foreman knows"},
		"its repos, the record's":      {`repos = ["api"]`, "repos: not a field Foreman knows"},
		"merge_to_main, retired":       {"merge_to_main = \"ceo\"", "merge_to_main: only the CEO merges, always; remove this line"},
		"unknown field":                {"wiki = \"docs\"", "wiki: not a field Foreman knows"},
		"main, now per repo":           {"main = \"main\"", "main: each code repo has its own default branch now"},
		"syntax error":                 {`checks = `, "line 1"},
		"multi-line describe":          {"description = \"\"\"a\nb\"\"\"", "one line"},
		"a landing prefix, no /":       {`landing_prefixes = ["f"]`, "landing_prefixes: \"f\""},
		"an agent's prefix":            {`landing_prefixes = ["wt/x/"]`, "never wt/ or lead/"},
	} {
		if _, p := ParseProject([]byte(tc.toml)); !hasProblem(p, tc.want) {
			t.Errorf("%s: problems %v, want one with %q", name, p, tc.want)
		}
	}
}

const teamExample = `
team = "atlas"
name = "Atlas"
purpose = "Builds features end to end."
lead = "mara"
playbook = [
  { lead = "Turns the goal into cards" },
  { members = "Build, then hand off" },
]

[[escalate]]
when = "Blocked for more than 30 minutes"
to = "lead"

[agents.mara]
name = "Mara"
role = "team_lead"
personality = "Calm."
skills = ["decomposition", "code review"]
instruction = """
Break the goal into cards.
"""

[agents.tove]
name = "Tove"
role = "security_auditor"
`

func TestTeamExampleParses(t *testing.T) {
	c, p := ParseTeam("atlas", []byte(teamExample))
	if len(p) > 0 {
		t.Fatalf("problems: %v", p)
	}
	if c.Lead != "mara" || len(c.Agents) != 2 || len(c.Playbook) != 2 || c.Playbook[1]["members"] == "" {
		t.Errorf("parsed %+v", c)
	}
}

// A team file from before 2026-11-02 has each member's look (the avatar,
// removed): it loads with no problem, and the next save drops it. Only the
// look is retired: anything else unknown is still a problem.
func TestAnOldLookLoadsAndGoes(t *testing.T) {
	old := strings.Replace(teamExample, "[agents.tove]", "[agents.mara.look]\ncolor = \"#D9825B\"\nhair = \"mohawk\"\n\n[agents.tove]", 1)
	c, p := ParseTeam("atlas", []byte(old))
	if len(p) > 0 {
		t.Fatalf("an old look: %v", p)
	}
	data, err := encodeTeam(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "look") || strings.Contains(string(data), "D9825B") {
		t.Errorf("the save kept the look:\n%s", data)
	}
	for _, src := range []string{
		strings.Replace(teamExample, "[agents.tove]", "[agents.mara.face]\ncolor = \"x\"\n\n[agents.tove]", 1),
		"look = \"x\"\n" + teamExample,
	} {
		if _, p := ParseTeam("atlas", []byte(src)); !hasProblem(p, "not a field Foreman knows") {
			t.Errorf("an unknown field passed: %v", p)
		}
	}
}

// What the UI saves comes back identical: nothing is lost in a rewrite.
func TestTeamSurvivesARewrite(t *testing.T) {
	c, _ := ParseTeam("atlas", []byte(teamExample))
	data, err := encodeTeam(c)
	if err != nil {
		t.Fatal(err)
	}
	again, p := ParseTeam("atlas", data)
	if len(p) > 0 {
		t.Fatalf("problems after rewrite: %v\n%s", p, data)
	}
	if !reflect.DeepEqual(c, again) {
		t.Errorf("rewrite changed the team:\n%+v\n%+v", c, again)
	}
}

func TestTeamProblems(t *testing.T) {
	base := teamExample
	for name, tc := range map[string]struct{ from, to, want string }{
		"file name":         {`team = "atlas"`, `team = "orion"`, "must equal the file name"},
		"lead not a member": {`lead = "mara"`, `lead = "ghost"`, `lead: "ghost"`},
		"role":              {`role = "team_lead"`, `role = "Team Lead"`, "agents.mara.role"},
		"skills":            {`skills = ["decomposition", "code review"]`, `skills = ["x", "x"]`, `"x" twice`},
		"playbook step":     {`{ lead = "Turns the goal into cards" }`, `{ lead = "a", qa = "b" }`, "playbook"},
		"agent id":          {`[agents.tove]`, `[agents.Tove_2]`, "agents.Tove_2: the agent id must be a slug"},
		"unknown field":     {`purpose =`, `mission = "x"` + "\npurpose =", "mission: not a field Foreman knows"},
		"no name":           {`name = "Atlas"`, `name = " "`, "name: required"},
		"escalation":        {`to = "lead"`, `to = ""`, "escalate[0]"},
		// Names are compared trimmed: "Mara " reads as Mara.
		"name twice": {`name = "Tove"`, `name = "Mara "`, `agents.tove.name: "Mara" is also mara's name`},
	} {
		src := strings.Replace(base, tc.from, tc.to, 1)
		if src == base {
			t.Fatalf("%s: the replacement didn't apply", name)
		}
		if _, p := ParseTeam("atlas", []byte(src)); !hasProblem(p, tc.want) {
			t.Errorf("%s: problems %v, want one with %q", name, p, tc.want)
		}
	}
}

func TestSettingsDefaultsAndProblems(t *testing.T) {
	s, p := ParseSettings(nil)
	if len(p) > 0 || !reflect.DeepEqual(s, DefaultSettings()) {
		t.Errorf("empty settings: %+v %v", s, p)
	}
	// A field set keeps the others at their defaults, booleans included.
	s, p = ParseSettings([]byte("port = 9000\n[notifications]\nfinishes = false"))
	if len(p) > 0 || s.Port != 9000 || s.MaxSessions != 12 || s.Notifications.Finishes || !s.Notifications.NeedsMe {
		t.Errorf("partial settings: %+v %v", s, p)
	}
	// listen is gone (Foreman listens on this machine only; the CEO,
	// 2026-10-30): one left in an older file is said, and the rest still loads.
	s, p = ParseSettings([]byte("listen = \"local\"\nport = 9000"))
	if s.Port != 9000 || !hasProblem(p, "listen: not a field Foreman knows") {
		t.Errorf("listen left over: %+v %v", s, p)
	}
	// A look the UI doesn't have reads as its default, its problem said; the
	// others stay as set.
	s, p = ParseSettings([]byte("theme = \"dark\"\naccent = \"chartreuse\"\nterminal_font = \"geist-mono\"\nterminal_font_size = 99"))
	if s.Theme != "dark" || s.Accent != "blue" || s.TerminalFont != "jetbrains-mono" || s.TerminalFontSize != 13 || len(p) != 3 {
		t.Errorf("a bad look: %+v %v", s, p)
	}
	for name, tc := range map[string]struct{ toml, want string }{
		"port":                            {`port = 70000`, "port"},
		"sessions":                        {`max_sessions = 0`, "max_sessions"},
		"engine":                          {`engine = "docker"`, "engine"},
		"harnesses":                       {"[harnesses.claude-code]\ncommand = \"x\"", "harnesses.claude-code.command: not a field Foreman knows"},
		"stuck":                           {`stuck_after_minutes = 0`, "stuck_after_minutes"},
		"log cap":                         {`log_cap_mb = -1`, "log_cap_mb"},
		"a forge (removed 2026-11-15)":    {"[global_forge]\nkind = \"none\"", "global_forge.kind: not a field Foreman knows"},
		"unknown":                         {`colour = "dark"`, "colour: not a field Foreman knows"},
		"a theme Foreman lacks":           {`theme = "sepia"`, "theme"},
		"instructions (moved 2026-11-15)": {`instructions = "x"`, "instructions: not a field Foreman knows"},
		"projects_folder (renamed)":       {`projects_folder = "~/w"`, "projects_folder: not a field Foreman knows"},
		"wrong type":                      {`port = "8888"`, "port"},
	} {
		if _, p := ParseSettings([]byte(tc.toml)); !hasProblem(p, tc.want) {
			t.Errorf("%s: problems %v, want one with %q", name, p, tc.want)
		}
	}
}

// The roster keeps the file's order through a parse and a save; the encoder
// alone would sort it.
func TestTeamKeepsItsRosterOrder(t *testing.T) {
	file := `team = "atlas"
name = "Atlas"
lead = "zoe"

[agents.zoe]
name = "Zoe"
role = "team_lead"

[agents.mara]
name = "Mara"
role = "engineer"

[agents.ivo]
name = "Ivo"
role = "qa"
`
	c, p := ParseTeam("atlas", []byte(file))
	if len(p) > 0 || !reflect.DeepEqual(c.Order, []string{"zoe", "mara", "ivo"}) {
		t.Fatalf("order %v, problems %v", c.Order, p)
	}
	data, err := encodeTeam(c)
	if err != nil {
		t.Fatal(err)
	}
	again, p := ParseTeam("atlas", data)
	if len(p) > 0 || !reflect.DeepEqual(again.Order, c.Order) {
		t.Errorf("after a save: order %v, problems %v\n%s", again.Order, p, data)
	}

	// An agent the order doesn't name (a new hire) goes last; a name with no
	// agent is dropped.
	c.Order = []string{"ivo", "gone"}
	data, _ = encodeTeam(c)
	again, _ = ParseTeam("atlas", data)
	if !reflect.DeepEqual(again.Order, []string{"ivo", "mara", "zoe"}) {
		t.Errorf("order %v\n%s", again.Order, data)
	}
}

func TestRosterPutsTheLeadFirstThenTheFileOrder(t *testing.T) {
	c := TeamConfig{
		Lead:   "mara",
		Agents: map[string]Soul{"mara": {}, "tove": {}, "ivo": {}, "ada": {}},
		Order:  []string{"tove", "mara", "ivo", "gone"},
	}
	if got := c.Roster(); !reflect.DeepEqual(got, []string{"mara", "tove", "ivo", "ada"}) {
		t.Errorf("roster %v", got)
	}
}

// A project.toml from before each repo had its own default, and before
// merge_to_main was retired, says only what replaced each line, not also that
// it is unknown; what it holds is ignored.
func TestRetiredLinesSayWhatReplacedThem(t *testing.T) {
	c, p := ParseProject([]byte("main = \"main\"\nmerge_to_main = \"lead\"\nchecks = \"go test\"\n"))
	if len(p) != 2 || !strings.HasPrefix(p[0], "main: ") || !strings.HasPrefix(p[1], "merge_to_main: ") || c.Checks != "go test" {
		t.Errorf("problems %v, config %+v", p, c)
	}
}

// A version is 128 bits of SHA-256, as 32 hex characters (wiki's
// FileVersion), for Foreman's files and wiki pages alike: a guard a hostile
// writer can't forge a collision for. Pinned to SHA-256("")'s first 16 bytes.
func TestVersionIs128BitsOfSHA256(t *testing.T) {
	if got := Version(nil); got != "e3b0c44298fc1c149afbf4c8996fb924" {
		t.Errorf("Version(nil) = %q", got)
	}
}
