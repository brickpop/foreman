//go:build podman

package podman

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/runner"
)

// The layout agents work in (wiki/product/architecture/sandboxes-and-git.md):
// their worktree mounted read-write, the repo's .git read-only, both at their
// host paths. Inside, git reads must work and every git write must fail.
func TestGitThroughAReadOnlyGitMount(t *testing.T) {
	r := engine(t)
	image, err := testBuilder(t, r, `RUN apk add --no-cache git`).Ensure(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	// Under the home folder: on macOS, the podman machine shares it.
	home, _ := os.UserHomeDir()
	base, err := os.MkdirTemp(home, ".foreman-git-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	repo := filepath.Join(base, "ledger", "ledger-api")
	worktree := filepath.Join(base, "ledger", ".sandboxes", "mara", "ledger-api")
	for _, args := range [][]string{
		{"init", "--quiet", "--initial-branch=main", repo},
		{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", "first"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := git.New()
	if err := g.Commit(ctx, repo, "readme", "README"); err != nil {
		t.Fatal(err)
	}
	if err := g.Prepare(ctx, repo, worktree, "wt/mara/main", "main"); err != nil {
		t.Fatal(err)
	}
	// Uncommitted work in the worktree, for status and diff to show.
	if err := os.WriteFile(filepath.Join(worktree, "README"), []byte("hello, edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	script := fmt.Sprintf(`cd %s
git status --short > /tmp/status 2>&1; echo "status-exit=$?"; cat /tmp/status
git log --oneline -1 > /dev/null && echo log-ok
git diff --stat > /dev/null && echo diff-ok
git blame README > /dev/null && echo blame-ok
git commit --quiet -am x > /dev/null 2>&1 && echo COMMITTED || echo commit-refused
git checkout --quiet -b other > /dev/null 2>&1 && echo BRANCHED || echo branch-refused
echo END`, worktree)
	p := start(t, r, runner.Spec{
		Image: image, Command: sh(script), Dir: worktree, Env: []string{"HOME=/tmp"},
		Mounts: []runner.Mount{
			{Source: worktree, Target: worktree},
			{Source: filepath.Join(repo, ".git"), Target: filepath.Join(repo, ".git"), ReadOnly: true},
		},
	})
	out := collect(p)
	out.waitFor(t, "END")
	out.mu.Lock()
	got := out.buf.String()
	out.mu.Unlock()
	for _, want := range []string{"status-exit=0", "M README", "log-ok", "diff-ok", "blame-ok", "commit-refused", "branch-refused"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// git status refreshes the index when it can; on a read-only .git it must
	// skip that without complaining.
	if strings.Contains(got, "fatal") || strings.Contains(got, "error") {
		t.Errorf("git complained:\n%s", got)
	}
	if strings.Contains(got, "warning") {
		t.Logf("git warned (not a failure):\n%s", got)
	}
}
