package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Experiment B (wiki/backlog/prove-b-shared-login.md), step 2: the refresh race.
// Every refresh revokes the previous refresh token, so concurrent sessions
// sharing one login can kill it. This marks the shared token as expired, then
// starts N containers at once that all need it, for one way of pointing the
// harness at the shared login file:
//
//	file     the file alone, bind-mounted into each private config dir (the design)
//	dir      the whole shared folder as the config dir
//	symlink  a link in each private config dir to the shared file
//
// Print mode (`claude -p`) is fine here: this measures the login, not sessions.

func proveBRace(r *report, variant string, n int) {
	if variant != "file" && variant != "dir" && variant != "symlink" {
		r.fail("unknown variant %q: file, dir or symlink", variant)
		return
	}
	p := connectPodman(r)
	if p == nil {
		return
	}
	var info struct {
		Host struct {
			MemTotal int64 `json:"memTotal"`
			CPUs     int   `json:"cpus"`
		} `json:"host"`
	}
	if err := p.call("GET", "/info", nil, nil, &info); err == nil {
		r.info("Podman host: %d CPUs, %.1f GB memory (each Claude process needs roughly 200-300 MB)",
			info.Host.CPUs, float64(info.Host.MemTotal)/(1<<30))
	}
	shared := credentialsDir()
	credsPath := filepath.Join(shared, ".credentials.json")
	before, err := readExpiry(credsPath)
	if !r.check(err, "read the shared login") {
		r.info("run `just prove-b-login` first")
		return
	}
	r.info("token currently expires %s", before.Format(time.RFC3339))
	removeLabelled(r, p, "b-race")

	// Round 0: the login must work before the race, or the race proves nothing.
	if raceRound(r, p, variant, 1, "round 0, baseline: one container on the current login") != 1 {
		r.fail("the shared login doesn't work even before the race: log in again (`just prove-b-login`), then re-run")
		return
	}
	r.pass("the login works before the race")

	// Round 1: the token is expired for everyone at once.
	if !r.check(setExpiry(credsPath, time.Now().Add(-time.Minute)), "marked the shared token as expired") {
		return
	}
	ok1 := raceRound(r, p, variant, n, "round 1, expired token, "+strconv.Itoa(n)+" containers at once")
	after, err := readExpiry(credsPath)
	if err == nil && after.After(time.Now()) {
		r.pass("the shared file now holds a refreshed token (expires %s)", after.Format(time.RFC3339))
	} else {
		r.fail("the shared file still holds the expired token: refreshes didn't reach it (%v)", err)
	}

	// Round 2: is the login still alive for everyone?
	ok2 := raceRound(r, p, variant, n, "round 2, the login after the race")
	switch {
	case ok1 == n && ok2 == n:
		r.pass("variant %q survives the refresh race: %d/%d, then %d/%d", variant, ok1, n, ok2, n)
	case ok2 == 0:
		r.fail("variant %q KILLED the shared login (round 2: 0/%d). Log in again (`just prove-b-login`) before trying another variant", variant, n)
	default:
		r.fail("variant %q: round 1 %d/%d, round 2 %d/%d", variant, ok1, n, ok2, n)
	}
}

// raceRound starts n containers at once, each asking Claude for one word, and
// returns how many succeeded.
func raceRound(r *report, p *podman, variant string, n int, label string) int {
	shared := credentialsDir()
	cwd, _ := os.Getwd()
	var wg sync.WaitGroup
	var mu sync.Mutex
	// Containers are created and started one at a time: doing both for many keep-id
	// containers at once races inside Podman's runtime (crun: "write to
	// ping_group_range"; see `just prove-storm`). The sessions then run concurrently,
	// which is what the race needs.
	var starting sync.Mutex
	ok := 0
	var failures []string
	begin := time.Now()
	for i := 1; i <= n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			private := filepath.Join(cwd, ".dev", "prove", "b-race", strconv.Itoa(i))
			_ = os.RemoveAll(private)
			_ = os.MkdirAll(private, 0o700)
			mounts := []mount{}
			switch variant {
			case "file":
				mounts = append(mounts,
					mount{Destination: configDir, Source: private, Type: "bind", Options: bindOptions()},
					mount{Destination: configDir + "/.credentials.json", Source: filepath.Join(shared, ".credentials.json"), Type: "bind", Options: bindOptions()})
			case "dir":
				mounts = append(mounts, mount{Destination: configDir, Source: shared, Type: "bind", Options: bindOptions()})
			case "symlink":
				_ = os.Symlink("/foreman/shared/.credentials.json", filepath.Join(private, ".credentials.json"))
				mounts = append(mounts,
					mount{Destination: configDir, Source: private, Type: "bind", Options: bindOptions()},
					mount{Destination: "/foreman/shared", Source: shared, Type: "bind", Options: bindOptions()})
			}
			starting.Lock() // create and start one at a time (see the storm experiment)
			id, err := p.create(createSpec{
				Name:    fmt.Sprintf("foreman-prove-b-race-%d-%s", i, newToken()[:6]),
				Image:   harnessImage,
				Command: []string{"claude", "-p", "Reply with exactly the word OK and nothing else."},
				Env:     map[string]string{"CLAUDE_CONFIG_DIR": configDir},
				Labels:  map[string]string{"foreman.prove": "b-race"},
				Mounts:  mounts,
				Userns:  keepID,
			})
			if err != nil {
				starting.Unlock()
				mu.Lock()
				failures = append(failures, fmt.Sprintf("%d: create: %v", i, err))
				mu.Unlock()
				return
			}
			defer func() { _ = p.remove(id) }()
			err = p.start(id)
			starting.Unlock()
			if err != nil {
				mu.Lock()
				failures = append(failures, fmt.Sprintf("%d: start: %v", i, err))
				mu.Unlock()
				return
			}
			code, err := p.wait(id)
			out, _ := p.logs(id)
			mu.Lock()
			defer mu.Unlock()
			if err == nil && code == 0 && strings.Contains(out, "OK") {
				ok++
			} else {
				failures = append(failures, fmt.Sprintf("%d: exit %d: %s", i, code, strings.TrimSpace(out)))
			}
		}(i)
	}
	wg.Wait()
	r.info("%s: %d/%d succeeded in %v", label, ok, n, time.Since(begin).Round(time.Second))
	for _, f := range failures {
		r.info("  %s", f)
	}
	return ok
}

// The login file: {"claudeAiOauth": {"accessToken", "refreshToken", "expiresAt" (ms), …}}.

func readExpiry(path string) (time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	var creds struct {
		OAuth *struct {
			ExpiresAt int64 `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return time.Time{}, err
	}
	if creds.OAuth == nil {
		return time.Time{}, fmt.Errorf("no claudeAiOauth login in %s", path)
	}
	// 0 is a real value Claude Code writes (seen after a recovered login): treat it as
	// "expired, refresh on next use", not as a missing field.
	return time.UnixMilli(creds.OAuth.ExpiresAt), nil
}

// setExpiry rewrites only expiresAt, keeping every other field as it is. It
// writes in place (no rename), so a file bind-mounted elsewhere stays the same file.
func setExpiry(path string, t time.Time) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	var oauth map[string]json.RawMessage
	if err := json.Unmarshal(all["claudeAiOauth"], &oauth); err != nil {
		return err
	}
	oauth["expiresAt"] = json.RawMessage(strconv.FormatInt(t.UnixMilli(), 10))
	all["claudeAiOauth"], _ = json.Marshal(oauth)
	out, err := json.Marshal(all)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(out)
	return err
}
