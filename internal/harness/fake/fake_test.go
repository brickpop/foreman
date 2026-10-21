package fake

import (
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
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

// Scripts written and run while other goroutines start processes all run:
// none is busy being written (without the lock, about 1 in 16 fails).
func TestWriteProgramWhileOthersStartProcesses(t *testing.T) {
	var failed atomic.Int64
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				program := filepath.Join(t.TempDir(), "harness.sh")
				if err := WriteProgram(program, "true"); err != nil {
					t.Error(err)
					return
				}
				if err := exec.Command(program).Run(); err != nil {
					failed.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	if n := failed.Load(); n > 0 {
		t.Errorf("%d of 200 scripts didn't run", n)
	}
}
