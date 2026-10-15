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
	Running string        `json:"running"` // the engine running now, "podman" or "direct": a change of the setting waits for a restart
	Engine  string        `json:"engine"`  // its state, e.g. "Podman 5.7.1 · running", or "direct (no isolation)"
	Podman  string        `json:"podman"`  // why Podman can't be chosen ("" when it is installed)
	Images  []ImageStatus `json:"images"`
}
