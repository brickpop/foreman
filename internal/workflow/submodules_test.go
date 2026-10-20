package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dependency is a repo to use as a submodule: three commits on main, tagged
// v0, v1 and v2. It returns its file:// URL, v1 and v2.
func (f *fixture) dependency() (url, v1, v2 string) {
	f.t.Helper()
	dir := filepath.Join(f.t.TempDir(), "forge-std")
	f.git(filepath.Dir(dir), "init", "--quiet", "--initial-branch=main", dir)
	for _, v := range []string{"v0", "v1", "v2"} {
		writeFile(f.t, filepath.Join(dir, "Test.sol"), v+"\n")
		f.git(dir, "add", "Test.sol")
		f.git(dir, "-c", "user.name=Lib", "-c", "user.email=lib@x", "commit", "--quiet", "-m", v)
		f.git(dir, "tag", v)
	}
	return "file://" + dir, f.git(dir, "rev-parse", "v1"), f.git(dir, "rev-parse", "v2")
}

func (f *fixture) read(path string) string {
	f.t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func TestSetSubmoduleAddsThenTeammatesGetIt(t *testing.T) {
	f := newFixture(t)
	kept := &actions{}
	f.w.Actions = kept
	url, v1, v2 := f.dependency()
	text := f.ok("tove", "set_submodule", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "v1", "url": url})
	if !strings.HasPrefix(text, "Committed in api (wt/tove/main, ") || !strings.HasSuffix(text, "): Add submodule lib/forge-std at v1 ("+url+").") {
		t.Errorf("%q", text)
	}
	if got := f.git(f.wt("tove", "api"), "rev-parse", "HEAD:lib/forge-std"); got != v1 {
		t.Errorf("pinned %s, want v1", got)
	}
	if a := kept.kept[len(kept.kept)-1]; a.Tool != "set_submodule" || a.Branch != "wt/tove/main" || len(a.Repos) != 1 ||
		a.Repos[0].Repo != "api" || a.Repos[0].Commit == "" || !strings.HasPrefix(a.Repos[0].Message, "Add submodule lib/forge-std") {
		t.Errorf("action %+v", a)
	}
	// The lead merges it in: the submodule comes filled.
	f.ok("mara", "merge", map[string]string{"branch": "wt/tove/main"})
	if got := f.read(filepath.Join(f.wt("mara", "api"), "lib/forge-std/Test.sol")); got != "v1\n" {
		t.Errorf("the lead's lib/forge-std holds %q", got)
	}
	// Moved to a branch of its repo.
	f.ok("tove", "set_submodule", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "main"})
	if got := f.git(f.wt("tove", "api"), "rev-parse", "HEAD:lib/forge-std"); got != v2 {
		t.Errorf("pinned %s, want main's v2", got)
	}
}

// Files changed in a submodule can't be committed: whatever needs committed
// work says so, and set_submodule discards them.
func TestChangesInASubmoduleBlockWhatNeedsCommittedWork(t *testing.T) {
	f := newFixture(t)
	url, _, _ := f.dependency()
	f.ok("tove", "set_submodule", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "v1", "url": url})
	writeFile(t, filepath.Join(f.wt("tove", "api"), "lib/forge-std/Test.sol"), "patched\n")
	for tool, args := range map[string]map[string]string{
		"switch":  {"branch": "wt/tove/LED-44"},
		"merge":   {"branch": "main"},
		"handoff": {"teammate": "mara", "branch": "wt/tove/main", "summary": "done"},
	} {
		if text := f.refused("tove", tool, args, "submodule_changed"); !strings.Contains(text, "api/lib/forge-std") {
			t.Errorf("%s: %q", tool, text)
		}
	}
	f.ok("tove", "set_submodule", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "v1"})
	if got := f.read(filepath.Join(f.wt("tove", "api"), "lib/forge-std/Test.sol")); got != "v1\n" {
		t.Errorf("the edit stayed: %q", got)
	}
	f.ok("tove", "switch", map[string]string{"branch": "wt/tove/LED-44"})
}

// A branch without the submodule doesn't keep its folder, and the way back
// brings it again.
func TestSwitchingFollowsTheBranchsSubmodules(t *testing.T) {
	f := newFixture(t)
	url, _, _ := f.dependency()
	f.ok("tove", "set_submodule", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "v1", "url": url})
	f.ok("tove", "switch", map[string]string{"branch": "wt/tove/LED-44"}) // from main: no submodule
	if _, err := os.Stat(filepath.Join(f.wt("tove", "api"), "lib/forge-std")); !os.IsNotExist(err) {
		t.Errorf("lib/forge-std is still there: %v", err)
	}
	writeFile(t, filepath.Join(f.wt("tove", "api"), "cal.go"), "package cal\n")
	f.ok("tove", "commit", commits("api", "cal")) // and the leftover isn't committed back
	if files := f.git(f.wt("tove", "api"), "show", "--name-only", "--format=", "HEAD"); files != "cal.go" {
		t.Errorf("committed %q", files)
	}
	f.ok("tove", "switch", map[string]string{"branch": "wt/tove/main"})
	if got := f.read(filepath.Join(f.wt("tove", "api"), "lib/forge-std/Test.sol")); got != "v1\n" {
		t.Errorf("back home, lib/forge-std holds %q", got)
	}
}

func TestASubmoduleConflictIsSettledThenCommitted(t *testing.T) {
	f := newFixture(t)
	url, _, v2 := f.dependency()
	f.ok("mara", "set_submodule", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "v1", "url": url})
	f.ok("tove", "merge", map[string]string{"branch": "lead/atlas/integration"})
	// From v1, each side moves the pin its own way: neither descends from the other.
	f.ok("tove", "set_submodule", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "v2"})
	f.ok("mara", "set_submodule", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "v0"})
	text := f.ok("mara", "merge", map[string]string{"branch": "wt/tove/main"})
	if !strings.Contains(text, "Conflicts in api (lib/forge-std)") || !strings.Contains(text, "set_submodule") {
		t.Fatalf("%q", text)
	}
	if text := f.refused("mara", "commit", commits("api", "merged"), "submodule_conflict"); !strings.Contains(text, "api/lib/forge-std") {
		t.Errorf("%q", text)
	}
	if text := f.ok("mara", "set_submodule", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "v2"}); text != "lib/forge-std is on v2 in api: commit api to finish the merge." {
		t.Errorf("%q", text)
	}
	f.ok("mara", "commit", commits("api", "merged"))
	if got := f.git(f.wt("mara", "api"), "rev-parse", "HEAD:lib/forge-std"); got != v2 {
		t.Errorf("settled on %s, want v2", got)
	}
	if parents := strings.Fields(f.git(f.wt("mara", "api"), "log", "-1", "--format=%P")); len(parents) != 2 {
		t.Error("the merge wasn't concluded")
	}
}

func TestSetSubmoduleRefusalsInTheTool(t *testing.T) {
	f := newFixture(t)
	url, _, _ := f.dependency()
	f.ok("tove", "set_submodule", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "v1", "url": url})
	for _, c := range []struct {
		agent string
		args  map[string]string
		code  string
	}{
		{"mara", map[string]string{"repo": "project", "path": "lib/x", "ref": "v1", "url": url}, "unknown_repo"},
		{"tove", map[string]string{"repo": "api", "path": "", "ref": "v1"}, "empty"},
		{"tove", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": ""}, "empty"},
		{"tove", map[string]string{"repo": "api", "path": "../web", "ref": "v1"}, "bad_path"},
		{"tove", map[string]string{"repo": "api", "path": "README", "ref": "v1"}, "not_a_submodule"},
		{"tove", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "v1", "url": url}, "path_taken"},
		{"tove", map[string]string{"repo": "api", "path": "lib/other", "ref": "v1", "url": "git@github.com:foundry-rs/forge-std.git"}, "bad_url"},
		{"tove", map[string]string{"repo": "api", "path": "lib/forge-std", "ref": "v9"}, "unknown_ref"},
	} {
		f.refused(c.agent, "set_submodule", c.args, c.code)
	}
}

// An agent that clones a repo into its worktree is told how to go on.
func TestCommitRefusesANestedRepo(t *testing.T) {
	f := newFixture(t)
	nested := filepath.Join(f.wt("tove", "api"), "vendor", "x")
	writeFile(t, filepath.Join(nested, "a"), "a\n")
	f.git(nested, "init", "--quiet")
	if text := f.refused("tove", "commit", commits("api", "vendored"), "nested_repo"); !strings.Contains(text, "api/vendor/x") || !strings.Contains(text, "set_submodule") {
		t.Errorf("%q", text)
	}
}
