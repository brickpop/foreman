package home

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A new machine: Foreman's HQ restored from the old one's origin, then a
// project brought here, its HQ cloned and its code repo cloned from its
// record.
func TestRestoreAndBringHere(t *testing.T) {
	old := loaded(t, nil)
	fresh := loaded(t, nil)
	root := remotes(t)
	served(t, root, "acme/api.git", "main")
	trunk := served(t, root, "acme/trunk.git", "trunk")

	ledger, _, err := old.NewProject(ctx, "ledger", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := old.NewProject(ctx, "relay", ""); err != nil {
		t.Fatal(err)
	}
	hq := backedBy(t, ledger.Path)
	if _, err := old.CloneRepo(ctx, "ledger", "api", "https://git.example.invalid/acme/api.git", ""); err != nil {
		t.Fatal(err)
	}
	foremans := backedBy(t, old.HQ())
	old.Sync(ctx) // records ledger's HQ origin, pushes it all

	fresh.Sync(ctx) // its start: Foreman's HQ has no origin yet
	if !fresh.Pristine(ctx) || len(fresh.Backups()) != 1 {
		t.Fatalf("a first start's HQ: pristine %v, backups %+v", fresh.Pristine(ctx), fresh.Backups())
	}
	before := headOf(t, fresh.HQ())
	for url, want := range map[string]string{
		filepath.Join(root, "nobody.git"):   "git clone",
		filepath.Join(root, "acme/api.git"): "doesn't look like Foreman's HQ",
		trunk:                               "no main branch",
	} {
		if err := fresh.Restore(ctx, url); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", url, err, want)
		}
	}
	if headOf(t, fresh.HQ()) != before || !fresh.Pristine(ctx) || exists(filepath.Join(fresh.Folder(), restoreDir)) {
		t.Fatal("a refused restore touched the HQ or left its clone")
	}

	if err := fresh.Restore(ctx, foremans); err != nil {
		t.Fatal(err)
	}
	if headOf(t, fresh.HQ()) != headOf(t, old.HQ()) || fresh.git.Origin(ctx, fresh.HQ()) != foremans {
		t.Error("not the old machine's HQ")
	}
	got := map[string]Project{}
	for _, p := range fresh.Projects() {
		got[p.ID] = p
	}
	if len(got) != 2 || got["ledger"].Here || got["relay"].Here || got["ledger"].Remote != hq || got["relay"].Remote != "" {
		t.Fatalf("projects %+v", got)
	}
	if fresh.Pristine(ctx) {
		t.Error("still pristine with projects")
	}
	if b := fresh.Backups(); len(b) != 0 {
		t.Errorf("the replaced HQ's warning stayed: %+v", b)
	}
	if err := fresh.Restore(ctx, foremans); !errors.Is(err, ErrExists) {
		t.Errorf("a second restore: %v", err)
	}

	// relay's HQ never had an origin: nowhere to bring it from.
	if _, err := fresh.BringHere(ctx, "relay"); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "no HQ origin") {
		t.Errorf("relay: %v", err)
	}
	if exists(filepath.Join(fresh.Folder(), "projects", "relay")) {
		t.Error("a refused bring left a folder")
	}
	p, err := fresh.BringHere(ctx, "ledger")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Here || headOf(t, p.Path) != headOf(t, hq) || len(p.Repos) != 1 || !strings.Contains(p.Repos[0].Problem, "not on this machine") {
		t.Errorf("brought %+v", p)
	}
	if _, err := fresh.BringHere(ctx, "ledger"); !errors.Is(err, ErrExists) {
		t.Errorf("brought twice: %v", err)
	}
	if p, err := fresh.CloneMissingRepo(ctx, "ledger", "api"); err != nil || !p.Repos[0].Ready() || len(p.Problems) != 0 {
		t.Errorf("its repo: %+v %v", p, err)
	}
	if _, err := fresh.BringHere(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
}

// A failed clone leaves no folder behind, the project's included.
func TestBringHereThatFailsLeavesNothing(t *testing.T) {
	h := loaded(t, nil)
	writeFile(t, h.HQ(projectsFile), "[ledger]\nhq = \""+filepath.Join(t.TempDir(), "gone.git")+"\"\n")
	reload(t, h)
	if _, err := h.BringHere(ctx, "ledger"); !errors.Is(err, ErrInvalid) {
		t.Errorf("%v", err)
	}
	if exists(filepath.Join(h.Folder(), "projects", "ledger")) {
		t.Error("left its folder")
	}
}

// Restore replaces only what the first start made: anything the CEO did
// since stays.
func TestPristine(t *testing.T) {
	for name, made := range map[string]func(t *testing.T) *Home{
		"a file not committed": func(t *testing.T) *Home {
			h := loaded(t, nil)
			writeFile(t, h.HQ("notes.md"), "mine\n")
			return h
		},
		"a commit": func(t *testing.T) *Home {
			h := loaded(t, nil)
			if _, err := h.SaveInstructions(ctx, "Answer in Catalan.\n", h.Instructions().Version); err != nil {
				t.Fatal(err)
			}
			return h
		},
		// Connected to a remote by the CEO: theirs, even with nothing in it.
		"an origin": func(t *testing.T) *Home {
			h := loaded(t, nil)
			gitOut(t, h.HQ(), "remote", "add", "origin", "git@example.com:ceo/hq.git")
			return h
		},
		// Put there by hand before the first start: in its first commit.
		"a team": func(t *testing.T) *Home {
			return loaded(t, func(root string) {
				writeFile(t, filepath.Join(hqOf(root), teamsDir, "atlas.toml"), "name = \"Atlas\"\n")
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := made(t)
			if h.Pristine(ctx) {
				t.Error("pristine")
			}
			before, _ := os.ReadDir(h.HQ())
			if err := h.Restore(ctx, h.HQ()); !errors.Is(err, ErrExists) {
				t.Errorf("restore: %v", err)
			}
			if after, _ := os.ReadDir(h.HQ()); len(after) != len(before) {
				t.Error("the HQ was touched")
			}
		})
	}
}
