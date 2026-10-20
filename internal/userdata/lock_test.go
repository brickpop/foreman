package userdata

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTheLockKeepsOneServer(t *testing.T) {
	home := t.TempDir()
	first, err := TakeLock(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Announce("http://127.0.0.1:8888/?token=abc"); err != nil {
		t.Fatal(err)
	}
	_, err = TakeLock(home)
	var running *RunningError
	if !errors.As(err, &running) || running.URL != "http://127.0.0.1:8888/?token=abc" {
		t.Fatalf("a second lock: %v %+v", err, running)
	}
	if info, err := os.Stat(filepath.Join(home, "server.lock")); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the lock holds the token: %v %v", info, err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	again, err := TakeLock(home)
	if err != nil {
		t.Fatalf("after a release: %v", err)
	}
	defer again.Release()
	// A shorter URL leaves nothing of the longer one behind.
	if err := again.Announce("http://x/"); err != nil {
		t.Fatal(err)
	}
	if _, err := TakeLock(home); !errors.As(err, &running) || running.URL != "http://x/" {
		t.Errorf("%+v", running)
	}
}

// A Foreman killed outright leaves no lock behind.
func TestALockDiesWithItsProcess(t *testing.T) {
	if os.Getenv("FOREMAN_HOLD_LOCK") != "" {
		if _, err := TakeLock(os.Getenv("FOREMAN_HOLD_LOCK")); err != nil {
			os.Exit(2)
		}
		os.Stdout.WriteString("held\n")
		time.Sleep(time.Minute)
		return
	}
	home := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestALockDiesWithItsProcess$")
	cmd.Env = append(os.Environ(), "FOREMAN_HOLD_LOCK="+home)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	if _, err := out.Read(buf); err != nil || !strings.HasPrefix(string(buf), "held") {
		t.Fatalf("the holder: %q %v", buf, err)
	}
	var running *RunningError
	if _, err := TakeLock(home); !errors.As(err, &running) {
		t.Fatalf("while held elsewhere: %v", err)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	l, err := TakeLock(home)
	if err != nil {
		t.Fatalf("after the holder was killed: %v", err)
	}
	_ = l.Release()
}
