package home

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// backedBy gives the HQ at dir a new bare origin holding what it has, and
// returns the origin.
func backedBy(t *testing.T, dir string) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), filepath.Base(filepath.Dir(dir))+".git")
	gitOut(t, dir, "clone", "--quiet", "--bare", dir, origin)
	gitOut(t, dir, "remote", "add", "origin", origin)
	return origin
}

// elsewhere commits file to origin's main as another machine would.
func elsewhere(t *testing.T, origin, file, content string) {
	t.Helper()
	clone := filepath.Join(t.TempDir(), "clone")
	gitOut(t, filepath.Dir(clone), "clone", "--quiet", origin, clone)
	writeFile(t, filepath.Join(clone, file), content)
	gitOut(t, clone, "add", file)
	gitOut(t, clone, "commit", "--quiet", "-m", "elsewhere: "+file)
	gitOut(t, clone, "push", "--quiet", "origin", "main")
}

func headOf(t *testing.T, dir string) string { return gitOut(t, dir, "rev-parse", "main") }

// A start brings in what the other machine pushed (a project renamed there,
// a note in its HQ), merges what both did, pushes, and records each HQ's
// origin.
func TestSyncBringsInAndPushes(t *testing.T) {
	h := loaded(t, nil)
	ledger, _, err := h.NewProject(ctx, "ledger", "")
	if err != nil {
		t.Fatal(err)
	}
	foremans, hq := backedBy(t, h.HQ()), backedBy(t, ledger.Path)
	elsewhere(t, foremans, projectsFile, "[ledger]\ndisplay_name = \"Renamed elsewhere\"\n")
	elsewhere(t, hq, "inbox/from-there.md", "there\n")
	// Here, meanwhile: a commit of the lead's in the project's HQ.
	writeFile(t, filepath.Join(ledger.Path, "inbox", "from-here.md"), "here\n")
	gitOut(t, ledger.Path, "add", "inbox")
	gitOut(t, ledger.Path, "commit", "--quiet", "-m", "here")

	h.Sync(ctx)
	if got := h.Backups(); len(got) != 0 {
		t.Errorf("backups %+v", got)
	}
	if p, _ := h.Project("ledger"); p.Name != "Renamed elsewhere" {
		t.Errorf("the record from elsewhere: %+v", p)
	}
	for _, f := range []string{"from-there.md", "from-here.md"} {
		if !exists(filepath.Join(ledger.Path, "inbox", f)) {
			t.Errorf("%s missing after the merge", f)
		}
	}
	if headOf(t, hq) != headOf(t, ledger.Path) || headOf(t, foremans) != headOf(t, h.HQ()) {
		t.Error("not pushed")
	}
	if got := readRecords(t, h)["ledger"]; got.HQ != hq || got.DisplayName != "Renamed elsewhere" {
		t.Errorf("record %+v", got)
	}
	if !strings.HasPrefix(gitOut(t, foremans, "log", "-1", "--format=%s", "main"), "Projects: HQ origins") {
		t.Error("the origins' commit wasn't pushed")
	}

	// On stop, what was committed since is pushed; what wasn't is left.
	if _, err := h.SetName(ctx, "ledger", "Ledger"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(ledger.Path, "draft.md"), "not committed\n")
	h.PushAll(ctx)
	if headOf(t, foremans) != headOf(t, h.HQ()) {
		t.Error("Foreman's HQ not pushed on stop")
	}
	if out := gitOut(t, hq, "ls-tree", "--name-only", "main"); strings.Contains(out, "draft.md") {
		t.Error("an uncommitted file was pushed")
	}
}

// What isn't backed up is listed, with what to do: an HQ with no origin
// (Foreman's, a project's, one made since), one whose sync failed. A
// conflict leaves the HQ as it was.
func TestSyncReportsWhatIsNotBackedUp(t *testing.T) {
	h := loaded(t, nil)
	ledger, _, err := h.NewProject(ctx, "ledger", "")
	if err != nil {
		t.Fatal(err)
	}
	relay, _, err := h.NewProject(ctx, "relay", "")
	if err != nil {
		t.Fatal(err)
	}
	hq := backedBy(t, relay.Path)
	elsewhere(t, hq, "index.md", "theirs\n")
	writeFile(t, filepath.Join(relay.Path, "index.md"), "ours\n")
	gitOut(t, relay.Path, "commit", "--quiet", "-am", "ours")
	ours := headOf(t, relay.Path)

	h.Sync(ctx)
	got := map[string]Backup{}
	for _, b := range h.Backups() {
		got[b.Project] = b
	}
	if b := got[""]; b.Path != h.HQ() || !strings.Contains(b.Problem, "no origin") || !strings.Contains(b.Problem, "remote add origin") {
		t.Errorf("Foreman's HQ: %+v", b)
	}
	if b := got["ledger"]; b.Path != ledger.Path || !strings.Contains(b.Problem, "no origin") {
		t.Errorf("ledger: %+v", b)
	}
	if b := got["relay"]; !strings.Contains(b.Problem, "not synced") || !strings.Contains(b.Problem, "index.md") {
		t.Errorf("relay: %+v", b)
	}
	if headOf(t, relay.Path) != ours {
		t.Error("the conflict moved relay's HQ")
	}
	if data, _ := os.ReadFile(filepath.Join(relay.Path, "index.md")); string(data) != "ours\n" {
		t.Errorf("index.md %q", data)
	}

	// Made since the sync: listed while it has no origin; a clone has one.
	if _, _, err := h.NewProject(ctx, "fresh", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.NewProject(ctx, "copy", hq); err != nil {
		t.Fatal(err)
	}
	got = map[string]Backup{}
	for _, b := range h.Backups() {
		got[b.Project] = b
	}
	if _, ok := got["fresh"]; !ok {
		t.Error("a new project with no origin isn't listed")
	}
	if b, ok := got["copy"]; ok {
		t.Errorf("a clone is listed: %+v", b)
	}
	// Removed: no longer listed.
	if err := h.RemoveProject(ctx, "ledger", true, false); err != nil {
		t.Fatal(err)
	}
	for _, b := range h.Backups() {
		if b.Project == "ledger" {
			t.Errorf("a removed project is listed: %+v", b)
		}
	}
}
