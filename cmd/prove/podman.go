package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// A minimal client for Podman's REST API (libpod) over its Unix socket: just
// what the experiments need. The real engine (runner/podman) is designed
// separately, once the experiments have answered.

const apiPrefix = "/v5.0.0/libpod"

type podman struct {
	socket string
	http   *http.Client
}

// findSocket locates Podman's API socket, starting the service if needed:
// FOREMAN_PODMAN_SOCKET if set; on macOS the podman machine's socket (starting
// the machine if it's stopped); on Linux the rootless socket (starting
// `podman system service` in the background if nothing listens).
func findSocket(r *report) (string, error) {
	if s := os.Getenv("FOREMAN_PODMAN_SOCKET"); s != "" {
		return s, nil
	}
	if runtime.GOOS == "darwin" {
		state, err := output("podman", "machine", "inspect", "--format", "{{.State}}")
		if err != nil {
			return "", fmt.Errorf("podman machine inspect: %w (is there a machine? `podman machine init`)", err)
		}
		if state != "running" {
			r.info("podman machine is %q: starting it", state)
			if out, err := exec.Command("podman", "machine", "start").CombinedOutput(); err != nil {
				return "", fmt.Errorf("podman machine start: %v: %s", err, out)
			}
		}
		return output("podman", "machine", "inspect", "--format", "{{.ConnectionInfo.PodmanSocket.Path}}")
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	sock := filepath.Join(dir, "podman", "podman.sock")
	if conn, err := net.Dial("unix", sock); err == nil {
		conn.Close()
		return sock, nil
	}
	r.info("no Podman service on %s: starting `podman system service`", sock)
	if err := exec.Command("podman", "system", "service", "--time=60", "unix://"+sock).Start(); err != nil {
		return "", fmt.Errorf("starting podman system service: %w", err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if conn, err := net.Dial("unix", sock); err == nil {
			conn.Close()
			return sock, nil
		}
	}
	return "", fmt.Errorf("the Podman service didn't come up on %s", sock)
}

func output(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func newPodman(socket string) *podman {
	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}
	return &podman{socket: socket, http: &http.Client{Transport: &http.Transport{DialContext: dial}}}
}

// call does one API request; out (if not nil) receives the decoded JSON body.
func (p *podman) call(method, path string, query url.Values, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	u := "http://d" + apiPrefix + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest(method, u, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (p *podman) version() (string, error) {
	var v struct{ Version string }
	err := p.call("GET", "/version", nil, nil, &v)
	return v.Version, err
}

// pull fetches an image; the API streams progress as JSON lines.
func (p *podman) pull(ref string) error {
	resp, err := p.http.Post("http://d"+apiPrefix+"/images/pull?reference="+url.QueryEscape(ref), "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, b)
	}
	dec := json.NewDecoder(resp.Body)
	for {
		var line struct {
			Error string `json:"error"`
		}
		if err := dec.Decode(&line); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if line.Error != "" {
			return errors.New(line.Error)
		}
	}
}

type mount struct {
	Destination string   `json:"destination"`
	Source      string   `json:"source"`
	Type        string   `json:"type"`
	Options     []string `json:"options,omitempty"`
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

// namespace is a user-namespace mode, e.g. keep-id with uid=1000,gid=1000: files
// the image's user (node, 1000) writes into mounts belong to the host user.
type namespace struct {
	Nsmode string `json:"nsmode"`
	Value  string `json:"value,omitempty"`
}

var keepID = &namespace{Nsmode: "keep-id", Value: "uid=1000,gid=1000"}

func (p *podman) create(spec createSpec) (string, error) {
	var out struct {
		ID       string   `json:"Id"`
		Warnings []string `json:"Warnings"`
	}
	err := p.call("POST", "/containers/create", nil, spec, &out)
	return out.ID, err
}

func (p *podman) start(id string) error {
	return p.call("POST", "/containers/"+id+"/start", nil, nil, nil)
}

func (p *podman) resize(id string, cols, rows int) error {
	q := url.Values{"w": {fmt.Sprint(cols)}, "h": {fmt.Sprint(rows)}}
	return p.call("POST", "/containers/"+id+"/resize", q, nil, nil)
}

func (p *podman) stop(id string, timeout int) error {
	return p.call("POST", "/containers/"+id+"/stop", url.Values{"timeout": {fmt.Sprint(timeout)}}, nil, nil)
}

func (p *podman) remove(id string) error {
	return p.call("DELETE", "/containers/"+id, url.Values{"force": {"true"}}, nil, nil)
}

type containerState struct {
	Running  bool
	ExitCode int
}

func (p *podman) inspect(id string) (containerState, error) {
	var out struct{ State containerState }
	err := p.call("GET", "/containers/"+id+"/json", nil, nil, &out)
	return out.State, err
}

// listByLabel returns the ids of the containers (running or not) carrying label=value.
func (p *podman) listByLabel(label, value string) ([]string, error) {
	filters, _ := json.Marshal(map[string][]string{"label": {label + "=" + value}})
	var out []struct {
		ID    string `json:"Id"`
		State string
	}
	err := p.call("GET", "/containers/json", url.Values{"all": {"true"}, "filters": {string(filters)}}, nil, &out)
	ids := make([]string, 0, len(out))
	for _, c := range out {
		ids = append(ids, c.ID)
	}
	return ids, err
}

// attach opens the container's terminal as a raw two-way stream. detachKeys is
// empty so that no key sequence (Podman's default is Ctrl-P Ctrl-Q) is ever
// swallowed: an agent's terminal must get every keystroke.
func (p *podman) attach(id string) (*stream, error) {
	conn, br, err := p.attachRaw(id)
	if err != nil {
		return nil, err
	}
	s := &stream{conn: conn}
	go s.pump(br)
	return s, nil
}

// attachRaw opens the attach stream: writes go to the conn, reads come from the reader.
func (p *podman) attachRaw(id string) (net.Conn, *bufio.Reader, error) {
	conn, err := net.Dial("unix", p.socket)
	if err != nil {
		return nil, nil, err
	}
	path := apiPrefix + "/containers/" + id + "/attach?stdin=true&stdout=true&stderr=true&stream=true&detachKeys="
	req := "POST " + path + " HTTP/1.1\r\nHost: d\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n"
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
		conn.Close()
		return nil, nil, fmt.Errorf("attach: HTTP %d", resp.StatusCode)
	}
	return conn, br, nil
}

// stream is an attached terminal: everything read is kept, so checks can wait
// for a string to appear and measure when it did.
type stream struct {
	conn net.Conn
	mu   sync.Mutex
	buf  []byte
}

func (s *stream) pump(r io.Reader) {
	chunk := make([]byte, 4096)
	for {
		n, err := r.Read(chunk)
		s.mu.Lock()
		s.buf = append(s.buf, chunk[:n]...)
		s.mu.Unlock()
		if err != nil {
			return
		}
	}
}

func (s *stream) write(text string) error {
	_, err := s.conn.Write([]byte(text))
	return err
}

// waitFor waits for want to appear in the output after offset `from` and
// returns the offset just past it.
func (s *stream) waitFor(want string, from int, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		i := bytes.Index(s.buf[min(from, len(s.buf)):], []byte(want))
		n := len(s.buf)
		s.mu.Unlock()
		if i >= 0 {
			return from + i + len(want), nil
		}
		if time.Now().After(deadline) {
			s.mu.Lock()
			tail := s.buf[max(0, n-200):]
			s.mu.Unlock()
			return n, fmt.Errorf("%q didn't appear within %v (last output: %q)", want, timeout, tail)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (s *stream) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.buf)
}

func (s *stream) close() { s.conn.Close() }

// wait blocks until the container exits and returns its exit code.
func (p *podman) wait(id string) (int, error) {
	var code int
	err := p.call("POST", "/containers/"+id+"/wait", url.Values{"condition": {"exited"}}, nil, &code)
	return code, err
}

// logs returns a container's output. Without a terminal, Podman frames each
// chunk with an 8-byte header (stream, 3 zero bytes, big-endian length).
func (p *podman) logs(id string) (string, error) {
	resp, err := p.http.Get("http://d" + apiPrefix + "/containers/" + id + "/logs?stdout=true&stderr=true")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for len(data) >= 8 {
		n := int(data[4])<<24 | int(data[5])<<16 | int(data[6])<<8 | int(data[7])
		if 8+n > len(data) {
			break
		}
		out.Write(data[8 : 8+n])
		data = data[8+n:]
	}
	return out.String(), nil
}
