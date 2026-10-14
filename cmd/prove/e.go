package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Experiment E (wiki/backlog/prove-e-prompt-and-mcp-config.md), Claude Code
// only for now: does the harness take its session prompt from a CLAUDE.md in
// its own config dir, and use a local (stdio) MCP server configured for it,
// here the foreman-agent stand-in? Needs the login from `just prove-b-login`.

const eCodeword = "PINEAPPLE-42"

func proveE(r *report) {
	p := connectPodman(r)
	if p == nil {
		return
	}
	cwd, _ := os.Getwd()
	creds := filepath.Join(credentialsDir(), ".credentials.json")
	if _, err := os.Stat(creds); err != nil {
		r.fail("no login yet: run `just prove-b-login` first")
		return
	}
	agent := filepath.Join(cwd, ".dev", "prove", "foreman-agent")
	if _, err := os.Stat(agent); err != nil {
		r.fail("the agent stub isn't built (the prove-e recipe builds it): %v", err)
		return
	}
	removeLabelled(r, p, "e")

	// The instance's private config dir: its prompt and its MCP config.
	private := filepath.Join(cwd, ".dev", "prove", "e-config")
	_ = os.RemoveAll(private)
	if !r.check(os.MkdirAll(private, 0o700), "created a private config dir for the instance") {
		return
	}
	prompt := "# Session prompt (written by Foreman)\n\nYou are Tove, an engineer on team Atlas.\n" +
		"When anyone asks for the codeword, the codeword is " + eCodeword + ".\n"
	mcp, _ := json.MarshalIndent(map[string]any{"mcpServers": map[string]any{"foreman": map[string]any{
		"command": "/opt/foreman-agent", "args": []string{"mcp"},
	}}}, "", "  ")
	secret := newToken()[:8]
	if !r.check(os.WriteFile(filepath.Join(private, "CLAUDE.md"), []byte(prompt), 0o600), "wrote the session prompt as CLAUDE.md") ||
		!r.check(os.WriteFile(filepath.Join(private, "mcp.json"), mcp, 0o600), "wrote the MCP config pointing at foreman-agent (stdio)") {
		return
	}

	question := "What is the codeword? Then call the ping tool of the foreman MCP server and tell me exactly what it returned. Answer in two short lines."
	id, err := p.create(createSpec{
		Name:  "foreman-prove-e-" + newToken()[:8],
		Image: harnessImage,
		Command: []string{"claude", "-p", question,
			"--mcp-config", configDir + "/mcp.json", "--strict-mcp-config",
			"--allowedTools", "mcp__foreman__ping"},
		Env: map[string]string{
			"CLAUDE_CONFIG_DIR":    configDir,
			"FOREMAN_PROVE_SECRET": secret,
			"FOREMAN_PROVE_LOG":    configDir + "/agent.log",
		},
		Labels: map[string]string{"foreman.prove": "e"},
		Mounts: []mount{
			{Destination: configDir, Source: private, Type: "bind", Options: bindOptions()},
			// The shared login: only this file, mounted into the private dir.
			{Destination: configDir + "/.credentials.json", Source: creds, Type: "bind", Options: bindOptions()},
			{Destination: "/opt/foreman-agent", Source: agent, Type: "bind", Options: []string{"ro"}},
		},
		Userns: keepID,
	})
	if !r.check(err, "created the container") {
		return
	}
	defer func() { _ = p.remove(id) }()
	if !r.check(p.start(id), "started `claude -p` in it") {
		return
	}
	code, err := p.wait(id)
	if !r.check(err, "waited for it to finish") {
		return
	}
	out, _ := p.logs(id)
	r.info("Claude answered (exit %d):\n%s", code, indent(out))
	agentLog, _ := os.ReadFile(filepath.Join(private, "agent.log"))

	if strings.Contains(out, eCodeword) {
		r.pass("the session prompt was read: it knew the codeword from CLAUDE.md")
	} else {
		r.fail("it didn't know the codeword: CLAUDE.md in CLAUDE_CONFIG_DIR wasn't used as the session prompt")
	}
	if strings.Contains(string(agentLog), "tools/list") {
		r.pass("the harness started foreman-agent as a stdio MCP server and listed its tools")
	} else {
		r.fail("foreman-agent was never asked for its tools (log: %q)", strings.TrimSpace(string(agentLog)))
	}
	if strings.Contains(out, "pong-"+secret) && strings.Contains(string(agentLog), "tool ping called") {
		r.pass("it called the tool and reported its answer (pong-%s)", secret)
	} else {
		r.fail("the tool's answer pong-%s didn't come back", secret)
	}
	fmt.Println()
}
