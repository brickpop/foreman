package agentlink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client is foreman-agent's side of the link: the address and the token
// come from the container's environment (FOREMAN_AGENT_URL, FOREMAN_AGENT_TOKEN).
type Client struct {
	URL   string // e.g. http://host.containers.internal:8889
	Token string
	HTTP  *http.Client
}

// hookTimeout keeps a hook from ever holding up the harness for long.
const hookTimeout = 5 * time.Second

// Tools lists the tools the caller may use.
func (c Client) Tools(ctx context.Context) ([]Tool, error) {
	var tools []Tool
	return tools, c.do(ctx, "GET", "/tools", nil, &tools)
}

// Call runs a tool; a refusal comes back as a Result with Error set.
func (c Client) Call(ctx context.Context, name string, args json.RawMessage) (Result, error) {
	var res Result
	return res, c.do(ctx, "POST", "/tools/"+url.PathEscape(name), args, &res)
}

// Hook forwards a hook event's payload.
func (c Client) Hook(ctx context.Context, event string, payload []byte) error {
	ctx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()
	return c.do(ctx, "POST", "/hooks/"+url.PathEscape(event), payload, nil)
}

func (c Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.URL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("agent link: %s %s: %s", method, path, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
