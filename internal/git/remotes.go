package git

import (
	"context"
	"errors"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
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
	_, fetchErr := remote(fetchCtx, dir, "fetch", "--all", "--prune", "--no-tags", "--no-recurse-submodules", "--no-write-fetch-head", "--quiet")
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

// remote runs a git command that reaches a remote with the CEO's own
// credentials (their helper, keychain or SSH agent), never waiting on a
// prompt: with nobody to answer it, a missing credential fails at once
// rather than hang the server on a terminal nobody watches.
func remote(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := command(ctx, dir, args...)
	cmd.Env = slices.DeleteFunc(cmd.Env, func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return name == "GIT_ASKPASS" || name == "SSH_ASKPASS"
	})
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0", "SSH_ASKPASS_REQUIRE=never", "GCM_INTERACTIVE=never")
	// A session of its own has no terminal for ssh to ask a passphrase on.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return output(cmd, args)
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

// Origin is where the repo at dir was cloned from, its origin's URL as set
// (not as this machine's insteadOf rewrites it), as one to keep where others
// read it: without the credentials an https URL may
// hold (https://<token>@host/…); "" when it has no origin.
func (g *CLI) Origin(ctx context.Context, dir string) string {
	// As configured: get-url applies the CEO's url.*.insteadOf rewrites,
	// which belong to this machine.
	out, err := run(ctx, dir, "config", "--get", "remote.origin.url")
	if err != nil {
		return ""
	}
	return WithoutCredentials(strings.TrimSpace(out))
}

// MatchOrigin checks the configured fetch URL and the destinations Git would
// use for this checkout and for a fresh clone from its parent directory.
func (g *CLI) MatchOrigin(ctx context.Context, dir, remote string) error {
	out, err := run(ctx, dir, "config", "--null", "--get-all", "remote.origin.url")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return errors.New("has no origin")
		}
		return errors.New("has an unreadable origin")
	}
	configured := strings.TrimSuffix(out, "\x00")
	if strings.ContainsRune(configured, '\x00') {
		return errors.New("has multiple origin URLs: configure a single fetch URL before sharing it")
	}
	if configured == "" {
		return errors.New("has no origin")
	}
	if originHasCredentials(configured) {
		return errors.New("has credentials in its origin: move them to a host credential helper before sharing this checkout")
	}
	if WithoutCredentials(configured) != WithoutCredentials(remote) && !sameGitHubRepo(configured, remote) {
		return errors.New("holds another repo")
	}

	// These commands expand insteadOf without contacting the remote.
	fetch, err := run(ctx, dir, "remote", "get-url", "origin")
	if err != nil {
		return errors.New("has an unreadable effective origin")
	}
	clone, err := run(ctx, filepath.Dir(dir), "ls-remote", "--get-url", "--", remote)
	if err != nil {
		return errors.New("cannot verify the clone URL after Git rewrites")
	}
	// A clone initializes its destination before fetching. A gitdir include
	// there can rewrite the submitted URL differently than in its parent.
	atDestination, err := run(ctx, dir, "ls-remote", "--get-url", "--", remote)
	if err != nil || atDestination != clone {
		return errors.New("cannot verify the clone URL across destination-specific Git rewrites")
	}
	fetch = strings.TrimSuffix(fetch, "\n")
	clone = strings.TrimSuffix(clone, "\n")
	if fetch == configured && clone == remote {
		return nil
	}
	// Relative filesystem destinations have different working directories.
	prefix, _, colon := strings.Cut(fetch, ":")
	relative := !filepath.IsAbs(fetch) && (!colon || strings.Contains(prefix, "/"))
	if relative || (fetch != clone && !sameGitHubRepo(fetch, clone)) {
		return errors.New("uses a different destination after Git URL rewrites")
	}
	return nil
}

func originHasCredentials(remote string) bool {
	scheme, rest, ok := strings.Cut(remote, "://")
	if !ok {
		return false
	}
	authority, _, _ := strings.Cut(rest, "/")
	user, _, hasUser := strings.Cut(authority, "@")
	return hasUser && (strings.EqualFold(scheme, "https") || strings.EqualFold(scheme, "http") || strings.Contains(user, ":"))
}

func sameGitHubRepo(a, b string) bool {
	path := githubRepoPath(a)
	return path != "" && path == githubRepoPath(b)
}

var githubPath = regexp.MustCompile(`^[a-zA-Z0-9-]+/[a-zA-Z0-9_-][a-zA-Z0-9_.-]*$`)

// githubRepoPath recognizes the two-part paths of standard GitHub clone
// URLs. Other servers can give the same path different meanings over
// different transports or with different SSH users.
func githubRepoPath(remote string) string {
	if strings.IndexFunc(remote, func(r rune) bool { return r > 127 }) >= 0 {
		return ""
	}
	var path string
	scpHost, scpPath, _ := strings.Cut(remote, ":")
	if host, ok := strings.CutPrefix(scpHost, "git@"); ok && strings.EqualFold(host, "github.com") {
		path = scpPath
	} else {
		u, err := url.Parse(remote)
		if err != nil || strings.ContainsAny(remote, "?#") || strings.Count(remote, "@") > 1 {
			return ""
		}
		var defaultPort string
		switch {
		case strings.HasPrefix(remote, "https://"):
			defaultPort = ":443"
		case strings.HasPrefix(remote, "ssh://git@"):
			defaultPort = ":22"
		default:
			return ""
		}
		if !strings.EqualFold(u.Host, "github.com") && !strings.EqualFold(u.Host, "github.com"+defaultPort) {
			return ""
		}
		path = strings.TrimPrefix(u.EscapedPath(), "/")
	}
	if !githubPath.MatchString(path) || strings.Contains(path, "..") {
		return ""
	}
	return strings.TrimSuffix(path, ".git")
}

// WithoutCredentials is a remote URL as it may be kept or shown: an http(s)
// URL without its user and password; other URLs (git@host:…,
// ssh://git@host/…, a path) name only a user, which stays.
func WithoutCredentials(remote string) string {
	u, err := url.Parse(remote)
	if err != nil || u.User == nil {
		return remote
	}
	switch u.Scheme {
	case "http", "https":
		u.User = nil
	default:
		u.User = url.User(u.User.Username())
	}
	return u.String()
}
