//go:build podman

package podman

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/runner"
)

// testBuilder builds a throwaway image whose Containerfile is unique to this
// run, so no earlier build can be reused by accident.
func testBuilder(t *testing.T, r *Runner, steps string) *Images {
	t.Helper()
	name := fmt.Sprintf("localhost/foreman-test/%s", strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")))
	file := fmt.Sprintf("FROM %s\n# run %d\n%s\n", testImage, time.Now().UnixNano(), steps)
	im := r.Images(name, []byte(file), t.TempDir())
	t.Cleanup(func() {
		_ = r.c.call(ctx, "DELETE", "/images/"+name, map[string][]string{"force": {"true"}}, nil, nil)
	})
	return im
}

func TestEnsureBuildsOnceAndRuns(t *testing.T) {
	r := engine(t)
	im := testBuilder(t, r, `RUN echo built > /built.txt`)
	if s := im.Status(ctx); s[0].State != "missing" {
		t.Errorf("before the build: %+v", s)
	}

	// Several callers at once share one build.
	refs := make(chan string, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ref, err := im.Ensure(ctx, "ledger")
			if err != nil {
				t.Error(err)
			}
			refs <- ref
		}()
	}
	wg.Wait()
	close(refs)
	var ref string
	for got := range refs {
		if ref != "" && got != ref {
			t.Errorf("different refs %q and %q", ref, got)
		}
		ref = got
	}
	if im.builds.Load() != 1 {
		t.Errorf("%d builds, want 1", im.builds.Load())
	}
	s := im.Status(ctx)[0]
	if s.State != "ready" || s.Ref != ref || !strings.Contains(s.Log, "echo built") {
		t.Errorf("after the build: %+v", s)
	}

	p := start(t, r, runner.Spec{Image: ref, Command: sh("cat /built.txt")})
	out := collect(p)
	out.waitFor(t, "built")

	// A new builder for the same Containerfile (a Foreman restart) finds it.
	again := r.Images(im.name, im.containerfile, im.shared.dir)
	if s := again.Status(ctx)[0]; s.State != "ready" {
		t.Errorf("after a restart: %+v", s)
	}
	if got, err := again.Ensure(ctx, "relay"); err != nil || got != ref || again.builds.Load() != 0 {
		t.Errorf("Ensure after a restart: %q %v, %d builds", got, err, again.builds.Load())
	}
}

func TestAChangedContainerfileIsANewImage(t *testing.T) {
	r := engine(t)
	im := testBuilder(t, r, `RUN true`)
	first, err := im.Ensure(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	changed := r.Images(im.name, append(im.containerfile, []byte("RUN echo more\n")...), im.shared.dir)
	second, err := changed.Ensure(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || changed.builds.Load() != 1 {
		t.Errorf("refs %q %q, %d builds", first, second, changed.builds.Load())
	}
}

func TestAFailedBuildKeepsItsLog(t *testing.T) {
	r := engine(t)
	im := testBuilder(t, r, `RUN echo about-to-fail && false`)
	if _, err := im.Ensure(ctx, ""); err == nil {
		t.Fatal("a failing build succeeded")
	}
	s := im.Status(ctx)[0]
	if s.State != "failed" || s.Ref != "" || !strings.Contains(s.Log, "about-to-fail") {
		t.Errorf("after a failed build: %+v", s)
	}
}

func TestRefreshRebuilds(t *testing.T) {
	r := engine(t)
	im := testBuilder(t, r, `RUN cat /proc/sys/kernel/random/uuid > /stamp`)
	ref, err := im.Ensure(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	stamp := func() string {
		p := start(t, r, runner.Spec{Image: ref, Command: sh("cat /stamp; echo END")})
		out := collect(p)
		out.waitFor(t, "END")
		out.mu.Lock()
		defer out.mu.Unlock()
		return out.buf.String()
	}
	before := stamp()
	if err := im.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if after := stamp(); after == before || im.builds.Load() != 2 {
		t.Errorf("Refresh reused the cache: %q then %q, %d builds", before, after, im.builds.Load())
	}
}
