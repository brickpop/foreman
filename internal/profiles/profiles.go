// Package profiles keeps the CEO's profiles: each a harness and its
// credentials, under a name, shared by every workspace on this machine
// (wiki/product/architecture/session-runner.md#credentials). A project uses
// one; its agents run on that harness with those credentials.
//
// <root>/<name>/profile.toml names the harness; the rest of the folder is the
// harness's own (for Claude Code, its login), which its login container
// writes and its agents mount.
package profiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// Default is the profile a project uses unless it names another. It always
// exists (Ensure) and can't be deleted.
const Default = "default"

var (
	// ErrNotFound means there is no profile of that name.
	ErrNotFound = errors.New("profiles: no such profile")
	// ErrExists means a profile of that name already exists.
	ErrExists = errors.New("profiles: a profile of that name exists")
	// ErrInvalid means a name isn't one (or the default can't be deleted).
	ErrInvalid = errors.New("profiles: invalid")
)

// name is a profile's name: one folder, never a path.
var name = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Valid reports whether s can name a profile: lowercase letters, digits and
// dashes, at most 40.
func Valid(s string) bool { return name.MatchString(s) && len(s) <= 40 }

// In is the profiles in the CEO's folder (internal/userdata).
func In(userDir string) Store { return Store{Root: filepath.Join(userDir, "profiles")} }

// Store is the profiles under one root, profiles/ in the CEO's folder
// (internal/userdata).
type Store struct{ Root string }

const file = "profile.toml"

type record struct {
	Harness string `toml:"harness"`
}

// Dir is a profile's folder: the harness's own files (its login folder).
// Only for a valid name: anything else would be a path.
func (s Store) Dir(profile string) string {
	if !Valid(profile) {
		return filepath.Join(s.Root, Default)
	}
	return filepath.Join(s.Root, profile)
}

// Get returns a profile (Connected unset: the harness's adapter says).
func (s Store) Get(profile string) (Profile, error) {
	if !Valid(profile) {
		return Profile{}, fmt.Errorf("%w: %s", ErrNotFound, profile)
	}
	var r record
	_, err := toml.DecodeFile(filepath.Join(s.Dir(profile), file), &r)
	if errors.Is(err, fs.ErrNotExist) {
		return Profile{}, fmt.Errorf("%w: %s", ErrNotFound, profile)
	}
	if err != nil {
		return Profile{}, fmt.Errorf("profile %s: %w", profile, err)
	}
	return Profile{Name: profile, Harness: r.Harness}, nil
}

// List returns the profiles: the default first, then by name. A folder
// without a readable profile.toml isn't one.
func (s Store) List() ([]Profile, error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	list := []Profile{}
	for _, e := range entries {
		if p, err := s.Get(e.Name()); err == nil && e.IsDir() {
			list = append(list, p)
		}
	}
	slices.SortFunc(list, func(a, b Profile) int {
		switch {
		case a.Name == Default:
			return -1
		case b.Name == Default:
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return list, nil
}

// Create makes a profile for harness, with no credentials yet.
func (s Store) Create(profile, harness string) (Profile, error) {
	if !Valid(profile) || harness == "" {
		return Profile{}, fmt.Errorf("%w: a name is lowercase letters, digits and dashes, at most 40", ErrInvalid)
	}
	if _, err := s.Get(profile); err == nil {
		return Profile{}, ErrExists
	}
	if err := s.write(profile, harness); err != nil {
		return Profile{}, err
	}
	return Profile{Name: profile, Harness: harness}, nil
}

func (s Store) write(profile, harness string) error {
	if err := os.MkdirAll(s.Dir(profile), 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir(profile), file), []byte(fmt.Sprintf("harness = %q\n", harness)), 0o600)
}

// Ensure makes the default profile, for harness, unless it exists.
func (s Store) Ensure(harness string) error {
	if _, err := s.Get(Default); !errors.Is(err, ErrNotFound) {
		return err
	}
	_, err := s.Create(Default, harness)
	return err
}

// Delete removes a profile and its credentials; never the default.
func (s Store) Delete(profile string) error {
	if profile == Default {
		return fmt.Errorf("%w: the default profile can't be deleted", ErrInvalid)
	}
	if _, err := s.Get(profile); err != nil {
		return err
	}
	return os.RemoveAll(s.Dir(profile))
}

// Adopt turns a workspace's own logins, from before profiles
// (<workspace>/.foreman/credentials/<harness>/), into profiles, and returns
// them: the default when it doesn't exist yet, else one named after the
// workspace (my-work, my-work-2…), so no login is lost or overwritten.
// connected says, per harness, whether a folder holds a login; a folder
// without one stays where it is.
func (s Store) Adopt(workspace string, connected map[string]func(dir string) bool) ([]Profile, error) {
	old := filepath.Join(workspace, ".foreman", "credentials")
	harnesses := make([]string, 0, len(connected))
	for h := range connected {
		harnesses = append(harnesses, h)
	}
	slices.Sort(harnesses)
	var adopted []Profile
	for _, h := range harnesses {
		src := filepath.Join(old, h)
		if !connected[h](src) {
			continue
		}
		// Free: no such profile, or one of this harness never connected.
		free := func(name string) bool {
			p, err := s.Get(name)
			return errors.Is(err, ErrNotFound) || err == nil && p.Harness == h && !connected[h](s.Dir(name))
		}
		profile := Default
		if !free(profile) {
			base := profileName(filepath.Base(workspace))
			profile = base
			for n := 2; !free(profile); n++ {
				profile = fmt.Sprintf("%s-%d", base, n)
			}
		}
		if err := move(src, s.Dir(profile)); err != nil {
			return adopted, fmt.Errorf("moving the %s login: %w", h, err)
		}
		if err := s.write(profile, h); err != nil {
			return adopted, err
		}
		adopted = append(adopted, Profile{Name: profile, Harness: h, Connected: true})
	}
	return adopted, nil
}

// profileName makes a profile name of a folder's: lowercase letters, digits
// and dashes ("My Work" → "my-work"), never the default's.
func profileName(folder string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(folder) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	s := b.String()
	if len(s) > 32 {
		s = strings.TrimRight(s[:32], "-")
	}
	if s == "" || s == Default {
		return "workspace"
	}
	return s
}
