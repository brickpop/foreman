package podman

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// buildAPI answers a build of a missing image whose id is "new". labelled
// answers the list of the name's images; DELETE of "used" is refused as
// Podman refuses an image a container uses. Each DELETE is recorded,
// with its query, in removed.
func buildAPI(t *testing.T, labelled func(http.ResponseWriter, *http.Request), removed *[]string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, apiPrefix)
		switch {
		case strings.HasSuffix(path, "/exists"):
			w.WriteHeader(http.StatusNotFound)
		case path == "/build":
			fmt.Fprintln(w, `{"stream":"STEP 1/1: FROM scratch\n"}`)
		case req.Method == "GET" && path == "/images/json":
			labelled(w, req)
		case req.Method == "GET" && strings.HasSuffix(path, "/json"):
			fmt.Fprint(w, `{"Id":"new"}`)
		case req.Method == "DELETE":
			*removed = append(*removed, path+"?"+req.URL.RawQuery)
			if path == "/images/used" {
				w.WriteHeader(http.StatusConflict)
				fmt.Fprint(w, `{"message":"image is in use by a container"}`)
			}
		default:
			t.Errorf("unexpected call %s %s", req.Method, path)
		}
	}
}

// A build removes the name's earlier images, but neither the one it built nor
// one a container still uses, and only by the name's label.
func TestABuildRemovesEarlierImages(t *testing.T) {
	var removed []string
	var filter string
	r := fakePodman(t, buildAPI(t, func(w http.ResponseWriter, req *http.Request) {
		filter = req.URL.Query().Get("filters")
		fmt.Fprint(w, `[{"Id":"old"},{"Id":"new"},{"Id":"used"}]`)
	}, &removed))
	im := r.Images("localhost/foreman/harness", []byte("FROM scratch\n"), t.TempDir())
	if _, err := im.Ensure(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if want := `{"label":["foreman.image=localhost/foreman/harness"]}`; filter != want {
		t.Errorf("listed by %s, want %s", filter, want)
	}
	// Never forced: Podman's refusal is what keeps an image in use.
	if want := []string{"/images/old?", "/images/used?"}; !slices.Equal(removed, want) {
		t.Errorf("removed %q, want %q", removed, want)
	}
	state, log := im.shared.state()
	if state != "ready" || !strings.Contains(log, "removed the earlier image old") ||
		!strings.Contains(log, "kept the earlier image used") || !strings.Contains(log, "in use by a container") {
		t.Errorf("after the build: %s, log:\n%s", state, log)
	}
}

// The build succeeded, so a failure to clean up after it doesn't fail it.
func TestAFailedPruneKeepsTheBuild(t *testing.T) {
	var removed []string
	r := fakePodman(t, buildAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message":"store is locked"}`)
	}, &removed))
	im := r.Images("localhost/foreman/harness", []byte("FROM scratch\n"), t.TempDir())
	if _, err := im.Ensure(context.Background(), ""); err != nil {
		t.Fatalf("a failed prune failed the build: %v", err)
	}
	state, log := im.shared.state()
	if len(removed) != 0 || state != "ready" || !strings.Contains(log, "not removing earlier images") ||
		!strings.Contains(log, "store is locked") {
		t.Errorf("removed %q; after the build: %s, log:\n%s", removed, state, log)
	}
}
