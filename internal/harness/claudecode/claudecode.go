// Package claudecode is the Claude Code harness adapter
// (wiki/product/architecture/harness-adapters.md). What it sets up was proven
// in the Prove epic: the session prompt as CLAUDE.md in a private config dir
// (E), the shared login file bind-mounted into it (B), and a session that
// starts straight at its prompt with permission checks off (D).
package claudecode

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/runner"
)

// Adapter launches Claude Code.
type Adapter struct {
	// Credentials is the harness's shared login folder on the host
	// (.foreman/credentials/claude-code), where Connect logs in.
	Credentials string
	// GitConfig is the CEO's ~/.gitconfig, mounted read-only; "" when there is none.
	GitConfig string
}

var _ harness.Adapter = Adapter{}

// ConfigDir is the instance's private Claude Code config dir, inside its
// sandbox: its settings, history and memory, and its session prompt.
func ConfigDir(sandbox string) string { return filepath.Join(sandbox, ".harness", "claude-code") }

// The image's user (images/Containerfile: node).
const containerHome = "/home/node"

// Launch starts Claude Code with permission checks off (the container is the
// boundary), its config in the instance's private dir, and the shared login.
func (a Adapter) Launch(s harness.Session) (harness.Launch, error) {
	login := filepath.Join(a.Credentials, ".credentials.json")
	if _, err := os.Stat(login); err != nil {
		return harness.Launch{}, harness.ErrNotConnected
	}
	dir := ConfigDir(s.Sandbox)
	account, err := a.account(dir, s.Sandbox)
	if err != nil {
		return harness.Launch{}, err
	}
	settings, _ := json.MarshalIndent(map[string]any{
		// No bypass-mode warning at startup.
		"skipDangerousModePermissionPrompt": true,
	}, "", "  ")

	command := s.Command
	if command == "" {
		command = a.Command()
	}
	args := []string{command, "--dangerously-skip-permissions"}
	if s.Continue && hasConversation(dir) {
		// The latest conversation in this config dir: always the instance's own.
		args = append(args, "--continue")
	}
	l := harness.Launch{
		Command: args,
		Env:     []string{"CLAUDE_CONFIG_DIR=" + dir},
		// The login file alone, so a refresh by any instance reaches all of
		// them (a symlink breaks that; experiment B).
		Mounts: []runner.Mount{{Source: login, Target: filepath.Join(dir, ".credentials.json")}},
		Files: []harness.File{
			{Path: filepath.Join(dir, "CLAUDE.md"), Data: []byte(s.Prompt), Mode: 0o600},
			{Path: filepath.Join(dir, "settings.json"), Data: settings, Mode: 0o600},
			{Path: filepath.Join(dir, ".claude.json"), Data: account, Mode: 0o600},
		},
	}
	if a.GitConfig != "" {
		if _, err := os.Stat(a.GitConfig); err == nil {
			l.Mounts = append(l.Mounts, runner.Mount{Source: a.GitConfig, Target: containerHome + "/.gitconfig", ReadOnly: true})
		}
	}
	return l, nil
}

// account is the instance's .claude.json: its own (history and all) once it
// has one, else the login's account record; either way with onboarding done
// and the sandbox trusted, so no dialog stops the session.
func (a Adapter) account(dir, sandbox string) ([]byte, error) {
	record := map[string]any{}
	data, err := os.ReadFile(filepath.Join(dir, ".claude.json"))
	if errors.Is(err, os.ErrNotExist) {
		data, err = os.ReadFile(filepath.Join(a.Credentials, ".claude.json"))
		if errors.Is(err, os.ErrNotExist) {
			data, err = []byte("{}"), nil
		}
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("claude code account record: %w", err)
	}
	record["hasCompletedOnboarding"] = true
	projects, _ := record["projects"].(map[string]any)
	if projects == nil {
		projects = map[string]any{}
	}
	project, _ := projects[sandbox].(map[string]any)
	if project == nil {
		project = map[string]any{}
	}
	project["hasTrustDialogAccepted"] = true
	projects[sandbox] = project
	record["projects"] = projects
	return json.MarshalIndent(record, "", "  ")
}

// hasConversation reports whether the config dir holds a conversation to
// continue: `--continue` fails without one (a session that crashed at once).
func hasConversation(dir string) bool {
	found, _ := filepath.Glob(filepath.Join(dir, "projects", "*", "*.jsonl"))
	return len(found) > 0
}

// Command is Claude Code's CLI.
func (Adapter) Command() string { return "claude" }

// Newline is Claude Code's documented quick escape, `\` then Enter, which works
// in any terminal; Shift+Enter needs a terminal set up for it.
func (Adapter) Newline() string { return "\\\r" }

// Connected reports whether the shared login exists.
func (a Adapter) Connected() bool {
	_, err := os.Stat(filepath.Join(a.Credentials, ".credentials.json"))
	return err == nil
}

// Login runs Claude Code on the shared login folder as its config dir, so the
// login lands there. It never asks to log in on its own: the CEO types /login
// (experiment B).
func (a Adapter) Login() (harness.Launch, error) {
	if err := os.MkdirAll(a.Credentials, 0o700); err != nil {
		return harness.Launch{}, err
	}
	return harness.Launch{
		Command: []string{a.Command()},
		Env:     []string{"CLAUDE_CONFIG_DIR=" + a.Credentials},
		Mounts:  []runner.Mount{{Source: a.Credentials, Target: a.Credentials}},
	}, nil
}
