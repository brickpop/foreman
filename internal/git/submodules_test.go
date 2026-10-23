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
	_, _, _, _, wts := withSubmodule(t, "tove")
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
