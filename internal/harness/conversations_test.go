package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOnlyThisDeploymentsConversationContinues(t *testing.T) {
	dir := t.TempDir()
	save := func(name string) {
		t.Helper()
		path := filepath.Join(dir, "sessions", "-work-", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	c := Conversations{Dir: dir, Glob: "sessions/*/*.jsonl"}
	if c.Continues() {
		t.Error("nothing saved, yet it continues")
	}
	save("last-deployment.jsonl")
	// Before any fresh start, whatever is there counts (a workspace from before the list).
	if !c.Continues() {
		t.Error("a conversation without a fresh-start list doesn't continue")
	}
	if err := WriteFiles([]File{c.FreshStart()}); err != nil {
		t.Fatal(err)
	}
	if c.Continues() {
		t.Error("the last deployment's conversation continues")
	}
	save("this-deployment.jsonl")
	if !c.Continues() {
		t.Error("this deployment's conversation doesn't continue")
	}
	// Files elsewhere in the dir aren't conversations.
	other := Conversations{Dir: dir, Glob: "projects/*/*.jsonl"}
	if other.Continues() {
		t.Error("another glob's files count")
	}
}
