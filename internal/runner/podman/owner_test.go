//go:build podman

package podman

import (
	"os"
	"syscall"
)

func fileOwner(info os.FileInfo) int {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return int(st.Uid)
	}
	return -1
}
