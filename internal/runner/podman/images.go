package podman

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/brickpop/foreman/internal/runner"
)

// Images builds an image through Podman's API from a Containerfile, phase 1:
// one image for every project (wiki/product/architecture/tech-stack.md#agent-images).
// The tag is a hash of the Containerfile, so the image is rebuilt only when
// the file changes.
type Images struct {
	c             *client
	name          string // e.g. localhost/foreman/harness
	containerfile []byte

	build sync.Mutex // one build at a time; concurrent Ensures share it

	mu     sync.Mutex // guards what Status reports
	state  string     // "", "missing", "building", "ready" or "failed"
	log    string
	builds int // builds started, for tests
}

var _ runner.Images = (*Images)(nil)

// Images returns the image builder for the image name, built from containerfile.
func (r *Runner) Images(name string, containerfile []byte) *Images {
	return &Images{c: r.c, name: name, containerfile: containerfile}
}

// maxLog bounds the build log kept for Settings → Maintenance (its end).
const maxLog = 256 << 10

func (im *Images) ref() string {
	sum := sha256.Sum256(im.containerfile)
	return im.name + ":" + hex.EncodeToString(sum[:6])
}

// Ensure returns the image, building it first if it doesn't exist.
func (im *Images) Ensure(ctx context.Context, _ string) (string, error) {
	im.build.Lock()
	defer im.build.Unlock()
	ref := im.ref()
	exists, err := im.exists(ctx, ref)
	if err != nil {
		return "", err
	}
	if exists {
		im.set("ready", "")
		return ref, nil
	}
	if err := im.run(ctx, ref, false); err != nil {
		return "", err
	}
	return ref, nil
}

// Refresh rebuilds without cache, pulling the latest base image.
func (im *Images) Refresh(ctx context.Context) error {
	im.build.Lock()
	defer im.build.Unlock()
	return im.run(ctx, im.ref(), true)
}

// Status reports the image's state; before anything was built or checked in
// this run, it asks Podman.
func (im *Images) Status(ctx context.Context) []runner.ImageStatus {
	im.mu.Lock()
	state, log := im.state, im.log
	im.mu.Unlock()
	if state == "" {
		state = "missing"
		if ok, err := im.exists(ctx, im.ref()); err == nil && ok {
			state = "ready"
		}
	}
	ref := ""
	if state == "ready" {
		ref = im.ref()
	}
	return []runner.ImageStatus{{Name: im.name, Ref: ref, State: state, Log: log}}
}

func (im *Images) set(state, log string) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.state = state
	if log != "" || state == "building" {
		im.log = log
	}
}

func (im *Images) appendLog(s string) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.log += s
	if len(im.log) > maxLog {
		im.log = im.log[len(im.log)-maxLog:]
	}
}

func (im *Images) exists(ctx context.Context, ref string) (bool, error) {
	err := im.c.call(ctx, "GET", "/images/"+url.PathEscape(ref)+"/exists", nil, nil, nil)
	var api *apiError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &api) && api.Status == http.StatusNotFound:
		return false, nil
	default:
		return false, err
	}
}

// run builds ref (and moves the name's latest tag to it), streaming the
// build's output into the log.
func (im *Images) run(ctx context.Context, ref string, fresh bool) error {
	im.mu.Lock()
	im.builds++
	im.mu.Unlock()
	im.set("building", "")
	err := im.post(ctx, ref, fresh)
	if err != nil {
		im.appendLog("\n" + err.Error() + "\n")
		im.set("failed", "")
		return fmt.Errorf("building %s: %w", im.name, err)
	}
	im.set("ready", "")
	return nil
}

func (im *Images) post(ctx context.Context, ref string, fresh bool) error {
	var buildContext bytes.Buffer
	tw := tar.NewWriter(&buildContext)
	if err := tw.WriteHeader(&tar.Header{Name: "Containerfile", Mode: 0o644, Size: int64(len(im.containerfile))}); err != nil {
		return err
	}
	if _, err := tw.Write(im.containerfile); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	labels, _ := json.Marshal(map[string]string{"foreman.image": im.name})
	q := url.Values{"t": {ref, im.name + ":latest"}, "dockerfile": {"Containerfile"}, "labels": {string(labels)}}
	if fresh {
		q.Set("nocache", "true")
		q.Set("pull", "true")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "http://podman"+apiPrefix+"/build?"+q.Encode(), &buildContext)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := im.c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &apiError{Method: "POST", Path: "/build", Status: resp.StatusCode, Message: errorMessage(body)}
	}
	// The answer is a stream of JSON lines: the build's output, then an error
	// or the image id.
	dec := json.NewDecoder(resp.Body)
	for {
		var line struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		if err := dec.Decode(&line); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		im.appendLog(line.Stream)
		if line.Error != "" {
			return errors.New(strings.TrimSpace(line.Error))
		}
	}
}
