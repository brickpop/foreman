package podman

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// client speaks Podman's REST API (libpod) over its Unix socket, with plain
// net/http: no bindings module, no `podman` CLI
// (wiki/product/architecture/session-runner.md#podman-engine-production).
type client struct {
	socket string
	http   *http.Client
}

const apiPrefix = "/v5.0.0/libpod"

func newClient(socket string) *client {
	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}
	return &client{socket: socket, http: &http.Client{Transport: &http.Transport{DialContext: dial}}}
}

// call does one API request; out (if not nil) receives the decoded JSON body.
func (c *client) call(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	u := "http://podman" + apiPrefix + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return &apiError{Method: method, Path: path, Status: resp.StatusCode, Message: errorMessage(data)}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// apiError is an error answer from Podman.
type apiError struct {
	Method, Path string
	Status       int
	Message      string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("podman: %s %s: %d %s", e.Method, e.Path, e.Status, e.Message)
}

// errorMessage keeps Podman's own message ({"cause":…,"message":…}).
func errorMessage(body []byte) string {
	var e struct{ Message string }
	if json.Unmarshal(body, &e) == nil && e.Message != "" {
		return e.Message
	}
	return strings.TrimSpace(string(body))
}

func (c *client) version(ctx context.Context) (string, error) {
	var v struct{ Version string }
	err := c.call(ctx, "GET", "/version", nil, nil, &v)
	return v.Version, err
}

type mount struct {
	Destination string   `json:"destination"`
	Source      string   `json:"source"`
	Type        string   `json:"type"`
	Options     []string `json:"options,omitempty"`
}

// namespace is a user-namespace mode, e.g. keep-id.
type namespace struct {
	Nsmode string `json:"nsmode"`
	Value  string `json:"value,omitempty"`
}

type createSpec struct {
	Name     string            `json:"name"`
	Image    string            `json:"image"`
	Command  []string          `json:"command"`
	Terminal bool              `json:"terminal"`
	Stdin    bool              `json:"stdin"`
	WorkDir  string            `json:"work_dir,omitempty"`
	Labels   map[string]string `json:"labels"`
	Mounts   []mount           `json:"mounts,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	Userns   *namespace        `json:"userns,omitempty"`
}

func (c *client) create(ctx context.Context, spec createSpec) (string, error) {
	var out struct {
		ID string `json:"Id"`
	}
	err := c.call(ctx, "POST", "/containers/create", nil, spec, &out)
	return out.ID, err
}

func (c *client) start(ctx context.Context, id string) error {
	return c.call(ctx, "POST", "/containers/"+id+"/start", nil, nil, nil)
}

func (c *client) resize(ctx context.Context, id string, cols, rows uint16) error {
	q := url.Values{"w": {fmt.Sprint(cols)}, "h": {fmt.Sprint(rows)}}
	return c.call(ctx, "POST", "/containers/"+id+"/resize", q, nil, nil)
}

// stop asks the container to end and kills it after timeout seconds.
func (c *client) stop(ctx context.Context, id string, timeout int) error {
	return c.call(ctx, "POST", "/containers/"+id+"/stop", url.Values{"timeout": {fmt.Sprint(timeout)}}, nil, nil)
}

func (c *client) remove(ctx context.Context, id string) error {
	return c.call(ctx, "DELETE", "/containers/"+id, url.Values{"force": {"true"}}, nil, nil)
}

// wait blocks until the container has exited and returns its exit code.
func (c *client) wait(ctx context.Context, id string) (int, error) {
	var code int
	err := c.call(ctx, "POST", "/containers/"+id+"/wait", url.Values{"condition": {"exited"}}, nil, &code)
	return code, err
}

type listed struct {
	ID     string            `json:"Id"`
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
}

// listRunning returns the running containers carrying label=value.
func (c *client) listRunning(ctx context.Context, label, value string) ([]listed, error) {
	filters, _ := json.Marshal(map[string][]string{"label": {label + "=" + value}, "status": {"running"}})
	var out []listed
	err := c.call(ctx, "GET", "/containers/json", url.Values{"filters": {string(filters)}}, nil, &out)
	return out, err
}

// attach opens the container's terminal as a raw two-way stream: writes go to
// the conn, reads come from the reader. detachKeys is empty so that no key
// sequence (Podman's default is Ctrl-P Ctrl-Q) is ever swallowed.
func (c *client) attach(ctx context.Context, id string) (net.Conn, *bufio.Reader, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", c.socket)
	if err != nil {
		return nil, nil, err
	}
	path := apiPrefix + "/containers/" + id + "/attach?stdin=true&stdout=true&stderr=true&stream=true&detachKeys="
	req := "POST " + path + " HTTP/1.1\r\nHost: podman\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, nil, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		conn.Close()
		return nil, nil, &apiError{Method: "POST", Path: "/containers/" + id + "/attach", Status: resp.StatusCode, Message: errorMessage(body)}
	}
	return conn, br, nil
}
