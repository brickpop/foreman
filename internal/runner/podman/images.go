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
	"sync/atomic"

	"github.com/brickpop/foreman/internal/runner"
)

// Images builds an image through Podman's API from a Containerfile, phase 1:
// one image for every project (wiki/product/architecture/tech-stack.md#agent-images).
// The tag is a hash of the Containerfile, so the image is rebuilt only when
// the file changes.
type Images struct {
	r             *Runner // reached through the engine, so a reconnection is followed
	name          string  // e.g. localhost/foreman/harness
	containerfile []byte
	// shared is the build as every Foreman on this machine sees it: the image is
	// the machine's, so one builds at a time and all show the same state.
	shared sharedBuild
	builds atomic.Int32 // builds this Foreman started, for tests
}

var _ runner.Images = (*Images)(nil)

// Images returns the image builder for the image name, built from
// containerfile; dir is where its build state is shared with the other
// Foremans on this machine (Foreman's home, internal/userdata).
func (r *Runner) Images(name string, containerfile []byte, dir string) *Images {
	key := strings.NewReplacer("/", "-", ":", "-").Replace(name)
	return &Images{r: r, name: name, containerfile: containerfile, shared: sharedBuild{dir: dir, key: key}}
}

// maxLog bounds the build log shown in Settings → System → Maintenance (its end).
const maxLog = 256 << 10

func (im *Images) ref() string {
	sum := sha256.Sum256(im.containerfile)
	return im.name + ":" + hex.EncodeToString(sum[:6])
}

// Ensure returns the image, building it first if it doesn't exist. While
// another Foreman builds it, Ensure waits, then finds it built.
func (im *Images) Ensure(ctx context.Context, _ string) (string, error) {
	unlock, err := im.shared.lock(ctx)
	if err != nil {
		return "", err
	}
	defer unlock()
	ref := im.ref()
	exists, err := im.exists(ctx, ref)
	if err != nil {
		return "", err
	}
	if exists {
		// Agents start on it: a failed Refresh before doesn't hold the
		// checklist on failed.
		return ref, im.shared.set("ready")
	}
	if err := im.run(ctx, ref, false); err != nil {
		return "", err
	}
	return ref, nil
}

// Refresh rebuilds without cache, pulling the latest base image.
func (im *Images) Refresh(ctx context.Context) error {
	unlock, err := im.shared.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return im.run(ctx, im.ref(), true)
}

// Status reports the image as every Foreman sees it: building (here or in
// another Foreman), failed, or else ready or missing as Podman says.
func (im *Images) Status(ctx context.Context) []runner.ImageStatus {
	state, log := im.shared.state()
	if state != "building" && state != "failed" {
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

func (im *Images) exists(ctx context.Context, ref string) (bool, error) {
	c, err := im.r.client()
	if err != nil {
		return false, err
	}
	err = c.call(ctx, "GET", "/images/"+url.PathEscape(ref)+"/exists", nil, nil, nil)
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
// build's output into the shared log. Called holding the lock.
func (im *Images) run(ctx context.Context, ref string, fresh bool) error {
	im.builds.Add(1)
	if err := im.shared.begin(); err != nil {
		return fmt.Errorf("building %s: %w", im.name, err)
	}
	if err := im.post(ctx, ref, fresh); err != nil {
		im.shared.appendLog("\n" + err.Error() + "\n")
		_ = im.shared.set("failed")
		return fmt.Errorf("building %s: %w", im.name, err)
	}
	return im.shared.set("ready")
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
	c, err := im.r.client()
	if err != nil {
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
	resp, err := c.http.Do(req)
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
		im.shared.appendLog(line.Stream)
		if line.Error != "" {
			return errors.New(strings.TrimSpace(line.Error))
		}
	}
}
