package harness

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Shared by the adapters' Describe and Asks
// (wiki/product/architecture/harness-adapters.md#activity-lines).

// Relative is path as the CEO reads it: relative to the working directory
// cwd when it is an absolute path inside it, else as it is.
func Relative(path, cwd string) string {
	if rel, err := filepath.Rel(cwd, path); cwd != "" && err == nil && filepath.IsAbs(path) && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

// MCPTool names a call of an MCP tool, mcp__<server>__<tool>: "Team tool:
// <tool>" for Foreman's (the team tools), "<server>: <tool>" for another's;
// false when name isn't one.
func MCPTool(name string) (string, bool) {
	server, tool, ok := strings.Cut(strings.TrimPrefix(name, "mcp__"), "__")
	if !ok || !strings.HasPrefix(name, "mcp__") {
		return "", false
	}
	if server == "foreman" {
		return "Team tool: " + tool, true
	}
	return server + ": " + tool, true
}

// Questions is what a question tool asks (Claude Code's AskUserQuestion,
// Codex's request_user_input, which take the same input): each question
// with its choices, "Q (A / B) · Q2"; "" when the input has none.
func Questions(input json.RawMessage) string {
	var in struct {
		Questions []struct {
			Question string `json:"question"`
			Options  []struct {
				Label string `json:"label"`
			} `json:"options"`
		} `json:"questions"`
	}
	if json.Unmarshal(input, &in) != nil {
		return ""
	}
	var asks []string
	for _, q := range in.Questions {
		var labels []string
		for _, o := range q.Options {
			labels = append(labels, o.Label)
		}
		if len(labels) > 0 {
			asks = append(asks, fmt.Sprintf("%s (%s)", q.Question, strings.Join(labels, " / ")))
		} else {
			asks = append(asks, q.Question)
		}
	}
	return strings.Join(asks, " · ")
}
