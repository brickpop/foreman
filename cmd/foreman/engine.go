package main

import (
	"fmt"
	"io"

	containerfiles "github.com/brickpop/foreman/images"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/runner/direct"
	"github.com/brickpop/foreman/internal/runner/podman"
)

// harnessImage is the phase 1 agent image (wiki/product/architecture/tech-stack.md#agent-images).
const harnessImage = "localhost/foreman/harness"

// engine picks the execution engine and its image builder from the `engine`
// setting (wiki/product/models/global-settings.md). pod is the Podman
// engine, not connected yet (the setup watcher reaches it), or nil for the
// Direct engine. images is where the images' build state is shared with the
// other Foremans on this machine (another home's, such as a test's).
func engine(choice, images string, stdout io.Writer) (r runner.Runner, im runner.Images, pod *podman.Runner) {
	if choice == "direct" {
		fmt.Fprintln(stdout, "WARNING: engine = \"direct\": agents run as plain processes on this machine, with NO isolation (development only)")
		return direct.Runner{}, direct.Images{}, nil
	}
	pod = podman.Open()
	return pod, pod.Images(harnessImage, containerfiles.Harness, images), pod
}
