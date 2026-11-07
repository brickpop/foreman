// Package codex is the Codex CLI harness adapter
// (wiki/product/architecture/harness-adapters.md#codex). Each instance gets
// its own Codex home (CODEX_HOME) in its harness folder: the session prompt
// as AGENTS.md, a config.toml with Foreman's MCP server and hooks, and
// auth.json linked to the profile's, which Codex follows and replaces in
// place, so a token refresh by any instance reaches all of them. Probed on
// Codex 0.167.0 (2026-11-06).
package codex

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/runner"
)

// Adapter launches Codex.
type Adapter struct {
	// GitConfig is the CEO's ~/.gitconfig, mounted read-only; "" when there is none.
	GitConfig string
}

var _ harness.Adapter = Adapter{}

// ConfigDir is the instance's private Codex home (CODEX_HOME), in its
// harness folder (harness.Session.Dir): its config, conversations and
// state, and its session prompt.
func ConfigDir(dir string) string { return filepath.Join(dir, "codex") }

// loginHome is Codex's home for the login, in the profile folder: apart
// from profile.toml, since agents mount it read-write.
func loginHome(profile string) string { return filepath.Join(profile, "codex") }

// authFile is Codex's login, in the login home; each instance's is a link
// to it.
const authFile = "auth.json"

// fileStore keeps the login in auth.json, never in the system keyring
// (Codex's "auto" uses the keyring where there is one: the Direct engine on a Mac).
const fileStore = `cli_auth_credentials_store="file"`

// Launch starts Codex with approvals and its own sandbox off (the container
// is the boundary), in the inline screen, with no background server, its
// home in the instance's private dir and the profile's login.
func (a Adapter) Launch(s harness.Session) (harness.Launch, error) {
	home := loginHome(s.Login)
	login := filepath.Join(home, authFile)
	if _, err := os.Stat(login); err != nil {
		return harness.Launch{}, harness.ErrNotConnected
	}
	dir := ConfigDir(s.Dir)
	config := map[string]any{
		"cli_auth_credentials_store":  "file",
		"check_for_update_on_startup": false, // no update notice in an agent's session
		// A trusted folder opens without the trust dialog, which would
		// answer the first line typed in (a lost message).
		"projects": map[string]any{s.Sandbox: map[string]any{"trust_level": "trusted"}},
	}
	args := []string{
		a.Command(),
		// Scrollback stays the pane's: a selection copies in the browser.
		"--no-alt-screen",
		// Nothing outlives the session (Codex otherwise starts a shared
		// background server that keeps its work going).
		"--no-daemon",
		"--dangerously-bypass-approvals-and-sandbox",
		// Its question tool, which Codex otherwise offers in plan mode only:
		// an agent asks in a dialog, as on Claude Code and Pi.
		"--enable", "default_mode_request_user_input",
	}
	if s.Link.Program != "" {
		// The team tools and the hooks, both through foreman-agent. Hooks
		// run only once trusted, which Codex asks in a dialog: these are
		// Foreman's own, so they are trusted for the session.
		env := map[string]string{}
		for _, kv := range s.Link.Env {
			k, v, _ := strings.Cut(kv, "=")
			env[k] = v
		}
		config["mcp_servers"] = map[string]any{"foreman": map[string]any{"command": s.Link.Program, "args": []string{"mcp"}, "env": env}}
		config["hooks"] = hooks(s.Link.Program)
		args = append(args, "--dangerously-bypass-hook-trust")
	}
	var settings bytes.Buffer
	if err := toml.NewEncoder(&settings).Encode(config); err != nil {
		return harness.Launch{}, err
	}
	files := []harness.File{
		{Path: filepath.Join(dir, "AGENTS.md"), Data: []byte(s.Prompt), Mode: 0o600},
		{Path: filepath.Join(dir, "config.toml"), Data: settings.Bytes(), Mode: 0o600},
		{Path: filepath.Join(dir, authFile), Link: login},
	}
	// As Claude Code's: a restart continues this deployment's conversation,
	// if it has one (the latest in the sandbox, which is all this home
	// holds), else starts fresh.
	saved := conversations(dir)
	continuing := s.Continue && saved.Continues()
	if !s.Continue {
		files = append(files, saved.FreshStart())
	}
	if continuing {
		args = append(args, "resume", "--last")
	}
	opener := s.Opener
	if continuing && !s.OpenerOnContinue {
		opener = ""
	}
	if opener == "" {
		opener = readyOpener
	}
	args = append(args, "--", opener) // an opener may start with "-"
	return harness.Launch{
		Command: args,
		Env:     append([]string{"CODEX_HOME=" + dir}, s.Link.Env...),
		// The login home at its own path, so the link resolves and a
		// refreshed login replaces the file there, for every instance.
		Mounts: append([]runner.Mount{{Source: home, Target: home}}, harness.GitConfig(a.GitConfig)...),
		Files:  files,
	}, nil
}

// readyOpener is the opener of a session that has none (every team agent's start: a deploy, a
// wake, a restart). Codex fires no hook until a turn begins (SessionStart runs inside the first
// turn, not at launch), so a session started with nothing to do would never say it is at its
// prompt, and its messages would wait for a turn nobody starts (the CEO, 2026-11-07). This short
// turn ends in a Stop: idle, and the messages follow.
const readyOpener = "Foreman started this session. Don't start on anything yet: reply only \"Ready.\" Your messages come next."

// hookEvents are the Codex hooks Foreman follows (Interrupt: the CEO's Esc,
// which ends a turn without a Stop).
var hookEvents = []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop", "Interrupt"}

// hooks is config.toml's [hooks]: every event runs `foreman-agent hook <event>`.
func hooks(program string) map[string]any {
	all := map[string]any{}
	for _, event := range hookEvents {
		group := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": program + " hook " + event}}}
		if event == "PreToolUse" || event == "PostToolUse" {
			group["matcher"] = "*"
		}
		all[event] = []any{group}
	}
	return all
}

// questionTool is Codex's tool to ask the user, in a dialog.
const questionTool = "request_user_input"

// Interpret reads a hook event: a question to the user is its question
// tool; a finished turn is Stop, or Interrupt when the CEO stopped it.
// SessionStart is busy, not idle as on Claude Code: Codex runs it at the
// start of the session's first turn, so idle there would deliver messages
// into that turn.
func (Adapter) Interpret(event string, payload []byte) harness.Activity {
	var p struct {
		ToolName string `json:"tool_name"`
	}
	_ = json.Unmarshal(payload, &p)
	switch event {
	case "Stop", "Interrupt":
		return harness.Idle
	case "SessionStart", "UserPromptSubmit", "PostToolUse":
		return harness.Busy
	case "PreToolUse":
		if p.ToolName == questionTool {
			return harness.Waiting
		}
		return harness.Busy
	}
	return harness.Unknown
}

// Asks is its question tool's questions, each with its choices.
func (Adapter) Asks(event string, payload []byte) string {
	var p struct {
		ToolName  string          `json:"tool_name"`
		ToolInput json.RawMessage `json:"tool_input"`
	}
	if event != "PreToolUse" || json.Unmarshal(payload, &p) != nil || p.ToolName != questionTool {
		return ""
	}
	return harness.Questions(p.ToolInput)
}

// Describe reads the same hooks for the CEO. A tool's result says nothing
// new (the line stays on the tool until the next one).
func (Adapter) Describe(event string, payload []byte) string {
	var p struct {
		Prompt    string         `json:"prompt"`
		ToolName  string         `json:"tool_name"`
		ToolInput map[string]any `json:"tool_input"`
		Cwd       string         `json:"cwd"`
	}
	_ = json.Unmarshal(payload, &p)
	switch event {
	case "SessionStart":
		return "Started"
	case "UserPromptSubmit":
		return "Working on “" + p.Prompt + "”"
	case "PreToolUse":
		return describeTool(p.ToolName, p.ToolInput, p.Cwd)
	case "Stop":
		return "Finished its turn"
	case "Interrupt":
		return "Stopped"
	}
	return ""
}

// describeTool names a tool call by what it works on. Codex reports its
// command tool as Bash; apply_patch is every edit, its input the patch.
func describeTool(name string, input map[string]any, cwd string) string {
	command, _ := input["command"].(string)
	switch name {
	case "Bash":
		return "Bash: " + command
	case "apply_patch":
		var paths []string
		for _, line := range strings.Split(command, "\n") {
			for _, op := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: "} {
				if path, ok := strings.CutPrefix(line, op); ok {
					paths = append(paths, harness.Relative(strings.TrimSpace(path), cwd))
				}
			}
		}
		return "Editing " + strings.Join(paths, ", ")
	case questionTool:
		return "Asked you a question"
	case "":
		return ""
	}
	if line, ok := harness.MCPTool(name); ok {
		return line
	}
	return name
}

// conversations are Codex's saved conversations in its home:
// sessions/<year>/<month>/<day>/rollout-….jsonl.
func conversations(dir string) harness.Conversations {
	return harness.Conversations{Dir: dir, Glob: filepath.Join("sessions", "*", "*", "*", "rollout-*.jsonl")}
}

// Command is Codex's CLI.
func (Adapter) Command() string { return "codex" }

// Newline is Ctrl+J, which Codex takes as a new line beside Shift+Enter,
// and which reaches it through any terminal.
func (Adapter) Newline() string { return "\n" }

// Connected reports whether the profile folder holds a login.
func (Adapter) Connected(dir string) bool {
	_, err := os.Stat(filepath.Join(loginHome(dir), authFile))
	return err == nil
}

// loginScript runs Codex's browser login ($1 and on) and relays its
// callback. Signing in sends the browser to http://localhost:1455/auth/callback,
// where Codex waits inside the container, which the CEO's browser can't
// reach; so the CEO pastes that address here and curl takes it in. With no
// container (the Direct engine) the browser reaches Codex itself, and the
// login ends without a paste.
const loginScript = `"$@" &
login=$!
paste_start=$'\e[200~' paste_end=$'\e[201~' # a terminal may wrap a paste in these
sleep 1
echo
echo "After you sign in, your browser goes to a page on localhost:1455 that doesn't load."
echo "Copy that page's address, paste it here and press Enter."
while kill -0 "$login" 2>/dev/null; do
  IFS= read -r -t 1 address || continue
  address=${address//"$paste_start"/}
  address=${address//"$paste_end"/}
  address=${address//[[:space:]]/}
  case $address in
  "") ;;
  http://localhost:1455/auth/callback\?* | http://127.0.0.1:1455/auth/callback\?*)
    curl -fsS -L -o /dev/null "http://127.0.0.1:1455/${address#*:1455/}" || echo "Codex didn't take that address: sign in again from the link above." ;;
  *) echo "That isn't the address to paste: it starts with http://localhost:1455/auth/callback?" ;;
  esac
done
wait "$login"
`

// Login runs Codex's browser login on the profile's login home, so the
// login lands there, and relays its callback (loginScript).
func (a Adapter) Login(dir string) (harness.Launch, error) {
	home := loginHome(dir)
	if err := os.MkdirAll(home, 0o700); err != nil {
		return harness.Launch{}, err
	}
	return harness.Launch{
		Command: []string{"bash", "-c", loginScript, "codex-login", a.Command(), "-c", fileStore, "login"},
		Env:     []string{"CODEX_HOME=" + home},
		Mounts:  []runner.Mount{{Source: home, Target: home}},
	}, nil
}
