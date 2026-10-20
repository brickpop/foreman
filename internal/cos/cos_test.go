package cos

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/home"
)

var ctx = context.Background()

// teams is a home's team files in memory: a save is refused on a stale
// version, as the home's is.
type teams struct {
	files   map[string]home.Team
	refuse  []string // what CheckTeam says
	saved   []home.TeamConfig
	version int
}

func (t *teams) Teams() []home.Team {
	var all []home.Team
	for _, team := range t.files {
		all = append(all, team)
	}
	return all
}

func (t *teams) Team(id string) (home.Team, error) {
	team, ok := t.files[id]
	if !ok {
		return home.Team{}, home.ErrNotFound
	}
	return team, nil
}

func (t *teams) CheckTeam(home.TeamConfig) []string { return t.refuse }

func (t *teams) SaveTeam(_ context.Context, c home.TeamConfig, version string) (home.Team, error) {
	if t.files[c.Team].Version != version {
		return home.Team{}, home.ErrConflict
	}
	t.version++
	saved := home.Team{ID: c.Team, Config: c, Version: string(rune('a' + t.version))}
	t.files[c.Team] = saved
	t.saved = append(t.saved, c)
	return saved, nil
}

type fixture struct {
	*CoS
	teams     *teams
	deployed  bool
	told      []string
	proposals []any // the cos.proposal events' data
}

func newFixture() *fixture {
	f := &fixture{teams: &teams{files: map[string]home.Team{
		"atlas": {ID: "atlas", Version: "v1", Config: home.TeamConfig{
			Team: "atlas", Name: "Atlas", Lead: "mara", Order: []string{"mara", "tove"},
			Agents: map[string]home.Soul{
				"mara": {Name: "Mara", Role: "team_lead", Look: home.Look{Color: "#D9825B"}},
				"tove": {Name: "Tove", Role: "engineer", Look: home.Look{Color: "#8DB86B"}},
			},
		}},
	}}}
	f.CoS = New(Deps{
		Teams:    f.teams,
		Deployed: func(context.Context, string) bool { return f.deployed },
		Publish: func(e events.Event) {
			if e.Type == "cos.proposal" {
				f.proposals = append(f.proposals, e.Data)
			}
		},
		Tell: func(text string) { f.told = append(f.told, text) },
	})
	return f
}

func (f *fixture) propose(t *testing.T, team string) (string, bool) {
	t.Helper()
	r := f.Call(ctx, Instance, ProposeTeam, json.RawMessage(team))
	return r.Text, r.Error
}

const vesper = `{"team": "vesper", "name": "Vesper", "purpose": "Tries to break things", "lead": "kestrel",
  "agents": {"kestrel": {"name": "Kestrel", "role": "team_lead"}, "nyx": {"name": "Nyx", "role": "qa", "skills": ["fuzzing"]}},
  "order": ["kestrel", "nyx"], "playbook": [{"lead": "Splits the work"}], "escalate": [{"when": "Blocked", "to": "ceo"}]}`

func TestANewTeamIsShownThenCreated(t *testing.T) {
	f := newFixture()
	if text, refused := f.propose(t, vesper); refused || !strings.Contains(text, "sees your proposal") {
		t.Fatalf("%q %v", text, refused)
	}
	p, ok := f.Proposal()
	if !ok || p.Team.Name != "Vesper" || p.Current != nil || len(f.proposals) != 1 {
		t.Fatalf("on screen: %+v %v %d", p, ok, len(f.proposals))
	}
	if len(f.teams.saved) != 0 {
		t.Fatal("a proposal wrote the team")
	}
	look := home.Look{Color: "#6FA8DC", Skin: "#E2AE85", Hair: "bun", HairColor: "#2B2420", Eyes: "dots", Beard: "none"}
	saved, err := f.Create(ctx, p.ID, map[string]home.Look{"nyx": look, "ghost": look})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID != "vesper" || f.teams.saved[0].Agents["nyx"].Look != look || f.teams.saved[0].Agents["kestrel"].Look != (home.Look{}) {
		t.Errorf("saved %+v", f.teams.saved)
	}
	if _, ok := f.Proposal(); ok || f.proposals[len(f.proposals)-1] != nil {
		t.Error("the proposal stayed on screen")
	}
	if len(f.told) != 1 || !strings.Contains(f.told[0], "created Vesper") {
		t.Errorf("told %q", f.told)
	}
	// Created once: the same proposal again is gone.
	if _, err := f.Create(ctx, p.ID, nil); !errors.Is(err, ErrStale) {
		t.Errorf("twice: %v", err)
	}
}

func TestOnlyTheProposalOnScreenIsCreated(t *testing.T) {
	f := newFixture()
	f.propose(t, vesper)
	first, _ := f.Proposal()
	f.propose(t, strings.Replace(vesper, `"Tries to break things"`, `"Breaks things"`, 1))
	second, _ := f.Proposal()
	if second.ID == first.ID || second.Team.Purpose != "Breaks things" {
		t.Fatalf("second %+v", second)
	}
	if _, err := f.Create(ctx, first.ID, nil); !errors.Is(err, ErrStale) || len(f.teams.saved) != 0 {
		t.Errorf("the replaced one: %v", err)
	}
	f.Drop()
	if _, err := f.Create(ctx, second.ID, nil); !errors.Is(err, ErrStale) || len(f.teams.saved) != 0 {
		t.Errorf("after the session ended: %v", err)
	}
}

// A change to the team between the proposal and the click writes nothing,
// and the Chief of Staff is asked to propose again from the file.
func TestATeamChangedMeanwhileIsNotOverwritten(t *testing.T) {
	f := newFixture()
	hire := `{"team": "atlas", "name": "Atlas", "lead": "mara", "agents": {
	  "mara": {"name": "Mara", "role": "team_lead", "look": {"color": "#D9825B"}},
	  "tove": {"name": "Tove", "role": "engineer", "look": {"color": "#8DB86B"}},
	  "arden": {"name": "Arden", "role": "engineer"}}}`
	f.propose(t, hire)
	p, _ := f.Proposal()
	if p.Current == nil || p.Current.Name != "Atlas" || len(p.Current.Agents) != 2 {
		t.Errorf("current %+v", p.Current)
	}
	if got := p.Team.Order; len(got) != 2 || got[0] != "mara" {
		t.Errorf("an existing team keeps its file's order: %v", got)
	}
	edited := f.teams.files["atlas"]
	edited.Version = "v2" // the CEO edited Tove on the Agent page
	f.teams.files["atlas"] = edited
	if _, err := f.Create(ctx, p.ID, nil); !errors.Is(err, ErrChanged) || len(f.teams.saved) != 0 {
		t.Fatalf("over a change: %v", err)
	}
	if _, ok := f.Proposal(); ok || len(f.told) != 1 || !strings.Contains(f.told[0], "propose it again") {
		t.Errorf("after: told %q", f.told)
	}
}

func TestRefusedProposalsAreNeverShown(t *testing.T) {
	f := newFixture()
	f.teams.refuse = []string{`lead: "ghost" is not an agent of the team`}
	if text, refused := f.propose(t, vesper); !refused || !strings.Contains(text, `"ghost"`) {
		t.Errorf("invalid: %q", text)
	}
	f.teams.refuse = nil
	for _, bad := range []string{`{"team": "x", "color": "red"}`, `not json`, `{"team": "x", "agents": {"a": {"name": "A", "role": "qa", "harness": "pi"}}}`} {
		if text, refused := f.propose(t, bad); !refused || !strings.HasPrefix(text, "invalid:") {
			t.Errorf("%s: %q", bad, text)
		}
	}
	// A deployed team keeps its members.
	f.deployed = true
	if text, refused := f.propose(t, `{"team": "atlas", "name": "Atlas", "lead": "mara", "agents": {"mara": {"name": "Mara", "role": "team_lead"}}}`); !refused || !strings.Contains(text, "tove is missing") {
		t.Errorf("a removal: %q", text)
	}
	if r := f.Call(ctx, Instance, "ask", nil); !r.Error {
		t.Error("another tool answered")
	}
	if _, ok := f.Proposal(); ok || len(f.proposals) != 0 {
		t.Error("a refused proposal reached the page")
	}
}

func TestThePromptNamesWhatIsTaken(t *testing.T) {
	f := newFixture()
	text := Prompt(f.teams.Teams(), "/ws/.foreman/teams")
	for _, want := range []string{"/ws/.foreman/teams", ProposeTeam, "Team ids: atlas.", "mara (Mara, atlas)", "tove (Tove, atlas)", "`qa` (QA / Tester): Try to break"} {
		if !strings.Contains(text, want) {
			t.Errorf("the prompt lacks %q", want)
		}
	}
	if empty := Prompt(nil, "/t"); !strings.Contains(empty, "Team ids: none yet.") {
		t.Error("no teams")
	}
}
