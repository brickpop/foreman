package workflow

import (
	"path/filepath"
	"strings"
	"testing"
)

// api gets an upstream with a commit main lacks; web has no remote at first.
func TestFetchThenMergeWhatCameUpstream(t *testing.T) {
	f := newFixture(t)
	api := filepath.Join(f.root, "ledger", "api")
	up := filepath.Join(f.root, "upstream-api")
	f.git(f.root, "clone", "--quiet", api, up)
	writeFile(t, filepath.Join(up, "pushed.go"), "package api\n")
	f.git(up, "add", "pushed.go")
	f.git(up, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "-m", "pushed upstream")
	f.git(api, "remote", "add", "origin", up)
	mainBefore := f.git(api, "rev-parse", "main")

	// Not fetched yet: there is no origin/main to merge.
	f.refused("mara", "merge", map[string]string{"branch": "origin/main"}, "branch_not_found")

	text := f.ok("mara", "fetch", nil)
	if !strings.Contains(text, "api: origin/main new") || !strings.Contains(text, "web: no remote") || !strings.Contains(text, `merge("origin/main")`) {
		t.Errorf("%q", text)
	}
	if f.git(api, "rev-parse", "main") != mainBefore {
		t.Error("the target moved")
	}
	if text := f.ok("mara", "fetch", nil); text != "api: up to date\nweb: no remote" {
		t.Errorf("again: %q", text)
	}

	// The remote's branch merges in like any other, where the repo has it.
	if text := f.ok("mara", "merge", map[string]string{"branch": "origin/main"}); text != "Merged origin/main into api." {
		t.Errorf("%q", text)
	}
	if f.git(f.wt("mara", "api"), "log", "-1", "--format=%s", "--", "pushed.go") != "pushed upstream" {
		t.Error("upstream's commit isn't on the lead branch")
	}

	// A remote that fails fails the call, and still the others are fetched.
	f.git(filepath.Join(f.root, "ledger", "web"), "remote", "add", "origin", filepath.Join(f.root, "nowhere"))
	f.git(up, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--allow-empty", "-m", "more")
	text = f.refused("mara", "fetch", nil, "fetch_failed")
	if !strings.Contains(text, "api: origin/main ") || !strings.Contains(text, "web: failed") {
		t.Errorf("%q", text)
	}
}
