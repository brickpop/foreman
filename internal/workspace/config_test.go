package workspace

import (
	"reflect"
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
project = "ledger"
description = "Settlement and reconciliation service."
repos = ["ledger-api", "ledger-web"]
checks = "npm test"
always_ask_ceo = ["new dependency"]

[forge]
kind = "forgejo"
url = "https://localhost:3000/ledger"
project_repo = "https://localhost:3000/ledger/ledger-project"
`

func TestProjectExampleParsesWithDefaults(t *testing.T) {
	c, p := ParseProject("ledger", []byte(projectExample))
	if len(p) > 0 {
		t.Fatalf("problems: %v", p)
	}
	if c.Main != "main" || c.MergeToMain != "ceo" || c.Forge.Kind != "forgejo" || len(c.Repos) != 2 {
		t.Errorf("parsed %+v", c)
	}
	minimal, p := ParseProject("ledger", []byte(`project = "ledger"`))
	if len(p) > 0 || minimal.Forge.Kind != "none" || minimal.Main != "main" {
		t.Errorf("minimal: %+v %v", minimal, p)
	}
}

func TestProjectProblems(t *testing.T) {
	for name, tc := range map[string]struct{ toml, want string }{
		"folder mismatch":     {`project = "relay"`, "must equal the folder name"},
		"missing id":          {`description = "x"`, "project: required"},
		"forge without urls":  {"project = \"ledger\"\n[forge]\nkind = \"github\"", "forge.url: required"},
		"forge kind":          {"project = \"ledger\"\n[forge]\nkind = \"bitbucket\"", "forge.kind"},
		"forge url scheme":    {"project = \"ledger\"\n[forge]\nkind = \"github\"\nurl = \"ftp://x\"\nproject_repo = \"https://x/y\"", "not an http(s) URL"},
		"the project repo":    {`project = "ledger"` + "\nrepos = [\"project\"]", "not a code repo"},
		"a path as repo":      {`project = "ledger"` + "\nrepos = [\"../etc\"]", "not a code repo"},
		"merge by agents":     {`project = "ledger"` + "\nmerge_to_main = \"lead\"", "merge_to_main"},
		"unknown field":       {`project = "ledger"` + "\nwiki = \"docs\"", "wiki: unknown field"},
		"syntax error":        {`project = `, "line 1"},
		"multi-line describe": {`project = "ledger"` + "\ndescription = \"\"\"a\nb\"\"\"", "one line"},
	} {
		if _, p := ParseProject("ledger", []byte(tc.toml)); !hasProblem(p, tc.want) {
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
harness = "claude-code"
personality = "Calm."
skills = ["decomposition", "code review"]
instruction = """
Break the goal into cards.
"""

[agents.mara.look]
color = "#D9825B"
skin = "#E2AE85"
hair = "bun"
hair_color = "#2B2420"
eyes = "dots"
beard = "none"

[agents.tove]
name = "Tove"
role = "security_auditor"
harness = "custom"
command = "./bin/audit"
`

func TestTeamExampleParses(t *testing.T) {
	c, p := ParseTeam("atlas", []byte(teamExample))
	if len(p) > 0 {
		t.Fatalf("problems: %v", p)
	}
	if c.Lead != "mara" || len(c.Agents) != 2 || c.Agents["mara"].Look.Hair != "bun" || len(c.Playbook) != 2 || c.Playbook[1]["members"] == "" {
		t.Errorf("parsed %+v", c)
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
		"file name":              {`team = "atlas"`, `team = "orion"`, "must equal the file name"},
		"lead not a member":      {`lead = "mara"`, `lead = "ghost"`, `lead: "ghost"`},
		"custom without command": {`command = "./bin/audit"`, `command = ""`, "agents.tove.command: required"},
		"harness":                {`harness = "claude-code"`, `harness = "gpt"`, "agents.mara.harness"},
		"role":                   {`role = "team_lead"`, `role = "Team Lead"`, "agents.mara.role"},
		"look":                   {`hair = "bun"`, `hair = "mohawk"`, "agents.mara.look.hair"},
		"skills":                 {`skills = ["decomposition", "code review"]`, `skills = ["x", "x"]`, `"x" twice`},
		"playbook step":          {`{ lead = "Turns the goal into cards" }`, `{ lead = "a", qa = "b" }`, "playbook"},
		"agent id":               {`[agents.tove]`, `[agents.Tove_2]`, "agents.Tove_2: the agent id must be a slug"},
		"unknown field":          {`purpose =`, `mission = "x"` + "\npurpose =", "mission: unknown field"},
		"no name":                {`name = "Atlas"`, `name = " "`, "name: required"},
		"escalation":             {`to = "lead"`, `to = ""`, "escalate[0]"},
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
	if len(p) > 0 || s.Port != 9000 || s.MaxSessions != 12 || s.Notifications.Finishes || !s.Notifications.NeedsMe || !s.TeamMessageMarkers {
		t.Errorf("partial settings: %+v %v", s, p)
	}
	for name, tc := range map[string]struct{ toml, want string }{
		"listen":     {`listen = "everywhere"`, "listen"},
		"port":       {`port = 70000`, "port"},
		"sessions":   {`max_sessions = 0`, "max_sessions"},
		"engine":     {`engine = "docker"`, "engine"},
		"harnesses":  {"[harnesses.claude-code]\ncommand = \"x\"", "harnesses.claude-code.command: unknown field"},
		"stuck":      {`stuck_after_minutes = 0`, "stuck_after_minutes"},
		"log cap":    {`log_cap_mb = -1`, "log_cap_mb"},
		"forge kind": {"[forges.bitbucket]\nurl = \"https://x\"", "forges.bitbucket"},
		"repo forge": {"[workspace_forge]\nkind = \"github\"", "workspace_forge.url: required"},
		"unknown":    {`theme = "dark"`, "theme: unknown field"},
		"wrong type": {`port = "8888"`, "port"},
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
harness = "claude-code"

[agents.mara]
name = "Mara"
role = "engineer"
harness = "claude-code"

[agents.ivo]
name = "Ivo"
role = "qa"
harness = "claude-code"
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
