package agentbin

import "testing"

func TestAnUnknownCPUTypeHasNoBinary(t *testing.T) {
	for _, arch := range []string{"", "s390x", "../embed.go", "amd64/../arm64"} {
		if Binary(arch) != nil {
			t.Errorf("Binary(%q) is not nil", arch)
		}
	}
}
