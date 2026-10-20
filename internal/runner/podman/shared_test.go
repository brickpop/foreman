package podman

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Two Foremans (two homes) share one image: one builds at a time, the
// other waits; both see the same state and log.
func TestOneBuildAtATimeAcrossForemans(t *testing.T) {
	dir := t.TempDir()
	here, there := sharedBuild{dir: dir, key: "harness"}, sharedBuild{dir: dir, key: "harness"}
	if state, log := there.state(); state != "" || log != "" || there.busy() {
		t.Errorf("nothing built yet: %q %q", state, log)
	}
	unlock, err := here.lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := here.begin(); err != nil {
		t.Fatal(err)
	}
	here.appendLog("STEP 1/3\n")
	if state, log := there.state(); state != "building" || log != "STEP 1/3\n" {
		t.Errorf("seen from the other: %q %q", state, log)
	}
	// The other waits for the lock, and gives up with its context.
	ctx, cancel := context.WithTimeout(context.Background(), 3*lockPoll)
	defer cancel()
	if _, err := there.lock(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a second build didn't wait: %v", err)
	}
	got := make(chan error, 1)
	go func() {
		release, err := there.lock(context.Background())
		if err == nil {
			release()
		}
		got <- err
	}()
	time.Sleep(lockPoll)
	if err := here.set("ready"); err != nil {
		t.Fatal(err)
	}
	unlock()
	select {
	case err := <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the waiting build never got the lock")
	}
	if state, _ := there.state(); state != "ready" || there.busy() {
		t.Errorf("after: %q, busy %v", state, there.busy())
	}
}

// A build whose Foreman stopped says building without holding the lock:
// every Foreman shows it failed, with its log.
func TestABuildCutShortIsFailed(t *testing.T) {
	b := sharedBuild{dir: t.TempDir(), key: "harness"}
	unlock, err := b.lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_ = b.begin()
	b.appendLog("STEP 2/3\n")
	unlock() // the Foreman exits mid-build
	state, log := b.state()
	if state != "failed" || !strings.HasPrefix(log, "STEP 2/3\n") || !strings.Contains(log, "exited") {
		t.Errorf("%q %q", state, log)
	}
	// The next build starts over: an empty log.
	unlock, _ = b.lock(context.Background())
	defer unlock()
	_ = b.begin()
	if _, log := b.state(); log != "" {
		t.Errorf("the old log stayed: %q", log)
	}
}

// Only the end of a long log is shown.
func TestTheLogsEnd(t *testing.T) {
	b := sharedBuild{dir: t.TempDir(), key: "harness"}
	unlock, _ := b.lock(context.Background())
	_ = b.begin()
	b.appendLog(strings.Repeat("a", maxLog))
	b.appendLog("the end\n")
	_ = b.set("failed")
	unlock()
	if state, log := b.state(); state != "failed" || len(log) != maxLog || !strings.HasSuffix(log, "the end\n") {
		t.Errorf("%q, %d bytes", state, len(log))
	}
}
