package git

import (
	"context"
	"errors"
	"slices"
	"strings"
	"syscall"
	"time"
)

// ErrNoRemote means the repo has no remote to fetch.
var ErrNoRemote = errors.New("git: the repo has no remote")

// fetchTimeout bounds a fetch: a remote that hangs fails, as one that refuses.
const fetchTimeout = 2 * time.Minute

// RemoteChange is a remote-tracking branch a fetch moved: From is empty for
// a new one, To for one gone upstream.
type RemoteChange struct{ Branch, From, To string }

// FetchRemotes fetches every remote of the repo at dir, as its config sets
// them, with the CEO's own credentials (their helper, keychain or SSH agent):
// the lead's fetch() (wiki/backlog/lead-fetch.md). Only the CEO writes a
// repo's config, so an agent can't point a remote elsewhere. It moves the
// remote-tracking branches alone, pruning those gone upstream: no tag, no
// local branch, no FETCH_HEAD, and no submodule (their URLs are the agents';
// set_submodule fetches those, without credentials). It never waits on a
// prompt: with nothing to answer it, a missing credential fails at once,
// rather than hang the server on a terminal nobody watches. A remote that
// fails leaves the others fetched: the moves are returned with the error.
func (g *CLI) FetchRemotes(ctx context.Context, dir string) ([]RemoteChange, error) {
	remotes, err := run(ctx, dir, "remote")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(remotes) == "" {
		return nil, ErrNoRemote
	}
	defer g.lock(ctx, dir)()
	before, err := remoteBranches(ctx, dir)
	if err != nil {
		return nil, err
	}
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	cmd := command(fetchCtx, dir, "fetch", "--all", "--prune", "--no-tags", "--no-recurse-submodules", "--no-write-fetch-head", "--quiet")
	cmd.Env = slices.DeleteFunc(cmd.Env, func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return name == "GIT_ASKPASS" || name == "SSH_ASKPASS"
	})
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0", "SSH_ASKPASS_REQUIRE=never", "GCM_INTERACTIVE=never")
	// A session of its own has no terminal for ssh to ask a passphrase on.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_, fetchErr := output(cmd, []string{"fetch"})
	after, err := remoteBranches(ctx, dir)
	if err != nil {
		return nil, errors.Join(fetchErr, err)
	}
	var changes []RemoteChange
	for branch, to := range after {
		if from := before[branch]; from != to {
			changes = append(changes, RemoteChange{Branch: branch, From: from, To: to})
		}
	}
	for branch, from := range before {
		if _, ok := after[branch]; !ok {
			changes = append(changes, RemoteChange{Branch: branch, From: from})
		}
	}
	slices.SortFunc(changes, func(a, b RemoteChange) int { return strings.Compare(a.Branch, b.Branch) })
	return changes, fetchErr
}

// remoteBranches are the repo's remote-tracking branches (origin/main) and
// their commits, without the symbolic ones (origin/HEAD).
func remoteBranches(ctx context.Context, dir string) (map[string]string, error) {
	out, err := run(ctx, dir, "for-each-ref", "--format=%(refname:lstrip=2) %(objectname) %(symref)", "refs/remotes/")
	if err != nil {
		return nil, err
	}
	branches := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if f := strings.Split(line, " "); len(f) == 3 && f[2] == "" {
			branches[f[0]] = f[1]
		}
	}
	return branches, nil
}

// HasRemoteBranch reports whether the repo has a remote-tracking branch
// (origin/main), which only a fetch makes.
func (g *CLI) HasRemoteBranch(ctx context.Context, dir, branch string) bool {
	_, err := run(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/remotes/"+branch)
	return err == nil
}
