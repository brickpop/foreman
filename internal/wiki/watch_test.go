package wiki

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

// watching is a workspace of project wikis under Watch: write files with
// write(t, w.dir(project), …) and read what it publishes from changes.
type watching struct {
	root     string
	wikis    *Wikis
	changes  chan Changed
	follow   func()
	mu       sync.Mutex
	projects []string
}

func watch(t *testing.T, projects ...string) *watching {
	t.Helper()
	ws := &watching{root: t.TempDir(), changes: make(chan Changed, 16), projects: projects}
	for _, p := range projects {
		write(t, ws.dir(p), map[string]string{"wiki.toml": "spec = \"0.1\"\n", "index.md": "# " + p + "\n"})
	}
	ws.wikis = New(func(project string) (string, error) {
		ws.mu.Lock()
		defer ws.mu.Unlock()
		if !slices.Contains(ws.projects, project) {
			return "", errors.New("no such project")
		}
		return ws.dir(project), nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	follow, err := ws.wikis.Watch(ctx, func() []string {
		ws.mu.Lock()
		defer ws.mu.Unlock()
		return slices.Clone(ws.projects)
	}, func(c Changed) { ws.changes <- c })
	if err != nil {
		t.Fatal(err)
	}
	ws.follow = follow
	return ws
}

func (ws *watching) dir(project string) string {
	return filepath.Join(ws.root, project, "project", "wiki")
}

func (ws *watching) setProjects(projects ...string) {
	ws.mu.Lock()
	ws.projects = projects
	ws.mu.Unlock()
	ws.follow()
}

func (ws *watching) next(t *testing.T) Changed {
	t.Helper()
	select {
	case c := <-ws.changes:
		return c
	case <-time.After(3 * time.Second):
		t.Fatal("no wiki.changed")
		return Changed{}
	}
}

func (ws *watching) quiet(t *testing.T) {
	t.Helper()
	select {
	case c := <-ws.changes:
		t.Fatalf("unexpected change %+v", c)
	case <-time.After(3 * debounce):
	}
}

func TestWatchPublishesWhatChanged(t *testing.T) {
	ws := watch(t, "ledger")
	dir := ws.dir("ledger")

	// A burst of writes is one rebuild, naming every entry it touched.
	write(t, dir, map[string]string{"a.md": note("one"), "backlog/b.md": note("two")})
	write(t, dir, map[string]string{"a.md": note("one, again")})
	c := ws.next(t)
	if c.Project != "ledger" || !slices.Equal(c.Paths, []string{"a.md", "backlog/b.md"}) || c.Versions["a.md"] == "" {
		t.Fatalf("change %+v", c)
	}
	ws.quiet(t)
	// Reads come from the rebuilt index.
	if e, err := ws.wikis.Entry("ledger", "a.md"); err != nil || e.Version != c.Versions["a.md"] {
		t.Errorf("entry %+v %v, want version %s", e, err, c.Versions["a.md"])
	}

	// Saving the same bytes, an editor's swap file, a file that isn't the wiki's: nothing.
	write(t, dir, map[string]string{"a.md": note("one, again"), ".a.md.swp": "x", "notes.txt": "x"})
	ws.quiet(t)

	// A folder made after Watch started is followed too.
	write(t, dir, map[string]string{"new/deep/c.md": note("three")})
	if c := ws.next(t); !slices.Equal(c.Paths, []string{"new/deep/c.md"}) {
		t.Errorf("new folder: %+v", c)
	}
	write(t, dir, map[string]string{"new/deep/c.md": note("three, edited")})
	if c := ws.next(t); !slices.Equal(c.Paths, []string{"new/deep/c.md"}) {
		t.Errorf("edit in the new folder: %+v", c)
	}

	// A removed entry's version is "".
	if err := os.Remove(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	if c := ws.next(t); !slices.Equal(c.Paths, []string{"a.md"}) || c.Versions["a.md"] != "" {
		t.Errorf("removal: %+v", c)
	}

	// wiki.toml is part of what the wiki serves (its boards).
	write(t, dir, map[string]string{"wiki.toml": "spec = \"0.1\"\n# boards\n"})
	if c := ws.next(t); !slices.Equal(c.Paths, []string{"wiki.toml"}) {
		t.Errorf("config: %+v", c)
	}

	// An image an entry links to appearing changes the entry's lookup table.
	write(t, dir, map[string]string{"d.md": note("![x](x.png)")})
	ws.next(t)
	write(t, dir, map[string]string{"x.png": "png"})
	if c := ws.next(t); len(c.Paths) != 0 {
		t.Errorf("image: %+v", c)
	}
	if e, _ := ws.wikis.Entry("ledger", "d.md"); len(e.Links) != 1 || !e.Links[0].Image {
		t.Errorf("the entry still says the image is missing: %+v", e.Links)
	}
}

func TestWatchFollowsProjects(t *testing.T) {
	ws := watch(t, "ledger")

	// A project added later: its wiki is followed once Watch hears of it.
	write(t, ws.dir("cal"), map[string]string{"wiki.toml": "spec = \"0.1\"\n", "index.md": "# cal\n"})
	ws.setProjects("ledger", "cal")
	time.Sleep(debounce) // the follow is asynchronous
	write(t, ws.dir("cal"), map[string]string{"a.md": note("")})
	if c := ws.next(t); c.Project != "cal" || !slices.Equal(c.Paths, []string{"a.md"}) {
		t.Errorf("new project: %+v", c)
	}

	// A project that went is forgotten: its files publish nothing.
	ws.setProjects("cal")
	time.Sleep(debounce)
	write(t, ws.dir("ledger"), map[string]string{"a.md": note("")})
	ws.quiet(t)
	if _, err := ws.wikis.Tree("ledger"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a forgotten project's wiki: %v", err)
	}
}

func TestWatchSeesAWikiAppear(t *testing.T) {
	ws := watch(t, "ledger")
	if err := os.MkdirAll(filepath.Join(ws.root, "bare", "project"), 0o755); err != nil {
		t.Fatal(err)
	}
	ws.setProjects("ledger", "bare")
	time.Sleep(debounce)
	if _, err := ws.wikis.Tree("bare"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no wiki yet: %v", err)
	}
	write(t, ws.dir("bare"), map[string]string{"wiki.toml": "spec = \"0.1\"\n", "index.md": "# bare\n"})
	if c := ws.next(t); c.Project != "bare" || !slices.Contains(c.Paths, "index.md") {
		t.Errorf("the new wiki: %+v", c)
	}
	if tree, err := ws.wikis.Tree("bare"); err != nil || len(tree) != 1 {
		t.Errorf("tree %+v %v", tree, err)
	}
}
