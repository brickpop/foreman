package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brickpop/foreman/images"
	"github.com/brickpop/foreman/internal/runner"
	podmanengine "github.com/brickpop/foreman/internal/runner/podman"
	"github.com/brickpop/foreman/internal/screen"
)

// Experiment D (wiki/backlog/prove-d-claude-delivery-hooks.md), on Foreman's
// real engine: an interactive Claude Code in a container, its hooks sent to a
// listener on the host through the agent stub, its screen read with
// internal/screen. Every hook event and the screen at each step go to
// .dev/prove/d/<date-time>/report.txt, one folder per run. Needs the login from `just prove-b-login`.

type hookEvent struct {
	At      time.Time
	Event   string
	Payload map[string]any
}

type dRun struct {
	r       *report
	proc    runner.Process
	scr     *screen.VT
	out     io.Writer // the report file
	project string
	private string // the instance's config dir, on the host

	mu    sync.Mutex
	hooks []hookEvent
}

func proveD(r *report, waitSeconds int) {
	ctx := context.Background()
	cwd, _ := os.Getwd()
	creds := filepath.Join(credentialsDir(), ".credentials.json")
	if _, err := os.Stat(creds); err != nil {
		r.fail("no login yet: run `just prove-b-login` first")
		return
	}
	agent := filepath.Join(cwd, ".dev", "prove", "foreman-agent")
	if _, err := os.Stat(agent); err != nil {
		r.fail("the agent stub isn't built (the prove-d recipe builds it): %v", err)
		return
	}
	// Each run keeps its own folder: earlier reports are never overwritten.
	dir := filepath.Join(cwd, ".dev", "prove", "d", time.Now().Format("20060102-150405"))
	r.info("this run's report: %s", filepath.Join(dir, "report.txt"))
	private, project := filepath.Join(dir, "config"), filepath.Join(dir, "project")
	for _, d := range []string{private, project} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			r.fail("%v", err)
			return
		}
	}
	report, err := os.Create(filepath.Join(dir, "report.txt"))
	if err != nil {
		r.fail("%v", err)
		return
	}
	defer report.Close()
	d := &dRun{r: r, out: report, project: project, private: private}

	engine, err := podmanengine.New(ctx)
	if !r.check(err, "connected to Podman through Foreman's engine") {
		return
	}
	defer engine.Close()
	r.info("building or finding the agent image (minutes the first time)")
	image, err := engine.Images(harnessImage, images.Harness).Ensure(ctx, "d")
	if !r.check(err, "the agent image is ready: "+image) {
		return
	}

	// The hook listener, on the host's loopback (experiment C).
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if !r.check(err, "listening for hook events on the host") {
		return
	}
	token := newToken()
	go func() { _ = http.Serve(listener, d.hookHandler(token)) }()
	port := listener.Addr().(*net.TCPAddr).Port

	if err := d.writeConfig(private, project); !r.check(err, "wrote the instance's config: hooks, MCP, prompt, onboarding and trust seeded") {
		return
	}
	proc, err := engine.Start(ctx, runner.Spec{
		Name:    "prove-d-" + newToken()[:6],
		Image:   image,
		Command: []string{"claude", "--dangerously-skip-permissions", "--mcp-config", configDir + "/mcp.json", "--strict-mcp-config", "--allowedTools", "mcp__foreman__wait", "mcp__foreman__ping"},
		Dir:     project,
		Env: []string{
			"CLAUDE_CONFIG_DIR=" + configDir,
			fmt.Sprintf("FOREMAN_AGENT_URL=http://host.containers.internal:%d", port),
			"FOREMAN_AGENT_TOKEN=" + token,
			"FOREMAN_PROVE_LOG=" + configDir + "/agent.log",
			"FOREMAN_PROVE_SECRET=d",
		},
		Mounts: []runner.Mount{
			{Source: private, Target: configDir},
			{Source: creds, Target: configDir + "/.credentials.json"},
			{Source: agent, Target: "/opt/foreman-agent", ReadOnly: true},
			{Source: project, Target: project},
		},
		Size:   runner.Size{Cols: 120, Rows: 40},
		Labels: map[string]string{"foreman.prove": "d"},
	})
	if !r.check(err, "started an interactive Claude Code") {
		return
	}
	d.proc = proc
	defer func() { _ = proc.Stop(5 * time.Second) }()
	d.scr = screen.New(runner.Size{Cols: 120, Rows: 40}, func(answer []byte) { _, _ = proc.Write(answer) })
	defer d.scr.Close()
	raw, _ := os.Create(filepath.Join(dir, "session.log"))
	defer raw.Close()
	go func() { _, _ = io.Copy(io.MultiWriter(d.scr, raw), proc) }()

	d.stepStartup()
	d.stepDeliverIdle()
	d.stepDeliverBusy()
	d.stepMultiline()
	d.stepWaitingForUser()
	d.stepToolTimeout(waitSeconds)
	d.stepRefreshMidSession(creds)
	r.info("everything observed: %s (hook events and screens) and %s (raw output)", report.Name(), raw.Name())
}

// writeConfig writes the private config dir: hooks to the agent stub, the MCP
// server, a short session prompt, and the account record from the login with
// onboarding and this folder's trust marked as done.
func (d *dRun) writeConfig(private, project string) error {
	hooks := map[string]any{}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Notification", "Stop"} {
		entry := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "/opt/foreman-agent hook " + event}}}
		if event == "PreToolUse" || event == "PostToolUse" {
			entry["matcher"] = "*"
		}
		hooks[event] = []any{entry}
	}
	files := map[string]any{
		// skipDangerousModePermissionPrompt: no bypass-mode warning at startup.
		"settings.json": map[string]any{"hooks": hooks, "skipDangerousModePermissionPrompt": true},
		"mcp.json": map[string]any{"mcpServers": map[string]any{"foreman": map[string]any{
			"command": "/opt/foreman-agent", "args": []string{"mcp"},
		}}},
	}
	account := map[string]any{}
	if data, err := os.ReadFile(filepath.Join(credentialsDir(), ".claude.json")); err == nil {
		_ = json.Unmarshal(data, &account)
	}
	account["hasCompletedOnboarding"] = true
	account["bypassPermissionsModeAccepted"] = true // the one-time confirmation of running without permission checks
	projects, _ := account["projects"].(map[string]any)
	if projects == nil {
		projects = map[string]any{}
	}
	projects[project] = map[string]any{"hasTrustDialogAccepted": true}
	account["projects"] = projects
	files[".claude.json"] = account
	for name, v := range files {
		data, _ := json.MarshalIndent(v, "", "  ")
		if err := os.WriteFile(filepath.Join(private, name), data, 0o600); err != nil {
			return err
		}
	}
	prompt := "# Session prompt (written by Foreman)\n\nYou are Tove, an engineer. Follow the user's instructions exactly and keep answers short.\n"
	return os.WriteFile(filepath.Join(private, "CLAUDE.md"), []byte(prompt), 0o600)
}

func (d *dRun) hookHandler(token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		var payload map[string]any
		_ = json.NewDecoder(req.Body).Decode(&payload)
		e := hookEvent{At: time.Now(), Event: strings.TrimPrefix(req.URL.Path, "/hook/"), Payload: payload}
		d.mu.Lock()
		d.hooks = append(d.hooks, e)
		d.mu.Unlock()
		short, _ := json.Marshal(payload)
		fmt.Fprintf(d.out, "%s HOOK %s %s\n", e.At.Format("15:04:05.000"), e.Event, short)
	})
}

// waitHook waits for an event named event after since that matches.
func (d *dRun) waitHook(event string, since time.Time, timeout time.Duration, match func(map[string]any) bool) (hookEvent, bool) {
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		d.mu.Lock()
		for _, e := range d.hooks {
			if e.Event == event && e.At.After(since) && (match == nil || match(e.Payload)) {
				d.mu.Unlock()
				return e, true
			}
		}
		d.mu.Unlock()
	}
	return hookEvent{}, false
}

func (d *dRun) count(event string, since time.Time, match func(map[string]any) bool) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := 0
	for _, e := range d.hooks {
		if e.Event == event && e.At.After(since) && (match == nil || match(e.Payload)) {
			n++
		}
	}
	return n
}

func (d *dRun) dump(label string) string {
	text := d.scr.String()
	fmt.Fprintf(d.out, "\n===== %s: SCREEN at %s =====\n%s\n===== end =====\n\n", label, time.Now().Format("15:04:05.000"), text)
	return text
}

// typeText sends text as if typed, then Enter as its own keystroke.
func (d *dRun) typeText(text string) {
	_, _ = d.proc.Write([]byte(text))
	time.Sleep(400 * time.Millisecond)
	_, _ = d.proc.Write([]byte("\r"))
}

// paste sends text as a bracketed paste, then Enter.
func (d *dRun) paste(text string) {
	_, _ = d.proc.Write([]byte("\x1b[200~" + text + "\x1b[201~"))
	time.Sleep(400 * time.Millisecond)
	_, _ = d.proc.Write([]byte("\r"))
}

func prompt(want string) func(map[string]any) bool {
	return func(p map[string]any) bool { return p["prompt"] == want }
}

func (d *dRun) stepStartup() {
	r := d.r
	// The bypass-mode warning comes before the session starts (no hook yet).
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		if d.count("SessionStart", time.Time{}, nil) > 0 {
			break
		}
		if strings.Contains(d.scr.String(), "Bypass Permissions mode") {
			d.dump("1 bypass-mode warning")
			r.fail("1. the bypass-mode warning still shows: the seeding doesn't cover it; accepting it with the keys to go on")
			_, _ = d.proc.Write([]byte("\x1b[B")) // Down: "Yes, I accept"
			time.Sleep(500 * time.Millisecond)
			_, _ = d.proc.Write([]byte("\r"))
			break
		}
	}
	if _, ok := d.waitHook("SessionStart", time.Time{}, 60*time.Second, nil); !ok {
		r.fail("1. no SessionStart hook within 60s: hooks don't reach the host (see report)")
	} else {
		r.pass("1. hooks reach the host from inside the container (SessionStart)")
	}
	time.Sleep(4 * time.Second)
	text := d.dump("1 startup")
	for _, dialog := range []string{"trust", "Trust", "theme", "Choose", "Login", "log in", "Select", "Bypass", "I accept"} {
		if strings.Contains(text, dialog) {
			r.fail("1. the session stopped at a dialog (%q on screen): the seeding isn't enough; screen in the report", dialog)
			return
		}
	}
	r.pass("1. the session reached its prompt with no onboarding, login or trust dialog")
}

func (d *dRun) deliverOnce(label, text string, send func(string)) {
	r := d.r
	since := time.Now()
	send(text)
	if _, ok := d.waitHook("Stop", since, 120*time.Second, nil); !ok {
		r.fail("%s: no Stop hook within 120s", label)
	}
	time.Sleep(time.Second)
	d.dump(label)
	switch n := d.count("UserPromptSubmit", since, prompt(text)); n {
	case 1:
		r.pass("%s: submitted once, unchanged (UserPromptSubmit carries the exact text)", label)
	default:
		r.fail("%s: %d submissions with the exact text", label, n)
		r.info("   sent:      %q", text)
		for _, got := range d.prompts(since) {
			r.info("   submitted: %q", got)
		}
	}
}

// prompts are the texts submitted since.
func (d *dRun) prompts(since time.Time) []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var list []string
	for _, e := range d.hooks {
		if e.Event == "UserPromptSubmit" && e.At.After(since) {
			list = append(list, fmt.Sprint(e.Payload["prompt"]))
		}
	}
	return list
}

// said reports whether Claude's own answers (not the echo of what was typed)
// contain want, from the session transcript in the private config dir.
func (d *dRun) said(want string) bool {
	files, _ := filepath.Glob(filepath.Join(d.private, "projects", "*", "*.jsonl"))
	for _, f := range files {
		data, _ := os.ReadFile(f)
		for _, line := range strings.Split(string(data), "\n") {
			var entry struct {
				Type    string `json:"type"`
				Message struct {
					Content json.RawMessage `json:"content"`
				} `json:"message"`
			}
			if json.Unmarshal([]byte(line), &entry) != nil || entry.Type != "assistant" {
				continue
			}
			if strings.Contains(string(entry.Message.Content), want) {
				return true
			}
		}
	}
	return false
}

func (d *dRun) stepDeliverIdle() {
	d.deliverOnce("2a. typed while idle", "Reply with exactly: ALPHA-1", d.typeText)
	d.deliverOnce("2b. pasted while idle", "Reply with exactly: BRAVO-2", d.paste)
	if d.said("ALPHA-1") && d.said("BRAVO-2") {
		d.r.pass("2. both were answered (session transcript)")
	} else {
		d.r.fail("2. not both answered: ALPHA-1 %v, BRAVO-2 %v", d.said("ALPHA-1"), d.said("BRAVO-2"))
	}
}

func (d *dRun) stepDeliverBusy() {
	r := d.r
	since := time.Now()
	d.typeText("Call the wait tool of the foreman MCP server with seconds=20, then reply with exactly: WAIT-DONE")
	if _, ok := d.waitHook("PreToolUse", since, 60*time.Second, func(p map[string]any) bool { return p["tool_name"] == "mcp__foreman__wait" }); !ok {
		r.fail("3. the wait tool was never called; skipping the busy delivery")
		return
	}
	time.Sleep(2 * time.Second)
	busy := time.Now()
	d.typeText("Also reply with exactly: CHARLIE-3")
	d.dump("3 right after a delivery mid-turn")
	// The turn ends with a Stop; a queued message may also get a turn of its own.
	d.waitHook("Stop", busy, 120*time.Second, nil)
	time.Sleep(5 * time.Second)
	d.dump("3 after the turn")
	ok := d.said("CHARLIE-3")
	n := d.count("UserPromptSubmit", busy, prompt("Also reply with exactly: CHARLIE-3"))
	r.info("3. mid-turn delivery: UserPromptSubmit for it %d time(s), %d Stop hook(s) since; answered (transcript): %v", n, d.count("Stop", busy, nil), ok)
	if n == 1 && ok {
		r.pass("3. a message typed mid-turn was queued and answered once")
	} else {
		r.fail("3. a message typed mid-turn: submitted %d time(s), answered %v (see the report)", n, ok)
	}
}

func (d *dRun) stepMultiline() {
	text := "Repeat back exactly what is between the markers, nothing else:\n<<<\nline one\n  indented `code` $HOME \"double\" 'single' ünïcødé 🚀\n\ttabbed\n>>>"
	d.deliverOnce("4. multi-line paste with special characters", text, d.paste)
}

func (d *dRun) stepWaitingForUser() {
	r := d.r
	// A plain question: a finished turn, then is anything else signalled?
	since := time.Now()
	d.typeText("Ask me whether I prefer red or blue, then stop and wait for my answer. Do nothing else.")
	if _, ok := d.waitHook("Stop", since, 120*time.Second, nil); ok {
		r.info("5a. a plain question ends the turn with Stop, like a finished turn")
	}
	if e, ok := d.waitHook("PreToolUse", since, 5*time.Second, func(p map[string]any) bool { return p["tool_name"] == "AskUserQuestion" }); ok {
		r.info("5a. it asked through its question dialog (AskUserQuestion): %v", e.Payload["tool_input"])
	}
	r.info("5a. waiting 70s for a notification…")
	if e, ok := d.waitHook("Notification", since, 70*time.Second, nil); ok {
		r.pass("5a. Notification fired after %v: %v (%v)", e.At.Sub(since).Round(time.Second), e.Payload["message"], e.Payload["notification_type"])
	} else {
		r.info("5a. no Notification within 70s of an open question")
	}
	d.dump("5a. the open question")
	// A question dialog takes arrow keys, not text: Esc dismisses it.
	_, _ = d.proc.Write([]byte("\x1b"))
	time.Sleep(3 * time.Second)
	d.typeText("Never mind. Reply with exactly: ECHO-5")
	d.waitHook("Stop", time.Now().Add(-time.Second), 120*time.Second, nil)

	// Permission checks are off (the container is the boundary): a command that
	// changes something runs straight through, with no dialog and no signal.
	since = time.Now()
	d.typeText("Use the Bash tool to run: touch permission-test.txt")
	_, stopped := d.waitHook("Stop", since, 120*time.Second, nil)
	d.dump("5b. a change with permission checks off")
	_, err := os.Stat(filepath.Join(d.project, "permission-test.txt"))
	switch {
	case !stopped || err != nil:
		r.fail("5b. the change didn't run straight through (turn ended %v, file made %v): see the report", stopped, err == nil)
	case d.count("Notification", since, nil) > 0:
		r.fail("5b. it ran, but something asked for attention on the way (Notification): see the report")
	default:
		r.pass("5b. with permission checks off, a change runs with no dialog")
	}
}

func (d *dRun) stepToolTimeout(seconds int) {
	r := d.r
	since := time.Now()
	r.info("6. a tool call held open for %ds…", seconds)
	d.typeText(fmt.Sprintf("Call the wait tool of the foreman MCP server with seconds=%d, then reply with exactly what it returned.", seconds))
	stop, ok := d.waitHook("Stop", since, time.Duration(seconds+120)*time.Second, nil)
	d.dump("6. after the long tool call")
	post, posted := d.waitHook("PostToolUse", since, time.Second, func(p map[string]any) bool { return p["tool_name"] == "mcp__foreman__wait" })
	switch {
	case !ok:
		r.fail("6. no Stop within %ds", seconds+120)
	case posted && strings.Contains(fmt.Sprint(post.Payload["tool_response"]), "waited"):
		r.pass("6. a tool call held open %ds completed (turn ended %v after the request)", seconds, stop.At.Sub(since).Round(time.Second))
	default:
		r.fail("6. the %ds tool call didn't complete normally (timed out or errored: see the report)", seconds)
	}
}

func (d *dRun) stepRefreshMidSession(creds string) {
	r := d.r
	if err := setExpiry(creds, time.Now().Add(-time.Minute)); !r.check(err, "7. marked the shared token as expired, mid-session") {
		return
	}
	d.deliverOnce("7. a turn that needs a token refresh", "Reply with exactly: DELTA-4", d.typeText)
	if !d.said("DELTA-4") {
		r.fail("7. no answer after the forced expiry: the session couldn't refresh")
	}
	if after, err := readExpiry(creds); err == nil && after.After(time.Now()) {
		r.pass("7. the shared file holds the refreshed token (expires %s)", after.Format(time.RFC3339))
	} else {
		r.fail("7. the shared file still holds the expired token (%v)", err)
	}
}
