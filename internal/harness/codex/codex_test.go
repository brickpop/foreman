package codex

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/runner"
)

func setup(t *testing.T) (Adapter, harness.Session) {
	t.Helper()
	login := t.TempDir()
	write(t, filepath.Join(login, "codex", "auth.json"), `{"tokens":{}}`)
	root := t.TempDir()
	sandbox, dir := filepath.Join(root, "ledger", "agents", "mara"), filepath.Join(root, ".foreman", "harness", "ledger", "mara")
	return Adapter{}, harness.Session{Instance: "mara@ledger", Sandbox: sandbox, Dir: dir, Prompt: "# You are Mara", Login: login}
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

func find(t *testing.T, l harness.Launch, name string) harness.File {
	t.Helper()
	for _, f := range l.Files {
		if filepath.Base(f.Path) == name {
			return f
		}
	}
	t.Fatalf("no %s among %v", name, l.Files)
	return harness.File{}
}

// config is the launch's config.toml, as Codex reads it.
func config(t *testing.T, l harness.Launch) map[string]any {
	t.Helper()
	f := find(t, l, "config.toml")
	if f.Mode != 0o600 {
		t.Errorf("config.toml mode %o", f.Mode)
	}
	var m map[string]any
	if _, err := toml.Decode(string(f.Data), &m); err != nil {
		t.Fatalf("config.toml: %v\n%s", err, f.Data)
	}
	return m
}

var base = []string{"codex", "--no-alt-screen", "--no-daemon", "--dangerously-bypass-approvals-and-sandbox", "--enable", "default_mode_request_user_input"}

func TestLaunch(t *testing.T) {
	a, s := setup(t)
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	dir := ConfigDir(s.Dir)
	// No opener given: the ready one, so the session says it is at its prompt.
	if !reflect.DeepEqual(l.Command, append(slices.Clone(base), "--", readyOpener)) {
		t.Errorf("command %v", l.Command)
	}
	if !reflect.DeepEqual(l.Env, []string{"CODEX_HOME=" + dir}) {
		t.Errorf("env %v", l.Env)
	}
	// The login home, at its own path (a refresh replaces auth.json there),
	// and not the rest of the profile folder (profile.toml).
	home := filepath.Join(s.Login, "codex")
	if !reflect.DeepEqual(l.Mounts, []runner.Mount{{Source: home, Target: home}}) {
		t.Errorf("mounts %+v", l.Mounts)
	}
	if f := find(t, l, "AGENTS.md"); string(f.Data) != "# You are Mara" || f.Path != filepath.Join(dir, "AGENTS.md") || f.Mode != 0o600 {
		t.Errorf("the session prompt isn't AGENTS.md in its home: %+v", f)
	}
	if f := find(t, l, "auth.json"); f.Link != filepath.Join(home, "auth.json") || f.Path != filepath.Join(dir, "auth.json") {
		t.Errorf("auth.json isn't a link to the profile's: %+v", f)
	}
	c := config(t, l)
	if c["cli_auth_credentials_store"] != "file" || c["check_for_update_on_startup"] != false {
		t.Errorf("config %v", c)
	}
	if trust := c["projects"].(map[string]any)[s.Sandbox]; !reflect.DeepEqual(trust, map[string]any{"trust_level": "trusted"}) {
		t.Errorf("the sandbox isn't trusted (the dialog would eat a message): %v", c["projects"])
	}
	if c["hooks"] != nil || c["mcp_servers"] != nil {
		t.Errorf("hooks or MCP without foreman-agent: %v", c)
	}
}

// With foreman-agent: the team tools and every hook through it, the hooks
// trusted for the session (Codex would ask in a dialog).
func TestLaunchLinksForeman(t *testing.T) {
	a, s := setup(t)
	s.Link = harness.Link{Program: "/opt/foreman-agent", Env: []string{"FOREMAN_AGENT_URL=http://host:7000", "FOREMAN_AGENT_TOKEN=t=1"}}
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(l.Command, "--dangerously-bypass-hook-trust") {
		t.Errorf("hooks untrusted: %v", l.Command)
	}
	if !reflect.DeepEqual(l.Env[1:], s.Link.Env) {
		t.Errorf("env %v", l.Env)
	}
	c := config(t, l)
	server := c["mcp_servers"].(map[string]any)["foreman"]
	want := map[string]any{"command": "/opt/foreman-agent", "args": []any{"mcp"}, "env": map[string]any{"FOREMAN_AGENT_URL": "http://host:7000", "FOREMAN_AGENT_TOKEN": "t=1"}}
	if !reflect.DeepEqual(server, want) {
		t.Errorf("mcp server %v", server)
	}
	hooks := c["hooks"].(map[string]any)
	if len(hooks) != len(hookEvents) {
		t.Errorf("hooks %v", hooks)
	}
	for _, event := range hookEvents {
		group := hooks[event].([]map[string]any)[0]
		handler := group["hooks"].([]map[string]any)[0]
		if handler["type"] != "command" || handler["command"] != "/opt/foreman-agent hook "+event {
			t.Errorf("%s: %v", event, group)
		}
		if tool := event == "PreToolUse" || event == "PostToolUse"; tool != (group["matcher"] == "*") {
			t.Errorf("%s matcher %v", event, group["matcher"])
		}
	}
}

func TestLaunchContinuesAndOpens(t *testing.T) {
	a, s := setup(t)
	gitconfig := filepath.Join(t.TempDir(), ".gitconfig")
	write(t, gitconfig, "[user]\n  name = CEO\n")
	a.GitConfig = gitconfig
	s.Continue = true
	s.Opener = "-v ship it"
	// Nothing to continue yet: a fresh start, with the opener after --.
	if l, _ := a.Launch(s); !reflect.DeepEqual(l.Command, append(slices.Clone(base), "--", s.Opener)) {
		t.Errorf("continue without a conversation: %v", l.Command)
	}
	write(t, filepath.Join(ConfigDir(s.Dir), "sessions", "2026", "11", "06", "rollout-2026-11-06T18-12-43-abc.jsonl"), "{}\n")
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	// Its opener was for the fresh start: continuing, it gets the ready one.
	if !reflect.DeepEqual(l.Command, append(slices.Clone(base), "resume", "--last", "--", readyOpener)) {
		t.Errorf("continuing: %v", l.Command)
	}
	s.OpenerOnContinue = true
	if l, _ := a.Launch(s); !reflect.DeepEqual(l.Command, append(slices.Clone(base), "resume", "--last", "--", s.Opener)) {
		t.Errorf("continuing, with an opener it gets: %v", l.Command)
	}
	if last := l.Mounts[len(l.Mounts)-1]; last != (runner.Mount{Source: gitconfig, Target: "/home/node/.gitconfig", ReadOnly: true}) {
		t.Errorf("gitconfig mount %+v", last)
	}
}

// A fresh start writes down the conversations there, which a later
// continue never picks up.
func TestFreshStartNeverContinuesAnOlderConversation(t *testing.T) {
	a, s := setup(t)
	write(t, filepath.Join(ConfigDir(s.Dir), "sessions", "2026", "10", "01", "rollout-old.jsonl"), "{}\n")
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := harness.WriteFiles(l.Files); err != nil {
		t.Fatal(err)
	}
	s.Continue = true
	if l, _ := a.Launch(s); slices.Contains(l.Command, "resume") {
		t.Errorf("continued the conversation from before the fresh start: %v", l.Command)
	}
}

func TestLaunchWithoutALogin(t *testing.T) {
	a, s := setup(t)
	s.Login = t.TempDir()
	if _, err := a.Launch(s); !errors.Is(err, harness.ErrNotConnected) {
		t.Errorf("err %v", err)
	}
}

// The launch files, written, give the instance the profile's login, and a
// refreshed one too.
func TestTheLoginIsSharedThroughTheLink(t *testing.T) {
	a, s := setup(t)
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := harness.WriteFiles(l.Files); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(s.Login, "codex", "auth.json"), `{"tokens":{"refreshed":true}}`)
	if data, _ := os.ReadFile(filepath.Join(ConfigDir(s.Dir), "auth.json")); string(data) != `{"tokens":{"refreshed":true}}` {
		t.Errorf("the instance reads %s", data)
	}
}

func TestConnected(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "auth.json"), "{}") // not where the login lands
	if (Adapter{}).Connected(dir) {
		t.Error("connected without codex/auth.json")
	}
	write(t, filepath.Join(dir, "codex", "auth.json"), "{}")
	if !(Adapter{}).Connected(dir) {
		t.Error("not connected with auth.json")
	}
}

func TestLogin(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profiles", "work")
	l, err := Adapter{}.Login(dir)
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(dir, "codex")
	if info, err := os.Stat(home); err != nil || !info.IsDir() {
		t.Errorf("the login home isn't made: %v", err)
	}
	want := []string{"bash", "-c", loginScript, "codex-login", "codex", "-c", `cli_auth_credentials_store="file"`, "login"}
	if !reflect.DeepEqual(l.Command, want) {
		t.Errorf("command %q", l.Command)
	}
	if !reflect.DeepEqual(l.Env, []string{"CODEX_HOME=" + home}) || !reflect.DeepEqual(l.Mounts, []runner.Mount{{Source: home, Target: home}}) {
		t.Errorf("env %v mounts %v", l.Env, l.Mounts)
	}
}

// Payloads as Codex 0.167.0 sends them (probed 2026-11-06), cut to what's read.
const (
	bash     = `{"hook_event_name":"PreToolUse","cwd":"/ws/mara","tool_name":"Bash","tool_input":{"command":"go test ./..."}}`
	question = `{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":[{"id":"color","header":"Color","question":"Which color?","options":[{"label":"Red","description":"r"},{"label":"Blue","description":"b"}]},{"id":"why","header":"Why","question":"Why?","options":[]}]}}`
	answered = `{"hook_event_name":"PostToolUse","tool_name":"request_user_input","tool_response":"{\"answers\":{}}"}`
	team     = `{"hook_event_name":"PreToolUse","tool_name":"mcp__foreman__handoff","tool_input":{}}`
	patch    = `{"hook_event_name":"PreToolUse","cwd":"/ws/mara","tool_name":"apply_patch","tool_input":{"command":"*** Begin Patch\n*** Update File: /ws/mara/ledger/main.go\n@@\n-a\n+b\n*** Add File: docs/new.md\n+x\n*** Delete File: /elsewhere/old.txt\n*** End Patch\n"}}`
)

func TestInterpret(t *testing.T) {
	cases := []struct {
		event, payload string
		want           harness.Activity
	}{
		{"SessionStart", `{"source":"startup"}`, harness.Busy}, // inside the first turn, never at launch
		{"UserPromptSubmit", `{"prompt":"go"}`, harness.Busy},
		{"PreToolUse", bash, harness.Busy},
		{"PreToolUse", question, harness.Waiting},
		{"PostToolUse", answered, harness.Busy},
		{"Stop", `{"last_assistant_message":"Done."}`, harness.Idle},
		{"Interrupt", `{}`, harness.Idle},
		{"SessionEnd", `{}`, harness.Unknown},
		{"PreToolUse", `not json`, harness.Busy},
	}
	for _, c := range cases {
		if got := (Adapter{}).Interpret(c.event, []byte(c.payload)); got != c.want {
			t.Errorf("%s %s: %q, want %q", c.event, c.payload, got, c.want)
		}
	}
}

func TestAsks(t *testing.T) {
	if got := (Adapter{}).Asks("PreToolUse", []byte(question)); got != "Which color? (Red / Blue) · Why?" {
		t.Errorf("asks %q", got)
	}
	for _, c := range [][2]string{{"PreToolUse", bash}, {"PostToolUse", answered}, {"PreToolUse", "not json"}} {
		if got := (Adapter{}).Asks(c[0], []byte(c[1])); got != "" {
			t.Errorf("%s %s asks %q", c[0], c[1], got)
		}
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct{ event, payload, want string }{
		{"SessionStart", `{"source":"resume"}`, "Started"},
		{"UserPromptSubmit", `{"prompt":"fix the tests"}`, "Working on “fix the tests”"},
		{"PreToolUse", bash, "Bash: go test ./..."},
		{"PreToolUse", patch, "Editing ledger/main.go, docs/new.md, /elsewhere/old.txt"},
		{"PreToolUse", question, "Asked you a question"},
		{"PreToolUse", team, "Team tool: handoff"},
		{"PreToolUse", `{"tool_name":"mcp__github__search"}`, "github: search"},
		{"PreToolUse", `{"tool_name":"view_image"}`, "view_image"},
		{"PostToolUse", answered, ""},
		{"Stop", `{}`, "Finished its turn"},
		{"Interrupt", `{}`, "Stopped"},
	}
	for _, c := range cases {
		if got := (Adapter{}).Describe(c.event, []byte(c.payload)); got != c.want {
			t.Errorf("%s %s: %q, want %q", c.event, c.payload, got, c.want)
		}
	}
}
