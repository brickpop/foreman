package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture's ivo has no worktree: it opens repos as it needs them
// (wiki/product/architecture/sandboxes-and-git.md#opening-a-repo).

func (f *fixture) opened(agent, repo string) bool {
	_, err := os.Stat(filepath.Join(f.wt(agent, repo), ".git"))
	return err == nil
}

// Opened while idle, a repo sits on no branch at its own default branch's head; opening it again
// changes nothing; a name not of the project is refused.
func TestOpenARepo(t *testing.T) {
	f := newFixture(t)
	if got := f.ok("ivo", "open", map[string]string{"repo": "web"}); !strings.Contains(got, "idle on no branch") || !strings.Contains(got, "Switch to a topic before you commit") {
		t.Errorf("open: %q", got)
	}
	web := f.wt("ivo", "web")
	if f.git(web, "branch", "--show-current") != "" || f.git(web, "rev-parse", "HEAD") != f.git(web, "rev-parse", "trunk") {
		t.Error("not idle at web's default branch, trunk")
	}
	if f.opened("ivo", "api") {
		t.Error("api opened too")
	}
	if got := f.ok("ivo", "open", map[string]string{"repo": "web"}); !strings.Contains(got, "open already") {
		t.Errorf("again: %q", got)
	}
	f.refused("ivo", "open", map[string]string{"repo": "nope"}, "unknown_repo")
	f.refused("ivo", "open", map[string]string{"repo": "../api"}, "unknown_repo")
	if f.has("wt/ledger/ivo/main") {
		t.Error("a branch was made")
	}
}

// A repo opened while the agent works on a topic joins it: made from that repo's default when
// the repo lacks it.
func TestOpenJoinsTheTopic(t *testing.T) {
	f := newFixture(t)
	f.ok("ivo", "open", map[string]string{"repo": "api"})
	f.ok("ivo", "switch", map[string]string{"branch": "wt/ledger/ivo/LED-7"})
	got := f.ok("ivo", "open", map[string]string{"repo": "web"})
	web := f.wt("ivo", "web")
	if !strings.Contains(got, "on wt/ledger/ivo/LED-7") || f.git(web, "branch", "--show-current") != "wt/ledger/ivo/LED-7" {
		t.Errorf("open: %q, on %q", got, f.git(web, "branch", "--show-current"))
	}
	if f.git(web, "rev-parse", "HEAD") != f.git(web, "rev-parse", "trunk") {
		t.Error("not made from trunk")
	}
}

// A switch needs an open repo to make a branch in, and opens the repos that have the branch already.
func TestSwitchOpensWhatHasTheBranch(t *testing.T) {
	f := newFixture(t)
	f.refused("ivo", "switch", map[string]string{"branch": "wt/ledger/ivo/new"}, "no_repos")
	// Its earlier work, in web only: a switch back opens web alone.
	web := filepath.Join(f.root, "ledger", "web")
	f.git(web, "branch", "wt/ledger/ivo/old", "trunk")
	got := f.ok("ivo", "switch", map[string]string{"branch": "wt/ledger/ivo/old"})
	if !f.opened("ivo", "web") || f.opened("ivo", "api") || f.git(f.wt("ivo", "web"), "branch", "--show-current") != "wt/ledger/ivo/old" {
		t.Errorf("switch back: %q", got)
	}
	if !strings.Contains(got, "Opened web, which had it.") || !strings.Contains(got, "Not open: api: open(repo) any your work needs.") {
		t.Errorf("result: %q", got)
	}
}

// Nothing is committed on no branch: commit, merge and set_submodule ask for a topic first. A
// switch to a new topic names it where the agent was idle, its edits kept, and then it commits.
func TestIdleWorktreesCommitNothing(t *testing.T) {
	f := newFixture(t)
	f.ok("ivo", "open", map[string]string{"repo": "api"})
	api := f.wt("ivo", "api")
	writeFile(t, filepath.Join(api, "fix.go"), "package api\n")
	f.refused("ivo", "commit", commits("api", "fix"), "no_branch")
	f.refused("ivo", "merge", map[string]string{"branch": "main"}, "no_branch")
	f.refused("ivo", "set_submodule", map[string]string{"repo": "api", "path": "lib/x", "ref": "v1"}, "no_branch")
	f.refused("ivo", "commit", commits("web", "fix"), "not_open")

	if got := f.ok("ivo", "switch", map[string]string{"branch": "wt/ledger/ivo/fix"}); !strings.Contains(got, "changes kept") {
		t.Errorf("switch: %q", got)
	}
	if data, _ := os.ReadFile(filepath.Join(api, "fix.go")); string(data) != "package api\n" {
		t.Error("the edit was lost")
	}
	f.ok("ivo", "commit", commits("api", "fix"))
	if f.git(api, "log", "-1", "--format=%s", "wt/ledger/ivo/fix") != "fix" {
		t.Error("not committed on the topic")
	}
	// With from, a switch never carries edits along, idle or not.
	writeFile(t, filepath.Join(api, "more.go"), "package api\n")
	f.refused("ivo", "switch", map[string]string{"branch": "wt/ledger/ivo/other", "from": "main"}, "uncommitted_changes")
}

// A merge opens the repos that have the branch, on the agent's topic; it never runs while idle,
// even with nothing open.
func TestMergeOpensWhatHasTheBranch(t *testing.T) {
	f := newFixture(t)
	f.refused("ivo", "merge", map[string]string{"branch": "wt/ledger/tove/main"}, "no_branch")
	if f.opened("ivo", "api") || f.opened("ivo", "web") {
		t.Fatal("a refused merge opened a repo")
	}
	writeFile(t, filepath.Join(f.wt("tove", "web"), "page.html"), "<p>tove's</p>\n")
	f.ok("tove", "commit", commits("web", "page"))

	f.ok("ivo", "open", map[string]string{"repo": "api"})
	f.ok("ivo", "switch", map[string]string{"branch": "wt/ledger/ivo/LED-8"})
	got := f.ok("ivo", "merge", map[string]string{"branch": "wt/ledger/tove/main"})
	web := f.wt("ivo", "web")
	if !f.opened("ivo", "web") || f.git(web, "branch", "--show-current") != "wt/ledger/ivo/LED-8" {
		t.Fatalf("web not opened on the topic: %q", got)
	}
	if data, _ := os.ReadFile(filepath.Join(web, "page.html")); string(data) != "<p>tove's</p>\n" {
		t.Errorf("not merged: %q", got)
	}
}

// worktrees/ is what can go (wiki/backlog/foreman-folder.md): a worktree
// folder deleted by hand is opened again, on the branch the agent's other
// worktrees are on, git's record of the old one pruned so the branch is free;
// with all of them gone the agent opens idle, and its branch, untouched,
// takes it back.
func TestADeletedWorktreeIsOpenedAgain(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.wt("tove", "web"), "lost.txt"), "not committed\n")
	if err := os.RemoveAll(f.wt("tove", "web")); err != nil {
		t.Fatal(err)
	}
	if got := f.ok("tove", "open", map[string]string{"repo": "web"}); !strings.Contains(got, "on wt/ledger/tove/main") {
		t.Errorf("open: %q", got)
	}
	if b := f.git(f.wt("tove", "web"), "branch", "--show-current"); b != "wt/ledger/tove/main" {
		t.Errorf("web is on %q", b)
	}

	if err := os.RemoveAll(filepath.Dir(f.wt("tove", "api"))); err != nil {
		t.Fatal(err)
	}
	if got := f.ok("tove", "open", map[string]string{"repo": "api"}); !strings.Contains(got, "idle on no branch") {
		t.Errorf("open with no worktree left: %q", got)
	}
	f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/main"})
	if b := f.git(f.wt("tove", "api"), "branch", "--show-current"); b != "wt/ledger/tove/main" {
		t.Errorf("api is on %q after the switch", b)
	}
}
