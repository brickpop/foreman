// Command fakeharness stands in for a coding harness in tests and development:
// it prints a coloured banner, shows a prompt, and answers each input line.
// Commands: `spam N` prints N numbered lines, `exit N` exits with code N,
// `tool <name> <json>` calls a team tool (`tool ask {"teammate":"tove","request":"hi"}`).
// Anything else is echoed back; a line ending in `\` continues on the next
// (as in Claude Code), so one input can span lines. An argument is the
// session's opener, answered first (as Claude Code's prompt argument). It
// writes plain "\n", as real programs do (the terminal adds the "\r").
//
// With Foreman's agent link in its environment (FOREMAN_AGENT_URL and
// FOREMAN_AGENT_TOKEN) it reports, as Claude Code's hooks do, when it starts
// on an input (UserPromptSubmit) and when it is back at its prompt (Stop), so
// messages reach it when it is idle. It grows with the features that need it;
// see wiki/product/architecture/tech-stack.md.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/brickpop/foreman/internal/agentlink"
)

func main() {
	var link *agentlink.Client
	if url := os.Getenv("FOREMAN_AGENT_URL"); url != "" {
		link = &agentlink.Client{URL: url, Token: os.Getenv("FOREMAN_AGENT_TOKEN")}
	}
	os.Exit(run(strings.Join(os.Args[1:], " "), os.Stdin, os.Stdout, link))
}

const prompt = "\x1b[38;5;214m❯\x1b[0m "

// run is the harness loop; it returns the exit code. link is nil without Foreman.
func run(opener string, in io.Reader, out io.Writer, link *agentlink.Client) int {
	ctx := context.Background()
	hook := func(event string) {
		if link != nil {
			_ = link.Hook(ctx, event, []byte("{}"))
		}
	}
	fmt.Fprint(out, "\x1b[1mfake harness\x1b[0m · \x1b[32mready\x1b[0m\n"+prompt)
	hook("SessionStart")
	if opener != "" {
		hook("UserPromptSubmit")
		fmt.Fprintf(out, "%s\nyou said: %s\n%s", opener, opener, prompt)
		hook("Stop")
	}
	lines := bufio.NewScanner(in)
	var held []string // the lines so far of an input continued with a trailing `\`
	for lines.Scan() {
		if text, more := strings.CutSuffix(lines.Text(), "\\"); more {
			held = append(held, text)
			continue
		}
		line := strings.TrimSpace(strings.Join(append(held, lines.Text()), "\n"))
		held = nil
		hook("UserPromptSubmit")
		cmd, arg, _ := strings.Cut(line, " ")
		switch cmd {
		case "exit":
			code, _ := strconv.Atoi(arg)
			fmt.Fprintf(out, "bye (%d)\n", code)
			return code
		case "spam":
			n, _ := strconv.Atoi(arg)
			for i := 1; i <= n; i++ {
				fmt.Fprintf(out, "line %d\n", i)
			}
		case "tool":
			fmt.Fprintln(out, callTool(ctx, link, arg))
		case "":
		default:
			fmt.Fprintf(out, "you said: %s\n", line)
		}
		fmt.Fprint(out, prompt)
		hook("Stop")
	}
	return 0
}

// callTool runs `<name> <json arguments>` through the agent link.
func callTool(ctx context.Context, link *agentlink.Client, arg string) string {
	if link == nil {
		return "tool: no agent link"
	}
	name, args, _ := strings.Cut(arg, " ")
	if args == "" {
		args = "{}"
	}
	if !json.Valid([]byte(args)) {
		return "tool: the arguments aren't JSON"
	}
	res, err := link.Call(ctx, name, json.RawMessage(args))
	if err != nil {
		return "tool: " + err.Error()
	}
	if res.Error {
		return "tool error: " + res.Text
	}
	return res.Text
}
