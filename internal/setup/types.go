package setup

// Wire types, generated into the UI by tygo (tygo.yaml).

// Step is one row of the setup checklist (wiki/product/features/first-run.md).
type Step struct {
	Kind    string       `json:"kind"`              // "podman", "machine", "running", "image", "init", "profile", "team" or "project"
	Subject string       `json:"subject"`           // the profile (profile) or project (repo); "" otherwise
	State   string       `json:"state"`             // "done", "todo", "working" or "failed"
	Detail  string       `json:"detail"`            // what it waits for, or why it failed
	Log     string       `json:"log"`               // a machine creation's or an image build's output (its end)
	Machine *MachineSize `json:"machine,omitempty"` // a new machine's sizes (kind machine, while todo)
}

// MachineSize is the size a new Podman machine may have: memory in GiB.
type MachineSize struct {
	MinMemory int `json:"minMemory"`
	MaxMemory int `json:"maxMemory"` // below minMemory: the Mac is too small for any
	Memory    int `json:"memory"`    // prefilled
	MaxCPUs   int `json:"maxCpus"`
	CPUs      int `json:"cpus"` // prefilled
}
