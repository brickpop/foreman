package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/repos"
)

// fakeRepos: project ledger, repo api, ref trunk, path README.md; it records
// each call's arguments.
type fakeRepos struct{ asked []string }

func (f *fakeRepos) check(project, repo, ref string) error {
	switch {
	case project != "ledger":
		return home.ErrNotFound
	case repo != "api":
		return repos.ErrNoRepo
	case ref != "trunk":
		return git.ErrNoRef
	}
	return nil
}

func (f *fakeRepos) Repos(_ context.Context, project string) ([]repos.Repo, error) {
	if err := f.check(project, "api", "trunk"); err != nil {
		return nil, err
	}
	return []repos.Repo{{Name: "api", Default: "trunk"}, {Name: "project", Project: true, Default: "main"}}, nil
}

func (f *fakeRepos) Tree(_ context.Context, project, repo, ref, path string) ([]git.TreeEntry, error) {
	f.asked = append(f.asked, "tree "+ref+" "+path)
	if err := f.check(project, repo, ref); err != nil {
		return nil, err
	}
	return []git.TreeEntry{{Name: "README.md", Kind: "file", Size: 6}}, nil
}

func (f *fakeRepos) File(_ context.Context, project, repo, ref, path string) (repos.File, error) {
	f.asked = append(f.asked, "file "+ref+" "+path)
	if err := f.check(project, repo, ref); err != nil {
		return repos.File{}, err
	}
	if path != "README.md" {
		return repos.File{}, git.ErrNoPath
	}
	return repos.File{Path: path, Size: 6, Text: "# API\n"}, nil
}

func (f *fakeRepos) Log(_ context.Context, project, repo, ref string, all bool, skip int) (repos.LogPage, error) {
	f.asked = append(f.asked, "log "+ref+" "+strconv.FormatBool(all)+" "+strconv.Itoa(skip))
	if err := f.check(project, repo, ref); err != nil {
		return repos.LogPage{}, err
	}
	return repos.LogPage{Commits: []git.Commit{}, Next: 0}, nil
}

func (f *fakeRepos) Image(_ context.Context, project, repo, ref, path string) ([]byte, string, error) {
	if err := f.check(project, repo, ref); err != nil {
		return nil, "", err
	}
	switch path {
	case "logo.png":
		return []byte("png"), "image/png", nil
	case "logo.svg":
		return []byte("<svg/>"), "image/svg+xml", nil
	}
	return nil, "", git.ErrNoPath
}

// A repo's image is served as the wiki's are: its type declared, never
// sniffed, an SVG unable to run anything.
func TestRepoImages(t *testing.T) {
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Repos: &fakeRepos{}, UI: builtUI})
	w := send(h, "GET", "/api/projects/ledger/repos/api/image?ref=trunk&path=logo.png", "")
	if w.Code != http.StatusOK || w.Body.String() != "png" || w.Header().Get("Content-Type") != "image/png" ||
		w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Content-Security-Policy") != "" {
		t.Errorf("png: %d %q %v", w.Code, w.Body, w.Header())
	}
	if w := send(h, "GET", "/api/projects/ledger/repos/api/image?ref=trunk&path=logo.svg", ""); w.Header().Get("Content-Security-Policy") != svgPolicy {
		t.Errorf("svg: %v", w.Header())
	}
	if w := send(h, "GET", "/api/projects/ledger/repos/api/image?ref=trunk&path=README.md", ""); w.Code != http.StatusNotFound {
		t.Errorf("not an image: %d", w.Code)
	}
}

func TestRepoReads(t *testing.T) {
	f := &fakeRepos{}
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Repos: f, UI: builtUI})
	var list []map[string]any
	decodeBody(t, get(t, h, "/api/projects/ledger/repos"), &list)
	if len(list) != 2 || list[1]["project"] != true {
		t.Errorf("repos %v", list)
	}
	var file map[string]any
	decodeBody(t, get(t, h, "/api/projects/ledger/repos/api/file?ref=trunk&path=README.md"), &file)
	if file["text"] != "# API\n" {
		t.Errorf("file %v", file)
	}
	for _, path := range []string{"/api/projects/ledger/repos/api/tree?ref=trunk&path=src%2Flib", "/api/projects/ledger/repos/api/log?ref=trunk&skip=50", "/api/projects/ledger/repos/api/log?ref=trunk&all=1"} {
		if w := send(h, "GET", path, ""); w.Code != http.StatusOK {
			t.Errorf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	if got := strings.Join(f.asked, ","); got != "file trunk README.md,tree trunk src/lib,log trunk false 50,log trunk true 0" {
		t.Errorf("asked %s", got)
	}
	for path, want := range map[string]string{
		"/api/projects/nope/repos":                             "No such project",
		"/api/projects/ledger/repos/nope/tree?ref=trunk":       "No such repo",
		"/api/projects/ledger/repos/api/tree?ref=gone":         "No such branch",
		"/api/projects/ledger/repos/api/file?ref=trunk&path=x": "No such file",
	} {
		if w := send(h, "GET", path, ""); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), want) {
			t.Errorf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	if w := send(h, "GET", "/api/projects/ledger/repos/api/log?ref=trunk&skip=-1", ""); w.Code != http.StatusBadRequest {
		t.Errorf("a negative skip: %d", w.Code)
	}
}
