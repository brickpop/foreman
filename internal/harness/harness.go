// Package harness is everything harness-specific (Claude Code, the fake
// harness in tests, and later Pi, Hermes, Codex, custom commands) behind one
// interface (wiki/product/architecture/building-blocks.md#3-harness-adapter-harness).
// Deliver and Diagnose join with Messaging.
package harness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brickpop/foreman/internal/runner"
)

// Session is what an adapter needs to launch one instance.
type Session struct {
	Instance string // e.g. mara@ledger
	// Sandbox is the instance's folder on the host (its agent's sandbox),
	// its working directory, mounted at the same path.
	Sandbox string
	// Dir is the instance's private harness folder on the host, in Foreman's
	// home, mounted at the same path: the adapter keeps the harness's config
	// and conversations in its own folder there (Dir/<harness>).
	Dir    string
	Prompt string // the composed session prompt
	// Login is the folder of the profile it runs on, on the host: the
	// harness's login, shared by every instance on it (internal/profiles).
	Login string
	// Continue: a restart within the deployment, which picks up the
	// instance's conversation of this deployment if the harness kept one.
	// False: the deployment's first start, always fresh.
	Continue bool
	// Opener is the first message of a session that starts fresh (not one
	// that continues a conversation), typed in as if by the CEO; the session
	// stays interactive. "" for none.
	Opener string
	// OpenerOnContinue gives the opener to a session that continues its
	// conversation too: the Chief of Staff's, whose every visit says what it
	// is for.
	OpenerOnContinue bool
	// Link is how the harness reaches Foreman: the team tools and the hooks
	// both go through foreman-agent. Zero when foreman-agent isn't built.
	Link Link
}

// Link is foreman-agent as the instance sees it.
type Link struct {
	Program string   // its path where the harness runs (in the container)
	Env     []string // FOREMAN_AGENT_URL and FOREMAN_AGENT_TOKEN
}

// Activity is what a hook event says the harness is doing
// (wiki/product/architecture/harness-adapters.md#idle-and-waiting-detection).
type Activity string

const (
	Unknown Activity = ""        // the event says nothing about it
	Busy    Activity = "busy"    // working on a turn
	Idle    Activity = "idle"    // at its prompt, finished
	Waiting Activity = "waiting" // waiting for the user: a question or a dialog
)

// Launch is how to start a harness: what the instance manager adds to the
// process spec (alongside the worktrees and repos), and the files to write first.
type Launch struct {
	Command []string
	Env     []string
	Mounts  []runner.Mount
	Files   []File
}

// File is written on the host before the process starts.
type File struct {
	Path string
	Data []byte
	Mode os.FileMode
}

// ErrNotConnected means the harness has no login yet: Settings → Agents → Profiles → Connect.
var ErrNotConnected = errors.New("harness: not connected (Settings → Agents → Profiles → Connect)")

// Adapter is one harness's adapter (claudecode.Adapter, fake.Adapter, …), as
// runner.Runner is one engine (direct.Runner, podman.Runner).
type Adapter interface {
	// Command is what it runs when the settings name no command.
	Command() string
	// Newline is the keystroke that starts a new line in its input without
	// sending it (the CEO's Shift+Enter).
	Newline() string
	Launch(Session) (Launch, error)
	// Interpret reads one of its hook events (the name and the JSON payload).
	Interpret(event string, payload []byte) Activity
	// Describe says, for the CEO, what a hook event shows it doing ("Bash: go
	// test ./...", "Finished its turn"); "" when the event says nothing new.
	Describe(event string, payload []byte) string
	// Login is how to run the harness's own login (Settings → Agents → Profiles →
	// Connect) into the login folder dir, in a terminal the CEO types into.
	Login(dir string) (Launch, error)
	// Connected reports whether the login folder dir holds a login.
	Connected(dir string) bool
}

// WriteFiles writes a launch's files, creating their folders.
func WriteFiles(files []File) error {
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(f.Path, f.Data, f.Mode); err != nil {
			return err
		}
	}
	return nil
}

// containerHome is the agent image's user's home (images/Containerfile: node).
const containerHome = "/home/node"

// GitConfig mounts the CEO's ~/.gitconfig read-only in the container, so
// agents commit as the CEO set git up; none when path is "" or missing.
func GitConfig(path string) []runner.Mount {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return []runner.Mount{{Source: path, Target: containerHome + "/.gitconfig", ReadOnly: true}}
}

// Mover is an adapter whose harness files its conversations by working
// folder (Claude Code, Pi): it carries an instance's over when its sandbox
// moves, so the agent continues where it was. Legacy: only a project's
// migration to its workspace moves a sandbox
// (wiki/backlog/remove-workspace-migration.md).
type Mover interface {
	// MoveConversations renames, in the harness folder dir, the conversations
	// of working folder from to those of to; none is fine.
	MoveConversations(dir, from, to string) error
}

// MoveConversations carries an instance's conversations over in each
// adapter's harness that files them by working folder; on a failure, those
// carried already go back.
func MoveConversations(adapters map[string]Adapter, dir, from, to string) error {
	var moved []Mover
	for _, a := range adapters {
		m, ok := a.(Mover)
		if !ok {
			continue
		}
		if err := m.MoveConversations(dir, from, to); err != nil {
			for _, back := range moved {
				_ = back.MoveConversations(dir, to, from)
			}
			return err
		}
		moved = append(moved, m)
	}
	return nil
}

// Rename renames a harness's folder of conversations, from to to: none is
// fine, a folder at to already is refused (it would merge two agents').
func Rename(from, to string) error {
	if _, err := os.Stat(from); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if _, err := os.Stat(to); err == nil {
		return fmt.Errorf("%s is there already", to)
	}
	return os.Rename(from, to)
}
