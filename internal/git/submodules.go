package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Submodules in the agents' worktrees
// (wiki/product/architecture/sandboxes-and-git.md#submodules). A worktree keeps
// its submodules' git data under its own git dir (<repo>/.git/worktrees/<name>/modules),
// read-only in the container like the rest of .git: Foreman checks them out,
// on the host, at the commits the branch pins, and agents move a pin with
// set_submodule. Nothing is written to the repo's own .git/config: the URLs
// are passed on each command.

var (
	// ErrBadPath means a submodule path that isn't a plain path inside the worktree.
	ErrBadPath = errors.New("git: not a path inside the worktree")
	// ErrNotSubmodule means the path isn't a submodule of the repo.
	ErrNotSubmodule = errors.New("git: not a submodule")
	// ErrSubmoduleExists means the path is taken: a submodule or files already.
	ErrSubmoduleExists = errors.New("git: the path is taken")
	// ErrBadURL means a URL Foreman doesn't fetch from.
	ErrBadURL = errors.New("git: not an allowed URL")
	// ErrUnknownRef means the submodule's repo has no such branch, tag or commit.
	ErrUnknownRef = errors.New("git: no such branch, tag or commit")
	// ErrNeedsCredentials means a fetch was refused for lack of credentials,
	// which Foreman never lends: a private repo.
	ErrNeedsCredentials = errors.New("git: the repo needs credentials")
)

// SubmoduleConflictError lists the submodules an unfinished merge left
// conflicted: no markers to resolve, a commit to pick.
type SubmoduleConflictError struct{ Paths []string }

func (e *SubmoduleConflictError) Error() string {
	return "git: submodule conflicts in " + strings.Join(e.Paths, ", ")
}

// NestedReposError lists the git repos inside a worktree that aren't its
// submodules: a commit would record each as a bare pointer to a commit no one
// else has.
type NestedReposError struct{ Paths []string }

func (e *NestedReposError) Error() string {
	return "git: repos inside the worktree that aren't submodules: " + strings.Join(e.Paths, ", ")
}

// SyncSubmodules checks the worktree's submodules out at the commits its
// branch pins, fetching what's missing, and removes the ones the branch no
// longer has (left behind by a switch or a merge, and otherwise committed
// back by the next commit), when nothing was changed in them. Nested
// submodules too when recursive.
func (g *CLI) SyncSubmodules(ctx context.Context, dir string, recursive bool) error {
	defer g.lock(ctx, dir)()
	if err := dropLeftovers(ctx, dir); err != nil {
		return err
	}
	return g.update(ctx, dir, recursive)
}

// update runs `submodule update --init` with the URLs of the branch's
// .gitmodules, so nothing is written to the repo's .git/config, and always as
// a checkout: a .gitmodules can ask for a merge, a rebase or none.
//
// One submodule at a time, so each finds its own way: an SSH URL is tried
// over https (a public repo answers without credentials), and failing that
// the submodule comes from the CEO's own checkout of the repo, locally: what
// the CEO fetched, which agents can already read (the repo's .git is mounted
// in their containers), so no credential is lent (the CEO, 2026-10-30).
func (g *CLI) update(ctx context.Context, dir string, recursive bool, paths ...string) error {
	urls, err := submoduleURLs(ctx, dir)
	if err != nil || len(urls) == 0 {
		return err
	}
	byPath, err := submoduleNames(ctx, dir)
	if err != nil {
		return err
	}
	links, err := run(ctx, dir, "ls-files", "--stage", "-z")
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range strings.Split(links, "\x00") {
		info, path, ok := strings.Cut(entry, "\t")
		if !ok || !strings.HasPrefix(info, "160000 ") || (len(paths) > 0 && !slices.Contains(paths, path)) {
			continue
		}
		name := byPath[path]
		err := g.updateOne(ctx, dir, recursive, name, httpsOf(urls[name]), path)
		if err == nil {
			continue
		}
		if copy := ceoCopy(ctx, dir, name); copy == "" {
			err = fmt.Errorf("%w; not in the CEO's checkout either (git submodule update --init %s there fills it)", err, path)
		} else if local := g.updateOne(ctx, dir, false, name, copy, path, "-c", "protocol.file.allow=always"); local == nil {
			// Never recursive with file allowed: a nested .gitmodules could
			// name any repo on the machine. Its nested ones go the usual way.
			if recursive {
				if err := g.update(ctx, filepath.Join(dir, path), true); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", path, err))
				}
			}
			continue
		} else {
			err = fmt.Errorf("%w; from the CEO's checkout: %v (a git fetch there may bring the commit)", err, local)
		}
		errs = append(errs, fmt.Errorf("%s: %w", path, err))
	}
	return errors.Join(errs...)
}

// updateOne checks the submodule at path out, from url (extra: more git
// config, put after fetch's own so it wins).
func (g *CLI) updateOne(ctx context.Context, dir string, recursive bool, name, url, path string, extra ...string) error {
	args := slices.Clone(extra)
	if name != "" {
		args = append(args, "-c", "submodule."+name+".url="+url, "-c", "submodule."+name+".update=checkout")
	}
	args = append(args, "submodule", "update", "--init", "--quiet")
	if recursive {
		args = append(args, "--recursive")
	}
	_, err := g.fetch(ctx, dir, append(args, "--", path)...)
	return err
}

// httpsOf is an SSH URL's https counterpart (git@host:path, ssh://user@host:port/path
// → https://host/path); any other URL as it is.
func httpsOf(url string) string {
	var host, path string
	if rest, ok := strings.CutPrefix(url, "ssh://"); ok {
		host, path, _ = strings.Cut(rest, "/")
		host, _, _ = strings.Cut(host[strings.Index(host, "@")+1:], ":")
	} else if h, p, ok := strings.Cut(url, ":"); ok && !strings.Contains(url, "://") && h != "" && !strings.Contains(h, "/") {
		host, path = h[strings.Index(h, "@")+1:], p
	} else {
		return url
	}
	return "https://" + host + "/" + strings.TrimPrefix(path, "/")
}

// ceoCopy is where the CEO's own checkout of the repo keeps the submodule's
// git data, "" when it has none.
func ceoCopy(ctx context.Context, dir, name string) string {
	common, err := run(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || !filepath.IsLocal(name) {
		return ""
	}
	copy := filepath.Join(strings.TrimSpace(common), "modules", name)
	if info, err := os.Stat(copy); err != nil || !info.IsDir() {
		return ""
	}
	return copy
}

// submoduleNames are the submodules .gitmodules names, by path.
func submoduleNames(ctx context.Context, dir string) (map[string]string, error) {
	out, err := run(ctx, dir, "config", "--file", ".gitmodules", "-z", "--get-regexp", `^submodule\..*\.path$`)
	if err != nil && strings.TrimSpace(out) == "" {
		return nil, nil // no path at all: git config exits 1
	}
	names := map[string]string{}
	for _, entry := range strings.Split(out, "\x00") {
		if key, path, ok := strings.Cut(entry, "\n"); ok {
			names[path] = strings.TrimSuffix(strings.TrimPrefix(key, "submodule."), ".path")
		}
	}
	return names, nil
}

// relinkSubmodules points each checked-out submodule of the worktree dir at
// its git data again, and the data back at it. Git writes both links as
// relative paths (the submodule's .git file, the data's core.worktree), so a
// move of the worktree breaks them and `worktree repair` leaves them: every
// git command in the worktree then fails (found 2026-10-30, after a project
// moved into its workspace). Nested submodules too.
//
// It runs on the host, and .gitmodules is the agent's to edit: it only ever
// rewrites a submodule's existing .git file, inside the worktree and through
// no symlink, to point into the worktree's own git dir.
func relinkSubmodules(ctx context.Context, dir string) error {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	out, err := run(ctx, root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}
	gitDir, err := filepath.EvalSymlinks(strings.TrimSpace(out))
	if err != nil {
		return err
	}
	names, err := submoduleNames(ctx, root)
	if err != nil {
		return err
	}
	for path, name := range names {
		if !filepath.IsLocal(name) || !filepath.IsLocal(path) {
			continue
		}
		data := filepath.Join(gitDir, "modules", name)
		sub := filepath.Join(root, path)
		if real, err := filepath.EvalSymlinks(sub); err != nil || real != sub {
			continue // not checked out, or reached through a symlink
		}
		if info, err := os.Lstat(filepath.Join(sub, ".git")); err != nil || !info.Mode().IsRegular() {
			continue // never checked out, or a repo of its own
		}
		if info, err := os.Stat(data); err != nil || !info.IsDir() {
			continue
		}
		toData, _ := filepath.Rel(sub, data)
		toSub, _ := filepath.Rel(data, sub)
		if err := os.WriteFile(filepath.Join(sub, ".git"), []byte("gitdir: "+toData+"\n"), 0o644); err != nil {
			return err
		}
		if _, err := run(ctx, root, "config", "--file", filepath.Join(data, "config"), "core.worktree", toSub); err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(data, "modules")); err == nil {
			if err := relinkSubmodules(ctx, sub); err != nil {
				return err
			}
		}
	}
	return nil
}

// submoduleURLs are the URLs .gitmodules gives, by submodule name.
func submoduleURLs(ctx context.Context, dir string) (map[string]string, error) {
	if _, err := os.Stat(filepath.Join(dir, ".gitmodules")); err != nil {
		return nil, nil
	}
	out, err := run(ctx, dir, "config", "--file", ".gitmodules", "-z", "--get-regexp", `^submodule\..*\.url$`)
	if err != nil && strings.TrimSpace(out) == "" {
		return nil, nil // no url at all: git config exits 1
	}
	urls := map[string]string{}
	for _, entry := range strings.Split(out, "\x00") {
		key, url, ok := strings.Cut(entry, "\n")
		if !ok {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(key, "submodule."), ".url")
		// A name with "=" would split the -c that passes its URL.
		if strings.Contains(name, "=") {
			return nil, fmt.Errorf("git: submodule name %q has an \"=\"", name)
		}
		urls[name] = url
	}
	return urls, nil
}

// dropLeftovers removes the checkouts of submodules the branch no longer
// has: a repo inside the worktree whose git dir is one of the worktree's
// submodules, with nothing changed in it. Its git dir stays, so a switch back
// restores it without a fetch.
func dropLeftovers(ctx context.Context, dir string) error {
	nested, err := nestedRepos(ctx, dir)
	if err != nil || len(nested) == 0 {
		return err
	}
	gitDir, err := run(ctx, dir, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}
	modules, err := filepath.EvalSymlinks(filepath.Join(strings.TrimSpace(gitDir), "modules"))
	if err != nil {
		return nil // no submodule ever checked out here
	}
	for _, p := range nested {
		sub := filepath.Join(dir, p)
		target, ok := gitFile(sub)
		if !ok || !strings.HasPrefix(target, modules+string(filepath.Separator)) {
			continue
		}
		if out, err := run(ctx, sub, "status", "--porcelain"); err != nil || strings.TrimSpace(out) != "" {
			continue // changed in it: the agent decides
		}
		if err := os.RemoveAll(sub); err != nil {
			return err
		}
	}
	return nil
}

// gitFile is where sub's .git file points, as a real path, if it is a file.
func gitFile(sub string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(sub, ".git"))
	if err != nil {
		return "", false
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
	if !ok {
		return "", false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(sub, target)
	}
	real, err := filepath.EvalSymlinks(target)
	return real, err == nil
}

// nestedRepos are the git repos inside the worktree that it doesn't track:
// git lists each as one untracked entry ending in "/".
func nestedRepos(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, p := range strings.Split(out, "\x00") {
		if strings.HasSuffix(p, "/") {
			repos = append(repos, strings.TrimSuffix(p, "/"))
		}
	}
	return repos, nil
}

// unmergedSubmodules are the submodules an unfinished merge left conflicted.
func unmergedSubmodules(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "ls-files", "--unmerged", "-z")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range strings.Split(out, "\x00") {
		info, path, ok := strings.Cut(entry, "\t")
		if ok && strings.HasPrefix(info, "160000 ") && !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// DirtySubmodules are the submodules with files changed or added in them:
// changes no commit can record (the repo stores only the commit a submodule
// is on), which a switch or a merge would carry along or trip on.
func (g *CLI) DirtySubmodules(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "status", "--porcelain=v2", "-z", "--ignore-submodules=none")
	if err != nil {
		return nil, err
	}
	var dirty []string
	records := strings.Split(out, "\x00")
	for i := 0; i < len(records); i++ {
		fields := strings.SplitN(records[i], " ", 9)
		switch fields[0] {
		case "1": // 1 XY sub mH mI mW hH hI path
			if len(fields) == 9 && len(fields[2]) == 4 && fields[2][0] == 'S' && (fields[2][2] == 'M' || fields[2][3] == 'U') {
				dirty = append(dirty, fields[8])
			}
		case "2": // a rename: its origin path is the next record
			i++
		}
	}
	return dirty, nil
}

// SetSubmodule puts the submodule at path on ref (a branch, a tag or a
// commit of its repo), discarding what was changed in it: with url it adds
// the submodule first, the path being free. The change is committed by the
// author with message, except during a merge, where it is only staged (it
// settles a submodule conflict) and the merge's commit records it; commit is
// then "". Already on ref, it only discards (ErrNothingToCommit).
func (g *CLI) SetSubmodule(ctx context.Context, dir, path, ref, url, message string, by Author) (commit string, err error) {
	path = filepath.ToSlash(filepath.Clean(path))
	if !filepath.IsLocal(path) || slices.Contains(strings.Split(path, "/"), ".git") {
		return "", ErrBadPath
	}
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, " \t\n") {
		return "", fmt.Errorf("%w: %q", ErrUnknownRef, ref)
	}
	defer g.lock(ctx, dir)()
	entries, err := run(ctx, dir, "ls-files", "--stage", "-z", "--", path)
	if err != nil {
		return "", err
	}
	sub := filepath.Join(dir, path)
	gitDir, err := run(ctx, dir, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	modules := filepath.Join(strings.TrimSpace(gitDir), "modules", path)
	files := []string{path}
	if url != "" {
		if !g.allowed(url) {
			return "", fmt.Errorf("%w: %s (only %s)", ErrBadURL, url, strings.Join(g.protocols(), ", "))
		}
		if entries != "" || !emptyOrMissing(sub) {
			return "", ErrSubmoduleExists
		}
		// A git dir left by an earlier submodule of that path would refuse the clone.
		if err := os.RemoveAll(modules); err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(modules), 0o755); err != nil {
			return "", err
		}
		if _, err := g.fetch(ctx, dir, "clone", "--quiet", "--no-checkout", "--separate-git-dir="+modules, "--", url, path); err != nil {
			return "", err
		}
		defer func() {
			if err != nil { // nothing half-added
				_ = os.RemoveAll(sub)
				_ = os.RemoveAll(modules)
			}
		}()
		files = append(files, ".gitmodules")
	} else {
		if !strings.HasPrefix(entries, "160000 ") {
			return "", ErrNotSubmodule
		}
		if _, ok := gitFile(sub); !ok {
			if err := g.update(ctx, dir, false, path); err != nil {
				return "", err
			}
		}
		if _, err := g.fetch(ctx, sub, "fetch", "--quiet", "--tags", "origin"); err != nil {
			return "", err
		}
	}
	target, err := g.resolve(ctx, sub, ref)
	if err != nil {
		return "", err
	}
	if _, err := run(ctx, sub, "checkout", "--quiet", "--force", "--detach", target); err != nil {
		return "", err
	}
	if url != "" {
		for _, kv := range [][2]string{{"path", path}, {"url", url}} {
			if _, err := run(ctx, dir, "config", "--file", ".gitmodules", "submodule."+path+"."+kv[0], kv[1]); err != nil {
				return "", err
			}
		}
	}
	if _, err := run(ctx, dir, append([]string{"add", "--"}, files...)...); err != nil {
		return "", err
	}
	if merging, err := g.Merging(ctx, dir); err != nil || merging {
		return "", err
	}
	// Already on that commit: only its changed files were discarded.
	if _, err := run(ctx, dir, append([]string{"diff", "--cached", "--quiet", "--"}, files...)...); err == nil {
		return "", ErrNothingToCommit
	}
	args := append(by.config(), append([]string{"commit", "--quiet", "--no-verify", "-m", message, "--"}, files...)...)
	if _, err := run(ctx, dir, args...); err != nil {
		return "", err
	}
	head, err := run(ctx, dir, "rev-parse", "HEAD")
	return strings.TrimSpace(head), err
}

// resolve finds ref in the submodule's repo: a branch of its remote, a tag, a
// commit; a commit no branch or tag reaches is fetched by its id.
func (g *CLI) resolve(ctx context.Context, sub, ref string) (string, error) {
	for _, r := range []string{"refs/remotes/origin/" + ref, "refs/tags/" + ref, ref} {
		if out, err := run(ctx, sub, "rev-parse", "--verify", "--quiet", r+"^{commit}"); err == nil {
			return strings.TrimSpace(out), nil
		}
	}
	if _, err := g.fetch(ctx, sub, "fetch", "--quiet", "origin", ref); err == nil {
		if out, err := run(ctx, sub, "rev-parse", "--verify", "--quiet", "FETCH_HEAD^{commit}"); err == nil {
			return strings.TrimSpace(out), nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrUnknownRef, ref)
}

func emptyOrMissing(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err != nil || len(entries) == 0
}

// protocols are what Foreman fetches over: https only, unless the CLI says
// otherwise (the tests' local repos).
func (g *CLI) protocols() []string {
	if len(g.FetchProtocols) == 0 {
		return []string{"https"}
	}
	return g.FetchProtocols
}

// allowed says whether Foreman fetches from url: a URL with one of its
// protocols (never scp-like ssh, a local path, or a URL relative to the
// repo's own remote).
func (g *CLI) allowed(url string) bool {
	scheme, _, ok := strings.Cut(url, "://")
	return ok && slices.Contains(g.protocols(), scheme)
}

// fetch runs a git command that reaches a remote, without the CEO's
// credentials: no global or system config (credential helpers, a keychain,
// url rewrites), no credential helper of the repo, no SSH agent, no prompt,
// an empty home (no .netrc), and only the allowed protocols, for the
// submodules a nested repo asks for too. An agent picks these URLs: lending
// it the CEO's credentials would let it pull any private repo the CEO can
// reach into its sandbox, and from there out to the network.
func (g *CLI) fetch(ctx context.Context, dir string, args ...string) (string, error) {
	home, err := os.MkdirTemp("", "foreman-fetch-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(home)
	config := []string{"-c", "protocol.allow=never", "-c", "credential.helper=", "-c", "http.extraHeader="}
	for _, p := range g.protocols() {
		config = append(config, "-c", "protocol."+p+".allow=always")
	}
	cmd := command(ctx, dir, append(config, args...)...)
	cmd.Env = slices.DeleteFunc(cmd.Env, func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return slices.Contains([]string{"HOME", "XDG_CONFIG_HOME", "SSH_AUTH_SOCK", "GIT_ASKPASS", "SSH_ASKPASS", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM"}, name)
	})
	cmd.Env = append(cmd.Env, "HOME="+home, "XDG_CONFIG_HOME="+home, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		for _, s := range []string{"terminal prompts disabled", "Authentication failed", "could not read Username"} {
			if strings.Contains(msg, s) {
				return "", fmt.Errorf("%w: %s", ErrNeedsCredentials, msg)
			}
		}
		return "", fmt.Errorf("git fetch: %w: %s", err, msg)
	}
	return out.String(), nil
}
