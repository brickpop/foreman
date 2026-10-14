package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The phase-1 agent image (images/Containerfile), built through Podman's API:
// the first step of experiments B, D and E, and a check of the image builder.

const harnessImage = "localhost/foreman/harness"

func proveImage(r *report) {
	p := connectPodman(r)
	if p == nil {
		return
	}
	begin := time.Now()
	if !r.check(p.build("images", "Containerfile", harnessImage), "built "+harnessImage+" from images/Containerfile through the API") {
		return
	}
	r.info("build took %v", time.Since(begin).Round(time.Second))
	code, out, err := runToEnd(p, harnessImage, "image", "claude --version && wiki --version && go version && bun --version && git --version")
	if r.check(err, "ran a container from the image") {
		if code == 0 {
			r.pass("the tools are inside:\n%s", indent(out))
		} else {
			r.fail("a tool is missing (exit %d):\n%s", code, indent(out))
		}
	}
}

// build sends dir as the build context and streams the build's output.
func (p *podman) build(dir, containerfile, tag string) error {
	var ctx bytes.Buffer
	tw := tar.NewWriter(&ctx)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if err := tw.WriteHeader(&tar.Header{Name: rel, Mode: 0o644, Size: int64(len(data))}); err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	q := url.Values{"t": {tag}, "dockerfile": {containerfile}}
	resp, err := p.http.Post("http://d"+apiPrefix+"/build?"+q.Encode(), "application/x-tar", &ctx)
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
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		if err := dec.Decode(&line); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if line.Error != "" {
			return fmt.Errorf("%s", line.Error)
		}
		fmt.Print(line.Stream) // the build log, live
	}
}

func indent(s string) string {
	return "        " + strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n        ")
}
