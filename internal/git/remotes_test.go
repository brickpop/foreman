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
