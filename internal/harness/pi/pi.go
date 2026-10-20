// Package pi is the Pi harness adapter (wiki/product/architecture/harness-adapters.md#pi).
// A Pi profile is a model server and one model on it (Pi's models.json),
// set in a form, not a login. Each instance gets its own Pi config dir
// inside its sandbox: a copy of that models.json for the container, the
// session prompt, Foreman's MCP server and the hooks extension. Probed on
// Pi 1.3.0 (2026-10-18): no dialog at startup with --no-approve, the opener
// as the message argument (with --continue too), Ctrl+J for a new line.
package pi

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brickpop/foreman/internal/harness"
)

// Adapter launches Pi.
type Adapter struct {
	// GitConfig is the CEO's ~/.gitconfig, mounted read-only; "" when there is none.
	GitConfig string
	// HostAlias is how a container reaches this machine
	// (host.containers.internal); "" when agents run on it (the Direct
	// engine), where localhost is localhost.
	HostAlias string
}

var _ harness.EndpointAdapter = Adapter{}

// ConfigDir is the instance's private Pi config dir (PI_CODING_AGENT_DIR),
// inside its sandbox: its models, sessions and settings, and its session prompt.
func ConfigDir(sandbox string) string { return filepath.Join(sandbox, ".harness", "pi") }

// modelsFile is Pi's custom models, in the profile folder and in each
// instance's config dir.
const modelsFile = "models.json"

//go:embed hooks.ts
var hooks string

// Launch starts Pi on the profile's model, with project trust declined, so
// no dialog stops the session and a repo's own .pi/ (extensions, settings)
// never loads: the container is the boundary, Foreman's setup is the one.
func (a Adapter) Launch(s harness.Session) (harness.Launch, error) {
	m, err := readModels(s.Login)
	if err != nil {
		return harness.Launch{}, err
	}
	provider, model, ok := m.endpoint()
	if !ok {
		return harness.Launch{}, harness.ErrNotConnected
	}
	m.forContainer(a.HostAlias)
	models, err := json.MarshalIndent(m.raw, "", "  ")
	if err != nil {
		return harness.Launch{}, err
	}
	dir := ConfigDir(s.Sandbox)
	files := []harness.File{
		{Path: filepath.Join(dir, modelsFile), Data: models, Mode: 0o600},
		{Path: filepath.Join(dir, "APPEND_SYSTEM.md"), Data: []byte(s.Prompt), Mode: 0o600},
	}
	// Regular mode: the pane's terminal owns the mouse and the scrollback, so a
	// selection copies in the browser and Copy gets the whole conversation (in
	// fullscreen, Pi copies into the container, which has no clipboard).
	args := []string{a.Command(), "--no-approve", "--tui-mode", "regular", "--provider", provider, "--model", model["id"].(string)}
	if s.Link.Program != "" {
		// The team tools, declared to the model like its own ("direct": by
		// default Pi reaches MCP tools only from its codemode scripts), and
		// the hooks, through the extension.
		env := map[string]string{}
		for _, kv := range s.Link.Env {
			k, v, _ := strings.Cut(kv, "=")
			env[k] = v
		}
		mcp, _ := json.MarshalIndent(map[string]any{"mcpServers": map[string]any{
			"foreman": map[string]any{"command": s.Link.Program, "args": []string{"mcp"}, "env": env, "exposure": "direct"},
		}}, "", "  ")
		program, _ := json.Marshal(s.Link.Program)
		extension := filepath.Join(dir, "foreman-hooks.ts")
		files = append(files,
			harness.File{Path: filepath.Join(dir, "mcp.json"), Data: mcp, Mode: 0o600},
			harness.File{Path: extension, Data: []byte(strings.Replace(hooks, "FOREMAN_AGENT;", string(program)+";", 1)), Mode: 0o600},
		)
		args = append(args, "--extension", extension)
	}
	// As Claude Code's: a restart continues this deployment's conversation,
	// if it has one; Pi's --continue would otherwise pick up an older one.
	saved := harness.Conversations{Dir: dir, Glob: filepath.Join("sessions", "*", "*.jsonl")}
	continuing := s.Continue && saved.Continues()
	if !s.Continue {
		files = append(files, saved.FreshStart())
	}
	if continuing {
		args = append(args, "--continue")
	}
	if s.Opener != "" && (!continuing || s.OpenerOnContinue) {
		args = append(args, "--", s.Opener) // an opener may start with "-"
	}
	return harness.Launch{
		Command: args,
		Env: append([]string{
			"PI_CODING_AGENT_DIR=" + dir,
			"PI_SKIP_VERSION_CHECK=1", // no update notice in an agent's session
			"PI_TELEMETRY=0",
		}, s.Link.Env...),
		Mounts: harness.GitConfig(a.GitConfig),
		Files:  files,
	}, nil
}

// Interpret reads the extension's events: a run begins, a tool is called,
// a dialog asks the user (ask_user) and closes, the run is over for good.
func (Adapter) Interpret(event string, _ []byte) harness.Activity {
	switch event {
	case "session_start", "agent_settled":
		return harness.Idle
	case "before_agent_start", "tool_call", "ui_prompt_end":
		return harness.Busy
	case "ui_prompt_start":
		return harness.Waiting
	}
	return harness.Unknown
}

// Describe reads the same events for the CEO.
func (Adapter) Describe(event string, payload []byte) string {
	var p struct {
		Prompt   string         `json:"prompt"`
		ToolName string         `json:"toolName"`
		Input    map[string]any `json:"input"`
		Aborted  bool           `json:"aborted"`
		Cwd      string         `json:"cwd"`
	}
	_ = json.Unmarshal(payload, &p)
	switch event {
	case "session_start":
		return "Started"
	case "before_agent_start":
		return "Working on “" + p.Prompt + "”"
	case "tool_call":
		return describeTool(p.ToolName, p.Input, p.Cwd)
	case "agent_settled":
		if p.Aborted {
			return "Stopped"
		}
		return "Finished its turn"
	}
	return ""
}

// describeTool names a call of Pi's tools by what it works on; absolute
// paths inside the working directory are shown relative to it.
func describeTool(name string, input map[string]any, cwd string) string {
	arg := func(key string) string {
		s, _ := input[key].(string)
		return s
	}
	path := func() string {
		p := arg("path")
		if rel, err := filepath.Rel(cwd, p); cwd != "" && err == nil && filepath.IsAbs(p) && !strings.HasPrefix(rel, "..") {
			return rel
		}
		return p
	}
	switch name {
	case "bash":
		return "Bash: " + arg("command")
	case "read":
		return "Reading " + path()
	case "edit":
		return "Editing " + path()
	case "write":
		return "Writing " + path()
	case "grep":
		return "Searching for " + arg("pattern")
	case "find":
		return "Looking for " + arg("pattern")
	case "ls":
		return "Listing " + path()
	case "ask_user":
		return "Asked you a question"
	case "codemode":
		return "Running a script"
	case "":
		return ""
	}
	// MCP tools are mcp__<server>__<tool>; Foreman's are the team tools.
	if server, tool, ok := strings.Cut(strings.TrimPrefix(name, "mcp__"), "__"); ok && strings.HasPrefix(name, "mcp__") {
		if server == "foreman" {
			return "Team tool: " + tool
		}
		return server + ": " + tool
	}
	return name
}

// Command is Pi's CLI.
func (Adapter) Command() string { return "pi" }

// Newline is Ctrl+J, which Pi binds to a new line beside Shift+Enter, and
// which reaches it through any terminal.
func (Adapter) Newline() string { return "\n" }

// Connected reports whether the profile folder names a server and a model.
func (Adapter) Connected(dir string) bool {
	m, err := readModels(dir)
	if err != nil {
		return false
	}
	_, _, ok := m.endpoint()
	return ok
}

// Login isn't Pi's way: its profile is set in a form (Endpoint, SetEndpoint).
func (Adapter) Login(string) (harness.Launch, error) {
	return harness.Launch{}, errors.New("a Pi profile is a model server, set in a form: Settings → Agents → Profiles → Connect")
}

// Endpoint is the profile's server and model; zero when none is set.
func (Adapter) Endpoint(dir string) (harness.Endpoint, error) {
	m, err := readModels(dir)
	if err != nil {
		return harness.Endpoint{}, err
	}
	_, model, ok := m.endpoint()
	if !ok {
		return harness.Endpoint{}, nil
	}
	p := m.provider()
	key, _ := p["apiKey"].(string)
	return harness.Endpoint{
		URL: p["baseUrl"].(string), Model: model["id"].(string),
		ContextWindow: number(model["contextWindow"]), MaxTokens: number(model["maxTokens"]),
		APIKeySet: key != "",
	}, nil
}

// SetEndpoint saves the profile's server and model in its models.json,
// keeping what the CEO added by hand to the file (samplingParams,
// thinkingLevelMap, …) while the model stays the same. One model per
// profile: any other is dropped.
func (Adapter) SetEndpoint(dir string, c harness.EndpointChange) error {
	c.URL, c.Model = strings.TrimSpace(c.URL), strings.TrimSpace(c.Model)
	if err := c.Check(); err != nil {
		return err
	}
	m, err := readModels(dir)
	if err != nil {
		return err
	}
	p := m.provider()
	p["baseUrl"] = c.URL
	if _, ok := p["api"]; !ok {
		p["api"] = "openai-completions"
	}
	if c.APIKey != "" {
		p["apiKey"] = c.APIKey
	}
	model := map[string]any{"id": c.Model}
	if _, current, ok := m.endpoint(); ok && current["id"] == c.Model {
		model = current
	}
	model["contextWindow"], model["maxTokens"] = c.ContextWindow, c.MaxTokens
	p["models"] = []any{model}
	data, err := json.MarshalIndent(m.raw, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, modelsFile), append(data, '\n'), 0o600)
}

// Detect lists the server's models, with the key set when none is given.
func (Adapter) Detect(ctx context.Context, dir, base, key string) ([]string, error) {
	if key == "" {
		if m, err := readModels(dir); err == nil {
			key, _ = m.provider()["apiKey"].(string)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return harness.Models(ctx, http.DefaultClient, base, key)
}

// models is a models.json as Pi reads it, kept whole so fields Foreman
// doesn't know survive a save.
type models struct {
	raw map[string]any
}

// readModels reads the folder's models.json; an empty one when there is none.
func readModels(dir string) (models, error) {
	m := models{raw: map[string]any{}}
	data, err := os.ReadFile(filepath.Join(dir, modelsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m.raw); err != nil || m.raw == nil {
		return models{raw: map[string]any{}}, fmt.Errorf("%s isn't Pi's models.json: %v", filepath.Join(dir, modelsFile), err)
	}
	return m, nil
}

// provider is the file's one provider (the first by name, if the CEO added
// more by hand), made when there is none: "local".
func (m models) provider() map[string]any {
	providers, _ := m.raw["providers"].(map[string]any)
	if providers == nil {
		providers = map[string]any{}
		m.raw["providers"] = providers
	}
	id := m.providerID()
	p, _ := providers[id].(map[string]any)
	if p == nil {
		p = map[string]any{}
		providers[id] = p
	}
	return p
}

func (m models) providerID() string {
	providers, _ := m.raw["providers"].(map[string]any)
	first := ""
	for id := range providers {
		if first == "" || id < first {
			first = id
		}
	}
	if first == "" {
		return "local"
	}
	return first
}

// endpoint is the provider's id and its model, when the file names a
// server and a model with an id.
func (m models) endpoint() (string, map[string]any, bool) {
	providers, _ := m.raw["providers"].(map[string]any)
	id := m.providerID()
	p, _ := providers[id].(map[string]any)
	if base, _ := p["baseUrl"].(string); base == "" {
		return "", nil, false
	}
	list, _ := p["models"].([]any)
	if len(list) == 0 {
		return "", nil, false
	}
	model, _ := list[0].(map[string]any)
	if name, _ := model["id"].(string); name == "" {
		return "", nil, false
	}
	return id, model, true
}

// forContainer makes the file right for an agent: localhost is the host
// alias in a container, and a key is always set, since Pi offers no model
// without one (a local server ignores it).
func (m models) forContainer(alias string) {
	p := m.provider()
	if key, _ := p["apiKey"].(string); key == "" {
		p["apiKey"] = "none"
	}
	base, _ := p["baseUrl"].(string)
	if u, err := url.Parse(base); err == nil && alias != "" && loopback(u.Hostname()) {
		if port := u.Port(); port != "" {
			u.Host = net.JoinHostPort(alias, port)
		} else {
			u.Host = alias
		}
		p["baseUrl"] = u.String()
	}
}

func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// number reads a JSON number as an int (0 when it isn't one).
func number(v any) int {
	f, _ := v.(float64)
	return int(f)
}
