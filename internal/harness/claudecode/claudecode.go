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
	// GitConfig is the CEO's ~/.gitconfig, mounted read-only; "" when there is none.
	GitConfig string
}

var _ harness.Adapter = Adapter{}

// ConfigDir is the instance's private Claude Code config dir, in its harness
// folder (harness.Session.Dir): its settings, history and memory, and its
// session prompt.
func ConfigDir(dir string) string { return filepath.Join(dir, "claude-code") }

// Launch starts Claude Code with permission checks off (the container is the
// boundary), its config in the instance's private dir, and the shared login.
func (a Adapter) Launch(s harness.Session) (harness.Launch, error) {
	login := filepath.Join(s.Login, ".credentials.json")
	if _, err := os.Stat(login); err != nil {
		return harness.Launch{}, harness.ErrNotConnected
	}
	dir := ConfigDir(s.Dir)
	account, err := account(dir, s.Login, s.Sandbox)
	if err != nil {
		return harness.Launch{}, err
	}
	config := map[string]any{
		// No bypass-mode warning at startup.
		"skipDangerousModePermissionPrompt": true,
		// The classic renderer: the fullscreen one takes the mouse (no plain
		// copy in the terminal pane) and the alternate screen (no scrollback).
		// Set explicitly, it also skips the "Try the new fullscreen renderer?"
		// dialog, which would answer the first line typed in (a lost message).
		"tui": "default",
		// No Co-Authored-By trailer or "Generated with" line asked of the
		// model: a commit carries its message alone (the CEO, 2026-10-29).
		// commit() drops either line if it comes anyway.
		"attribution": map[string]string{"commit": "", "pr": ""},
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
	// A restart continues the latest conversation of this deployment, if it
	// has one; else the session starts fresh (with the opener, if any). The
	// config dir outlives deployments, so a fresh start writes down the
	// conversations already there: those are never continued.
	saved := conversations(dir)
	continuing := s.Continue && saved.Continues()
	if !s.Continue {
		files = append(files, saved.FreshStart())
	}
	if continuing {
		args = append(args, "--continue")
	}
	if s.Opener != "" && (!continuing || s.OpenerOnContinue) {
		// A prompt argument opens an interactive session with it as the first message
		// (only -p would make it one-shot).
		args = append(args, s.Opener)
	}
	l := harness.Launch{
		Command: args,
		Env:     append([]string{"CLAUDE_CONFIG_DIR=" + dir}, s.Link.Env...),
		// The login file alone, so a refresh by any instance reaches all of
		// them (a symlink breaks that; experiment B).
		Mounts: append([]runner.Mount{{Source: login, Target: filepath.Join(dir, ".credentials.json")}}, harness.GitConfig(a.GitConfig)...),
		Files: append([]harness.File{
			{Path: filepath.Join(dir, "CLAUDE.md"), Data: []byte(s.Prompt), Mode: 0o600},
			{Path: filepath.Join(dir, "settings.json"), Data: settings, Mode: 0o600},
			{Path: filepath.Join(dir, ".claude.json"), Data: account, Mode: 0o600},
		}, files...),
	}
	return l, nil
}

// account is the instance's .claude.json: its own (history and all) once it
// has one, else the login's account record; either way with onboarding done
// and the sandbox trusted, so no dialog stops the session.
func account(dir, login, sandbox string) ([]byte, error) {
	record := map[string]any{}
	data, err := os.ReadFile(filepath.Join(dir, ".claude.json"))
	if errors.Is(err, os.ErrNotExist) {
		data, err = os.ReadFile(filepath.Join(login, ".claude.json"))
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

// Asks is its AskUserQuestion's questions, each with its choices, or a
// permission prompt's message.
func (Adapter) Asks(event string, payload []byte) string {
	var p struct {
		ToolName         string          `json:"tool_name"`
		ToolInput        json.RawMessage `json:"tool_input"`
		NotificationType string          `json:"notification_type"`
		Message          string          `json:"message"`
	}
	if json.Unmarshal(payload, &p) != nil {
		return ""
	}
	switch {
	case event == "PreToolUse" && p.ToolName == "AskUserQuestion":
		return harness.Questions(p.ToolInput)
	case event == "Notification" && p.NotificationType == "permission_prompt":
		return p.Message
	}
	return ""
}

// Describe reads the same hooks for the CEO: the message it took, the tool it
// is about to run, its question, the end of its turn. A tool's result says
// nothing new (the line stays on the tool until the next one).
func (Adapter) Describe(event string, payload []byte) string {
	var p struct {
		Prompt           string         `json:"prompt"`
		ToolName         string         `json:"tool_name"`
		ToolInput        map[string]any `json:"tool_input"`
		NotificationType string         `json:"notification_type"`
		Cwd              string         `json:"cwd"`
	}
	_ = json.Unmarshal(payload, &p)
	switch event {
	case "SessionStart":
		return "Started"
	case "UserPromptSubmit":
		return "Working on “" + p.Prompt + "”"
	case "PreToolUse":
		return describeTool(p.ToolName, p.ToolInput, p.Cwd)
	case "Notification":
		if p.NotificationType == "permission_prompt" {
			return "Waiting for a permission"
		}
	case "Stop":
		return "Finished its turn"
	}
	return ""
}

// describeTool names a tool call by what it works on; paths inside the
// working directory are shown relative to it.
func describeTool(name string, input map[string]any, cwd string) string {
	arg := func(key string) string {
		s, _ := input[key].(string)
		return s
	}
	path := func(key string) string { return harness.Relative(arg(key), cwd) }
	switch name {
	case "Bash":
		return "Bash: " + arg("command")
	case "Read":
		return "Reading " + path("file_path")
	case "Edit", "MultiEdit":
		return "Editing " + path("file_path")
	case "Write":
		return "Writing " + path("file_path")
	case "NotebookEdit":
		return "Editing " + path("notebook_path")
	case "Grep":
		return "Searching for " + arg("pattern")
	case "Glob":
		return "Looking for " + arg("pattern")
	case "WebFetch":
		return "Fetching " + arg("url")
	case "WebSearch":
		return "Searching the web for " + arg("query")
	case "Task", "Agent":
		return "Subagent: " + arg("description")
	case "TodoWrite":
		return "Updating its to-do list"
	case "AskUserQuestion":
		return "Asked you a question"
	case "":
		return ""
	}
	if line, ok := harness.MCPTool(name); ok {
		return line
	}
	return name
}

// conversations are Claude Code's saved conversations in the config dir.
func conversations(dir string) harness.Conversations {
	return harness.Conversations{Dir: dir, Glob: filepath.Join("projects", "*", "*.jsonl")}
}

// Command is Claude Code's CLI.
func (Adapter) Command() string { return "claude" }

// Newline is Claude Code's documented quick escape, `\` then Enter, which works
// in any terminal; Shift+Enter needs a terminal set up for it.
func (Adapter) Newline() string { return "\\\r" }

// Connected reports whether the login folder holds a login.
func (Adapter) Connected(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".credentials.json"))
	return err == nil
}

// Login runs Claude Code on the login folder as its config dir, so the
// login lands there. It never asks to log in on its own: the CEO types /login
// (experiment B).
func (a Adapter) Login(dir string) (harness.Launch, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return harness.Launch{}, err
	}
	return harness.Launch{
		Command: []string{a.Command()},
		Env:     []string{"CLAUDE_CONFIG_DIR=" + dir},
		Mounts:  []runner.Mount{{Source: dir, Target: dir}},
	}, nil
}
