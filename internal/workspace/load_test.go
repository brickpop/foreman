package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/git"
)

var ctx = context.Background()

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "--quiet", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
}

func gitLog(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "log", "--format=%s", "--name-only").CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %v %s", err, out)
	}
	return string(out)
}

// loaded returns a fresh workspace, after setup has written its files.
func loaded(t *testing.T, setup func(root string)) *Workspace {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	ws, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if setup != nil {
		setup(ws.Root())
	}
	if err := ws.Load(ctx, git.New()); err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestLoadMakesTheWorkspaceRepoWithWhatIsThere(t *testing.T) {
	ws := loaded(t, func(root string) {
		writeFile(t, filepath.Join(root, ".foreman", "teams", "atlas.toml"), teamExample)
		writeFile(t, filepath.Join(root, ".foreman", "state.db"), "runtime")
		writeFile(t, filepath.Join(root, ".foreman", "access-token"), "secret")
	})
	log := gitLog(t, ws.Path())
	if !strings.Contains(log, "Workspace repo") || !strings.Contains(log, "teams/atlas.toml") || !strings.Contains(log, ".gitignore") {
		t.Errorf("first commit:\n%s", log)
	}
	if strings.Contains(log, "state.db") || strings.Contains(log, "access-token") {
		t.Errorf("runtime files were committed:\n%s", log)
	}
	// Loading again changes nothing.
	if err := ws.Load(ctx, git.New()); err != nil {
		t.Fatal(err)
	}
	if again := gitLog(t, ws.Path()); again != log {
		t.Errorf("a second load committed:\n%s", again)
	}
}

func TestProjectsAndTheirRepos(t *testing.T) {
	ws := loaded(t, func(root string) {
		writeFile(t, filepath.Join(root, "ledger", "project", "project.toml"), `project = "ledger"`)
		gitRepo(t, filepath.Join(root, "ledger", "project"))
		gitRepo(t, filepath.Join(root, "ledger", "ledger-api"))
		gitRepo(t, filepath.Join(root, "ledger", "ledger-web"))
		gitRepo(t, filepath.Join(root, "ledger", ".sandboxes", "mara"))
		writeFile(t, filepath.Join(root, "ledger", "notes", "x.md"), "not a repo")
		writeFile(t, filepath.Join(root, "relay", "project", "project.toml"), "project = \"relay\"\nrepos = [\"relay-core\"]")
		gitRepo(t, filepath.Join(root, "candidate", "some-repo"))
		writeFile(t, filepath.Join(root, ".hidden", "project", "project.toml"), `project = ".hidden"`)
	})
	var ids []string
	for _, p := range ws.Projects() {
		ids = append(ids, p.ID)
	}
	if strings.Join(ids, ",") != "ledger,relay" {
		t.Errorf("projects %v", ids)
	}
	ledger, _ := ws.Project("ledger")
	if strings.Join(ledger.Repos, ",") != "ledger-api,ledger-web" || len(ledger.Problems) > 0 {
		t.Errorf("ledger repos %v, problems %v", ledger.Repos, ledger.Problems)
	}
	relay, _ := ws.Project("relay")
	if len(relay.Repos) != 0 || !hasProblem(relay.Problems, `"relay-core" is not a git repo`) {
		t.Errorf("relay %+v", relay)
	}
	if _, err := ws.Project("candidate"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a candidate is a project: %v", err)
	}
}

// A file broken mid-edit keeps the last good config, with its problems.
func TestABrokenFileKeepsItsLastGoodConfig(t *testing.T) {
	var root string
	ws := loaded(t, func(r string) {
		root = r
		writeFile(t, filepath.Join(r, "ledger", "project", "project.toml"), "project = \"ledger\"\nchecks = \"npm test\"")
	})
	writeFile(t, filepath.Join(root, "ledger", "project", "project.toml"), "project = \"ledger\"\nchecks = ")
	ws.reload()
	p, _ := ws.Project("ledger")
	if p.Config.Checks != "npm test" || len(p.Problems) == 0 {
		t.Errorf("after breaking: %+v", p)
	}
	// A project that was never good shows its problems with what parsed.
	writeFile(t, filepath.Join(root, "relay", "project", "project.toml"), `project = "wrong"`)
	ws.reload()
	if r, err := ws.Project("relay"); err != nil || !hasProblem(r.Problems, "must equal the folder name") {
		t.Errorf("relay %+v %v", r, err)
	}
}

func TestAgentsAreUniqueAcrossTeams(t *testing.T) {
	ws := loaded(t, func(root string) {
		writeFile(t, filepath.Join(root, ".foreman", "teams", "atlas.toml"), teamExample)
		other := strings.NewReplacer(`team = "atlas"`, `team = "orion"`, `name = "Atlas"`, `name = "Orion"`).Replace(teamExample)
		writeFile(t, filepath.Join(root, ".foreman", "teams", "orion.toml"), other)
	})
	for _, id := range []string{"atlas", "orion"} {
		team, _ := ws.Team(id)
		if !hasProblem(team.Problems, "agents.mara: the id is also used") || !hasProblem(team.Problems, `"Mara" is also used`) {
			t.Errorf("%s problems %v", id, team.Problems)
		}
	}
}

func TestSaveTeamWritesCommitsAndGuardsTheVersion(t *testing.T) {
	ws := loaded(t, nil)
	var changes []Change
	ws.notify = func(c Change) { changes = append(changes, c) }
	c, _ := ParseTeam("atlas", []byte(teamExample))

	saved, err := ws.SaveTeam(ctx, c, "")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version == "" || len(saved.Problems) > 0 {
		t.Errorf("saved %+v", saved)
	}
	if len(changes) != 1 || changes[0].Kind != "team" || changes[0].ID != "atlas" || changes[0].Version != saved.Version {
		t.Errorf("changes %v", changes)
	}
	if log := gitLog(t, ws.Path()); !strings.Contains(log, "Atlas: saved in the UI\n\nteams/atlas.toml") {
		t.Errorf("log:\n%s", log)
	}

	// A save from a stale version is refused, and writes nothing.
	c.Purpose = "Something else"
	if _, err := ws.SaveTeam(ctx, c, "stale"); !errors.Is(err, ErrConflict) {
		t.Errorf("stale save: %v", err)
	}
	if _, err := ws.SaveTeam(ctx, c, ""); !errors.Is(err, ErrConflict) {
		t.Errorf("creating an existing team: %v", err)
	}
	second, err := ws.SaveTeam(ctx, c, saved.Version)
	if err != nil {
		t.Fatal(err)
	}
	if second.Config.Purpose != "Something else" || second.Version == saved.Version {
		t.Errorf("second save %+v", second)
	}
	if n := strings.Count(gitLog(t, ws.Path()), "Atlas: saved in the UI"); n != 2 {
		t.Errorf("%d commits, want 2", n)
	}
	// Saving what is already there changes nothing, and makes no empty commit.
	if _, err := ws.SaveTeam(ctx, c, second.Version); err != nil {
		t.Errorf("unchanged save: %v", err)
	}
	if n := strings.Count(gitLog(t, ws.Path()), "Atlas: saved in the UI"); n != 2 {
		t.Errorf("an unchanged save committed: %d commits", n)
	}
}

func TestInvalidSavesWriteNothing(t *testing.T) {
	ws := loaded(t, func(root string) {
		writeFile(t, filepath.Join(root, ".foreman", "teams", "atlas.toml"), teamExample)
	})
	c, _ := ParseTeam("atlas", []byte(teamExample))
	c.Lead = "ghost"
	var invalid *InvalidError
	if _, err := ws.SaveTeam(ctx, c, ""); !errors.As(err, &invalid) || !hasProblem(invalid.Problems, "lead") {
		t.Errorf("invalid save: %v", err)
	}
	// Another team can't reuse an agent of atlas.
	orion, _ := ParseTeam("atlas", []byte(teamExample))
	orion.Team, orion.Name = "orion", "Orion"
	if _, err := ws.SaveTeam(ctx, orion, ""); !errors.As(err, &invalid) || !hasProblem(invalid.Problems, "also used") {
		t.Errorf("duplicate agent across teams: %v", err)
	}
	if _, err := os.Stat(ws.Path("teams", "orion.toml")); err == nil {
		t.Error("an invalid team was written")
	}
	if _, err := ws.SaveSettings(ctx, Settings{}, ""); !errors.As(err, &invalid) {
		t.Errorf("invalid settings: %v", err)
	}
	if _, err := os.Stat(ws.Path("settings.toml")); err == nil {
		t.Error("invalid settings were written")
	}
}

func TestSaveSettings(t *testing.T) {
	ws := loaded(t, nil)
	if f := ws.Settings(); f.Version != "" || f.Settings.Port != 8888 {
		t.Errorf("before any save %+v", f)
	}
	s := DefaultSettings()
	s.MaxSessions = 4
	v, err := ws.SaveSettings(ctx, s, "")
	if err != nil {
		t.Fatal(err)
	}
	if f := ws.Settings(); f.Version != v || f.Settings.MaxSessions != 4 {
		t.Errorf("after save %+v", f)
	}
	if again, err := ws.SaveSettings(ctx, s, v); err != nil || again != v {
		t.Errorf("an unchanged save: %q %v", again, err)
	}
	if _, err := ws.SaveSettings(ctx, s, ""); !errors.Is(err, ErrConflict) {
		t.Errorf("creating settings that exist: %v", err)
	}
	s.MaxSessions = 8
	if _, err := ws.SaveSettings(ctx, s, "stale"); !errors.Is(err, ErrConflict) {
		t.Errorf("stale settings save: %v", err)
	}
}

// The watcher reports what the CEO changes on disk, and ignores Foreman's own
// churn in .foreman/.
func TestWatchReportsChangesOnDisk(t *testing.T) {
	var root string
	ws := loaded(t, func(r string) {
		root = r
		writeFile(t, filepath.Join(r, "ledger", "project", "project.toml"), `project = "ledger"`)
	})
	got := make(chan Change, 16)
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := ws.Watch(wctx, func(c Change) { got <- c }); err != nil {
		t.Fatal(err)
	}
	expect := func(want Change) {
		t.Helper()
		deadline := time.After(3 * time.Second)
		for {
			select {
			case c := <-got:
				if c.Kind == want.Kind && c.ID == want.ID {
					return
				}
			case <-deadline:
				t.Fatalf("never got %+v", want)
			}
		}
	}
	quiet := func() {
		t.Helper()
		select {
		case c := <-got:
			t.Fatalf("unexpected change %+v", c)
		case <-time.After(3 * debounce):
		}
	}

	writeFile(t, filepath.Join(root, ".foreman", "state.db"), "churn")
	writeFile(t, filepath.Join(root, ".foreman", "logs", "x.log"), "churn")
	quiet()

	writeFile(t, filepath.Join(root, "ledger", "project", "project.toml"), "project = \"ledger\"\nchecks = \"make\"")
	expect(Change{Kind: "project", ID: "ledger"})

	gitRepo(t, filepath.Join(root, "ledger", "ledger-api"))
	expect(Change{Kind: "projects"})

	writeFile(t, filepath.Join(root, "relay", "project", "project.toml"), `project = "relay"`)
	expect(Change{Kind: "projects"})
	// The new project is watched too.
	writeFile(t, filepath.Join(root, "relay", "project", "project.toml"), "project = \"relay\"\nchecks = \"go test\"")
	expect(Change{Kind: "project", ID: "relay"})

	writeFile(t, filepath.Join(root, ".foreman", "teams", "atlas.toml"), teamExample)
	expect(Change{Kind: "team", ID: "atlas"})

	writeFile(t, filepath.Join(root, ".foreman", "settings.toml"), "port = 9000")
	expect(Change{Kind: "settings"})
}
