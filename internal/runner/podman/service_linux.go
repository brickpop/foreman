package podman

import (
	"os/exec"
	"syscall"
)

// ownGroup keeps the terminal's Ctrl-C from the Podman service Foreman starts,
// so Foreman can still stop its containers through it as it exits; the
// service still ends if Foreman is killed.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
}
