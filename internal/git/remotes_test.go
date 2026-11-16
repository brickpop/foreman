package git

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// upstream: a repo cloned as dir's origin, with main and feat/x, and the
// clone's own branch work on top of main.
func upstream(t *testing.T) (g *CLI, dir, up string) {
	t.Helper()
	up, g = repo(t), New()
	write(t, filepath.Join(up, "README"), "hello\n")
	if err := g.Commit(ctx, up, "first", "README"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, up, "branch", "feat/x")
	dir = filepath.Join(t.TempDir(), "clone")
	mustRun(t, up, "clone", "--quiet", up, dir)
	mustRun(t, dir, "branch", "work")
	return g, dir, up
}

func TestFetchRemotesMovesOnlyTheRemoteBranches(t *testing.T) {
	g, dir, up := upstream(t)
	if changes, err := g.FetchRemotes(ctx, dir); err != nil || changes != nil {
		t.Fatalf("nothing new upstream: %v %v", changes, err)
	}
	before := map[string]string{}
	for _, b := range []string{"main", "work"} {
		before[b] = mustRun(t, dir, "rev-parse", b)
	}
	// Upstream: main moves, a branch is new, one is deleted, a tag is made.
	from := mustRun(t, up, "rev-parse", "main")
	change(t, g, up, "README", "moved\n", Author{})
	to := mustRun(t, up, "rev-parse", "main")
	mustRun(t, up, "branch", "feat/y")
	mustRun(t, up, "branch", "-D", "feat/x")
	mustRun(t, up, "tag", "v1")

	changes, err := g.FetchRemotes(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []RemoteChange{{Branch: "origin/feat/x", From: from}, {Branch: "origin/feat/y", To: to}, {Branch: "origin/main", From: from, To: to}}
	if !reflect.DeepEqual(changes, want) {
		t.Errorf("changes %+v, want %+v", changes, want)
	}
	for b, at := range before {
		if now := mustRun(t, dir, "rev-parse", b); now != at {
			t.Errorf("local %s moved: %s → %s", b, at, now)
		}
	}
	if tags := mustRun(t, dir, "tag"); tags != "" {
		t.Errorf("tags fetched: %q", tags)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "FETCH_HEAD")); err == nil {
		t.Error("FETCH_HEAD written")
	}
	if !g.HasRemoteBranch(ctx, dir, "origin/feat/y") || g.HasRemoteBranch(ctx, dir, "origin/feat/x") || g.HasRemoteBranch(ctx, dir, "work") {
		t.Error("HasRemoteBranch")
	}
	if changes, err := g.FetchRemotes(ctx, dir); err != nil || changes != nil {
		t.Errorf("a second fetch: %v %v", changes, err)
	}
}

func TestFetchRemotesWithoutAnyRemote(t *testing.T) {
	if _, err := New().FetchRemotes(ctx, repo(t)); !errors.Is(err, ErrNoRemote) {
		t.Errorf("err %v", err)
	}
}

// A remote that fails, here asking for a password nobody can give, fails at
// once, and the others are fetched all the same.
func TestFetchRemotesNeverWaitsOnAPrompt(t *testing.T) {
	g, dir, up := upstream(t)
	locked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer locked.Close()
	mustRun(t, dir, "remote", "add", "locked", locked.URL+"/repo.git")
	change(t, g, up, "README", "moved\n", Author{})

	start := time.Now()
	changes, err := g.FetchRemotes(ctx, dir)
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Errorf("err %v, want the locked remote's", err)
	}
	if time.Since(start) > 30*time.Second {
		t.Errorf("took %v", time.Since(start))
	}
	if len(changes) != 1 || changes[0].Branch != "origin/main" {
		t.Errorf("origin not fetched: %+v", changes)
	}
}

// Origin is kept where others read it (projects.toml, pushed with Foreman's
// HQ): a token in an https URL never goes with it.
func TestOrigin(t *testing.T) {
	g, dir, up := upstream(t)
	if got := g.Origin(ctx, dir); got != up {
		t.Errorf("a clone: %q, want %q", got, up)
	}
	if got := g.Origin(ctx, repo(t)); got != "" {
		t.Errorf("no origin: %q", got)
	}
	if got := g.Origin(ctx, filepath.Join(t.TempDir(), "gone")); got != "" {
		t.Errorf("no repo: %q", got)
	}
	// This machine's rewrites stay out of it.
	mustRun(t, dir, "config", "url.git@github.com:.insteadOf", "https://github.com/")
	mustRun(t, dir, "remote", "set-url", "origin", "https://github.com/acme/api.git")
	if got := g.Origin(ctx, dir); got != "https://github.com/acme/api.git" {
		t.Errorf("rewritten: %q", got)
	}
	for url, want := range map[string]string{
		"https://ghp_secret@github.com/acme/api.git":       "https://github.com/acme/api.git",
		"https://ceo:hunter2@git.example.com/acme/api.git": "https://git.example.com/acme/api.git",
		"http://ceo:hunter2@localhost:3000/api":            "http://localhost:3000/api",
		"ssh://git:hunter2@github.com/acme/api.git":        "ssh://git@github.com/acme/api.git",
		"ssh://git@github.com/acme/api.git":                "ssh://git@github.com/acme/api.git",
		"git@github.com:acme/api.git":                      "git@github.com:acme/api.git",
		"https://github.com/acme/api.git":                  "https://github.com/acme/api.git",
	} {
		mustRun(t, dir, "remote", "set-url", "origin", url)
		if got := g.Origin(ctx, dir); got != want {
			t.Errorf("%s: %q, want %q", url, got, want)
		}
	}
}

// A repo with no folder on this machine must not make git act on the
// server's own folder.
func TestNoFolderFails(t *testing.T) {
	g, dir, _ := upstream(t)
	t.Chdir(dir) // where git -C "" would act: a repo with main, a commit and a remote
	if _, err := g.RootCommit(ctx, ""); err == nil {
		t.Error("RootCommit: no error")
	}
	if _, err := g.FetchRemotes(ctx, ""); !errors.Is(err, ErrNoFolder) {
		t.Errorf("FetchRemotes: %v", err)
	}
	if g.HasBranch(ctx, "", "main") {
		t.Error("HasBranch: true")
	}
}
