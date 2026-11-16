package userdata

import (
	"testing"
)

// ~/.foreman on macOS and Linux alike, whatever XDG_DATA_HOME says.
func TestDir(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_DATA_HOME", "/data/share")
	if got, err := Dir(); err != nil || got != "/home/u/.foreman" {
		t.Errorf("%s %v", got, err)
	}
}
