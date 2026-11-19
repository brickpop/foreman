package home

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const root40 = "0123456789abcdef0123456789abcdef01234567"

func TestParseRecords(t *testing.T) {
	good := `
[ledger]
display_name = "The ledger"
hidden = true
hq = "git@github.com:acme/ledger-hq.git"

[ledger.repos.api]
display_name = "Ledger API"
remote = "git@github.com:acme/api.git"
root_commit = "` + root40 + `"
default_branch = "trunk"

[relay]
`
	records, problems := ParseRecords([]byte(good))
	want := map[string]Record{
		"ledger": {DisplayName: "The ledger", Hidden: true, HQ: "git@github.com:acme/ledger-hq.git", Repos: map[string]RepoRecord{
			"api": {DisplayName: "Ledger API", Remote: "git@github.com:acme/api.git", RootCommit: root40, DefaultBranch: "trunk"},
		}},
		"relay": {Repos: map[string]RepoRecord{}},
	}
	if len(problems) != 0 || !reflect.DeepEqual(records, want) {
		t.Errorf("%+v %v", records, problems)
	}
	if _, problems := ParseRecords(nil); problems == nil || len(problems) != 0 {
		t.Errorf("an empty file: %#v", problems)
	}

	repo := func(name, root, branch string) string {
		return "[ledger.repos." + name + "]\nroot_commit = \"" + root + "\"\ndefault_branch = \"" + branch + "\"\n"
	}
	for _, tc := range []struct{ name, file, says string }{
		{"not TOML", "[ledger", "expected"},
		{"a field Foreman doesn't know", "[ledger]\nname = \"x\"\n", "ledger.name: not a field"},
		{"an id that isn't a slug", "[Ledger]\n", "Ledger: not a project id"},
		{"an id that is a path", "[\"../x\"]\n", "../x: not a project id"},
		{"a long display name", "[ledger]\ndisplay_name = \"" + strings.Repeat("x", maxDisplayName+1) + "\"\n", "ledger.display_name"},
		{"a display name of two lines", "[ledger]\ndisplay_name = \"a\\nb\"\n", "ledger.display_name: A name is one line."},
		{"a repo's long display name", repo("api", root40, "main") + "display_name = \"" + strings.Repeat("x", maxDisplayName+1) + "\"\n", "ledger.repos.api.display_name: A name has at most 80 characters."},
		{"a repo name that isn't a folder", "[ledger.repos.\"a/b\"]\nroot_commit = \"" + root40 + "\"\ndefault_branch = \"main\"\n", "a repo name is"},
		{"the HQ's name", repo("HQ", root40, "main"), "the HQ's name"},
		{"two repos in different cases", repo("api", root40, "main") + repo("API", root40, "main"), "the same name as"},
		{"a short root commit", repo("api", "0123abc", "main"), "root_commit"},
		{"no root commit", "[ledger.repos.api]\ndefault_branch = \"main\"\n", "root_commit"},
		{"a root commit in capitals", repo("api", strings.ToUpper(root40), "main"), "root_commit"},
		{"no default branch", repo("api", root40, ""), "default_branch"},
		{"an agent's branch", repo("api", root40, "wt/ledger/mara/x"), "default_branch"},
	} {
		if _, problems := ParseRecords([]byte(tc.file)); !hasProblem(problems, tc.says) {
			t.Errorf("%s: %v, want one saying %q", tc.name, problems, tc.says)
		}
	}
	sha256 := strings.Repeat("ab", 32)
	if _, problems := ParseRecords([]byte(repo("api", sha256, "main"))); len(problems) != 0 {
		t.Errorf("a SHA-256 repo's root commit: %v", problems)
	}
}

// readRecords is projects.toml as written.
func readRecords(t *testing.T, h *Home) map[string]Record {
	t.Helper()
	data, err := os.ReadFile(h.HQ(projectsFile))
	if err != nil {
		t.Fatal(err)
	}
	records, problems := ParseRecords(data)
	if len(problems) != 0 {
		t.Fatalf("%v\n%s", problems, data)
	}
	return records
}

// What travels between machines is in projects.toml, committed to Foreman's
// HQ at each change, with no credentials; the database keeps only this
// machine's part.
func TestTheRecordsTravelInProjectsToml(t *testing.T) {
	h := loaded(t, nil)
	if _, _, err := h.NewProject(ctx, "ledger", "", ""); err != nil {
		t.Fatal(err)
	}
	api := codeRepo(t, "dev/api")
	gitOut(t, api, "remote", "add", "origin", "https://ghp_secret@github.com/acme/api.git")
	gitOut(t, api, "branch", "trunk")
	if _, err := h.AddRepo(ctx, "ledger", "api", api, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SetName(ctx, "ledger", "The ledger"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SetHidden(ctx, "ledger", true); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SetRepoDefault(ctx, "ledger", "api", "trunk"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.RenameRepo(ctx, "ledger", "api", "Ledger API"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SetProfile(ctx, "ledger", "work"); err != nil {
		t.Fatal(err)
	}
	// A project cloned from another's HQ records where it came from.
	ledger, _ := h.Project("ledger")
	if _, _, err := h.NewProject(ctx, "relay", "", ledger.Path); err != nil {
		t.Fatal(err)
	}

	root := gitOut(t, api, "rev-list", "--max-parents=0", "HEAD")
	want := map[string]Record{
		"ledger": {DisplayName: "The ledger", Hidden: true, Repos: map[string]RepoRecord{
			"ledger-api": {DisplayName: "Ledger API", Remote: "https://github.com/acme/api.git", RootCommit: root, DefaultBranch: "trunk"},
		}},
		"relay": {HQ: ledger.Path, Repos: map[string]RepoRecord{}},
	}
	if got := readRecords(t, h); !reflect.DeepEqual(got, want) {
		t.Errorf("projects.toml:\n%+v\nwant\n%+v", got, want)
	}
	if data, _ := os.ReadFile(h.HQ(projectsFile)); strings.Contains(string(data), "ghp_secret") || strings.Contains(string(data), "work") {
		t.Errorf("a credential or this machine's profile in projects.toml:\n%s", data)
	}
	if status := gitOut(t, h.HQ(), "status", "--porcelain"); status != "" {
		t.Errorf("not committed: %s", status)
	}
	log := gitOut(t, h.HQ(), "log", "--format=%s")
	for _, msg := range []string{"Projects: add ledger", "Projects: add api to ledger", "Projects: rename ledger's api to ledger-api", "Projects: add relay"} {
		if !strings.Contains(log, msg) {
			t.Errorf("no commit %q in\n%s", msg, log)
		}
	}
	machine, err := h.machine.List(ctx)
	if err != nil || len(machine) != 2 || machine[0].Profile != "work" || len(machine[0].Repos) != 1 || machine[0].Repos[0].Path != api || machine[0].Repos[0].Name != "ledger-api" {
		t.Errorf("this machine's part: %+v %v", machine, err)
	}

	if _, err := h.RemoveRepo(ctx, "ledger", "ledger-api"); err != nil {
		t.Fatal(err)
	}
	if err := h.RemoveProject(ctx, "relay", false, false); err != nil {
		t.Fatal(err)
	}
	if got := readRecords(t, h); len(got) != 1 || len(got["ledger"].Repos) != 0 {
		t.Errorf("after removing: %+v", got)
	}
	if machine, _ := h.machine.List(ctx); len(machine) != 1 || len(machine[0].Repos) != 0 {
		t.Errorf("this machine's part after removing: %+v", machine)
	}
}

// A project made on another machine (its record arrived in projects.toml)
// shows here with its repos not on this machine yet, until each is located.
func TestARecordFromAnotherMachine(t *testing.T) {
	h := loaded(t, nil)
	api := codeRepo(t, "dev/api")
	other := codeRepo(t, "dev/other")
	root := gitOut(t, api, "rev-list", "--max-parents=0", "HEAD")
	record := "[ledger]\ndisplay_name = \"The ledger\"\n\n[ledger.repos.api]\nroot_commit = \"" + root + "\"\ndefault_branch = \"main\"\n"
	writeFile(t, h.HQ(projectsFile), record)
	reload(t, h)
	p, err := h.Project("ledger")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "The ledger" || len(p.Repos) != 1 || p.Repos[0].Path != "" || p.Repos[0].Ready() || !hasProblem(p.Problems, "repos.api: not on this machine yet") {
		t.Errorf("%+v", p)
	}
	if _, err := h.SetRepoDefault(ctx, "ledger", "api", "main"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a default for a repo not here: %v", err)
	}
	if _, err := h.RenameRepo(ctx, "ledger", "api", "core"); !errors.Is(err, ErrInvalid) {
		t.Errorf("renaming a repo not here: %v", err)
	}
	if _, err := h.LocateRepo(ctx, "ledger", "api", other); !errors.Is(err, ErrInvalid) {
		t.Errorf("located at another repo: %v", err)
	}
	p, err = h.LocateRepo(ctx, "ledger", "api", api)
	if err != nil || p.Repos[0].Path != api || !p.Repos[0].Ready() {
		t.Errorf("located: %+v %v", p, err)
	}
	if data, _ := os.ReadFile(h.HQ(projectsFile)); string(data) != record {
		t.Errorf("locating wrote projects.toml (where a repo is belongs to this machine):\n%s", data)
	}
}

// A projects.toml the CEO broke by hand keeps the projects as last read, and
// nothing writes over it until it is fixed.
func TestABrokenProjectsToml(t *testing.T) {
	h := loaded(t, nil)
	if _, _, err := h.NewProject(ctx, "ledger", "", ""); err != nil {
		t.Fatal(err)
	}
	if got := h.RecordsProblems(); got == nil || len(got) != 0 {
		t.Errorf("no problems: %#v", got)
	}
	writeFile(t, h.HQ(projectsFile), "[ledger]\nnmae = \"typo\"\n")
	reload(t, h)
	if !hasProblem(h.RecordsProblems(), "ledger.nmae") {
		t.Errorf("problems %v", h.RecordsProblems())
	}
	if _, err := h.Project("ledger"); err != nil {
		t.Errorf("the project went with the file's problem: %v", err)
	}
	if _, err := h.SetName(ctx, "ledger", "The ledger"); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "fix it first") {
		t.Errorf("a change over a broken file: %v", err)
	}
	if _, _, err := h.NewProject(ctx, "relay", "", ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("a new project over a broken file: %v", err)
	}
	if exists(filepath.Join(h.Folder(), "projects", "relay")) {
		t.Error("a refused new project left its folder")
	}
	if data, _ := os.ReadFile(h.HQ(projectsFile)); !strings.Contains(string(data), "nmae") {
		t.Error("the CEO's file was written over")
	}
	writeFile(t, h.HQ(projectsFile), "[ledger]\ndisplay_name = \"Fixed\"\n")
	reload(t, h)
	if p, _ := h.Project("ledger"); len(h.RecordsProblems()) != 0 || p.Name != "Fixed" {
		t.Errorf("fixed: %v %+v", h.RecordsProblems(), p)
	}
	// A record taken out by hand takes the project out.
	writeFile(t, h.HQ(projectsFile), "")
	reload(t, h)
	if len(h.Projects()) != 0 {
		t.Errorf("projects %+v", h.Projects())
	}
}
