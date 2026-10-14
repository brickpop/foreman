// Command proveagent is a stand-in for foreman-agent in the Prove epic's
// experiments (wiki/backlog/epics/prove.md); it runs inside agent containers.
//
//	proveagent mcp           a local (stdio) MCP server with two tools: "ping", and "wait" (blocks for some seconds)
//	proveagent hook <event>  forwards a harness hook event (JSON on stdin) to the host
//
// Configuration comes from the environment: FOREMAN_PROVE_SECRET (what ping
// answers), FOREMAN_PROVE_LOG (a file each call is appended to),
// FOREMAN_AGENT_URL and FOREMAN_AGENT_TOKEN (where hooks go). Throwaway:
// removed when the epic ends; the real foreman-agent is designed separately.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: proveagent mcp | hook <event>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "mcp":
		serveMCP(os.Stdin, os.Stdout)
	case "hook":
		event := ""
		if len(os.Args) > 2 {
			event = os.Args[2]
		}
		if err := forwardHook(event, os.Stdin); err != nil {
			logLine("hook %s: %v", event, err)
			os.Exit(0) // never break the harness because the host is unreachable
		}
	default:
		os.Exit(2)
	}
}

func logLine(format string, a ...any) {
	path := os.Getenv("FOREMAN_PROVE_LOG")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().UTC().Format(time.RFC3339Nano)+" "+format+"\n", a...)
}

type request struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// serveMCP speaks MCP's stdio transport: one JSON-RPC message per line.
func serveMCP(in io.Reader, out io.Writer) {
	lines := bufio.NewScanner(in)
	lines.Buffer(make([]byte, 1<<20), 1<<20)
	enc := json.NewEncoder(out)
	var mu sync.Mutex // replies come from waits running in the background too
	var waits sync.WaitGroup
	defer waits.Wait()
	reply := func(id json.RawMessage, result any) {
		mu.Lock()
		defer mu.Unlock()
		_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	for lines.Scan() {
		var req request
		if json.Unmarshal(lines.Bytes(), &req) != nil {
			continue
		}
		logLine("mcp %s", req.Method)
		switch req.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(req.Params, &p)
			reply(req.ID, map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "foreman", "version": "prove"},
			})
		case "ping":
			reply(req.ID, map[string]any{})
		case "tools/list":
			reply(req.ID, map[string]any{"tools": []any{
				map[string]any{
					"name":        "ping",
					"description": "Answers with a secret word, proving the call reached Foreman.",
					"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
				},
				map[string]any{
					"name":        "wait",
					"description": "Blocks for the given number of seconds, then answers. For testing how long a tool call may take.",
					"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
						"seconds": map[string]any{"type": "number"},
					}, "required": []string{"seconds"}},
				},
			}})
		case "tools/call":
			var call struct {
				Name      string `json:"name"`
				Arguments struct {
					Seconds float64 `json:"seconds"`
				} `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &call)
			logLine("tool %s called", call.Name)
			if call.Name == "wait" {
				waits.Add(1)
				go func(id json.RawMessage, d time.Duration) {
					defer waits.Done()
					time.Sleep(d)
					logLine("tool wait answered after %v", d)
					reply(id, map[string]any{"content": []any{map[string]any{
						"type": "text", "text": fmt.Sprintf("waited %v", d),
					}}})
				}(req.ID, time.Duration(call.Arguments.Seconds*float64(time.Second)))
				continue
			}
			reply(req.ID, map[string]any{"content": []any{map[string]any{
				"type": "text", "text": "pong-" + os.Getenv("FOREMAN_PROVE_SECRET"),
			}}})
		default:
			if len(req.ID) > 0 { // a request, not a notification
				mu.Lock()
				_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID,
					"error": map[string]any{"code": -32601, "message": "method not found"}})
				mu.Unlock()
			}
		}
	}
}

// forwardHook posts the hook's JSON payload to the host, as foreman-agent will.
func forwardHook(event string, in io.Reader) error {
	payload, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	logLine("hook %s %s", event, bytes.TrimSpace(payload))
	url := os.Getenv("FOREMAN_AGENT_URL")
	if url == "" {
		return nil
	}
	req, err := http.NewRequest("POST", url+"/hook/"+event, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("FOREMAN_AGENT_TOKEN"))
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
