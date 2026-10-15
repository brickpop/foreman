// Command foreman-agent runs inside every agent container and links its
// harness to Foreman (wiki/product/architecture/team-tools.md#transport):
//
//	foreman-agent mcp           a stdio MCP server, "foreman", relaying the team tools
//	foreman-agent hook <event>  forwards a harness hook event (its JSON on stdin)
//
// Where Foreman is and who this instance is come from the environment:
// FOREMAN_AGENT_URL and FOREMAN_AGENT_TOKEN. Foreman mounts this binary into
// the container; it is never built into the image.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	link := agentlink.Client{URL: getenv("FOREMAN_AGENT_URL"), Token: getenv("FOREMAN_AGENT_TOKEN")}
	ctx := context.Background()
	switch {
	case len(args) == 1 && args[0] == "mcp":
		transport := &mcp.IOTransport{Reader: io.NopCloser(stdin), Writer: nopCloser{stdout}}
		if err := serve(ctx, link, transport); err != nil {
			fmt.Fprintln(stderr, "foreman-agent:", err)
			return 1
		}
		return 0
	case len(args) == 2 && args[0] == "hook":
		payload, err := io.ReadAll(stdin)
		if err == nil {
			err = link.Hook(ctx, args[1], payload)
		}
		if err != nil {
			fmt.Fprintln(stderr, "foreman-agent: hook:", err)
		}
		return 0 // a hook never fails the harness, even with Foreman unreachable
	default:
		fmt.Fprintln(stderr, "usage: foreman-agent mcp | hook <event>")
		return 2
	}
}

// serve offers the tools Foreman lists for this instance, relaying each call.
func serve(ctx context.Context, link agentlink.Client, transport mcp.Transport) error {
	tools, err := link.Tools(ctx)
	if err != nil {
		return fmt.Errorf("listing the team tools: %w", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "foreman", Version: "1"}, nil)
	for _, t := range tools {
		server.AddTool(&mcp.Tool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema},
			func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				res, err := link.Call(ctx, t.Name, req.Params.Arguments)
				if err != nil {
					// The agent reads it, and can try again once Foreman is back.
					res = agentlink.Result{Text: "Foreman is unreachable: " + err.Error(), Error: true}
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: res.Text}}, IsError: res.Error}, nil
			})
	}
	err = server.Run(ctx, transport)
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return nil // the harness closed the session
	}
	return err
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }
