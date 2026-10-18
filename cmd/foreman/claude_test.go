package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/agentbin"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/setup"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/userdata"
)

// The Claude Code integration test (wiki/backlog/claude-integration-tests.md):
// a real Claude Code, in Podman, driven through Foreman's whole stack and its
// API, as the CEO would. It tells us a Claude Code update didn't break the
// session start, delivery, the hooks or the team tools. It needs the CEO's
// login (the user's, in the profile given) and Podman, takes minutes and
// spends tokens, so it runs only with FOREMAN_CLAUDE_PROFILE set (just test-claude).

func TestClaudeCode(t *testing.T) {
	profile := os.Getenv("FOREMAN_CLAUDE_PROFILE")
	if profile == "" {
		t.Skip("set FOREMAN_CLAUDE_PROFILE to a Claude Code profile, usually default (just test-claude)")
	}
	dir, err := userdata.Dir()
	if err != nil {
		t.Fatal(err)
	}
	login := profiles.In(dir).Dir(profile)
	if _, err := os.Stat(filepath.Join(login, ".credentials.json")); err != nil {
		t.Fatalf("no login in %s: connect the profile under Settings → Global → Profiles first", login)
	}
	if agentbin.Binary(runtime.GOARCH) == nil {
		t.Fatal("foreman-agent isn't built: run just agent")
	}
	c := startClaudeWorkspace(t, profile)

	// 1. The lead starts on its goal: a session with no dialog in the way,
	// whose hooks say running, then idle.
	dep := c.post("/api/deployments", map[string]string{"team": "itest", "project": "proj",
		"goal": "Reply with only the word READY, and nothing else."})
	if !strings.Contains(dep, `"itest@proj"`) {
		t.Fatalf("deploy: %s", dep)
	}
	c.waitStatus("lena@proj", "idle", 4*time.Minute)
	c.saw("lena@proj", "running")
	c.said("lena", "READY")

	// 2. A line typed in is sent once, as one prompt (not a line break in a
	// paste, not answering a dialog).
	c.post("/api/instances/lena@proj/line", map[string]string{"text": "Reply with only the word PONG."})
	c.waitTurn("lena@proj", 3*time.Minute)
	if n := c.prompts("lena", "Reply with only the word PONG."); n != 1 {
		t.Errorf("the line reached Claude Code %d times, want once", n)
	}
	c.said("lena", "PONG")

	// 3. A question to the user is awaiting; answering it goes back to idle.
	c.post("/api/instances/lena@proj/line", map[string]string{"text": "Use your AskUserQuestion tool to ask me \"Proceed?\" with the options Yes and No. Then, whatever I answer, reply with only the word DONE."})
	c.waitStatus("lena@proj", "awaiting", 3*time.Minute)
	c.post("/api/instances/lena@proj/line", map[string]string{"text": ""}) // Enter: the first option
	c.waitStatus("lena@proj", "idle", 3*time.Minute)

	// 4. The team tools: the lead asks the asleep member, who is woken, gets
	// one line, reads the message and replies; the reply reaches the lead.
	c.post("/api/instances/lena@proj/line", map[string]string{"text": "Use your ask tool to ask Milo: \"What is 2+2? Answer with your reply tool, the number only.\" Then wait for his reply and tell me what he said."})
	reply := c.waitMessage(func(m store.TeamMessage) bool {
		return m.Kind == "reply" && m.FromInstance == "milo@proj" && m.State == store.MessageDelivered
	}, 6*time.Minute)
	if !strings.Contains(reply.Body, "4") {
		t.Errorf("Milo's reply: %q", reply.Body)
	}
	c.saw("milo@proj", "running")
	if n := c.toolCalls("milo", "read_message") + c.toolCalls("milo", "reply"); n < 2 {
		t.Errorf("Milo called read_message and reply %d times together, want both", n)
	}
	c.waitStatus("lena@proj", "idle", 3*time.Minute)
	c.said("lena", "4")
}

type claudeWorkspace struct {
	t        *testing.T
	root     string
	base     string // http://127.0.0.1:<port>
	token    string
	statuses map[string][]string // each instance's statuses, as seen
}

// startClaudeWorkspace makes a workspace with a two-agent team on a project
// with one code repo, on the CEO's login profile (the login itself: a refresh
// during the test lands there, never in a copy), and runs Foreman on it.
// It lives under .dev/: Podman's machine on macOS shares /Users, not the
// system's temp folder. It is removed when the test passes.
func startClaudeWorkspace(t *testing.T, profile string) *claudeWorkspace {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".dev", "claude-test", time.Now().Format("20060102-150405")))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".foreman/settings.toml": "engine = \"podman\"\n\n[project_profiles]\nproj = \"" + profile + "\"\n",
		".foreman/teams/itest.toml": "team = \"itest\"\nname = \"Itest\"\nlead = \"lena\"\n\n" +
			"[agents.lena]\nname = \"Lena\"\nrole = \"team_lead\"\nharness = \"claude-code\"\n\n" +
			"[agents.milo]\nname = \"Milo\"\nrole = \"engineer\"\nharness = \"claude-code\"\n",
		"proj/project/project.toml": "project = \"proj\"\n",
		"proj/app/README.md":        "# app\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{"proj/project", "proj/app"} {
		gitIn(t, filepath.Join(root, dir), "init", "--quiet", "--initial-branch=main")
		gitIn(t, filepath.Join(root, dir), "add", "-A")
		gitIn(t, filepath.Join(root, dir), "-c", "user.name=test", "-c", "user.email=test@localhost", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "first")
	}

	port := freePorts(t)
	var out bytes.Buffer
	go func() {
		err := run([]string{root, "--yes", "--no-open", "--port", port}, strings.NewReader(""), &out)
		t.Logf("foreman stopped: %v\n%s", err, out.String())
	}()
	c := &claudeWorkspace{t: t, root: root, base: "http://127.0.0.1:" + port, statuses: map[string][]string{}}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		if data, err := os.ReadFile(filepath.Join(root, ".foreman", "access-token")); err == nil {
			c.token = strings.TrimSpace(string(data))
			if res, err := c.do(http.MethodGet, "/api/workspace", nil); err == nil && res.StatusCode == http.StatusOK {
				res.Body.Close()
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("Foreman didn't start:\n%s", out.String())
		}
		time.Sleep(200 * time.Millisecond)
	}
	// Foreman serves before Podman answers: agents start once the checklist
	// says Podman runs and the instances are restored.
	for !c.podmanRuns() {
		if time.Now().After(deadline) {
			t.Fatalf("Podman never ran:\n%s", out.String())
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Cleanup(func() {
		_, _ = c.do(http.MethodDelete, "/api/deployments/itest@proj", nil)
		if t.Failed() {
			t.Logf("kept for a look: %s (session logs in .foreman/logs)", root)
			return
		}
		_ = os.RemoveAll(root)
	})
	return c
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

// freePorts finds a port whose next one (the agent port) is free too.
func freePorts(t *testing.T) string {
	t.Helper()
	for range 20 {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := l.Addr().(*net.TCPAddr).Port
		next, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port+1))
		l.Close()
		if err == nil {
			next.Close()
			return fmt.Sprint(port)
		}
	}
	t.Fatal("no two free ports in a row")
	return ""
}

func (c *claudeWorkspace) do(method, path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

func (c *claudeWorkspace) post(path string, body any) string {
	c.t.Helper()
	res, err := c.do(http.MethodPost, path, body)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		c.t.Fatalf("POST %s: %d %s", path, res.StatusCode, data)
	}
	return string(data)
}

func (c *claudeWorkspace) get(path string, into any) {
	c.t.Helper()
	res, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(into); err != nil {
		c.t.Fatalf("GET %s: %v", path, err)
	}
}

// poll records every instance's status as it changes.
func (c *claudeWorkspace) poll() map[string]string {
	var list []store.Instance
	c.get("/api/instances", &list)
	now := map[string]string{}
	for _, i := range list {
		now[i.ID] = i.Status
		seen := c.statuses[i.ID]
		if len(seen) == 0 || seen[len(seen)-1] != i.Status {
			c.statuses[i.ID] = append(seen, i.Status)
		}
	}
	return now
}

func (c *claudeWorkspace) waitStatus(id, status string, timeout time.Duration) {
	c.t.Helper()
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
		if c.poll()[id] == status {
			return
		}
	}
	c.t.Fatalf("%s never %s; seen %v (session log in %s)", id, status, c.statuses[id], filepath.Join(c.root, ".foreman", "logs"))
}

// waitTurn waits for a turn: running, then idle again.
func (c *claudeWorkspace) waitTurn(id string, timeout time.Duration) {
	c.t.Helper()
	before := len(c.statuses[id])
	c.waitStatus(id, "running", timeout)
	c.waitStatus(id, "idle", timeout)
	if len(c.statuses[id]) <= before {
		c.t.Fatalf("%s had no turn; seen %v", id, c.statuses[id])
	}
}

func (c *claudeWorkspace) saw(id, status string) {
	c.t.Helper()
	if !slices.Contains(c.statuses[id], status) {
		c.t.Errorf("%s was never %s; seen %v", id, status, c.statuses[id])
	}
}

func (c *claudeWorkspace) waitMessage(match func(store.TeamMessage) bool, timeout time.Duration) store.TeamMessage {
	c.t.Helper()
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		c.poll()
		var list []store.TeamMessage
		c.get("/api/messages?deployment=itest@proj", &list)
		for _, m := range list {
			if match(m) {
				return m
			}
		}
	}
	var list []store.TeamMessage
	c.get("/api/messages?deployment=itest@proj", &list)
	c.t.Fatalf("no such message; the traffic: %+v; statuses %v", list, c.statuses)
	return store.TeamMessage{}
}

// transcript is an agent's conversation, as Claude Code saved it.
func (c *claudeWorkspace) transcript(agent string) []map[string]any {
	c.t.Helper()
	files, _ := filepath.Glob(filepath.Join(c.root, "proj", ".sandboxes", agent, ".harness", "claude-code", "projects", "*", "*.jsonl"))
	var lines []map[string]any
	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			c.t.Fatal(err)
		}
		scan := bufio.NewScanner(file)
		scan.Buffer(nil, 16<<20)
		for scan.Scan() {
			var line map[string]any
			if json.Unmarshal(scan.Bytes(), &line) == nil {
				lines = append(lines, line)
			}
		}
		file.Close()
	}
	return lines
}

// prompts counts the prompts the agent received with exactly this text: what
// was typed or delivered, as Claude Code took it.
func (c *claudeWorkspace) prompts(agent, text string) int {
	n := 0
	for _, l := range c.transcript(agent) {
		if msg, ok := l["message"].(map[string]any); ok && l["type"] == "user" {
			if s, ok := msg["content"].(string); ok && strings.TrimSpace(s) == text {
				n++
			}
		}
	}
	return n
}

// said fails unless the agent's answers come to include the text. Claude
// Code writes its transcript a little after the Stop hook says idle, so it
// waits for it.
func (c *claudeWorkspace) said(agent, text string) {
	c.t.Helper()
	for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(time.Second) {
		if c.answered(agent, text) {
			return
		}
	}
	c.t.Errorf("%s never said %q", agent, text)
}

func (c *claudeWorkspace) answered(agent, text string) bool {
	for _, l := range c.transcript(agent) {
		if l["type"] != "assistant" {
			continue
		}
		msg, _ := l["message"].(map[string]any)
		blocks, _ := msg["content"].([]any)
		for _, b := range blocks {
			if block, ok := b.(map[string]any); ok && block["type"] == "text" && strings.Contains(fmt.Sprint(block["text"]), text) {
				return true
			}
		}
	}
	return false
}

// toolCalls counts the agent's calls of a Foreman team tool.
func (c *claudeWorkspace) toolCalls(agent, tool string) int {
	n := 0
	for _, l := range c.transcript(agent) {
		if l["type"] != "assistant" {
			continue
		}
		msg, _ := l["message"].(map[string]any)
		blocks, _ := msg["content"].([]any)
		for _, b := range blocks {
			if block, ok := b.(map[string]any); ok && block["type"] == "tool_use" && block["name"] == "mcp__foreman__"+tool {
				n++
			}
		}
	}
	return n
}

// podmanRuns reports whether the setup checklist says Podman runs.
func (c *claudeWorkspace) podmanRuns() bool {
	res, err := c.do(http.MethodGet, "/api/setup", nil)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	var steps []setup.Step
	if json.NewDecoder(res.Body).Decode(&steps) != nil {
		return false
	}
	for _, s := range steps {
		if s.Kind == "running" {
			return s.State == "done"
		}
	}
	return false
}
