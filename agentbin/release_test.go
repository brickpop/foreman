//go:build release

package agentbin

import (
	"bytes"
	"debug/elf"
	"testing"
)

// Each embedded foreman-agent is a static Linux program for its CPU type: a
// mix-up would mount a binary the container can't run.
func TestBothCPUTypesAreEmbedded(t *testing.T) {
	for arch, machine := range map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64} {
		data := Binary(arch)
		f, err := elf.NewFile(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: not an ELF program: %v", arch, err)
		}
		if f.Machine != machine || f.OSABI != elf.ELFOSABI_NONE && f.OSABI != elf.ELFOSABI_LINUX {
			t.Errorf("%s: built for %v (%v)", arch, f.Machine, f.OSABI)
		}
		for _, p := range f.Progs {
			if p.Type == elf.PT_INTERP {
				t.Errorf("%s: dynamically linked; the image may lack its loader", arch)
			}
		}
	}
}
