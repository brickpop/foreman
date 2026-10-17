//go:build !linux

package podman

import "os/exec"

// ownGroup is Linux's: elsewhere Foreman never starts the Podman service.
func ownGroup(*exec.Cmd) {}
