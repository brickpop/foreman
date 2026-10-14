package fake

import (
	"testing"

	"github.com/brickpop/foreman/internal/harness"
)

func TestLaunchRunsTheProgramWithItsPrompt(t *testing.T) {
	l, err := Adapter{Program: "/bin/fakeharness"}.Launch(harness.Session{Sandbox: "/ws/ledger/.sandboxes/mara", Prompt: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	const prompt = "/ws/ledger/.sandboxes/mara/.harness/fake/PROMPT.md"
	if len(l.Command) != 1 || l.Command[0] != "/bin/fakeharness" || l.Env[0] != "FOREMAN_PROMPT_FILE="+prompt ||
		len(l.Files) != 1 || l.Files[0].Path != prompt || string(l.Files[0].Data) != "hi" {
		t.Errorf("launch %+v", l)
	}
}
