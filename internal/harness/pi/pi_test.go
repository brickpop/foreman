package pi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/runner"
)

func setup(t *testing.T) (Adapter, harness.Session) {
	t.Helper()
	profile := t.TempDir()
	a := Adapter{HostAlias: "host.containers.internal"}
	if err := a.SetEndpoint(profile, harness.EndpointChange{
		URL: "http://localhost:11434/v1", Model: "qwen3:30b", ContextWindow: 262144, MaxTokens: 131072,
	}); err != nil {
		t.Fatal(err)
	}
	return a, harness.Session{Instance: "mara@ledger", Sandbox: t.TempDir(), Dir: t.TempDir(), Prompt: "You are Mara.", Login: profile}
}

func file(t *testing.T, l harness.Launch, name string) []byte {
	t.Helper()
	for _, f := range l.Files {
		if filepath.Base(f.Path) == name {
			if f.Mode != 0o600 {
				t.Errorf("%s mode %o", name, f.Mode)
			}
			return f.Data
		}
	}
	t.Fatalf("no %s in %v", name, l.Files)
	return nil
}

func provider(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m struct {
		Providers map[string]map[string]any `json:"providers"`
	}
	if err := json.Unmarshal(data, &m); err != nil || len(m.Providers) != 1 {
		t.Fatalf("models.json %s: %v", data, err)
	}
	for _, p := range m.Providers {
		return p
	}
	return nil
}

func TestLaunch(t *testing.T) {
	a, s := setup(t)
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"pi", "--no-approve", "--tui-mode", "regular", "--provider", "local", "--model", "qwen3:30b"}
	if !reflect.DeepEqual(l.Command, want) {
		t.Errorf("command %v", l.Command)
	}
	dir := ConfigDir(s.Dir)
	if !slices.Contains(l.Env, "PI_CODING_AGENT_DIR="+dir) || !slices.Contains(l.Env, "PI_SKIP_VERSION_CHECK=1") {
		t.Errorf("env %v", l.Env)
	}
	for _, f := range l.Files {
		if !strings.HasPrefix(f.Path, dir+string(filepath.Separator)) {
			t.Errorf("%s is outside the instance's config dir", f.Path)
		}
	}
	if string(file(t, l, "APPEND_SYSTEM.md")) != "You are Mara." {
		t.Error("the prompt isn't Pi's appended system prompt")
	}
	// The container's copy: localhost is the host alias, and a key is set.
	p := provider(t, file(t, l, "models.json"))
	if p["baseUrl"] != "http://host.containers.internal:11434/v1" || p["apiKey"] != "none" || p["api"] != "openai-completions" {
		t.Errorf("provider %v", p)
	}
	// The profile's own file is untouched.
	if e, _ := a.Endpoint(s.Login); e.URL != "http://localhost:11434/v1" || e.APIKeySet {
		t.Errorf("profile %+v", e)
	}
	if len(l.Mounts) != 0 {
		t.Errorf("mounts %v", l.Mounts)
	}
}

// Only loopback hosts are rewritten, and only in a container.
func TestTheHostAlias(t *testing.T) {
	for base, want := range map[string]string{
		"http://localhost:11434/v1":     "http://host.containers.internal:11434/v1",
		"http://127.0.0.1:8080/v1":      "http://host.containers.internal:8080/v1",
		"http://[::1]:8080/v1":          "http://host.containers.internal:8080/v1",
		"https://localhost/v1":          "https://host.containers.internal/v1",
		"http://gpu-box.lan:11434/v1":   "http://gpu-box.lan:11434/v1",
		"https://api.example.com/v1":    "https://api.example.com/v1",
		"http://localhost.example.com/": "http://localhost.example.com/",
	} {
		a, s := setup(t)
		if err := a.SetEndpoint(s.Login, harness.EndpointChange{URL: base, Model: "m", ContextWindow: 10, MaxTokens: 5}); err != nil {
			t.Fatal(err)
		}
		l, err := a.Launch(s)
		if err != nil {
			t.Fatal(err)
		}
		if got := provider(t, file(t, l, "models.json"))["baseUrl"]; got != want {
			t.Errorf("%s became %s, want %s", base, got, want)
		}
		a.HostAlias = "" // the Direct engine: agents run on this machine
		l, _ = a.Launch(s)
		if got := provider(t, file(t, l, "models.json"))["baseUrl"]; got != base {
			t.Errorf("direct: %s became %s", base, got)
		}
	}
}

func TestLaunchWithTheAgentLink(t *testing.T) {
	a, s := setup(t)
	s.Link = harness.Link{Program: "/opt/foreman/foreman-agent", Env: []string{"FOREMAN_AGENT_URL=http://agent", "FOREMAN_AGENT_TOKEN=t0k"}}
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	extension := filepath.Join(ConfigDir(s.Dir), "foreman-hooks.ts")
	if i := slices.Index(l.Command, "--extension"); i < 0 || l.Command[i+1] != extension {
		t.Errorf("command %v", l.Command)
	}
	hooks := string(file(t, l, "foreman-hooks.ts"))
	if !strings.Contains(hooks, `name: "ask_user"`) {
		t.Error("no ask_user tool")
	}
	if !strings.Contains(hooks, `const program: string = "/opt/foreman/foreman-agent";`) || strings.Contains(hooks, "FOREMAN_AGENT;") {
		t.Errorf("the program isn't filled in:\n%s", hooks)
	}
	for _, event := range []string{"session_start", "before_agent_start", "tool_call", "ui_prompt_start", "ui_prompt_end", "agent_settled"} {
		if a.Interpret(event, nil) == harness.Unknown || !strings.Contains(hooks, `pi.on("`+event+`"`) {
			t.Errorf("%s isn't relayed and read", event)
		}
	}
	var mcp struct {
		Servers map[string]struct {
			Command  string            `json:"command"`
			Args     []string          `json:"args"`
			Env      map[string]string `json:"env"`
			Exposure string            `json:"exposure"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(file(t, l, "mcp.json"), &mcp); err != nil {
		t.Fatal(err)
	}
	f := mcp.Servers["foreman"]
	if len(mcp.Servers) != 1 || f.Command != s.Link.Program || !reflect.DeepEqual(f.Args, []string{"mcp"}) || f.Env["FOREMAN_AGENT_TOKEN"] != "t0k" || f.Exposure != "direct" {
		t.Errorf("mcp %+v", mcp)
	}
	if !slices.Contains(l.Env, "FOREMAN_AGENT_TOKEN=t0k") {
		t.Errorf("env %v", l.Env)
	}
}

func TestARestartContinuesOnlyThisDeploymentsConversation(t *testing.T) {
	a, s := setup(t)
	save := func(name string) {
		t.Helper()
		path := filepath.Join(ConfigDir(s.Dir), "sessions", "--work--", name)
		_ = os.MkdirAll(filepath.Dir(path), 0o700)
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	save("last-deployment.jsonl")
	s.Opener = "-v is the goal" // starts with a dash: still the message
	l, err := a.Launch(s)
	if err != nil {
		t.Fatal(err)
	}
	if got := l.Command[len(l.Command)-2:]; !reflect.DeepEqual(got, []string{"--", s.Opener}) || slices.Contains(l.Command, "--continue") {
		t.Errorf("first start %v", l.Command)
	}
	if err := harness.WriteFiles(l.Files); err != nil {
		t.Fatal(err)
	}
	s.Continue = true
	if l, _ := a.Launch(s); slices.Contains(l.Command, "--continue") || l.Command[len(l.Command)-1] != s.Opener {
		t.Errorf("restart with nothing of this deployment: %v", l.Command)
	}
	save("this-deployment.jsonl")
	l, _ = a.Launch(s)
	if !slices.Contains(l.Command, "--continue") || slices.Contains(l.Command, s.Opener) {
		t.Errorf("restart with a conversation: %v", l.Command)
	}
	// The Chief of Staff's: continued, and told what for.
	s.OpenerOnContinue = true
	if l, _ := a.Launch(s); !slices.Contains(l.Command, "--continue") || l.Command[len(l.Command)-1] != s.Opener {
		t.Errorf("opener on continue: %v", l.Command)
	}
}

func TestNotConnected(t *testing.T) {
	a := Adapter{}
	empty := t.TempDir()
	if a.Connected(empty) {
		t.Error("an empty folder is connected")
	}
	if _, err := a.Launch(harness.Session{Login: empty, Sandbox: t.TempDir()}); !errors.Is(err, harness.ErrNotConnected) {
		t.Errorf("launch: %v", err)
	}
	for name, content := range map[string]string{
		"not json":   "{",
		"no models":  `{"providers":{"x":{"baseUrl":"http://h/v1","models":[]}}}`,
		"no url":     `{"providers":{"x":{"models":[{"id":"m"}]}}}`,
		"no id":      `{"providers":{"x":{"baseUrl":"http://h/v1","models":[{}]}}}`,
		"wrong type": `{"providers":{"x":{"baseUrl":7,"models":"m"}}}`,
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, modelsFile), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if a.Connected(dir) {
			t.Errorf("%s: connected", name)
		}
		if _, err := a.Launch(harness.Session{Login: dir, Sandbox: t.TempDir()}); err == nil {
			t.Errorf("%s: launched", name)
		}
	}
	if _, err := a.Login(empty); err == nil {
		t.Error("Pi has a login terminal")
	}
}

// A save keeps what the CEO added by hand while the model stays, and the key
// unless a new one is given.
func TestSetEndpointKeepsHandEdits(t *testing.T) {
	a := Adapter{}
	dir := t.TempDir()
	hand := `{"$schema":"https://pi.dev/schemas/models.schema.json","providers":{"ollama":{"baseUrl":"http://old/v1","api":"openai-responses","apiKey":"sk-secret",
	  "models":[{"id":"qwen","reasoning":true,"samplingParams":{"temperature":0.8},"thinkingLevelMap":{"high":null}},{"id":"other"}]}}}`
	if err := os.WriteFile(filepath.Join(dir, modelsFile), []byte(hand), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.SetEndpoint(dir, harness.EndpointChange{URL: " http://gpu:11434/v1 ", Model: "qwen", ContextWindow: 1000, MaxTokens: 500}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, modelsFile))
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	p := m["providers"].(map[string]any)["ollama"].(map[string]any)
	model := p["models"].([]any)[0].(map[string]any)
	if m["$schema"] == nil || p["api"] != "openai-responses" || p["apiKey"] != "sk-secret" || p["baseUrl"] != "http://gpu:11434/v1" ||
		model["reasoning"] != true || model["samplingParams"] == nil || model["thinkingLevelMap"] == nil || model["contextWindow"] != 1000.0 ||
		len(p["models"].([]any)) != 1 {
		t.Errorf("saved %s", data)
	}
	e, err := a.Endpoint(dir)
	if err != nil || e != (harness.Endpoint{URL: "http://gpu:11434/v1", Model: "qwen", ContextWindow: 1000, MaxTokens: 500, APIKeySet: true}) {
		t.Errorf("endpoint %+v %v", e, err)
	}
	// Another model: the old one's settings don't follow it; a new key replaces the old.
	if err := a.SetEndpoint(dir, harness.EndpointChange{URL: "http://gpu:11434/v1", APIKey: "sk-new", Model: "llama", ContextWindow: 1000, MaxTokens: 500}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(dir, modelsFile))
	if strings.Contains(string(data), "samplingParams") || !strings.Contains(string(data), `"sk-new"`) {
		t.Errorf("saved %s", data)
	}
	if info, _ := os.Stat(filepath.Join(dir, modelsFile)); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %o", info.Mode().Perm())
	}
	// Refused: nothing written.
	if err := a.SetEndpoint(dir, harness.EndpointChange{URL: "nope", Model: "x", ContextWindow: 1, MaxTokens: 1}); !errors.Is(err, harness.ErrInvalid) {
		t.Errorf("invalid: %v", err)
	}
	if after, _ := os.ReadFile(filepath.Join(dir, modelsFile)); string(after) != string(data) {
		t.Error("a refused change was written")
	}
}

func TestDetectUsesTheKeySet(t *testing.T) {
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		w.Write([]byte(`{"data":[{"id":"qwen3:30b"}]}`))
	}))
	defer srv.Close()
	a := Adapter{}
	dir := t.TempDir()
	if ids, err := a.Detect(context.Background(), dir, srv.URL+"/v1", ""); err != nil || len(ids) != 1 {
		t.Fatalf("detect %v %v", ids, err)
	}
	_ = a.SetEndpoint(dir, harness.EndpointChange{URL: srv.URL + "/v1", APIKey: "sk-set", Model: "m", ContextWindow: 2, MaxTokens: 1})
	_, _ = a.Detect(context.Background(), dir, srv.URL+"/v1", "")
	_, _ = a.Detect(context.Background(), dir, srv.URL+"/v1", "sk-typed")
	if !reflect.DeepEqual(auth, []string{"", "Bearer sk-set", "Bearer sk-typed"}) {
		t.Errorf("auth %q", auth)
	}
}

func TestDescribe(t *testing.T) {
	a := Adapter{}
	for _, c := range []struct{ event, payload, want string }{
		{"session_start", `{}`, "Started"},
		{"before_agent_start", `{"prompt":"fix the tests"}`, "Working on “fix the tests”"},
		{"tool_call", `{"toolName":"bash","input":{"command":"go test ./..."}}`, "Bash: go test ./..."},
		{"tool_call", `{"toolName":"edit","input":{"path":"/w/ledger/main.go"},"cwd":"/w/ledger"}`, "Editing main.go"},
		{"tool_call", `{"toolName":"read","input":{"path":"/etc/hosts"},"cwd":"/w/ledger"}`, "Reading /etc/hosts"},
		{"tool_call", `{"toolName":"write","input":{"path":"notes.md"},"cwd":"/w"}`, "Writing notes.md"},
		{"tool_call", `{"toolName":"mcp__foreman__send_message","input":{}}`, "Team tool: send_message"},
		{"tool_call", `{"toolName":"ask_user","input":{"question":"Which?"}}`, "Asked you a question"},
		{"ui_prompt_start", `{"title":"Which?"}`, ""},
		{"tool_call", `{"toolName":"mcp__github__search","input":{}}`, "github: search"},
		{"agent_settled", `{"aborted":false}`, "Finished its turn"},
		{"agent_settled", `{"aborted":true}`, "Stopped"},
		{"agent_end", `{}`, ""},
	} {
		if got := a.Describe(c.event, []byte(c.payload)); got != c.want {
			t.Errorf("%s %s: %q, want %q", c.event, c.payload, got, c.want)
		}
	}
	// ask_user's dialog: waiting while it is open, working again once answered.
	if a.Interpret("ui_prompt_start", nil) != harness.Waiting || a.Interpret("ui_prompt_end", nil) != harness.Busy {
		t.Error("the dialog")
	}
	if a.Interpret("agent_end", nil) != harness.Unknown || a.Interpret("tool_call", nil) != harness.Busy || a.Interpret("agent_settled", nil) != harness.Idle {
		t.Error("interpret")
	}
}

func TestGitConfigMount(t *testing.T) {
	a, s := setup(t)
	a.GitConfig = filepath.Join(t.TempDir(), "gitconfig")
	_ = os.WriteFile(a.GitConfig, []byte("[user]\n"), 0o600)
	l, _ := a.Launch(s)
	if !reflect.DeepEqual(l.Mounts, []runner.Mount{{Source: a.GitConfig, Target: "/home/node/.gitconfig", ReadOnly: true}}) {
		t.Errorf("mounts %v", l.Mounts)
	}
}

// A moved sandbox's conversations follow it, under the folder Pi 1.3.0 names
// for the new working folder.
func TestMoveConversations(t *testing.T) {
	a, s := setup(t)
	sessions := filepath.Join(ConfigDir(s.Dir), "sessions")
	if err := a.MoveConversations(s.Dir, "/a", "/b"); err != nil {
		t.Errorf("nothing to move: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sessions, "--Users-ceo-.foreman-sandboxes-ledger-mara--"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := a.MoveConversations(s.Dir, "/Users/ceo/.foreman/sandboxes/ledger/mara", "/Users/ceo/workspace/foreman/ledger/agents/mara"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sessions, "--Users-ceo-workspace-foreman-ledger-agents-mara--")); err != nil {
		t.Error("the conversations didn't move")
	}
}
