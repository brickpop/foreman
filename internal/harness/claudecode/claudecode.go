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
	"strings"

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
	config := map[string]any{
		// No bypass-mode warning at startup.
		"skipDangerousModePermissionPrompt": true,
	}
	args := []string{a.Command(), "--dangerously-skip-permissions"}
	var files []harness.File
	if s.Link.Program != "" {
		// Its hooks and the team tools, both through foreman-agent; only this
		// MCP server (--strict-mcp-config), whatever else is configured.
		config["hooks"] = hooks(s.Link.Program)
		env := map[string]string{}
		for _, kv := range s.Link.Env {
			k, v, _ := strings.Cut(kv, "=")
			env[k] = v
		}
		mcp, _ := json.MarshalIndent(map[string]any{"mcpServers": map[string]any{
			"foreman": map[string]any{"command": s.Link.Program, "args": []string{"mcp"}, "env": env},
		}}, "", "  ")
		files = append(files, harness.File{Path: filepath.Join(dir, "mcp.json"), Data: mcp, Mode: 0o600})
		args = append(args, "--mcp-config", filepath.Join(dir, "mcp.json"), "--strict-mcp-config")
	}
	settings, _ := json.MarshalIndent(config, "", "  ")
	if s.Continue && hasConversation(dir) {
		// The latest conversation in this config dir: always the instance's own.
		args = append(args, "--continue")
	}
	if s.Opener != "" {
		// A prompt argument opens an interactive session with it as the first message
		// (only -p would make it one-shot).
		args = append(args, s.Opener)
	}
	l := harness.Launch{
		Command: args,
		Env:     append([]string{"CLAUDE_CONFIG_DIR=" + dir}, s.Link.Env...),
		// The login file alone, so a refresh by any instance reaches all of
		// them (a symlink breaks that; experiment B).
		Mounts: []runner.Mount{{Source: login, Target: filepath.Join(dir, ".credentials.json")}},
		Files: append([]harness.File{
			{Path: filepath.Join(dir, "CLAUDE.md"), Data: []byte(s.Prompt), Mode: 0o600},
			{Path: filepath.Join(dir, "settings.json"), Data: settings, Mode: 0o600},
			{Path: filepath.Join(dir, ".claude.json"), Data: account, Mode: 0o600},
		}, files...),
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

// hookEvents are the Claude Code hooks Foreman follows (experiment D).
var hookEvents = []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Notification", "Stop"}

// hooks is settings.json's "hooks": every event runs `foreman-agent hook <event>`.
func hooks(program string) map[string]any {
	all := map[string]any{}
	for _, event := range hookEvents {
		entry := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": program + " hook " + event}}}
		if event == "PreToolUse" || event == "PostToolUse" {
			entry["matcher"] = "*"
		}
		all[event] = []any{entry}
	}
	return all
}

// Interpret reads a hook event (experiment D): a question to the user is its
// AskUserQuestion tool, then a permission_prompt notification; a finished turn
// is Stop.
func (Adapter) Interpret(event string, payload []byte) harness.Activity {
	var p struct {
		ToolName         string `json:"tool_name"`
		NotificationType string `json:"notification_type"`
	}
	_ = json.Unmarshal(payload, &p)
	switch event {
	case "SessionStart", "Stop":
		return harness.Idle
	case "UserPromptSubmit", "PostToolUse":
		return harness.Busy
	case "PreToolUse":
		if p.ToolName == "AskUserQuestion" {
			return harness.Waiting
		}
		return harness.Busy
	case "Notification":
		switch p.NotificationType {
		case "permission_prompt":
			return harness.Waiting
		case "idle_prompt":
			return harness.Idle
		}
	}
	return harness.Unknown
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
