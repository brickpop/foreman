package runner

// Wire types, generated into the UI by tygo (tygo.yaml).

// ImageStatus is one image's state.
type ImageStatus struct {
	Name  string `json:"name"`  // e.g. localhost/foreman/harness
	Ref   string `json:"ref"`   // the exact tag Ensure returns, "" when there is none
	State string `json:"state"` // "missing", "building", "ready" or "failed"
	Log   string `json:"log"`   // the last build's output (its end, when long)
}

// Maintenance is the engine's state and its images, for Settings → Maintenance.
type Maintenance struct {
	Engine string        `json:"engine"` // e.g. "Podman 5.7.1 · running", or "direct (no isolation)"
	Images []ImageStatus `json:"images"`
}
