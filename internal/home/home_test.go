package home

import (
	"os"
	"path/filepath"
	"testing"
)

// The home is created on the first start, private, and opened as it is after.
func TestOpenMakesAPrivateHome(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", ".foreman")
	h, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(h.Root()) || h.Root() != dir || h.Path("teams") != filepath.Join(dir, "teams") {
		t.Errorf("Root %q, Path %q", h.Root(), h.Path("teams"))
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("home %v %v: readable by others", info, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.toml"), []byte("port = 9000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "settings.toml")); string(data) != "port = 9000\n" {
		t.Error("opening again touched the home")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(file); err == nil {
		t.Error("a file as the home")
	}
}

func TestAccessTokenIsStableAndPrivate(t *testing.T) {
	h, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := h.AccessToken(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 40 {
		t.Errorf("token %q is too short to be unguessable", first)
	}
	again, err := h.AccessToken(false)
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Error("token changed between starts without rotation")
	}
	info, err := os.Stat(h.Path(tokenFile))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("token file permissions %o, want 600", perm)
	}
}

func TestAccessTokenRotation(t *testing.T) {
	h, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, _ := h.AccessToken(false)
	rotated, err := h.AccessToken(true)
	if err != nil {
		t.Fatal(err)
	}
	if rotated == first {
		t.Error("rotation kept the old token")
	}
	after, _ := h.AccessToken(false)
	if after != rotated {
		t.Error("rotated token was not persisted")
	}
}

func TestAccessTokenRegeneratedWhenEmpty(t *testing.T) {
	h, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.Path(tokenFile), []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := h.AccessToken(false)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Error("an empty token file produced an empty token")
	}
}

// The agent link's secret survives restarts (running containers keep their
// tokens), is private, and differs from the access token.
func TestAgentSecret(t *testing.T) {
	h, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := h.AgentSecret()
	if err != nil || len(first) < 40 {
		t.Fatalf("secret %q %v", first, err)
	}
	again, _ := h.AgentSecret()
	token, _ := h.AccessToken(false)
	if string(again) != string(first) || string(first) == token {
		t.Error("the secret changed, or is the access token")
	}
	info, err := os.Stat(h.Path("credentials", "agent-link"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("secret file: %v %v", info, err)
	}
}
