package home

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/store"
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

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func gitLog(t *testing.T, dir string) string {
	t.Helper()
	return gitOut(t, dir, "log", "--format=%s", "--name-only")
}

// codeRepo makes a git repo with a first commit at path, in the CEO's home
// folder unless path is absolute.
func codeRepo(t *testing.T, path string) string {
	t.Helper()
	if !filepath.IsAbs(path) {
		path = filepath.Join(os.Getenv("HOME"), path)
	}
	// The path in it: two repos made in the same second are still two.
	writeFile(t, filepath.Join(path, "README.md"), "# "+path+"\n")
	gitOut(t, path, "init", "--quiet", "--initial-branch=main")
	gitOut(t, path, "add", "README.md")
	gitOut(t, path, "commit", "--quiet", "-m", "first")
	return path
}

// loaded returns a fresh home, after setup has written its files, with its
// own database: .foreman in the CEO's home folder (HOME), a temporary one.
func loaded(t *testing.T, setup func(root string)) *Home {
	t.Helper()
	h, err := load(t, setup)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// load is loaded, returning the error Load returned.
func load(t *testing.T, setup func(root string)) (*Home, error) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "ceo")
	t.Setenv("GIT_AUTHOR_EMAIL", "ceo@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "ceo")
	t.Setenv("GIT_COMMITTER_EMAIL", "ceo@example.invalid")
	t.Setenv("HOME", t.TempDir())
	h, err := Open(filepath.Join(os.Getenv("HOME"), ".foreman"))
	if err != nil {
		t.Fatal(err)
	}
	if setup != nil {
		setup(h.Root())
	}
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return h, h.Load(ctx, Deps{Git: git.New(), Projects: db.Projects()})
}

// hqOf is Foreman's HQ of the home at root, in the default Foreman folder.
func hqOf(root string) string { return filepath.Join(filepath.Dir(root), "foreman", "hq") }

// reload loads h again, as a restart would.
func reload(t *testing.T, h *Home) {
	t.Helper()
	startedIn(t, h, h.start)
}

// startedIn loads h again as if Foreman were started in dir.
func startedIn(t *testing.T, h *Home, dir string) {
	t.Helper()
	if err := h.Load(ctx, Deps{Git: h.git, Projects: h.records, Deployed: h.deployed, Start: dir}); err != nil {
		t.Fatal(err)
	}
}

// Foreman's HQ is made on first start, with the teams put there by hand,
// an empty instructions.md and a .gitignore, in one commit; nothing of the
// home is in it, and the agents' worktrees are in no repo.
func TestLoadMakesForemansHQWithWhatIsThere(t *testing.T) {
	h := loaded(t, func(root string) {
		writeFile(t, filepath.Join(hqOf(root), "teams", "atlas.toml"), teamExample)
		writeFile(t, filepath.Join(root, "state.db"), "runtime")
		writeFile(t, filepath.Join(root, "access-token"), "secret")
		writeFile(t, filepath.Join(filepath.Dir(hqOf(root)), "worktrees", "ledger", "mara", "x"), "")
	})
	log := gitLog(t, h.HQ())
	if !strings.HasPrefix(log, "Foreman's HQ") || strings.Count(log, "\n\n") != 1 {
		t.Errorf("not one first commit:\n%s", log)
	}
	for _, f := range []string{"teams/atlas.toml", ".gitignore", "instructions.md", "wiki.toml", "index.md"} {
		if !strings.Contains(log, f) {
			t.Errorf("%s not committed:\n%s", f, log)
		}
	}
	if data, err := os.ReadFile(h.HQ("instructions.md")); err != nil || len(data) != 0 || h.Instructions().Version == "" {
		t.Errorf("instructions.md %q %v, read %+v", data, err, h.Instructions())
	}
	if status := gitOut(t, h.HQ(), "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Errorf("not committed:\n%s", status)
	}
	if out, err := exec.Command("git", "-C", filepath.Join(h.Folder(), "worktrees", "ledger", "mara"), "rev-parse", "--git-dir").CombinedOutput(); err == nil {
		t.Errorf("a sandbox is inside a repo: %s", out)
	}
	// A wiki at its root, as a project's HQ, the instructions not an entry.
	writeFile(t, h.HQ("instructions.md"), "Be brief.\n")
	if bin, err := exec.LookPath("wiki"); err == nil {
		if out, err := exec.Command(bin, "--root", h.HQ(), "check").CombinedOutput(); err != nil || !strings.Contains(string(out), "no issues") {
			t.Errorf("wiki check: %v\n%s", err, out)
		}
		if out, _ := exec.Command(bin, "--root", h.HQ(), "list").CombinedOutput(); strings.Contains(string(out), "instructions") {
			t.Errorf("instructions.md is an entry:\n%s", out)
		}
	}
	gitOut(t, h.HQ(), "checkout", "--", "instructions.md")
	// Loading again changes nothing.
	reload(t, h)
	if again := gitLog(t, h.HQ()); again != log {
		t.Errorf("a second load committed:\n%s", again)
	}
	// One whose making stopped before its first commit is made at the next start.
	h = loaded(t, func(root string) {
		writeFile(t, filepath.Join(hqOf(root), "teams", "atlas.toml"), teamExample)
		gitOut(t, hqOf(root), "init", "--quiet")
	})
	if got := gitOut(t, h.HQ(), "log", "--format=%s"); got != "Foreman's HQ" || len(h.Teams()) != 1 {
		t.Errorf("a halfway HQ: log %q, teams %d", got, len(h.Teams()))
	}
	// An HQ with no teams yet is made too.
	if h := loaded(t, nil); gitOut(t, h.HQ(), "log", "--format=%s") != "Foreman's HQ" || !exists(h.HQ("teams")) {
		t.Error("an empty HQ wasn't made")
	}
}

// A project.toml broken mid-edit keeps the last good config, with its
// problems; one that is gone is a problem, with the defaults.
func TestABrokenProjectFileKeepsItsLastGoodConfig(t *testing.T) {
	h := loaded(t, nil)
	p, _, err := h.NewProject(ctx, "ledger", "")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(p.Path, "project.toml")
	writeFile(t, file, "checks = \"npm test\"\n")
	h.reload()
	writeFile(t, file, "checks = ")
	h.reload()
	p, _ = h.Project("ledger")
	if p.Config.Checks != "npm test" || !hasProblem(p.Problems, "line 1") || p.Source != "checks = " {
		t.Errorf("after breaking: %+v", p)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	h.reload()
	p, _ = h.Project("ledger")
	if p.Version != "" || !hasProblem(p.Problems, "project.toml: not found in "+p.Path) {
		t.Errorf("without project.toml: %+v", p)
	}
}

// An agent's id and name are its team's own: two teams may each have a mara
// called Mara (the CEO, 2026-11-02: a role is the natural name).
func TestTeamsShareAgentIdsAndNames(t *testing.T) {
	h := loaded(t, func(root string) {
		writeFile(t, filepath.Join(hqOf(root), "teams", "atlas.toml"), teamExample)
		other := strings.NewReplacer(`team = "atlas"`, `team = "orion"`, `name = "Atlas"`, `name = "Orion"`).Replace(teamExample)
		writeFile(t, filepath.Join(hqOf(root), "teams", "orion.toml"), other)
	})
	for _, id := range []string{"atlas", "orion"} {
		if team, _ := h.Team(id); len(team.Problems) != 0 || team.Config.Agents["mara"].Name != "Mara" {
			t.Errorf("%s: %+v", id, team)
		}
	}
}

func TestSaveTeamWritesCommitsAndGuardsTheVersion(t *testing.T) {
	h := loaded(t, nil)
	var changes []Change
	h.notify = func(c Change) { changes = append(changes, c) }
	c, _ := ParseTeam("atlas", []byte(teamExample))

	saved, err := h.SaveTeam(ctx, c, "")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version == "" || len(saved.Problems) > 0 {
		t.Errorf("saved %+v", saved)
	}
	if len(changes) != 1 || changes[0].Kind != "team" || changes[0].ID != "atlas" || changes[0].Version != saved.Version {
		t.Errorf("changes %v", changes)
	}
	if log := gitLog(t, h.HQ()); !strings.Contains(log, "Atlas: saved in the UI\n\nteams/atlas.toml") {
		t.Errorf("log:\n%s", log)
	}

	// A save from a stale version is refused, and writes nothing.
	c.Purpose = "Something else"
	if _, err := h.SaveTeam(ctx, c, "stale"); !errors.Is(err, ErrConflict) {
		t.Errorf("stale save: %v", err)
	}
	if _, err := h.SaveTeam(ctx, c, ""); !errors.Is(err, ErrConflict) {
		t.Errorf("creating an existing team: %v", err)
	}
	second, err := h.SaveTeam(ctx, c, saved.Version)
	if err != nil {
		t.Fatal(err)
	}
	if second.Config.Purpose != "Something else" || second.Version == saved.Version {
		t.Errorf("second save %+v", second)
	}
	if n := strings.Count(gitLog(t, h.HQ()), "Atlas: saved in the UI"); n != 2 {
		t.Errorf("%d commits, want 2", n)
	}
	// Saving what is already there changes nothing, and makes no empty commit.
	if _, err := h.SaveTeam(ctx, c, second.Version); err != nil {
		t.Errorf("unchanged save: %v", err)
	}
	if n := strings.Count(gitLog(t, h.HQ()), "Atlas: saved in the UI"); n != 2 {
		t.Errorf("an unchanged save committed: %d commits", n)
	}
}

func TestInvalidSavesWriteNothing(t *testing.T) {
	h := loaded(t, func(root string) {
		writeFile(t, filepath.Join(hqOf(root), "teams", "atlas.toml"), teamExample)
	})
	c, _ := ParseTeam("atlas", []byte(teamExample))
	c.Lead = "ghost"
	var invalid *InvalidError
	if _, err := h.SaveTeam(ctx, c, ""); !errors.As(err, &invalid) || !hasProblem(invalid.Problems, "lead") {
		t.Errorf("invalid save: %v", err)
	}
	// Two of orion's agents can't share a name.
	orion, _ := ParseTeam("atlas", []byte(teamExample))
	orion.Team, orion.Name = "orion", "Orion"
	tove := orion.Agents["tove"]
	tove.Name = "Mara"
	orion.Agents["tove"] = tove
	if _, err := h.SaveTeam(ctx, orion, ""); !errors.As(err, &invalid) || !hasProblem(invalid.Problems, "is also mara's name") {
		t.Errorf("a name twice in a team: %v", err)
	}
	if _, err := os.Stat(h.HQ("teams", "orion.toml")); err == nil {
		t.Error("an invalid team was written")
	}
	if _, err := h.SaveSettings(ctx, Settings{}, ""); !errors.As(err, &invalid) {
		t.Errorf("invalid settings: %v", err)
	}
	if _, err := os.Stat(h.Path("settings.toml")); err == nil {
		t.Error("invalid settings were written")
	}
}

func TestSaveSettings(t *testing.T) {
	h := loaded(t, nil)
	if f := h.Settings(); f.Version != "" || f.Settings.Port != 8888 {
		t.Errorf("before any save %+v", f)
	}
	s := DefaultSettings()
	s.MaxSessions = 4
	v, err := h.SaveSettings(ctx, s, "")
	if err != nil {
		t.Fatal(err)
	}
	if f := h.Settings(); f.Version != v || f.Settings.MaxSessions != 4 {
		t.Errorf("after save %+v", f)
	}
	if again, err := h.SaveSettings(ctx, s, v); err != nil || again != v {
		t.Errorf("an unchanged save: %q %v", again, err)
	}
	if _, err := h.SaveSettings(ctx, s, ""); !errors.Is(err, ErrConflict) {
		t.Errorf("creating settings that exist: %v", err)
	}
	s.MaxSessions = 8
	if _, err := h.SaveSettings(ctx, s, "stale"); !errors.Is(err, ErrConflict) {
		t.Errorf("stale settings save: %v", err)
	}
}

// The watcher reports what the CEO changes on disk, and ignores Foreman's own
// churn in the home: the database, the logs, the sandboxes, an HQ's
// commits.
func TestWatchReportsChangesOnDisk(t *testing.T) {
	h := loaded(t, nil)
	root := h.Root()
	ledger, _, err := h.NewProject(ctx, "ledger", "")
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan Change, 16)
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := h.Watch(wctx, func(c Change) { got <- c }); err != nil {
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

	writeFile(t, filepath.Join(root, "state.db"), "churn")
	writeFile(t, filepath.Join(root, "logs", "x.log"), "churn")
	writeFile(t, filepath.Join(ledger.Sandbox("mara"), "api", "x"), "churn")
	writeFile(t, filepath.Join(ledger.HarnessDir("mara"), "claude-code", "x"), "churn")
	writeFile(t, filepath.Join(ledger.Path, "spec.md"), "# Spec\n")
	gitOut(t, ledger.Path, "add", ".")
	gitOut(t, ledger.Path, "commit", "--quiet", "-m", "spec")
	// A folder under projects/ with no record is no project; files are no config.
	writeFile(t, filepath.Join(h.Folder(), "projects", "stray", "hq", "project.toml"), "")
	writeFile(t, filepath.Join(ledger.Files, "inbox", "brief.md"), "x")
	quiet()

	writeFile(t, filepath.Join(ledger.Path, "project.toml"), "checks = \"make\"")
	expect(Change{Kind: "project", ID: "ledger"})

	writeFile(t, filepath.Join(hqOf(root), "teams", "atlas.toml"), teamExample)
	expect(Change{Kind: "team", ID: "atlas"})

	writeFile(t, filepath.Join(root, "settings.toml"), "port = 9000")
	expect(Change{Kind: "settings"})

	writeFile(t, h.HQ("instructions.md"), "Be brief.\n")
	expect(Change{Kind: "instructions"})

	// A project made after the watch started is watched too, and a repo added
	// is told at once.
	relay, _, err := h.NewProject(ctx, "relay", "")
	if err != nil {
		t.Fatal(err)
	}
	expect(Change{Kind: "projects"})
	writeFile(t, filepath.Join(relay.Path, "project.toml"), "checks = \"go test\"")
	expect(Change{Kind: "project", ID: "relay"})
	if _, err := h.AddRepo(ctx, "relay", "api", codeRepo(t, "dev/api"), ""); err != nil {
		t.Fatal(err)
	}
	expect(Change{Kind: "projects"})
}

// CheckTeam refuses what SaveTeam would, writing nothing: the file's rules,
// a team id that isn't one folder name. Another team's agents are no concern.
func TestCheckTeam(t *testing.T) {
	h := loaded(t, func(root string) {
		writeFile(t, filepath.Join(hqOf(root), "teams", "atlas.toml"), teamExample)
	})
	c, _ := ParseTeam("atlas", []byte(teamExample))
	if p := h.CheckTeam(c); len(p) != 0 {
		t.Errorf("the team as it is: %v", p)
	}
	for _, id := range []string{"../escape", "Orion", "a/b", ""} {
		bad := c
		bad.Team = id
		if p := h.CheckTeam(bad); len(p) == 0 || !strings.HasPrefix(p[0], "team:") {
			t.Errorf("team %q: %v", id, p)
		}
	}
	orion := c
	orion.Team, orion.Name = "orion", "Orion"
	if p := h.CheckTeam(orion); len(p) != 0 {
		t.Errorf("atlas's agents in orion: %v", p)
	}
	if _, err := os.Stat(h.HQ("teams", "orion.toml")); err == nil {
		t.Error("checking wrote the team")
	}
}

// The subscriber is told after the home's lock is released, so it can
// read the home back (the instance manager reads max_sessions on a
// settings change): under the lock it waited on itself, and every request
// after it (found 2026-10-18: choosing a project's profile hung Foreman).
func TestTheSubscriberCanReadTheHome(t *testing.T) {
	var root string
	h := loaded(t, func(r string) { root = r })
	read := make(chan int, 16)
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := h.Watch(wctx, func(c Change) {
		if c.Kind == "settings" {
			read <- h.Settings().Settings.MaxSessions
		}
	}); err != nil {
		t.Fatal(err)
	}
	expect := func(want int, how string) {
		t.Helper()
		select {
		case got := <-read:
			if got != want {
				t.Errorf("%s: read %d, want %d", how, got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the subscriber never read the home", how)
		}
	}
	s := DefaultSettings()
	s.MaxSessions = 3
	saved := make(chan error, 1)
	go func() {
		_, err := h.SaveSettings(ctx, s, "")
		saved <- err
	}()
	select {
	case err := <-saved:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the save never returned")
	}
	expect(3, "a save in the UI")
	// A change by hand, through the watcher.
	writeFile(t, filepath.Join(root, "settings.toml"), "max_sessions = 5\n")
	expect(5, "an edit on disk")
	done := make(chan struct{})
	go func() { h.Teams(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the home stayed locked")
	}
}

// The CEO's instructions are instructions.md in Foreman's HQ: a save keeps
// them exactly, version-guarded, in one commit; settings.toml never holds
// them; a hand edit is read on reload.
func TestInstructions(t *testing.T) {
	h := loaded(t, nil)
	text := "# Rules\n- Say \"done\" only when tests pass.\n- Paths like C:\\tmp and `'''` stay.\n\"\"\"\nport = 1\n"
	got, err := h.SaveInstructions(ctx, text, h.Instructions().Version)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != text || got != h.Instructions() {
		t.Errorf("saved %+v, read %+v", got, h.Instructions())
	}
	if data, _ := os.ReadFile(h.HQ("instructions.md")); string(data) != text {
		t.Errorf("file %q", data)
	}
	if !strings.HasPrefix(gitLog(t, h.HQ()), "Instructions: saved in the UI\n\ninstructions.md") {
		t.Errorf("not committed:\n%s", gitLog(t, h.HQ()))
	}
	if exists(h.Path("settings.toml")) {
		t.Error("settings.toml was written")
	}
	if _, err := h.SaveInstructions(ctx, "stale", "0000"); !errors.Is(err, ErrConflict) {
		t.Errorf("a stale version: %v", err)
	}
	writeFile(t, h.HQ("instructions.md"), "By hand.\n")
	reload(t, h)
	if h.Instructions().Text != "By hand.\n" {
		t.Errorf("after a hand edit %+v", h.Instructions())
	}
}

// Settings are this machine's: settings.toml in the home, in no repo, the
// look of the UI included; a save rewrites them, and refuses a look Foreman
// doesn't have. foreman_folder is never the API's: not served, and a save
// keeps the file's whatever it is sent (the CEO, 2026-11-16).
func TestSettingsStayOnThisMachine(t *testing.T) {
	h := loaded(t, func(root string) {
		writeFile(t, filepath.Join(root, "settings.toml"), "foreman_folder = \"~/f\"\n")
	})
	s := h.Settings().Settings
	if data, _ := json.Marshal(h.Settings()); strings.Contains(string(data), "~/f") || strings.Contains(strings.ToLower(string(data)), "foremanfolder") {
		t.Errorf("foreman_folder is served: %s", data)
	}
	s.Theme, s.Accent, s.TerminalFont, s.TerminalFontSize = "dark", "teal", "fira-code", 15
	sent := s
	sent.ForemanFolder = "~/elsewhere"
	if _, err := h.SaveSettings(ctx, sent, h.Settings().Version); err != nil {
		t.Fatal(err)
	}
	reload(t, h)
	if got := h.Settings().Settings; got != s || got.ForemanFolder != "~/f" {
		t.Errorf("read back %+v", got)
	}
	if _, err := exec.Command("git", "-C", h.Root(), "rev-parse", "--git-dir").CombinedOutput(); err == nil {
		t.Error("the home is in a repo")
	}
	for name, edit := range map[string]func(*Settings){
		"theme":              func(s *Settings) { s.Theme = "sepia" },
		"accent":             func(s *Settings) { s.Accent = "red" },
		"terminal_font":      func(s *Settings) { s.TerminalFont = "comic" },
		"terminal_font_size": func(s *Settings) { s.TerminalFontSize = MaxFontSize + 1 },
	} {
		bad := s
		edit(&bad)
		var invalid *InvalidError
		if _, err := h.SaveSettings(ctx, bad, h.Settings().Version); !errors.As(err, &invalid) || !strings.HasPrefix(invalid.Problems[0], name+":") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
