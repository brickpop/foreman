package podman

import (
	"reflect"
	"testing"

	"github.com/brickpop/foreman/internal/runner"
)

func TestContainerNamesAreValidPodmanNames(t *testing.T) {
	for in, want := range map[string]string{
		"mara@ledger":     "foreman-mara-ledger",
		"tove@ledger-web": "foreman-tove-ledger-web",
		"@odd name!":      "foreman-odd-name",
		"x.y_z":           "foreman-x.y_z",
	} {
		if got := containerName(in); got != want {
			t.Errorf("containerName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateSpec(t *testing.T) {
	spec := runner.Spec{
		Name: "mara@ledger", Image: "localhost/foreman/harness", Command: []string{"claude"},
		Dir: "/ws/ledger/.sandboxes/mara",
		Env: []string{"CLAUDE_CONFIG_DIR=/home/node/.claude", "A=b=c", "malformed"},
		Mounts: []runner.Mount{
			{Source: "/ws/ledger/.sandboxes/mara", Target: "/ws/ledger/.sandboxes/mara"},
			{Source: "/ws/ledger/ledger-api/.git", Target: "/ws/ledger/ledger-api/.git", ReadOnly: true},
		},
		Labels: map[string]string{"foreman.instance": "mara@ledger"},
	}
	for _, selinux := range []bool{false, true} {
		cs := (&Runner{selinux: selinux}).createSpec(spec)
		if cs.Name != "foreman-mara-ledger" || !cs.Terminal || !cs.Stdin || cs.WorkDir != spec.Dir || cs.Userns != imageUser {
			t.Errorf("spec %+v", cs)
		}
		wantEnv := map[string]string{"TERM": "xterm-256color", "CLAUDE_CONFIG_DIR": "/home/node/.claude", "A": "b=c"}
		if !reflect.DeepEqual(cs.Env, wantEnv) {
			t.Errorf("env %v", cs.Env)
		}
		if cs.Labels[managedLabel] != "true" || cs.Labels["foreman.instance"] != "mara@ledger" {
			t.Errorf("labels %v", cs.Labels)
		}
		rw, ro := []string{"rw"}, []string{"ro"}
		if selinux {
			rw, ro = []string{"rw", "z"}, []string{"ro", "z"}
		}
		if !reflect.DeepEqual(cs.Mounts[0].Options, rw) || !reflect.DeepEqual(cs.Mounts[1].Options, ro) || cs.Mounts[1].Destination != spec.Mounts[1].Target {
			t.Errorf("selinux %v: mounts %+v", selinux, cs.Mounts)
		}
	}
	// The spec's labels can't unmark a container: List would lose it.
	spec.Labels[managedLabel] = "false"
	if cs := (&Runner{}).createSpec(spec); cs.Labels[managedLabel] != "true" {
		t.Errorf("a spec label unmarked the container: %v", cs.Labels)
	}
}
