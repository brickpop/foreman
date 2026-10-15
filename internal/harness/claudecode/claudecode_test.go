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
	return Adapter{Credentials: creds}, harness.Session{Instance: "mara@ledger", Sandbox: sandbox, Prompt: "# You are Mara"}
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
	want := []runner.Mount{{Source: filepath.Join(a.Credentials, ".credentials.json"), Target: filepath.Join(dir, ".credentials.json")}}
	if !reflect.DeepEqual(l.Mounts, want) {
		t.Errorf("mounts %+v", l.Mounts)
	}
	if file(t, l, "CLAUDE.md")["text"] != "# You are Mara" {
		t.Error("the session prompt isn't CLAUDE.md")
	}
	if file(t, l, "settings.json")["skipDangerousModePermissionPrompt"] != true {
		t.Error("the bypass-mode warning isn't skipped")
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
	// The opener is the last argument, a prompt: interactive, never -p.
	s.Opener = "Your goal on ledger: -v ship it"
	if l, _ := a.Launch(s); !reflect.DeepEqual(l.Command, []string{"claude", "--dangerously-skip-permissions", "--continue", s.Opener}) {
		t.Errorf("with an opener: %v", l.Command)
	}
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
	if _, err := (Adapter{Credentials: t.TempDir()}).Launch(s); !errors.Is(err, harness.ErrNotConnected) {
		t.Errorf("without a login: %v", err)
	}
	write(t, filepath.Join(ConfigDir(s.Sandbox), ".claude.json"), "{broken")
	if _, err := a.Launch(s); err == nil {
		t.Error("a broken account record was accepted")
	}
}

func TestLoginAndConnected(t *testing.T) {
	creds := filepath.Join(t.TempDir(), "credentials", "claude-code")
	a := Adapter{Credentials: creds}
	if a.Connected() {
		t.Error("connected without a login")
	}
	l, err := a.Login()
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
	if !a.Connected() {
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
