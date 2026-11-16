package git

import (
	"errors"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// local fetches from the tests' own repos, which are local paths.
func local() *CLI {
	g := New()
	g.FetchProtocols = []string{"file"}
	return g
}

// lib is a dependency's repo: main at "v2", tagged v1 one commit earlier.
func lib(t *testing.T) (dir, v1, v2 string) {
	t.Helper()
	dir = codeRepo(t, "main")
	write(t, filepath.Join(dir, "lib.sol"), "v1\n")
	if err := New().Commit(ctx, dir, "v1", "lib.sol"); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "tag", "v1")
	v1 = gitOut(t, dir, "rev-parse", "HEAD")
	write(t, filepath.Join(dir, "lib.sol"), "v2\n")
	if err := New().Commit(ctx, dir, "v2", "lib.sol"); err != nil {
		t.Fatal(err)
	}
	return dir, v1, gitOut(t, dir, "rev-parse", "HEAD")
}

// withSubmodule is a code repo whose main pins lib at v1 under lib/dep, added
// by the CEO's own git, and a worktree of it per agent.
func withSubmodule(t *testing.T, agents ...string) (r, libDir, v1, v2 string, wts map[string]string) {
	t.Helper()
	libDir, v1, v2 = lib(t)
	r, wts = worktrees(t)
	gitOut(t, r, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", "file://"+libDir, "lib/dep")
	gitOut(t, filepath.Join(r, "lib/dep"), "checkout", "--quiet", v1)
	gitOut(t, r, "add", "lib/dep")
	gitOut(t, r, "-c", "user.name=CEO", "-c", "user.email=ceo@x", "commit", "--quiet", "-m", "lib at v1")
	for _, a := range agents {
		wts[a] = filepath.Join(t.TempDir(), a, "api")
		if err := New().Prepare(ctx, r, wts[a], "wt/"+a+"/main", "main"); err != nil {
			t.Fatal(err)
		}
	}
	return r, libDir, v1, v2, wts
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSyncChecksOutThePinsWithoutTouchingTheReposConfig(t *testing.T) {
	r, _, _, _, wts := withSubmodule(t, "tove")
	wt := wts["tove"]
	if !emptyOrMissing(filepath.Join(wt, "lib/dep")) {
		t.Fatal("a new worktree came with its submodule: the test proves nothing")
	}
	config := read(t, filepath.Join(r, ".git/config"))
	if err := local().SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(wt, "lib/dep/lib.sol")); got != "v1\n" {
		t.Errorf("lib/dep at %q, want the pinned v1", got)
	}
	// Its git data is the worktree's own, under the repo's .git, mounted read-only in the container.
	if target, _ := gitFile(filepath.Join(wt, "lib/dep")); !strings.Contains(target, filepath.Join(".git", "worktrees")) {
		t.Errorf("lib/dep's git dir is %s", target)
	}
	if read(t, filepath.Join(r, ".git/config")) != config {
		t.Error("syncing a worktree wrote the CEO's .git/config")
	}
	if clean, err := New().Clean(ctx, wt); err != nil || !clean {
		t.Errorf("clean after a sync: %v, %v", clean, err)
	}
	// Again: nothing to do, nothing fetched.
	if err := New().SyncSubmodules(ctx, wt, false); err != nil {
		t.Errorf("a second sync, over https only, needed a fetch: %v", err)
	}
}

func TestSyncFetchesOnlyOverHTTPS(t *testing.T) {
	r, _, _, _, wts := withSubmodule(t, "tove")
	// The CEO never checked it out, so nothing to fall back on.
	if err := os.RemoveAll(filepath.Join(r, ".git/modules/lib/dep")); err != nil {
		t.Fatal(err)
	}
	err := New().SyncSubmodules(ctx, wts["tove"], false)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("a file:// submodule fetched with the default protocols: %v", err)
	}
	if !emptyOrMissing(filepath.Join(wts["tove"], "lib/dep")) {
		t.Error("lib/dep was filled anyway")
	}
}

func TestSyncIgnoresWhatGitmodulesAsksForUpdates(t *testing.T) {
	r, _, _, _, _ := withSubmodule(t)
	gitOut(t, r, "config", "--file", ".gitmodules", "submodule.lib/dep.update", "none")
	gitOut(t, r, "-c", "user.name=CEO", "-c", "user.email=ceo@x", "commit", "--quiet", "-am", "update none")
	wt := filepath.Join(t.TempDir(), "api")
	if err := New().Prepare(ctx, r, wt, "wt/x/main", "main"); err != nil {
		t.Fatal(err)
	}
	if err := local().SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(wt, "lib/dep/lib.sol")); got != "v1\n" {
		t.Errorf("update = none in .gitmodules was obeyed: lib/dep at %q", got)
	}
}

// A branch without the submodule leaves its folder behind, which the next
// commit would add back: the sync removes it, and brings it back on return.
func TestSyncRemovesASubmoduleTheBranchDoesntHave(t *testing.T) {
	r, _, _, _, wts := withSubmodule(t, "tove")
	gitOut(t, r, "branch", "bare", "main~1") // before the submodule
	wt, g := wts["tove"], local()
	if err := g.SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Switch(ctx, wt, "bare", "main"); err != nil {
		t.Fatal(err)
	}
	if nested, _ := nestedRepos(ctx, wt); !slices.Equal(nested, []string{"lib/dep"}) {
		t.Fatalf("after a plain switch, nested repos %v: git no longer leaves it behind?", nested)
	}
	if err := g.SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wt, "lib/dep")); !os.IsNotExist(err) {
		t.Errorf("lib/dep is still there: %v", err)
	}
	if clean, _ := g.Clean(ctx, wt); !clean {
		t.Error("not clean after the sync")
	}
	if _, err := g.Switch(ctx, wt, "wt/tove/main", "main"); err != nil {
		t.Fatal(err)
	}
	if err := New().SyncSubmodules(ctx, wt, false); err != nil { // no fetch: its git dir was kept
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(wt, "lib/dep/lib.sol")); got != "v1\n" {
		t.Errorf("lib/dep back at %q", got)
	}
}

func TestALeftoverWithChangesStaysAndCantBeCommitted(t *testing.T) {
	r, _, _, _, wts := withSubmodule(t, "tove")
	gitOut(t, r, "branch", "bare", "main~1")
	wt, g := wts["tove"], local()
	if err := g.SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Switch(ctx, wt, "bare", "main"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wt, "lib/dep/lib.sol"), "patched\n")
	if err := g.SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(wt, "lib/dep/lib.sol")); got != "patched\n" {
		t.Fatal("a leftover with changes was removed")
	}
	_, err := g.CommitAll(ctx, wt, "everything", tove)
	var nested *NestedReposError
	if !errors.As(err, &nested) || !slices.Equal(nested.Paths, []string{"lib/dep"}) {
		t.Errorf("committing a leftover: %v", err)
	}
}

// An agent that clones a repo into its worktree can't commit it as a pointer.
func TestCommitRefusesARepoInsideTheWorktree(t *testing.T) {
	_, wts := worktrees(t, "tove")
	wt := wts["tove"]
	write(t, filepath.Join(wt, "vendor/x/a"), "a\n")
	if err := New().Init(ctx, filepath.Join(wt, "vendor/x"), "main"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wt, "vendor/x/a"), "a\n")
	write(t, filepath.Join(wt, "app.go"), "package app\n")
	_, err := New().CommitAll(ctx, wt, "app", tove)
	var nested *NestedReposError
	if !errors.As(err, &nested) || !slices.Equal(nested.Paths, []string{"vendor/x"}) {
		t.Fatalf("got %v", err)
	}
	if head := gitOut(t, wt, "log", "-1", "--format=%s"); head == "app" {
		t.Error("committed anyway")
	}
}

func TestDirtySubmodulesCountWhatever(t *testing.T) {
	r, _, _, _, wts := withSubmodule(t, "tove")
	wt, g := wts["tove"], local()
	if err := g.SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	if dirty, err := g.DirtySubmodules(ctx, wt); err != nil || len(dirty) != 0 {
		t.Fatalf("dirty right after a sync: %v, %v", dirty, err)
	}
	write(t, filepath.Join(wt, "lib/dep/lib.sol"), "patched\n")
	write(t, filepath.Join(wt, "lib/dep/new file.sol"), "x\n")
	// .gitmodules can tell status to look away: Foreman doesn't.
	gitOut(t, wt, "config", "--file", ".gitmodules", "submodule.lib/dep.ignore", "all")
	dirty, err := g.DirtySubmodules(ctx, wt)
	if err != nil || !slices.Equal(dirty, []string{"lib/dep"}) {
		t.Errorf("dirty %v, %v", dirty, err)
	}
	if clean, _ := g.Clean(ctx, wt); clean {
		t.Error("clean with a changed submodule")
	}
	_ = r
}

func TestSetSubmoduleMovesThePinAndDiscardsEdits(t *testing.T) {
	_, _, v1, v2, wts := withSubmodule(t, "tove")
	wt, g := wts["tove"], local()
	if err := g.SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wt, "lib/dep/lib.sol"), "patched\n")
	sha, err := g.SetSubmodule(ctx, wt, "lib/dep", "main", "", "Move lib/dep to main", tove)
	if err != nil {
		t.Fatal(err)
	}
	if got := gitOut(t, wt, "rev-parse", "HEAD:lib/dep"); got != v2 {
		t.Errorf("pinned %s, want main's %s", got, v2)
	}
	if got := gitOut(t, wt, "log", "-1", "--format=%H|%an|%s"); got != sha+"|Tove|Move lib/dep to main" {
		t.Errorf("HEAD %s", got)
	}
	if got := read(t, filepath.Join(wt, "lib/dep/lib.sol")); got != "v2\n" {
		t.Errorf("lib/dep holds %q", got)
	}
	// A tag, and a bare commit id.
	for _, ref := range []string{"v1", v2[:12]} {
		if _, err := g.SetSubmodule(ctx, wt, "lib/dep", ref, "", "to "+ref, tove); err != nil {
			t.Errorf("%s: %v", ref, err)
		}
	}
	if got := gitOut(t, wt, "rev-parse", "HEAD~1:lib/dep"); got != v1 {
		t.Errorf("by tag: %s, want %s", got, v1)
	}
	if clean, _ := g.Clean(ctx, wt); !clean {
		t.Error("not clean after set_submodule")
	}
}

func TestSetSubmoduleRefusals(t *testing.T) {
	_, libDir, _, _, wts := withSubmodule(t, "tove")
	wt, g := wts["tove"], local()
	if err := g.SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	head := gitOut(t, wt, "rev-parse", "HEAD")
	for _, c := range []struct {
		name, path, ref, url string
		want                 error
	}{
		{"outside the worktree", "../x", "v1", "", ErrBadPath},
		{"absolute", "/tmp/x", "v1", "", ErrBadPath},
		{"into .git", "lib/.git/x", "v1", "", ErrBadPath},
		{"an option as ref", "lib/dep", "--upload-pack=touch /tmp/p", "", ErrUnknownRef},
		{"no such ref", "lib/dep", "v9", "", ErrUnknownRef},
		{"not a submodule", "README", "v1", "", ErrNotSubmodule},
		{"adding over a submodule", "lib/dep", "v1", "file://" + libDir, ErrSubmoduleExists},
		{"adding over files", "README", "v1", "file://" + libDir, ErrSubmoduleExists},
		{"scp-like ssh", "lib/other", "v1", "git@github.com:x/y.git", ErrBadURL},
		{"a local path", "lib/other", "v1", libDir, ErrBadURL},
	} {
		if _, err := g.SetSubmodule(ctx, wt, c.path, c.ref, c.url, "x", tove); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	if _, err := New().SetSubmodule(ctx, wt, "lib/other", "v1", "file://"+libDir, "x", tove); !errors.Is(err, ErrBadURL) {
		t.Errorf("file:// with the default protocols: %v", err)
	}
	if got := gitOut(t, wt, "rev-parse", "HEAD"); got != head {
		t.Error("a refusal committed")
	}
	if clean, _ := g.Clean(ctx, wt); !clean {
		t.Error("a refusal left changes")
	}
}

func TestSetSubmoduleAddsOnTheBranchOnly(t *testing.T) {
	r, libDir, v1, _, wts := withSubmodule(t, "tove", "ilya")
	g := local()
	config := read(t, filepath.Join(r, ".git/config"))
	if _, err := g.SetSubmodule(ctx, wts["tove"], "lib/second", "v1", "file://"+libDir, "Add lib/second", tove); err != nil {
		t.Fatal(err)
	}
	wt := wts["tove"]
	if got := gitOut(t, wt, "rev-parse", "HEAD:lib/second"); got != v1 {
		t.Errorf("pinned %s", got)
	}
	if got := gitOut(t, wt, "config", "--file", ".gitmodules", "submodule.lib/second.url"); got != "file://"+libDir {
		t.Errorf(".gitmodules url %q", got)
	}
	if files := gitOut(t, wt, "show", "--name-only", "--format=", "HEAD"); files != ".gitmodules\nlib/second" {
		t.Errorf("the commit holds %q", files)
	}
	if read(t, filepath.Join(r, ".git/config")) != config {
		t.Error("adding a submodule wrote the CEO's .git/config")
	}
	// A teammate who merges the branch gets it filled by the sync.
	if _, err := g.Merge(ctx, wts["ilya"], "wt/tove/main", tove); err != nil {
		t.Fatal(err)
	}
	if err := g.SyncSubmodules(ctx, wts["ilya"], false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(wts["ilya"], "lib/second/lib.sol")); got != "v1\n" {
		t.Errorf("ilya's lib/second holds %q", got)
	}
	// Re-added after a removal: the old git dir doesn't get in the way.
	gitOut(t, wt, "rm", "--quiet", "-r", "lib/second")
	gitOut(t, wt, "-c", "user.name=T", "-c", "user.email=t@x", "commit", "--quiet", "-am", "drop")
	if _, err := g.SetSubmodule(ctx, wt, "lib/second", "main", "file://"+libDir, "Again", tove); err != nil {
		t.Errorf("re-adding: %v", err)
	}
}

// Two branches moved the same pin: no markers, a commit to pick.
func TestASubmoduleConflictIsSettledBySetSubmodule(t *testing.T) {
	_, _, v1, v2, wts := withSubmodule(t, "tove", "ilya")
	g := local()
	for _, a := range []string{"tove", "ilya"} {
		if err := g.SyncSubmodules(ctx, wts[a], false); err != nil {
			t.Fatal(err)
		}
	}
	// ilya moves to v2; tove to a commit of its own on the lib's history.
	if _, err := g.SetSubmodule(ctx, wts["ilya"], "lib/dep", "main", "", "to v2", tove); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SetSubmodule(ctx, wts["tove"], "lib/dep", v1+"~1", "", "back", tove); err != nil {
		t.Fatal(err)
	}
	wt := wts["tove"]
	conflicts, err := g.Merge(ctx, wt, "wt/ilya/main", tove)
	if err != nil || !slices.Equal(conflicts, []string{"lib/dep"}) {
		t.Fatalf("conflicts %v, %v", conflicts, err)
	}
	_, err = g.CommitAll(ctx, wt, "merged", tove)
	var sub *SubmoduleConflictError
	if !errors.As(err, &sub) || !slices.Equal(sub.Paths, []string{"lib/dep"}) {
		t.Fatalf("committing an unsettled submodule: %v", err)
	}
	sha, err := g.SetSubmodule(ctx, wt, "lib/dep", "main", "", "x", tove)
	if err != nil || sha != "" {
		t.Fatalf("settling: %q, %v (during a merge it only stages)", sha, err)
	}
	if _, err := g.CommitAll(ctx, wt, "merged", tove); err != nil {
		t.Fatal(err)
	}
	if got := gitOut(t, wt, "rev-parse", "HEAD:lib/dep"); got != v2 {
		t.Errorf("settled on %s, want %s", got, v2)
	}
	if parents := strings.Fields(gitOut(t, wt, "log", "-1", "--format=%P")); len(parents) != 2 {
		t.Error("the merge wasn't concluded")
	}
}

func TestRecursiveIsOptional(t *testing.T) {
	inner, _, _ := lib(t)
	outer, _, _ := lib(t)
	gitOut(t, outer, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", "file://"+inner, "lib/inner")
	gitOut(t, outer, "-c", "user.name=CEO", "-c", "user.email=ceo@x", "commit", "--quiet", "-m", "inner")
	r, wts := worktrees(t)
	gitOut(t, r, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", "file://"+outer, "lib/outer")
	gitOut(t, r, "-c", "user.name=CEO", "-c", "user.email=ceo@x", "commit", "--quiet", "-m", "outer")
	for _, recursive := range []bool{false, true} {
		wt := filepath.Join(t.TempDir(), "api")
		branch := "wt/a/main"
		if recursive {
			branch = "wt/b/main"
		}
		if err := New().Prepare(ctx, r, wt, branch, "main"); err != nil {
			t.Fatal(err)
		}
		if err := local().SyncSubmodules(ctx, wt, recursive); err != nil {
			t.Fatal(err)
		}
		_, err := os.Stat(filepath.Join(wt, "lib/outer/lib/inner/lib.sol"))
		if filled := err == nil; filled != recursive {
			t.Errorf("recursive %v: the nested submodule filled: %v", recursive, filled)
		}
	}
	_ = wts
}

func TestAnSSHURLIsTriedOverHTTPS(t *testing.T) {
	for url, want := range map[string]string{
		"git@github.com:aragon/platform-doc.git": "https://github.com/aragon/platform-doc.git",
		"github.com:/aragon/x.git":               "https://github.com/aragon/x.git",
		"ssh://git@github.com/aragon/x.git":      "https://github.com/aragon/x.git",
		"ssh://git@gitea.local:2222/team/x.git":  "https://gitea.local/team/x.git",
		"https://github.com/aragon/x.git":        "https://github.com/aragon/x.git",
		"../sibling.git":                         "../sibling.git",
		"file:///Users/ceo/lib":                  "file:///Users/ceo/lib",
		"/Users/ceo/lib":                         "/Users/ceo/lib",
	} {
		if got := httpsOf(url); got != want {
			t.Errorf("%s: %s, want %s", url, got, want)
		}
	}
}

// pinnedOverSSH is a code repo whose main pins lib at v1 under lib/dep with an
// SSH URL no https counterpart answers (a private repo, to Foreman), and a
// worktree of it.
func pinnedOverSSH(t *testing.T) (r, wt, v1 string) {
	t.Helper()
	r, _, v1, _, wts := withSubmodule(t, "tove")
	gitOut(t, r, "config", "--file", ".gitmodules", "submodule.lib/dep.url", "git@127.0.0.1:aragon/lib.git")
	gitOut(t, r, "add", ".gitmodules")
	gitOut(t, r, "-c", "user.name=CEO", "-c", "user.email=ceo@x", "commit", "--quiet", "-m", "lib over ssh")
	gitOut(t, wts["tove"], "merge", "--quiet", "--ff-only", "main")
	return r, wts["tove"], v1
}

// A private submodule comes from the CEO's own checkout when https can't
// reach it: what the CEO fetched, with no credentials lent.
func TestAPrivateSubmoduleComesFromTheCEOsCheckout(t *testing.T) {
	_, wt, v1 := pinnedOverSSH(t)
	if err := New().SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(wt, "lib/dep/lib.sol")); got != "v1\n" {
		t.Errorf("lib/dep at %q, want the pinned v1", got)
	}
	if head := gitOut(t, filepath.Join(wt, "lib/dep"), "rev-parse", "HEAD"); head != v1 {
		t.Errorf("lib/dep on %s", head)
	}
}

// The fallback reads the CEO's copy of the submodule, never the URL: an agent
// pointing .gitmodules at another repo on the machine gets nothing from it.
func TestTheFallbackNeverFetchesTheAgentsURL(t *testing.T) {
	_, wt, v1 := pinnedOverSSH(t)
	secret := codeRepo(t, "main")
	write(t, filepath.Join(secret, "secret.txt"), "the CEO's\n")
	if err := New().Commit(ctx, secret, "secret", "secret.txt"); err != nil {
		t.Fatal(err)
	}
	gitOut(t, wt, "config", "--file", ".gitmodules", "submodule.lib/dep.url", "file://"+secret)
	if err := New().SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	if head := gitOut(t, filepath.Join(wt, "lib/dep"), "rev-parse", "HEAD"); head != v1 {
		t.Errorf("lib/dep on %s, want the pinned v1 from the CEO's copy", head)
	}
	if out := gitOut(t, filepath.Join(wt, "lib/dep"), "log", "--all", "--format=%s"); strings.Contains(out, "secret") {
		t.Errorf("the agent's URL was fetched: %q", out)
	}
}

// The fallback allows local paths for one clone, never recursively: a nested
// .gitmodules naming a repo on the machine gets nothing from it.
func TestTheFallbackDoesntFetchNestedLocalURLs(t *testing.T) {
	secret, _, _ := lib(t)
	outer, _, _ := lib(t)
	gitOut(t, outer, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", "file://"+secret, "lib/inner")
	gitOut(t, outer, "-c", "user.name=CEO", "-c", "user.email=ceo@x", "commit", "--quiet", "-m", "inner")
	r, _ := worktrees(t)
	gitOut(t, r, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", "file://"+outer, "lib/outer")
	gitOut(t, r, "config", "--file", ".gitmodules", "submodule.lib/outer.url", "git@127.0.0.1:aragon/outer.git")
	gitOut(t, r, "add", ".gitmodules", "lib/outer")
	gitOut(t, r, "-c", "user.name=CEO", "-c", "user.email=ceo@x", "commit", "--quiet", "-m", "outer over ssh")
	wt := filepath.Join(t.TempDir(), "tove", "api")
	if err := New().Prepare(ctx, r, wt, "wt/tove/main", "main"); err != nil {
		t.Fatal(err)
	}
	err := New().SyncSubmodules(ctx, wt, true)
	if err == nil || !strings.Contains(err.Error(), "lib/inner") {
		t.Errorf("the nested local URL wasn't refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, "lib/outer/lib.sol")); err != nil {
		t.Errorf("lib/outer didn't come from the CEO's checkout: %v", err)
	}
	if !emptyOrMissing(filepath.Join(wt, "lib/outer/lib/inner")) {
		t.Error("the nested submodule came from a local path")
	}
}

// Not in the CEO's checkout either: the error says how to make it come.
func TestAPrivateSubmoduleTheCEOHasntCheckedOut(t *testing.T) {
	r, wt, _ := pinnedOverSSH(t)
	if err := os.RemoveAll(filepath.Join(r, ".git/modules/lib/dep")); err != nil {
		t.Fatal(err)
	}
	err := New().SyncSubmodules(ctx, wt, false)
	if err == nil || !strings.Contains(err.Error(), "git submodule update --init lib/dep") {
		t.Errorf("%v", err)
	}
}

// moved puts a worktree one folder deeper, as moving a project by hand does,
// and tells its repo (git worktree repair, as Prepare does).
func moved(t *testing.T, r, wt string) string {
	t.Helper()
	to := filepath.Join(t.TempDir(), "worktrees", "ledger", "tove", "api")
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(wt, to); err != nil {
		t.Fatal(err)
	}
	gitOut(t, r, "worktree", "repair", to)
	return to
}

// A worktree moved with its submodules checked out: git keeps their links
// relative, and `worktree repair` leaves them, so every git command in it
// failed (found 2026-10-30, after a project moved into its workspace). The
// next start relinks them.
func TestPrepareRelinksTheSubmodulesOfAMovedWorktree(t *testing.T) {
	r, _, v1, _, wts := withSubmodule(t, "tove")
	if err := local().SyncSubmodules(ctx, wts["tove"], false); err != nil {
		t.Fatal(err)
	}
	wt := moved(t, r, wts["tove"])
	if _, err := New().DirtySubmodules(ctx, wt); err == nil {
		t.Fatal("the moved worktree's submodule still works: the test proves nothing")
	}
	if err := New().Prepare(ctx, r, wt, "wt/tove/main", "main"); err != nil {
		t.Fatal(err)
	}
	if dirty, err := New().DirtySubmodules(ctx, wt); err != nil || len(dirty) > 0 {
		t.Errorf("after Prepare: %v %v", dirty, err)
	}
	dep := filepath.Join(wt, "lib/dep")
	real, _ := filepath.EvalSymlinks(dep)
	if top := gitOut(t, dep, "rev-parse", "--show-toplevel"); top != real {
		t.Errorf("the submodule's worktree is %s, want %s (its core.worktree)", top, real)
	}
	if head := gitOut(t, dep, "rev-parse", "HEAD"); head != v1 {
		t.Errorf("the submodule lost its commit: %s", head)
	}
	// Relative still, as git writes them: a later move breaks them the same
	// way, and the start after it mends them again.
	if link := read(t, filepath.Join(dep, ".git")); !strings.HasPrefix(link, "gitdir: ../") {
		t.Errorf("link %q", link)
	}
	if err := local().SyncSubmodules(ctx, wt, false); err != nil {
		t.Errorf("sync after the relink: %v", err)
	}
}

// The relink runs on the host over a .gitmodules the agent can edit: a path
// out of the worktree, through a symlink, or a name out of the git dir
// rewrites nothing.
func TestTheRelinkStaysInTheWorktree(t *testing.T) {
	r, _, _, _, wts := withSubmodule(t, "tove")
	wt := wts["tove"]
	if err := local().SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	planted := "gitdir: the CEO's own\n"
	write(t, filepath.Join(outside, "x", ".git"), planted)
	if err := os.Symlink(filepath.Join(outside, "x"), filepath.Join(wt, "link")); err != nil {
		t.Fatal(err)
	}
	up, _ := filepath.Rel(wt, filepath.Join(outside, "x"))
	// Each borrows lib/dep's name, whose git data exists: only the checks on
	// the path stop the rewrite.
	modules := filepath.Join(wt, ".gitmodules")
	write(t, modules, read(t, modules)+
		"[submodule \"lib/dep\"]\n\tpath = "+up+"\n"+
		"[submodule \"lib/dep\"]\n\tpath = link\n"+
		"[submodule \"../../../dep\"]\n\tpath = lib/dep\n\turl = https://x\n")
	if err := New().Prepare(ctx, r, wt, "wt/tove/main", "main"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(outside, "x", ".git")); got != planted {
		t.Errorf("a .git outside the worktree was rewritten: %q", got)
	}
	if link := read(t, filepath.Join(wt, "lib/dep/.git")); !strings.Contains(link, "/modules/lib/dep") {
		t.Errorf("lib/dep relinked by another name: %q", link)
	}
}

// Nested submodules (recursive_submodules) are relinked too.
func TestPrepareRelinksNestedSubmodules(t *testing.T) {
	inner, _, _ := lib(t)
	outer, _, _ := lib(t)
	gitOut(t, outer, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", "file://"+inner, "lib/inner")
	gitOut(t, outer, "-c", "user.name=CEO", "-c", "user.email=ceo@x", "commit", "--quiet", "-m", "inner")
	r, _ := worktrees(t)
	gitOut(t, r, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", "file://"+outer, "lib/outer")
	gitOut(t, r, "-c", "user.name=CEO", "-c", "user.email=ceo@x", "commit", "--quiet", "-m", "outer")
	wt := filepath.Join(t.TempDir(), "tove", "api")
	if err := New().Prepare(ctx, r, wt, "wt/tove/main", "main"); err != nil {
		t.Fatal(err)
	}
	if err := local().SyncSubmodules(ctx, wt, true); err != nil {
		t.Fatal(err)
	}
	wt = moved(t, r, wt)
	if err := New().Prepare(ctx, r, wt, "wt/tove/main", "main"); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(wt, "lib/outer/lib/inner")
	real, _ := filepath.EvalSymlinks(nested)
	if top := gitOut(t, nested, "rev-parse", "--show-toplevel"); top != real {
		t.Errorf("the nested submodule's worktree is %s, want %s", top, real)
	}
	if err := local().SyncSubmodules(ctx, wt, true); err != nil {
		t.Errorf("sync after the relink: %v", err)
	}
}

// The CEO's credentials never reach a fetch: not the global config's helper,
// not the repo's, not ~/.netrc. A public repo still comes.
func TestFetchesGoWithoutTheCEOsCredentials(t *testing.T) {
	libDir, v1, _ := lib(t)
	gitOut(t, libDir, "config", "http.receivepack", "false")
	exec := strings.TrimSpace(gitOut(t, ".", "--exec-path"))
	var withAuth atomic.Int32
	var private atomic.Bool // read by the server's goroutines
	private.Store(true)
	backend := &cgi.Handler{Path: filepath.Join(exec, "git-http-backend"), Env: []string{"GIT_PROJECT_ROOT=" + filepath.Dir(libDir), "GIT_HTTP_EXPORT_ALL=1"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			withAuth.Add(1)
		}
		if private.Load() {
			w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		backend.ServeHTTP(w, r)
	}))
	defer server.Close()
	url := server.URL + "/" + filepath.Base(libDir)

	// Everything a CEO's machine may lend: a helper in the global config and in the repo's, a .netrc.
	home := t.TempDir()
	write(t, filepath.Join(home, "creds"), strings.Replace(server.URL, "http://", "http://ceo:secret@", 1)+"\n")
	write(t, filepath.Join(home, ".gitconfig"), "[credential]\n\thelper = store --file="+filepath.Join(home, "creds")+"\n")
	write(t, filepath.Join(home, ".netrc"), "machine 127.0.0.1 login ceo password secret\n")
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))

	r, wts := worktrees(t, "tove")
	gitOut(t, r, "config", "credential.helper", "store --file="+filepath.Join(home, "creds"))
	g := New()
	g.FetchProtocols = []string{"http"}
	_, err := g.SetSubmodule(ctx, wts["tove"], "lib/dep", "v1", url, "Add lib/dep", tove)
	if !errors.Is(err, ErrNeedsCredentials) {
		t.Errorf("a private repo: %v", err)
	}
	if n := withAuth.Load(); n > 0 {
		t.Fatalf("the CEO's credentials were sent %d times", n)
	}
	if _, err := os.Stat(filepath.Join(wts["tove"], "lib/dep")); !os.IsNotExist(err) {
		t.Error("a failed add left lib/dep behind")
	}
	private.Store(false)
	if _, err := g.SetSubmodule(ctx, wts["tove"], "lib/dep", "v1", url, "Add lib/dep", tove); err != nil {
		t.Fatalf("a public repo: %v", err)
	}
	if got := gitOut(t, wts["tove"], "rev-parse", "HEAD:lib/dep"); got != v1 {
		t.Errorf("pinned %s", got)
	}
}

// A submodule comes from the CEO's checkout when it holds the pinned commit:
// nothing downloaded, and its whole history there.
func TestASubmoduleComesFromTheCEOsCheckoutFirst(t *testing.T) {
	r, _, v1, _, wts := withSubmodule(t, "tove")
	wt := wts["tove"]
	if err := New().SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	dep := filepath.Join(wt, "lib/dep")
	if head := gitOut(t, dep, "rev-parse", "HEAD"); head != v1 {
		t.Errorf("lib/dep on %s, want v1", head)
	}
	copy, err := filepath.EvalSymlinks(filepath.Join(r, ".git/modules/lib/dep"))
	if err != nil {
		t.Fatal(err)
	}
	if from := gitOut(t, dep, "remote", "get-url", "origin"); from != copy {
		t.Errorf("lib/dep came from %s, not the CEO's checkout %s", from, copy)
	}
}

// A commit the CEO's checkout lacks comes from the network, that commit only:
// even one that isn't its branch's tip.
func TestASubmoduleTheCEOLacksIsFetchedShallow(t *testing.T) {
	_, libDir, _, _, wts := withSubmodule(t, "tove")
	wt := wts["tove"]
	commit := func(text string) string {
		write(t, filepath.Join(libDir, "lib.sol"), text)
		if err := New().Commit(ctx, libDir, text, "lib.sol"); err != nil {
			t.Fatal(err)
		}
		return gitOut(t, libDir, "rev-parse", "HEAD")
	}
	v3 := commit("v3\n")
	commit("v4\n")
	// The agent pins v3, which the CEO never fetched.
	gitOut(t, wt, "update-index", "--cacheinfo", "160000,"+v3+",lib/dep")
	gitOut(t, wt, "-c", "user.name=Tove", "-c", "user.email=tove@x", "commit", "--quiet", "-m", "lib at v3")
	if err := local().SyncSubmodules(ctx, wt, false); err != nil {
		t.Fatal(err)
	}
	dep := filepath.Join(wt, "lib/dep")
	if head := gitOut(t, dep, "rev-parse", "HEAD"); head != v3 {
		t.Errorf("lib/dep on %s, want v3", head)
	}
	if shallow := gitOut(t, dep, "rev-parse", "--is-shallow-repository"); shallow != "true" {
		t.Error("lib/dep was fetched with its whole history")
	}
}

// A worktree with its submodule checked out goes like any other (git alone
// refuses it, however clean); one whose submodule holds changes stays.
func TestRemoveWorktreeWithASubmodule(t *testing.T) {
	r, _, _, _, wts := withSubmodule(t, "tove", "mara")
	g := local()
	for _, a := range []string{"tove", "mara"} {
		if err := g.SyncSubmodules(ctx, wts[a], false); err != nil {
			t.Fatal(err)
		}
	}
	if emptyOrMissing(filepath.Join(wts["tove"], "lib/dep")) {
		t.Fatal("the submodule isn't checked out: the test proves nothing")
	}
	write(t, filepath.Join(wts["mara"], "lib/dep", "draft.txt"), "not committed\n")
	if err := g.RemoveWorktree(ctx, r, wts["mara"]); !errors.Is(err, ErrUncommitted) {
		t.Errorf("a change in its submodule: %v", err)
	}
	if read(t, filepath.Join(wts["mara"], "lib/dep", "draft.txt")) != "not committed\n" {
		t.Error("the change went")
	}
	if err := g.RemoveWorktree(ctx, r, wts["tove"]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wts["tove"]); !os.IsNotExist(err) {
		t.Errorf("the folder stayed: %v", err)
	}
}
