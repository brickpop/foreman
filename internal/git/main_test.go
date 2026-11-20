package git

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMain runs the package's tests with a clean git environment: an empty
// home, no global or system config, and none of the machine's GIT_* variables
// or SSH agent. So no test depends on how the machine running it set up git
// (hooks, fsmonitor, credential helpers, url rewrites, config passed through
// GIT_CONFIG_COUNT), only on what each test sets on purpose with t.Setenv.
// The machine's EMAIL goes too: git reads it for an identity, and command()
// gives its own when there is none.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "foreman-git-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "GIT_") || name == "SSH_AUTH_SOCK" || name == "EMAIL" {
			os.Unsetenv(name)
		}
	}
	for name, value := range map[string]string{
		"HOME":                home,
		"XDG_CONFIG_HOME":     home,
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
	} {
		os.Setenv(name, value)
	}
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

// testDeadline bounds each test's git commands: the whole package runs in
// seconds, so a test still waiting after this is stuck on a command.
const testDeadline = 30 * time.Second

var testContexts sync.Map // *testing.T → context.Context

// testCtx is the context a test's git commands run under, shared by the
// helpers it calls: a deadline of its own, so a hung command fails that test,
// naming the command, instead of holding the package until go test's timeout.
func testCtx(t *testing.T) context.Context {
	if c, ok := testContexts.Load(t); ok {
		return c.(context.Context)
	}
	c, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(func() {
		cancel()
		testContexts.Delete(t)
	})
	testContexts.Store(t, c)
	return c
}
