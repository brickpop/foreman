package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// The stub must speak MCP well enough for a real harness: answer initialize,
// list its tool, answer a call, and stay quiet on notifications.
func TestServeMCP(t *testing.T) {
	t.Setenv("FOREMAN_PROVE_SECRET", "s3")
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"ping","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"nope"}`,
		`not json`,
	}, "\n")
	var out bytes.Buffer
	serveMCP(strings.NewReader(in), &out)
	var replies []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("bad reply %q: %v", line, err)
		}
		replies = append(replies, m)
	}
	if len(replies) != 4 {
		t.Fatalf("%d replies, want 4 (no reply to the notification or to garbage): %s", len(replies), out.String())
	}
	if v := replies[0]["result"].(map[string]any)["protocolVersion"]; v != "2025-06-18" {
		t.Errorf("initialize didn't echo the protocol version: %v", v)
	}
	if !strings.Contains(out.String(), `"name":"ping"`) {
		t.Error("tools/list doesn't list ping")
	}
	if !strings.Contains(out.String(), `"text":"pong-s3"`) {
		t.Error("tools/call didn't answer with the secret")
	}
	if _, ok := replies[3]["error"]; !ok {
		t.Error("an unknown method got no error")
	}
}
