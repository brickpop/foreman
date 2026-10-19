package claudecode

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/runner"
)

func setup(t *testing.T) (Adapter, harness.Session) {
	t.Helper()
	creds := t.TempDir()
	write(t, filepath.Join(creds, ".credentials.json"), `{"claudeAiOauth":{}}`)
	write(t, filepath.Join(creds, ".claude.json"), `{"oauthAccount":{"emailAddress":"x"},"projects":{"/elsewhere":{"hasTrustDialogAccepted":false}}}`)
	sandbox := filepath.Join(t.TempDir(), "ledger", ".sandboxes", "mara")
	return Adapter{}, harness.Session{Instance: "mara@ledger", Sandbox: sandbox, Prompt: "# You are Mara", Login: creds}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func file(t *testing.T, l harness.Launch, name string) map[string]any {
	t.Helper()
	for _, f := range l.Files {
		if filepath.Base(f.Path) == name {
			if f.Mode != 0o600 {
				t.Errorf("%s mode %o", name, f.Mode)
			}
			if name == "CLAUDE.md" {
				return map[string]any{"text": string(f.Data)}
			}
			var m map[string]any
			if err := json.Unmarshal(f.Data, &m); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			return m
		}
	}
	t.Fatalf("no %s among %v", name, l.Files)
	return nil
}

func TestLaunch(t *testing.T) {
	a, s := setup(t)
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	dir := ConfigDir(s.Sandbox)
	if !reflect.DeepEqual(l.Command, []string{"claude", "--dangerously-skip-permissions"}) {
		t.Errorf("command %v", l.Command)
	}
	if !reflect.DeepEqual(l.Env, []string{"CLAUDE_CONFIG_DIR=" + dir}) {
		t.Errorf("env %v", l.Env)
	}
	want := []runner.Mount{{Source: filepath.Join(s.Login, ".credentials.json"), Target: filepath.Join(dir, ".credentials.json")}}
	if !reflect.DeepEqual(l.Mounts, want) {
		t.Errorf("mounts %+v", l.Mounts)
	}
	if file(t, l, "CLAUDE.md")["text"] != "# You are Mara" {
		t.Error("the session prompt isn't CLAUDE.md")
	}
	if file(t, l, "settings.json")["skipDangerousModePermissionPrompt"] != true {
		t.Error("the bypass-mode warning isn't skipped")
	}
	if file(t, l, "settings.json")["tui"] != "default" {
		t.Error("the renderer isn't pinned to the classic one (the fullscreen dialog would eat a message)")
	}
	account := file(t, l, ".claude.json")
	projects := account["projects"].(map[string]any)
	if account["hasCompletedOnboarding"] != true || account["oauthAccount"] == nil ||
		projects[s.Sandbox].(map[string]any)["hasTrustDialogAccepted"] != true || projects["/elsewhere"] == nil {
		t.Errorf("account record %v", account)
	}
}

// A restart keeps the instance's own record (its history), with the flags set again.
func TestRelaunchKeepsTheInstancesOwnRecord(t *testing.T) {
	a, s := setup(t)
	write(t, filepath.Join(ConfigDir(s.Sandbox), ".claude.json"), `{"numStartups":7,"projects":{}}`)
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	account := file(t, l, ".claude.json")
	if account["numStartups"] != float64(7) || account["oauthAccount"] != nil || account["hasCompletedOnboarding"] != true {
		t.Errorf("relaunch record %v", account)
	}
}

func TestLaunchOptions(t *testing.T) {
	a, s := setup(t)
	gitconfig := filepath.Join(t.TempDir(), ".gitconfig")
	write(t, gitconfig, "[alias]\n  st = status\n")
	a.GitConfig = gitconfig
	s.Continue = true
	// Nothing to continue yet: no --continue.
	if l, _ := a.Launch(s); len(l.Command) != 2 {
		t.Errorf("continue without a conversation: %v", l.Command)
	}
	write(t, filepath.Join(ConfigDir(s.Sandbox), "projects", "-ws-ledger", "abc.jsonl"), "{}\n")
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(l.Command, []string{"claude", "--dangerously-skip-permissions", "--continue"}) {
		t.Errorf("command %v", l.Command)
	}
	// A session that continues doesn't get the opener again.
	s.Opener = "Your goal on ledger: -v ship it"
	if l, _ := a.Launch(s); !reflect.DeepEqual(l.Command, []string{"claude", "--dangerously-skip-permissions", "--continue"}) {
		t.Errorf("continuing, with an opener: %v", l.Command)
	}
	// The Chief of Staff's does: each visit says what it is for.
	s.OpenerOnContinue = true
	if l, _ := a.Launch(s); !reflect.DeepEqual(l.Command, []string{"claude", "--dangerously-skip-permissions", "--continue", s.Opener}) {
		t.Errorf("continuing, with an opener it gets: %v", l.Command)
	}
	s.OpenerOnContinue = false
	if last := l.Mounts[len(l.Mounts)-1]; last != (runner.Mount{Source: gitconfig, Target: "/home/node/.gitconfig", ReadOnly: true}) {
		t.Errorf("gitconfig mount %+v", last)
	}
	// A gitconfig that doesn't exist mounts nothing.
	a.GitConfig = filepath.Join(t.TempDir(), "missing")
	if l, _ := a.Launch(s); len(l.Mounts) != 1 {
		t.Errorf("mounts %+v", l.Mounts)
	}
}

func TestLaunchRefusals(t *testing.T) {
	a, s := setup(t)
	if _, err := a.Launch(harness.Session{Sandbox: s.Sandbox, Login: t.TempDir()}); !errors.Is(err, harness.ErrNotConnected) {
		t.Errorf("without a login: %v", err)
	}
	write(t, filepath.Join(ConfigDir(s.Sandbox), ".claude.json"), "{broken")
	if _, err := a.Launch(s); err == nil {
		t.Error("a broken account record was accepted")
	}
}

func TestLoginAndConnected(t *testing.T) {
	creds := filepath.Join(t.TempDir(), "credentials", "claude-code")
	var a Adapter
	if a.Connected(creds) {
		t.Error("connected without a login")
	}
	l, err := a.Login(creds)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(creds); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("the login folder: %v %v", info, err)
	}
	if !reflect.DeepEqual(l.Command, []string{"claude"}) || l.Env[0] != "CLAUDE_CONFIG_DIR="+creds ||
		!reflect.DeepEqual(l.Mounts, []runner.Mount{{Source: creds, Target: creds}}) {
		t.Errorf("login %+v", l)
	}
	write(t, filepath.Join(creds, ".credentials.json"), "{}")
	if !a.Connected(creds) {
		t.Error("not connected with a login")
	}
}

// With the agent link, Claude Code gets the foreman MCP server (and only it)
// and every followed hook runs foreman-agent; the link's env reaches both.
func TestLaunchWithTheAgentLink(t *testing.T) {
	a, s := setup(t)
	s.Opener = "Your goal on ledger: ship"
	s.Link = harness.Link{Program: "/opt/foreman/foreman-agent", Env: []string{"FOREMAN_AGENT_URL=http://h:1", "FOREMAN_AGENT_TOKEN=t0k"}}
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	dir := ConfigDir(s.Sandbox)
	want := []string{"claude", "--dangerously-skip-permissions", "--mcp-config", filepath.Join(dir, "mcp.json"), "--strict-mcp-config", s.Opener}
	if !reflect.DeepEqual(l.Command, want) {
		t.Errorf("command %v", l.Command)
	}
	if !reflect.DeepEqual(l.Env, []string{"CLAUDE_CONFIG_DIR=" + dir, "FOREMAN_AGENT_URL=http://h:1", "FOREMAN_AGENT_TOKEN=t0k"}) {
		t.Errorf("env %v", l.Env)
	}
	server := file(t, l, "mcp.json")["mcpServers"].(map[string]any)["foreman"].(map[string]any)
	if server["command"] != "/opt/foreman/foreman-agent" || !reflect.DeepEqual(server["args"], []any{"mcp"}) ||
		!reflect.DeepEqual(server["env"], map[string]any{"FOREMAN_AGENT_URL": "http://h:1", "FOREMAN_AGENT_TOKEN": "t0k"}) {
		t.Errorf("mcp server %v", server)
	}
	hooks := file(t, l, "settings.json")["hooks"].(map[string]any)
	if len(hooks) != len(hookEvents) {
		t.Errorf("hooks %v", hooks)
	}
	for _, event := range hookEvents {
		entry := hooks[event].([]any)[0].(map[string]any)
		command := entry["hooks"].([]any)[0].(map[string]any)["command"]
		if command != "/opt/foreman/foreman-agent hook "+event {
			t.Errorf("%s runs %v", event, command)
		}
		if _, matched := entry["matcher"]; matched != (event == "PreToolUse" || event == "PostToolUse") {
			t.Errorf("%s matcher %v", event, entry["matcher"])
		}
	}
	// Without the link: no MCP config, no hooks.
	s.Link = harness.Link{}
	l, _ = a.Launch(s)
	if len(l.Command) != 3 || file(t, l, "settings.json")["hooks"] != nil {
		t.Errorf("without the link: %v", l.Command)
	}
	for _, f := range l.Files {
		if filepath.Base(f.Path) == "mcp.json" {
			t.Error("an MCP config without the link")
		}
	}
}

// What experiment D saw each event mean.
func TestInterpret(t *testing.T) {
	var a Adapter
	for _, tc := range []struct {
		event, payload string
		want           harness.Activity
	}{
		{"SessionStart", `{}`, harness.Idle},
		{"UserPromptSubmit", `{"prompt":"hi"}`, harness.Busy},
		{"PreToolUse", `{"tool_name":"Bash"}`, harness.Busy},
		{"PreToolUse", `{"tool_name":"AskUserQuestion"}`, harness.Waiting},
		{"PostToolUse", `{"tool_name":"AskUserQuestion"}`, harness.Busy},
		{"Notification", `{"notification_type":"permission_prompt"}`, harness.Waiting},
		{"Notification", `{"notification_type":"idle_prompt"}`, harness.Idle},
		{"Notification", `{"notification_type":"auth_success"}`, harness.Unknown},
		{"Stop", `{"last_assistant_message":"done"}`, harness.Idle},
		{"PreCompact", `{}`, harness.Unknown},
		{"PreToolUse", `not json`, harness.Busy},
	} {
		if got := a.Interpret(tc.event, []byte(tc.payload)); got != tc.want {
			t.Errorf("%s %s: %q, want %q", tc.event, tc.payload, got, tc.want)
		}
	}
}

func TestDescribe(t *testing.T) {
	var a Adapter
	for _, tc := range []struct{ event, payload, want string }{
		{"SessionStart", `{"source":"startup"}`, "Started"},
		{"UserPromptSubmit", `{"prompt":"Message msg-1 from Tic"}`, "Working on “Message msg-1 from Tic”"},
		{"PreToolUse", `{"tool_name":"Bash","tool_input":{"command":"go test ./...\necho done"}}`, "Bash: go test ./...\necho done"},
		// Inside the working directory, relative; outside it, as given.
		{"PreToolUse", `{"tool_name":"Edit","cwd":"/ws/p/.sandboxes/tic","tool_input":{"file_path":"/ws/p/.sandboxes/tic/api/x.go"}}`, "Editing api/x.go"},
		{"PreToolUse", `{"tool_name":"Read","cwd":"/ws/p/.sandboxes/tic","tool_input":{"file_path":"/ws/p/.sandboxes/tico/x.go"}}`, "Reading /ws/p/.sandboxes/tico/x.go"},
		{"PreToolUse", `{"tool_name":"Write","tool_input":{"file_path":"notes.md"}}`, "Writing notes.md"},
		{"PreToolUse", `{"tool_name":"Grep","tool_input":{"pattern":"func main"}}`, "Searching for func main"},
		{"PreToolUse", `{"tool_name":"Task","tool_input":{"description":"Find the tests"}}`, "Subagent: Find the tests"},
		{"PreToolUse", `{"tool_name":"AskUserQuestion"}`, "Asked you a question"},
		{"PreToolUse", `{"tool_name":"mcp__foreman__commit","tool_input":{}}`, "Team tool: commit"},
		{"PreToolUse", `{"tool_name":"mcp__github__create_issue"}`, "github: create_issue"},
		{"PreToolUse", `{"tool_name":"SomethingNew","tool_input":{"x":1}}`, "SomethingNew"},
		// A field of another type is left out, never a crash.
		{"PreToolUse", `{"tool_name":"Bash","tool_input":{"command":42}}`, "Bash: "},
		{"PreToolUse", `not json`, ""},
		{"PostToolUse", `{"tool_name":"Bash"}`, ""},
		{"Notification", `{"notification_type":"permission_prompt"}`, "Waiting for a permission"},
		{"Notification", `{"notification_type":"idle_prompt"}`, ""},
		{"Stop", `{}`, "Finished its turn"},
		{"PreCompact", `{}`, ""},
	} {
		if got := a.Describe(tc.event, []byte(tc.payload)); got != tc.want {
			t.Errorf("%s %s: %q, want %q", tc.event, tc.payload, got, tc.want)
		}
	}
}

// writeFiles writes a launch's files, as the session runner does.
func writeFiles(t *testing.T, l harness.Launch) {
	t.Helper()
	for _, f := range l.Files {
		write(t, f.Path, string(f.Data))
	}
}

// The config dir outlives deployments: a restart continues only a
// conversation of this deployment, and a session that starts fresh gets the
// opener (the lead's goal), as an argument: interactive, never -p.
func TestARestartContinuesOnlyThisDeploymentsConversation(t *testing.T) {
	a, s := setup(t)
	dir := ConfigDir(s.Sandbox)
	write(t, filepath.Join(dir, "projects", "-ws-ledger", "last-deployment.jsonl"), "{}\n")
	s.Opener = "Your goal on ledger: ship it"
	// The deployment's first start: fresh, with the goal, whatever is there.
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(l.Command, []string{"claude", "--dangerously-skip-permissions", s.Opener}) {
		t.Errorf("first start %v", l.Command)
	}
	writeFiles(t, l)
	// It exited before saving a conversation: the restart is fresh again,
	// with the goal, and never picks up the last deployment's conversation.
	s.Continue = true
	if l, _ := a.Launch(s); !reflect.DeepEqual(l.Command, []string{"claude", "--dangerously-skip-permissions", s.Opener}) {
		t.Errorf("restart with nothing of this deployment: %v", l.Command)
	}
	write(t, filepath.Join(dir, "projects", "-ws-ledger", "this-deployment.jsonl"), "{}\n")
	restart, _ := a.Launch(s)
	if !reflect.DeepEqual(restart.Command, []string{"claude", "--dangerously-skip-permissions", "--continue"}) {
		t.Errorf("restart with a conversation: %v", restart.Command)
	}
	// A restart doesn't rewrite the list of what was there before.
	for _, f := range restart.Files {
		if filepath.Base(f.Path) == "foreman-fresh-start" {
			t.Error("a restart rewrote the fresh-start list")
		}
	}
}
