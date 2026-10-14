package main

import (
	"context"
	"fmt"
	"io"

	"github.com/brickpop/foreman/images"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/runner/direct"
	"github.com/brickpop/foreman/internal/runner/podman"
)

// harnessImage is the phase 1 agent image (wiki/product/architecture/tech-stack.md#agent-images).
const harnessImage = "localhost/foreman/harness"

// engine picks the execution engine and its image builder from the `engine`
// setting (wiki/product/models/workspace-settings.md). close ends what it
// started (a Podman service on Linux); containers keep running.
func engine(ctx context.Context, choice string, stdout io.Writer) (runner.Runner, runner.Images, func(), error) {
	if choice == "direct" {
		fmt.Fprintln(stdout, "WARNING: engine = \"direct\": agents run as plain processes on this machine, with NO isolation (development only)")
		return direct.Runner{}, direct.Images{}, func() {}, nil
	}
	r, err := podman.New(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("podman: %w", err)
	}
	return r, r.Images(harnessImage, images.Harness), r.Close, nil
}
