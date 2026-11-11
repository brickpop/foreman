package repos

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/brickpop/foreman/internal/git"
)

// docs commits a small documentation tree to api's trunk: guides beside each
// other, a folder, a wiki bundle (its wiki.toml), an image and a symlink
// named like one.
func docs(t *testing.T, b Browser) {
	t.Helper()
	api := filepath.Join(b.Config.(config).root, "ledger", "api")
	write(t, filepath.Join(api, "guides", "install.md"), `# Install a plugin

See [Deploy](./deploy.md), [bare](deploy.md), [setup](deploy.md#set-up), [query](deploy.md?plain=1),
[home](../README.md), [root](/README.md), [missing](./missing.md), [folder](./sub/), [folder bare](sub),
[spaced](my%20file.md), [escape](../../x.md), [escape from root](/../x.md), [the repo](../),
[logo](../logo.png), [linked](../link.png), [web](https://example.com), [here](#install-a-plugin).
`+"`[code](./code.md)`"+`

## Install a plugin
`)
	write(t, filepath.Join(api, "guides", "deploy.md"), "# Deploy\n")
	write(t, filepath.Join(api, "guides", "my file.md"), "spaced\n")
	write(t, filepath.Join(api, "guides", "sub", "index.md"), "# Sub\n")
	write(t, filepath.Join(api, "wiki", "wiki.toml"), "")
	write(t, filepath.Join(api, "wiki", "index.md"), "# Wiki\n")
	write(t, filepath.Join(api, "wiki", "notes", "a.md"), "[top](/index.md) [above](/../README.md) [up](../../README.md) [dots](..notes/a.md) [out](../../../x.md)\n")
	write(t, filepath.Join(api, "top.md"), "[dots](..notes/a.md) [up](../x.md)\n")
	if err := os.Symlink("logo.png", filepath.Join(api, "link.png")); err != nil {
		t.Fatal(err)
	}
	gitIn(t, api, "add", ".")
	gitIn(t, api, "commit", "--quiet", "-m", "docs")
}

func TestMarkdownLinksResolveAgainstTheCommit(t *testing.T) {
	b := fixture(t)
	docs(t, b)
	f, err := b.File(ctx, "ledger", "api", "trunk", "guides/install.md")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Link{}
	for _, l := range f.Links {
		got[l.Raw] = l
	}
	for raw, want := range map[string]Link{
		"./deploy.md":       {To: "guides/deploy.md", Exists: true},
		"deploy.md":         {To: "guides/deploy.md", Exists: true},
		"deploy.md#set-up":  {To: "guides/deploy.md", Anchor: "set-up", Exists: true},
		"deploy.md?plain=1": {To: "guides/deploy.md", Exists: true},
		"../README.md":      {To: "README.md", Exists: true},
		"/README.md":        {To: "README.md", Exists: true}, // no wiki.toml above: the repo's root
		"./missing.md":      {To: "guides/missing.md"},
		"./sub/":            {To: "guides/sub", Exists: true, Folder: true},
		"sub":               {To: "guides/sub", Exists: true, Folder: true},
		"my%20file.md":      {To: "guides/my file.md", Exists: true},
		"../../x.md":        {Outside: true},
		"/../x.md":          {Outside: true},
		"../":               {To: "", Exists: true, Folder: true},
		"../logo.png":       {To: "logo.png", Exists: true, Image: true},
		"../link.png":       {To: "link.png", Exists: true}, // a symlink: never loaded
	} {
		want.Raw = raw
		if got[raw] != want {
			t.Errorf("%s: %+v, want %+v", raw, got[raw], want)
		}
		delete(got, raw)
	}
	// Web links, same-page anchors and links in code are left to the browser.
	if len(got) != 0 {
		t.Errorf("unexpected links: %+v", got)
	}
	if len(f.Headings) != 2 || f.Headings[0].ID != "install-a-plugin" || f.Headings[1].ID != "install-a-plugin-1" || f.Headings[1].BodyLine != 9 {
		t.Errorf("headings %+v", f.Headings)
	}

	// Below a wiki.toml, a root-absolute link starts at that folder, and a link
	// climbing out of the wiki, either way it is written, still reaches the repo
	// (wiki/ is not the repo's root): it is outside only when it climbs out of the
	// repo. "..notes" is a name, not a climb (segment-exact).
	f, err = b.File(ctx, "ledger", "api", "trunk", "wiki/notes/a.md")
	if err != nil {
		t.Fatal(err)
	}
	wiki := map[string]Link{}
	for _, l := range f.Links {
		wiki[l.Raw] = l
	}
	for raw, want := range map[string]Link{
		"/index.md":       {To: "wiki/index.md", Exists: true},
		"/../README.md":   {To: "README.md", Exists: true},
		"../../README.md": {To: "README.md", Exists: true},
		"..notes/a.md":    {To: "wiki/notes/..notes/a.md"},
		"../../../x.md":   {Outside: true},
	} {
		want.Raw = raw
		if wiki[raw] != want {
			t.Errorf("wiki %s: %+v, want %+v", raw, wiki[raw], want)
		}
	}
	if len(f.Links) != 5 {
		t.Errorf("wiki links %+v", f.Links)
	}
	// At the repo's root, with no wiki above: ../x climbs out of the repo, and
	// "..notes" stays a name.
	f, err = b.File(ctx, "ledger", "api", "trunk", "top.md")
	if err != nil {
		t.Fatal(err)
	}
	top := map[string]Link{}
	for _, l := range f.Links {
		top[l.Raw] = l
	}
	if top["..notes/a.md"] != (Link{Raw: "..notes/a.md", To: "..notes/a.md"}) || top["../x.md"] != (Link{Raw: "../x.md", Outside: true}) {
		t.Errorf("top links %+v", f.Links)
	}
	// Another branch is read as it is: the docs aren't on wt/tove/x.
	if f, err := b.File(ctx, "ledger", "api", "wt/tove/x", "README.md"); err != nil || len(f.Links) != 0 {
		t.Errorf("README on the agent's branch: %+v %v", f.Links, err)
	}
}

func TestImages(t *testing.T) {
	b := fixture(t)
	docs(t, b)
	content, contentType, err := b.Image(ctx, "ledger", "api", "trunk", "logo.png")
	if err != nil || string(content) != "\x89PNG\x00\x01" || contentType != "image/png" {
		t.Errorf("logo: %q %q %v", content, contentType, err)
	}
	for _, p := range []string{
		"README.md",          // not an image
		"link.png",           // a symlink named like one
		"missing.png",        // not there
		"../outside.png",     // out of the repo
		"guides/../../x.png", // out of the repo, the long way
	} {
		if _, _, err := b.Image(ctx, "ledger", "api", "trunk", p); !errors.Is(err, git.ErrNoPath) {
			t.Errorf("%s: %v", p, err)
		}
	}
	if _, _, err := b.Image(ctx, "ledger", "api", "gone", "logo.png"); !errors.Is(err, git.ErrNoRef) {
		t.Errorf("unknown ref: %v", err)
	}
	api := filepath.Join(b.Config.(config).root, "ledger", "api")
	write(t, filepath.Join(api, "huge.png"), string(make([]byte, ImageLimit+1)))
	gitIn(t, api, "add", "huge.png")
	gitIn(t, api, "commit", "--quiet", "-m", "huge")
	if _, _, err := b.Image(ctx, "ledger", "api", "trunk", "huge.png"); !errors.Is(err, git.ErrNoPath) {
		t.Errorf("over the limit: %v", err)
	}
}
