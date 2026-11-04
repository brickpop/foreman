package store

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

var ctx = context.Background()

func open(t *testing.T) *DB {
	t.Helper()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func version(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// --- opening and migrations --------------------------------------------------

func TestOpenCreatesTheSchemaAndReopensWithTheData(t *testing.T) {
	// Home folders may have spaces and characters a SQLite URI treats specially.
	path := filepath.Join(t.TempDir(), "my work#space?x=1 100%", "state.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("state.db not where asked: %v", err)
	}
	// The schema version is the number of migrations: no constant to bump.
	if files, _ := fs.Glob(migrationFiles, "migrations/*.sql"); version(t, db.sql) != len(files) || len(files) == 0 {
		t.Errorf("version %d after the first open, %d migrations", version(t, db.sql), len(files))
	}
	var mode string
	_ = db.sql.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Errorf("journal mode %q, want wal", mode)
	}
	if _, err := db.Deployments().Deploy(ctx, "atlas", "ledger"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	again, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()
	if _, err := again.Deployments().Get(ctx, "atlas@ledger"); err != nil {
		t.Errorf("data lost on reopen: %v", err)
	}
}

func TestANewerDatabaseIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.sql.Exec("PRAGMA user_version = 99")
	_ = db.Close()
	if _, err := Open(ctx, path); !errors.Is(err, ErrTooNew) {
		t.Errorf("open of a newer database: %v", err)
	}
}

func rawDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func migrations(files ...string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for i := 0; i < len(files); i += 2 {
		fsys["migrations/"+files[i]] = &fstest.MapFile{Data: []byte(files[i+1])}
	}
	return fsys
}

// Each step applies only what is new: a database at version 1 gets 2 and 3.
func TestMigrationsApplyFromEveryEarlierVersion(t *testing.T) {
	all := []string{
		"001_a.sql", "CREATE TABLE a (x INTEGER);",
		"002_b.sql", "CREATE TABLE b (x INTEGER);",
		"003_a_y.sql", "ALTER TABLE a ADD COLUMN y TEXT;",
	}
	for from := 0; from <= 3; from++ {
		db := rawDB(t)
		if err := migrate(ctx, db, migrations(all[:2*from]...)); err != nil {
			t.Fatalf("to %d: %v", from, err)
		}
		if err := migrate(ctx, db, migrations(all...)); err != nil {
			t.Fatalf("from %d: %v", from, err)
		}
		if v := version(t, db); v != 3 {
			t.Errorf("from %d: version %d", from, v)
		}
		if _, err := db.Exec("INSERT INTO a (x, y) VALUES (1, 'y'); INSERT INTO b (x) VALUES (1)"); err != nil {
			t.Errorf("from %d: schema incomplete: %v", from, err)
		}
	}
}

// A failing migration leaves the database at the last good version, with
// nothing of the failed one applied.
func TestAFailedMigrationIsRolledBack(t *testing.T) {
	db := rawDB(t)
	err := migrate(ctx, db, migrations(
		"001_a.sql", "CREATE TABLE a (x INTEGER);",
		"002_bad.sql", "CREATE TABLE b (x INTEGER); THIS IS NOT SQL;",
	))
	if err == nil || !strings.Contains(err.Error(), "002_bad.sql") {
		t.Fatalf("err = %v, want one naming 002_bad.sql", err)
	}
	if v := version(t, db); v != 1 {
		t.Errorf("version %d, want 1", v)
	}
	if _, err := db.Exec("INSERT INTO b (x) VALUES (1)"); err == nil {
		t.Error("half of the failed migration was applied")
	}
}

func TestMigrationsMustBeNumberedWithoutGaps(t *testing.T) {
	err := migrate(ctx, rawDB(t), migrations(
		"001_a.sql", "CREATE TABLE a (x INTEGER);",
		"003_c.sql", "CREATE TABLE c (x INTEGER);",
	))
	if err == nil || !strings.Contains(err.Error(), "out of sequence") {
		t.Errorf("a gap was accepted: %v", err)
	}
}

// --- deployments -------------------------------------------------------------

func TestDeployStopAndRedeploy(t *testing.T) {
	deps := open(t).Deployments()
	before := time.Now().Add(-time.Second)
	d, err := deps.Deploy(ctx, "atlas", "ledger")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "atlas@ledger" || d.Status != DeploymentActive || d.CreatedAt.Before(before) || !d.StoppedAt.IsZero() {
		t.Errorf("deployed %+v", d)
	}
	if _, err := deps.Deploy(ctx, "atlas", "ledger"); !errors.Is(err, ErrActive) {
		t.Errorf("second deploy while active: %v", err)
	}

	if err := deps.Stop(ctx, d.ID, ""); err != nil {
		t.Fatal(err)
	}
	stopped, _ := deps.Get(ctx, d.ID)
	if stopped.Status != DeploymentStopped || stopped.StoppedAt.IsZero() {
		t.Errorf("stopped %+v", stopped)
	}
	if err := deps.Stop(ctx, d.ID, ""); err != nil {
		t.Errorf("stopping twice: %v", err)
	}
	if again, _ := deps.Get(ctx, d.ID); !again.StoppedAt.Equal(stopped.StoppedAt) {
		t.Error("stopping twice moved stopped_at")
	}

	// Re-deploying reactivates the same row: same id and creation.
	r, err := deps.Deploy(ctx, "atlas", "ledger")
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != d.ID || r.Status != DeploymentActive || !r.CreatedAt.Equal(d.CreatedAt) || !r.StoppedAt.IsZero() {
		t.Errorf("redeployed %+v, first %+v", r, d)
	}
}

func TestDeploymentLookupsAndRules(t *testing.T) {
	deps := open(t).Deployments()
	if _, err := deps.Get(ctx, "nobody@nowhere"); !errors.Is(err, ErrNotFound) {
		t.Errorf("get unknown: %v", err)
	}
	if err := deps.Stop(ctx, "nobody@nowhere", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("stop unknown: %v", err)
	}
	for _, pair := range [][2]string{{"atlas", "relay"}, {"atlas", "ledger"}} {
		if _, err := deps.Deploy(ctx, pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	// One team per project: another is refused until the first stops.
	if _, err := deps.Deploy(ctx, "build", "ledger"); !errors.Is(err, ErrProjectTaken) {
		t.Errorf("a second team on ledger: %v", err)
	}
	if err := deps.Stop(ctx, "atlas@ledger", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := deps.Deploy(ctx, "build", "ledger"); err != nil {
		t.Fatal(err)
	}
	// Nor can the first come back while the second is there.
	if _, err := deps.Deploy(ctx, "atlas", "ledger"); !errors.Is(err, ErrProjectTaken) {
		t.Errorf("atlas back on ledger beside build: %v", err)
	}
	list, err := deps.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, d := range list {
		ids = append(ids, d.ID)
	}
	if got := strings.Join(ids, ","); got != "atlas@ledger,atlas@relay,build@ledger" {
		t.Errorf("List = %s", got)
	}
}

// The index holds the invariant even past Deploy's check: a second active
// deployment on a project can't be written at all.
func TestOneActiveDeploymentPerProjectInTheSchema(t *testing.T) {
	db := open(t)
	if _, err := db.Deployments().Deploy(ctx, "atlas", "ledger"); err != nil {
		t.Fatal(err)
	}
	insert := "INSERT INTO deployments (id, team_id, project_id, created_at, status) VALUES (?, ?, 'ledger', '2026-10-31T00:00:00Z', ?)"
	if _, err := db.sql.Exec(insert, "build@ledger", "build", DeploymentActive); err == nil {
		t.Error("a second active deployment on ledger was written")
	}
	if _, err := db.sql.Exec(insert, "orion@ledger", "orion", DeploymentStopped); err != nil {
		t.Errorf("a stopped one beside it: %v", err)
	}
}

// Two simultaneous deploys of one pair: exactly one wins, the other is told.
func TestConcurrentDeploysOfOnePair(t *testing.T) {
	deps := open(t).Deployments()
	const n = 64
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := deps.Deploy(ctx, "atlas", "ledger")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	won := 0
	for err := range errs {
		switch {
		case err == nil:
			won++
		case !errors.Is(err, ErrActive):
			t.Errorf("unexpected error: %v", err)
		}
	}
	if won != 1 {
		t.Errorf("%d deploys succeeded, want 1", won)
	}
}

// --- instances ---------------------------------------------------------------

func deployed(t *testing.T, db *DB, team, project string) {
	t.Helper()
	if _, err := db.Deployments().Deploy(ctx, team, project); err != nil {
		t.Fatal(err)
	}
}

func TestInstanceRoundTripsEveryField(t *testing.T) {
	db := open(t)
	deployed(t, db, "atlas", "ledger")
	at := time.Date(2026, 10, 14, 9, 30, 0, 123456789, time.UTC)
	full := Instance{
		ID: "mara@ledger", DeploymentID: "atlas@ledger", AgentID: "mara", Status: "running",
		Task: "LED-44", PID: 4242, PtyID: "c0ffee", SandboxPath: "/ws/ledger/.sandboxes/mara",
		Harness: "claude-code", Profile: "work", HarnessSessionID: "s-1", StartedAt: at, Restarts: 2,
		StopReason: "app has no branch main", PromptHash: "9f2c",
	}
	bare := Instance{ID: "tove@ledger", DeploymentID: "atlas@ledger", AgentID: "tove", Status: "starting",
		SandboxPath: "/ws/ledger/.sandboxes/tove", Harness: "claude-code"}
	for _, want := range []Instance{full, bare} {
		if err := db.Instances().Put(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := db.Instances().Get(ctx, want.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !got.StartedAt.Equal(want.StartedAt) {
			t.Errorf("started %v, want %v", got.StartedAt, want.StartedAt)
		}
		got.StartedAt, want.StartedAt = time.Time{}, time.Time{}
		if got != want {
			t.Errorf("got %+v\nwant %+v", got, want)
		}
	}
}

func TestPutReplacesTheWholeInstance(t *testing.T) {
	db := open(t)
	deployed(t, db, "atlas", "ledger")
	inst := Instance{ID: "mara@ledger", DeploymentID: "atlas@ledger", AgentID: "mara", Status: "running", Task: "LED-44",
		SandboxPath: "/s", Harness: "claude-code"}
	_ = db.Instances().Put(ctx, inst)
	inst.Status, inst.Task, inst.Restarts = "stopped", "", 1
	if err := db.Instances().Put(ctx, inst); err != nil {
		t.Fatal(err)
	}
	got, _ := db.Instances().Get(ctx, inst.ID)
	if got.Status != "stopped" || got.Task != "" || got.Restarts != 1 {
		t.Errorf("after replace %+v", got)
	}
}

func TestInstanceRules(t *testing.T) {
	db := open(t)
	deployed(t, db, "atlas", "ledger")
	ok := Instance{ID: "mara@ledger", DeploymentID: "atlas@ledger", AgentID: "mara", Status: "running", SandboxPath: "/s", Harness: "claude-code"}
	for name, bad := range map[string]func(Instance) Instance{
		"unknown deployment": func(i Instance) Instance { i.DeploymentID = "ghost@ledger"; return i },
		"unknown status":     func(i Instance) Instance { i.Status = "napping"; return i },
		"negative restarts":  func(i Instance) Instance { i.Restarts = -1; return i },
	} {
		if err := db.Instances().Put(ctx, bad(ok)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if _, err := db.Instances().Get(ctx, "nobody@ledger"); !errors.Is(err, ErrNotFound) {
		t.Errorf("get unknown: %v", err)
	}
}

func TestListReturnsOneDeploymentsInstances(t *testing.T) {
	db := open(t)
	deployed(t, db, "atlas", "ledger")
	deployed(t, db, "atlas", "relay")
	for _, i := range []Instance{
		{ID: "tove@ledger", DeploymentID: "atlas@ledger"},
		{ID: "mara@ledger", DeploymentID: "atlas@ledger"},
		{ID: "mara@relay", DeploymentID: "atlas@relay"},
	} {
		i.AgentID, i.Status, i.SandboxPath, i.Harness = "a", "idle", "/s", "claude-code"
		if err := db.Instances().Put(ctx, i); err != nil {
			t.Fatal(err)
		}
	}
	list, err := db.Instances().List(ctx, "atlas@ledger")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "mara@ledger" || list[1].ID != "tove@ledger" {
		t.Errorf("List = %+v", list)
	}
	if none, _ := db.Instances().List(ctx, "build@ledger"); len(none) != 0 {
		t.Errorf("List of an unknown deployment = %+v", none)
	}
}

// WAL: readers are never blocked by a write in progress.
func TestReadsDoNotWaitForAWrite(t *testing.T) {
	db := open(t)
	deployed(t, db, "atlas", "ledger")
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("UPDATE deployments SET stop_reason = 'changing' WHERE id = 'atlas@ledger'"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		d, err := db.Deployments().Get(ctx, "atlas@ledger")
		if err == nil && d.StopReason != "" {
			err = errors.New("read saw an uncommitted write: " + d.StopReason)
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Fatal("a read waited for the open write")
	}
}

// --- team messages ------------------------------------------------------------

func deployTwo(t *testing.T, db *DB) {
	t.Helper()
	for _, p := range []string{"ledger", "relay"} {
		if _, err := db.Deployments().Deploy(ctx, "atlas", p); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMessagesAreNumberedPerDeployment(t *testing.T) {
	db := open(t)
	deployTwo(t, db)
	m := db.Messages()
	add := func(dep, from, to, kind string) TeamMessage {
		t.Helper()
		msg, err := m.Add(ctx, TeamMessage{DeploymentID: dep, FromInstance: from, ToInstance: to, Kind: kind, Body: "b"})
		if err != nil {
			t.Fatal(err)
		}
		return msg
	}
	a := add("atlas@ledger", "mara@ledger", "tove@ledger", "ask")
	b := add("atlas@ledger", "tove@ledger", "mara@ledger", "ask")
	c := add("atlas@relay", "mara@relay", "tove@relay", "ask")
	if a.Ref != "msg-1" || b.Ref != "msg-2" || c.Ref != "msg-1" || a.State != MessageQueued || a.CreatedAt.IsZero() {
		t.Errorf("refs %s %s %s, state %s", a.Ref, b.Ref, c.Ref, a.State)
	}
	// Past nine: numbered, not sorted as text.
	for range 9 {
		add("atlas@ledger", "mara@ledger", "tove@ledger", "notice")
	}
	if last := add("atlas@ledger", "mara@ledger", "tove@ledger", "notice"); last.Ref != "msg-12" {
		t.Errorf("after msg-11: %s", last.Ref)
	}
	got, err := m.Get(ctx, "atlas@ledger", "msg-2")
	if err != nil || got.ID != b.ID || got.FromInstance != "tove@ledger" {
		t.Errorf("get %+v %v", got, err)
	}
	if _, err := m.Get(ctx, "atlas@relay", "msg-2"); !errors.Is(err, ErrNotFound) {
		t.Errorf("another deployment's ref: %v", err)
	}
}

func TestConcurrentMessagesGetDistinctRefs(t *testing.T) {
	db := open(t)
	deployTwo(t, db)
	var wg sync.WaitGroup
	refs := make(chan string, 20)
	for range 20 {
		wg.Go(func() {
			msg, err := db.Messages().Add(ctx, TeamMessage{DeploymentID: "atlas@ledger", FromInstance: "a", ToInstance: "b", Kind: "ask", Body: "x"})
			if err != nil {
				t.Error(err)
			}
			refs <- msg.Ref
		})
	}
	wg.Wait()
	close(refs)
	seen := map[string]bool{}
	for r := range refs {
		if seen[r] {
			t.Errorf("%s twice", r)
		}
		seen[r] = true
	}
}

func TestMessageQueueRepliesAndExpiry(t *testing.T) {
	db := open(t)
	deployTwo(t, db)
	m := db.Messages()
	ask, _ := m.Add(ctx, TeamMessage{DeploymentID: "atlas@ledger", FromInstance: "mara@ledger", ToInstance: "tove@ledger", Kind: "ask", Body: "build cal"})
	other, _ := m.Add(ctx, TeamMessage{DeploymentID: "atlas@relay", FromInstance: "mara@relay", ToInstance: "tove@relay", Kind: "ask", Body: "x"})
	if _, err := m.ReplyTo(ctx, "atlas@ledger", ask.Ref); !errors.Is(err, ErrNotFound) {
		t.Errorf("no reply yet: %v", err)
	}
	reply, _ := m.Add(ctx, TeamMessage{DeploymentID: "atlas@ledger", FromInstance: "tove@ledger", ToInstance: "mara@ledger", Kind: "reply",
		InReplyTo: ask.Ref, Body: "done", ArtifactRef: "wt/tove/cal"})
	if got, err := m.ReplyTo(ctx, "atlas@ledger", ask.Ref); err != nil || got.ID != reply.ID || got.ArtifactRef != "wt/tove/cal" {
		t.Errorf("reply %+v %v", got, err)
	}
	if q, _ := m.Queued(ctx, "tove@ledger"); len(q) != 1 || q[0].ID != ask.ID {
		t.Errorf("queued for tove: %+v", q)
	}
	if err := m.SetState(ctx, []int64{ask.ID}, MessageDelivered, ""); err != nil {
		t.Fatal(err)
	}
	got, _ := m.Get(ctx, "atlas@ledger", ask.Ref)
	if got.State != MessageDelivered || got.DeliveredAt.IsZero() {
		t.Errorf("delivered: %+v", got)
	}
	if q, _ := m.Queued(ctx, "tove@ledger"); len(q) != 0 {
		t.Errorf("still queued: %+v", q)
	}
	// Expiry ends only what waits, in that deployment.
	expired, err := m.Expire(ctx, "atlas@ledger", "deployment_stopped")
	if err != nil || len(expired) != 1 || expired[0].ID != reply.ID || expired[0].State != MessageExpired {
		t.Errorf("expired %+v %v", expired, err)
	}
	if got, _ := m.Get(ctx, "atlas@ledger", ask.Ref); got.State != MessageDelivered {
		t.Errorf("a delivered message expired: %s", got.State)
	}
	if got, _ := m.Get(ctx, "atlas@relay", other.Ref); got.State != MessageQueued {
		t.Errorf("another deployment's message expired: %s", got.State)
	}
	if list, _ := m.List(ctx, "atlas@ledger"); len(list) != 2 || list[0].ID != ask.ID {
		t.Errorf("list %+v", list)
	}
}

func TestMessageRules(t *testing.T) {
	db := open(t)
	deployTwo(t, db)
	for what, msg := range map[string]TeamMessage{
		"unknown deployment": {DeploymentID: "nobody@ledger", FromInstance: "a", ToInstance: "b", Kind: "ask", Body: "x"},
		"unknown kind":       {DeploymentID: "atlas@ledger", FromInstance: "a", ToInstance: "b", Kind: "gossip", Body: "x"},
	} {
		if _, err := db.Messages().Add(ctx, msg); err == nil {
			t.Errorf("%s accepted", what)
		}
	}
}

func TestActions(t *testing.T) {
	db := open(t)
	if _, err := db.Deployments().Deploy(ctx, "atlas", "ledger"); err != nil {
		t.Fatal(err)
	}
	a := db.Actions()
	if list, err := a.List(ctx, "atlas@ledger"); err != nil || len(list) != 0 {
		t.Fatalf("empty: %v %v", list, err)
	}
	commit, err := a.Add(ctx, TeamAction{DeploymentID: "atlas@ledger", Instance: "tove@ledger", Tool: "commit",
		Repos: []ActionRepo{{Repo: "api", Message: "cal v0", Branch: "wt/tove/main", Commit: "1a2b3c4d"}}})
	if err != nil || commit.ID == 0 || commit.CreatedAt.IsZero() {
		t.Fatalf("%+v %v", commit, err)
	}
	if _, err := a.Add(ctx, TeamAction{DeploymentID: "atlas@ledger", Instance: "tove@ledger", Tool: "switch", Branch: "main", Error: "branch_not_allowed: …"}); err != nil {
		t.Fatal(err)
	}
	list, err := a.List(ctx, "atlas@ledger")
	if err != nil || len(list) != 2 {
		t.Fatalf("%v %v", list, err)
	}
	if got := list[0]; len(got.Repos) != 1 || got.Repos[0].Message != "cal v0" || got.Repos[0].Commit != "1a2b3c4d" || got.Error != "" {
		t.Errorf("commit %+v", got)
	}
	if got := list[1]; got.Tool != "switch" || got.Branch != "main" || got.Error == "" || got.Repos == nil {
		t.Errorf("refused switch %+v", got)
	}
	if _, err := a.Add(ctx, TeamAction{DeploymentID: "nope", Instance: "x", Tool: "commit"}); err == nil {
		t.Error("an action of no deployment was stored")
	}
}

func TestWorkItems(t *testing.T) {
	db := open(t)
	deps := db.Deployments()
	for _, p := range []struct{ team, project string }{{"orion", "ledger"}, {"atlas", "ledger"}, {"atlas", "relay"}} {
		if _, err := deps.Deploy(ctx, p.team, p.project); err != nil {
			t.Fatal(err)
		}
		if p.team == "orion" { // a project has one team at a time
			if err := deps.Stop(ctx, "orion@ledger", ""); err != nil {
				t.Fatal(err)
			}
		}
	}
	items := db.WorkItems()
	w, err := items.Save(ctx, WorkItem{DeploymentID: "atlas@ledger", AuthorInstance: "tove@ledger", Branch: "wt/tove/LED-44", Stage: StageHanded})
	if err != nil || w.ID != "wi-1" {
		t.Fatalf("saved %+v %v", w, err)
	}
	if _, err := items.Save(ctx, WorkItem{DeploymentID: "orion@ledger", AuthorInstance: "lumen@ledger", Branch: "wt/lumen/x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := items.Save(ctx, WorkItem{DeploymentID: "atlas@relay", AuthorInstance: "tove@relay", Branch: "wt/tove/y"}); err != nil {
		t.Fatal(err)
	}
	// One work item per branch of a deployment.
	if _, err := items.Save(ctx, WorkItem{DeploymentID: "atlas@ledger", AuthorInstance: "tove@ledger", Branch: "wt/tove/LED-44"}); err == nil {
		t.Error("a second work item for the same branch")
	}
	w.Stage, w.Note, w.ChecksStatus, w.CardRef = StageIntegrated, "tests pass", "pass", "backlog/LED-44.md"
	if _, err := items.Save(ctx, w); err != nil {
		t.Fatal(err)
	}
	got, err := items.Find(ctx, "atlas@ledger", "wt/tove/LED-44")
	if err != nil || got != w {
		t.Errorf("found %+v %v, want %+v", got, err, w)
	}
	if _, err := items.Find(ctx, "atlas@ledger", "wt/tove/nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown branch: %v", err)
	}
	for _, id := range []string{"wi-99", "x-1", "wi-"} {
		if _, err := items.Save(ctx, WorkItem{ID: id, DeploymentID: "atlas@ledger", Branch: "b"}); !errors.Is(err, ErrNotFound) {
			t.Errorf("saving %s: %v", id, err)
		}
	}
	ids := func(list []WorkItem, err error) []string {
		var out []string
		for _, w := range list {
			out = append(out, w.ID)
		}
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got := ids(items.ForProject(ctx, "ledger")); !slices.Equal(got, []string{"wi-1", "wi-2"}) {
		t.Errorf("project ledger: %v", got)
	}
	if got := ids(items.ForDeployment(ctx, "atlas@relay")); !slices.Equal(got, []string{"wi-3"}) {
		t.Errorf("deployment atlas@relay: %v", got)
	}
}

func TestMergeRequests(t *testing.T) {
	db := open(t)
	if _, err := db.Deployments().Deploy(ctx, "atlas", "ledger"); err != nil {
		t.Fatal(err)
	}
	mrs := db.MergeRequests()
	m, err := mrs.Save(ctx, MergeRequest{ProjectID: "ledger", DeploymentID: "atlas@ledger", AuthorInstance: "mara@ledger", Repo: "api", Branch: "lead/atlas/integration", Entry: "backlog/led-44.md"})
	if err != nil || m.ID != "mr-1" || m.State != MergeOpen || m.CreatedAt.IsZero() {
		t.Fatalf("saved %+v %v", m, err)
	}
	// One open per (project, repo, branch); another repo or branch is another.
	if _, err := mrs.Save(ctx, MergeRequest{ProjectID: "ledger", AuthorInstance: "mara@ledger", Repo: "api", Branch: "lead/atlas/integration"}); err == nil {
		t.Error("a second open merge request for the same branch")
	}
	if _, err := mrs.Save(ctx, MergeRequest{ProjectID: "ledger", AuthorInstance: "mara@ledger", Repo: "web", Branch: "lead/atlas/integration"}); err != nil {
		t.Fatal(err)
	}
	if got, err := mrs.OpenFor(ctx, "ledger", "api", "lead/atlas/integration"); err != nil || got.ID != "mr-1" {
		t.Errorf("open for api: %+v %v", got, err)
	}
	// A review is found by any of its branch's open merge requests.
	m.ReviewID = "rev-1"
	if m, err = mrs.Save(ctx, m); err != nil {
		t.Fatal(err)
	}
	if id, err := mrs.OpenReview(ctx, "ledger", "lead/atlas/integration"); err != nil || id != "rev-1" {
		t.Errorf("open review %q %v", id, err)
	}
	if id, err := mrs.OpenReview(ctx, "ledger", "lead/atlas/other"); err != nil || id != "" {
		t.Errorf("no open review %q %v", id, err)
	}
	if list, _ := mrs.ForReview(ctx, "rev-1"); len(list) != 1 || list[0].ID != "mr-1" {
		t.Errorf("for review %+v", list)
	}
	m.State, m.MergedAt, m.Conflicting, m.Note = MergeMerged, time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC), true, "x"
	m.ReviewedHead, m.Head, m.Base = "abc123", "def456", "abc000"
	if _, err := mrs.Save(ctx, m); err != nil {
		t.Fatal(err)
	}
	got, err := mrs.Get(ctx, "mr-1")
	if err != nil || got.State != MergeMerged || !got.MergedAt.Equal(m.MergedAt) || !got.Conflicting || got.Entry != "backlog/led-44.md" || got.ReviewID != "rev-1" || got.ReviewedHead != "abc123" || got.Head != "def456" || got.Base != "abc000" {
		t.Errorf("got %+v %v", got, err)
	}
	// A merged one is no longer the branch's open review.
	if id, _ := mrs.OpenReview(ctx, "ledger", "lead/atlas/integration"); id != "" {
		t.Errorf("open review after the merge %q", id)
	}
	// Once merged, the branch can be requested again.
	if _, err := mrs.OpenFor(ctx, "ledger", "api", "lead/atlas/integration"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a merged one is not open: %v", err)
	}
	if _, err := mrs.Save(ctx, MergeRequest{ProjectID: "ledger", AuthorInstance: "mara@ledger", Repo: "api", Branch: "lead/atlas/integration"}); err != nil {
		t.Errorf("requested again after the merge: %v", err)
	}
	if list, _ := mrs.ForProject(ctx, "ledger"); len(list) != 3 {
		t.Errorf("project: %d", len(list))
	}
	if list, _ := mrs.Open(ctx); len(list) != 2 {
		t.Errorf("open: %d", len(list))
	}
	for _, id := range []string{"mr-9", "wi-1", ""} {
		if _, err := mrs.Get(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("get %q: %v", id, err)
		}
	}
}

func TestRequests(t *testing.T) {
	db := open(t)
	reqs := db.Requests()
	r, err := reqs.Save(ctx, Request{Kind: "review", ProjectID: "ledger", DeploymentID: "atlas@ledger", InstanceID: "mara@ledger",
		Summary: "Mara wants it merged", Source: "agent", ReviewID: "rev-1", Links: []Link{{Label: "branch", Href: "x"}}})
	if err != nil || r.ID != "req-1" || r.Status != RequestOpen || r.CreatedAt.IsZero() {
		t.Fatalf("saved %+v %v", r, err)
	}
	if _, err := reqs.Save(ctx, Request{Kind: "review", ProjectID: "ledger", Summary: "other", Body: "Line one.\n\nLine two.", Source: "agent", ReviewID: "rev-2"}); err != nil {
		t.Fatal(err)
	}
	got, err := reqs.ForReview(ctx, "rev-1")
	if err != nil || got.ID != "req-1" || got.Body != "" || got.Options == nil || len(got.Links) != 1 || got.Links[0].Label != "branch" {
		t.Errorf("for rev-1: %+v %v", got, err)
	}
	r.Status, r.Answer, r.ResolvedAt = RequestResolved, "merged", time.Now().UTC()
	if _, err := reqs.Save(ctx, r); err != nil {
		t.Fatal(err)
	}
	if _, err := reqs.ForReview(ctx, "rev-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a resolved review is not open: %v", err)
	}
	if open, _ := reqs.Open(ctx); len(open) != 1 || open[0].ID != "req-2" || open[0].Body != "Line one.\n\nLine two." {
		t.Errorf("open %+v", open)
	}
	if _, err := reqs.Save(ctx, Request{ID: "req-9", Kind: "review", Summary: "x", Source: "agent"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown id: %v", err)
	}
}

// --- projects -------------------------------------------------------------------

// A project is archived and unarchived on its own; one never added can't be.
func TestArchiveAProject(t *testing.T) {
	p := open(t).Projects()
	for _, id := range []string{"ledger", "aragon"} {
		if err := p.Create(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
	}
	archived := func() []bool {
		list, err := p.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var got []bool
		for _, r := range list {
			got = append(got, r.Archived)
		}
		return got
	}
	if got := archived(); !slices.Equal(got, []bool{false, false}) {
		t.Errorf("new projects: %v", got)
	}
	if err := p.SetArchived(ctx, "Ledger", true); err != nil {
		t.Fatal(err)
	}
	if got := archived(); !slices.Equal(got, []bool{false, true}) {
		t.Errorf("ledger archived: %v", got)
	}
	if err := p.SetArchived(ctx, "ledger", false); err != nil {
		t.Fatal(err)
	}
	if got := archived(); !slices.Equal(got, []bool{false, false}) {
		t.Errorf("ledger unarchived: %v", got)
	}
	if err := p.SetArchived(ctx, "nope", true); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
}

func TestProjectsAndTheirRepos(t *testing.T) {
	p := open(t).Projects()
	if list, err := p.List(ctx); err != nil || len(list) != 0 {
		t.Fatalf("empty: %v %v", list, err)
	}
	for _, id := range []string{"ledger", "aragon"} {
		if err := p.Create(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
	}
	// A Mac takes Ledger and ledger for one folder.
	if err := p.Create(ctx, "Ledger", ""); !errors.Is(err, ErrExists) {
		t.Errorf("a name in another case: %v", err)
	}
	app := RepoRecord{Name: "app", Path: "/home/ceo/dev/aragon/app", RootCommit: "c0ffee", Default: "master"}
	sdk := RepoRecord{Name: "sdk", Path: "/home/ceo/dev/aragon/sdk", RootCommit: "beef", Default: "main"}
	for _, r := range []RepoRecord{app, sdk} {
		if err := p.AddRepo(ctx, "aragon", r); err != nil {
			t.Fatal(err)
		}
	}
	// One repo in two projects.
	if err := p.AddRepo(ctx, "ledger", app); err != nil {
		t.Errorf("a repo in a second project: %v", err)
	}
	for name, r := range map[string]RepoRecord{
		"a name in another case": {Name: "App", Path: "/elsewhere", RootCommit: "1"},
		"the same path":          {Name: "app2", Path: app.Path, RootCommit: "c0ffee"},
	} {
		if err := p.AddRepo(ctx, "aragon", r); !errors.Is(err, ErrExists) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := p.AddRepo(ctx, "nope", app); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
	if err := p.SetProfile(ctx, "aragon", "work"); err != nil {
		t.Fatal(err)
	}
	list, err := p.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("%v %v", list, err)
	}
	aragon := list[0]
	if aragon.ID != "aragon" || aragon.Profile != "work" || aragon.CreatedAt.IsZero() ||
		!reflect.DeepEqual(withoutTargets(aragon.Repos), []RepoRecord{app, sdk}) || targetNames(aragon.Repos[0]) != "master*" {
		t.Errorf("aragon: %+v", aragon)
	}
	if list[1].Profile != "" || len(list[1].Repos) != 1 {
		t.Errorf("ledger: %+v", list[1])
	}

	// A default branch of its own per repo, changed in one project only; the
	// old one stays a target, and setting it back moves only the flag.
	if err := p.SetRepoDefault(ctx, "aragon", "app", TargetRecord{Name: "trunk", From: "c0ffee", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if list, _ := p.List(ctx); list[0].Repos[0].Default != "trunk" || targetNames(list[0].Repos[0]) != "trunk*,master" || list[0].Repos[1].Default != "main" || list[1].Repos[0].Default != "master" {
		t.Errorf("after SetRepoDefault: %+v", list)
	}
	if err := p.SetRepoDefault(ctx, "aragon", "app", TargetRecord{Name: "master"}); err != nil {
		t.Fatal(err)
	}
	if list, _ := p.List(ctx); targetNames(list[0].Repos[0]) != "master*,trunk" || list[0].Repos[0].Targets[0].From != "" {
		t.Errorf("set back: %+v", list[0].Repos[0].Targets)
	}

	// Locate and remove, each touching only that project's repo.
	if err := p.MoveRepo(ctx, "aragon", "app", "/home/ceo/src/app"); err != nil {
		t.Fatal(err)
	}
	if err := p.MoveRepo(ctx, "aragon", "app", "/home/ceo/dev/aragon/sdk"); !errors.Is(err, ErrExists) {
		t.Errorf("onto another repo's path: %v", err)
	}
	if err := p.RemoveRepo(ctx, "aragon", "sdk"); err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{
		"locate a repo that isn't there":        p.MoveRepo(ctx, "aragon", "sdk", "/x"),
		"remove it again":                       p.RemoveRepo(ctx, "aragon", "sdk"),
		"a profile for no project":              p.SetProfile(ctx, "nope", "work"),
		"a default for a repo that isn't there": p.SetRepoDefault(ctx, "aragon", "sdk", TargetRecord{Name: "main"}),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	list, _ = p.List(ctx)
	if !reflect.DeepEqual(withoutTargets(list[0].Repos), []RepoRecord{{Name: "app", Path: "/home/ceo/src/app", RootCommit: "c0ffee", Default: "master"}}) || !reflect.DeepEqual(withoutTargets(list[1].Repos), []RepoRecord{app}) {
		t.Errorf("after locate and remove: %+v", list)
	}
}

// Renaming a repo moves the open merge requests on it; closed ones keep the
// name they had.
func TestRenameRepo(t *testing.T) {
	db := open(t)
	p := db.Projects()
	if err := p.Create(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	for _, r := range []RepoRecord{{Name: "api", Path: "/h/api", RootCommit: "a"}, {Name: "web", Path: "/h/web", RootCommit: "b"}} {
		if err := p.AddRepo(ctx, "ledger", r); err != nil {
			t.Fatal(err)
		}
	}
	mrs := db.MergeRequests()
	open, _ := mrs.Save(ctx, MergeRequest{ProjectID: "ledger", AuthorInstance: "mara@ledger", Repo: "api", Branch: "lead/atlas/integration"})
	closed, _ := mrs.Save(ctx, MergeRequest{ProjectID: "ledger", AuthorInstance: "mara@ledger", Repo: "api", Branch: "lead/atlas/old", State: MergeMerged})
	other, _ := mrs.Save(ctx, MergeRequest{ProjectID: "relay", AuthorInstance: "kai@relay", Repo: "api", Branch: "lead/orion/integration"})
	if err := p.RenameRepo(ctx, "ledger", "api", "ledger-api"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		id, want string
	}{{open.ID, "ledger-api"}, {closed.ID, "api"}, {other.ID, "api"}} {
		if got, _ := mrs.Get(ctx, c.id); got.Repo != c.want {
			t.Errorf("%s: repo %q, want %q", c.id, got.Repo, c.want)
		}
	}
	list, _ := p.List(ctx)
	if names := []string{list[0].Repos[0].Name, list[0].Repos[1].Name}; names[0] != "ledger-api" || names[1] != "web" {
		t.Errorf("repos %v", names)
	}
	if err := p.RenameRepo(ctx, "ledger", "ledger-api", "WEB"); !errors.Is(err, ErrExists) {
		t.Errorf("onto another repo's name, in another case: %v", err)
	}
	if err := p.RenameRepo(ctx, "ledger", "gone", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such repo: %v", err)
	}
	if got, _ := mrs.Get(ctx, open.ID); got.Repo != "ledger-api" {
		t.Error("a refused rename changed a merge request")
	}
}

// Removing a project deletes it and its whole history, and nothing of
// another project's; the name can then start clean.
func TestRemoveDeletesAProjectsHistory(t *testing.T) {
	db := open(t)
	p := db.Projects()
	for _, id := range []string{"ledger", "relay"} {
		if err := p.Create(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
		if err := p.AddRepo(ctx, id, RepoRecord{Name: "api", Path: "/h/" + id, RootCommit: "a"}); err != nil {
			t.Fatal(err)
		}
		dep, err := db.Deployments().Deploy(ctx, "atlas", id)
		if err != nil {
			t.Fatal(err)
		}
		inst := "mara@" + id
		if err := db.Instances().Put(ctx, Instance{ID: inst, DeploymentID: dep.ID, AgentID: "mara", Status: "asleep", SandboxPath: "/s", Harness: "fake"}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Messages().Add(ctx, TeamMessage{DeploymentID: dep.ID, FromInstance: inst, ToInstance: inst, Kind: "ask", Body: "b"}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Actions().Add(ctx, TeamAction{DeploymentID: dep.ID, Instance: inst, Tool: "commit"}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.WorkItems().Save(ctx, WorkItem{DeploymentID: dep.ID, AuthorInstance: inst, Branch: "wt/mara/x"}); err != nil {
			t.Fatal(err)
		}
		mr, err := db.MergeRequests().Save(ctx, MergeRequest{ProjectID: id, DeploymentID: dep.ID, AuthorInstance: inst, Repo: "api", Branch: "lead/atlas/integration"})
		if err != nil {
			t.Fatal(err)
		}
		rev, err := db.Reviews().Save(ctx, Review{ProjectID: id, Branch: mr.Branch})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ReviewComments().Save(ctx, ReviewComment{ReviewID: rev.ID, Body: "b"}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Requests().Save(ctx, Request{Kind: "review", ProjectID: id, DeploymentID: dep.ID, Summary: "s", Source: "agent", ReviewID: rev.ID}); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.Remove(ctx, "ledger"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"projects", "project_repos", "deployments", "instances", "team_messages", "team_actions", "work_items", "merge_requests", "requests", "reviews", "review_comments"} {
		var n int
		if err := db.sql.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("%s has %d rows, want relay's one", table, n)
		}
	}
	if err := p.Remove(ctx, "ledger"); !errors.Is(err, ErrNotFound) {
		t.Errorf("removed twice: %v", err)
	}
	// Added again: a new deployment, not the old one revived.
	if err := p.Create(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	dep, err := db.Deployments().Deploy(ctx, "atlas", "ledger")
	if err != nil {
		t.Fatal(err)
	}
	if insts, _ := db.Instances().List(ctx, dep.ID); len(insts) != 0 {
		t.Errorf("the old instances came back: %+v", insts)
	}
}

// Foreman's own stop keeps its reason; the CEO's later Stop doesn't replace
// it, and a redeploy clears it.
func TestStopReason(t *testing.T) {
	deps := open(t).Deployments()
	d, _ := deps.Deploy(ctx, "atlas", "ledger")
	if err := deps.Stop(ctx, d.ID, "its project is gone"); err != nil {
		t.Fatal(err)
	}
	if err := deps.Stop(ctx, d.ID, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := deps.Get(ctx, d.ID); got.StopReason != "its project is gone" {
		t.Errorf("reason %q", got.StopReason)
	}
	r, _ := deps.Deploy(ctx, "atlas", "ledger")
	if r.StopReason != "" {
		t.Errorf("redeployed with reason %q", r.StopReason)
	}
}

// A repo added before migration 004 was on the project's main branch, which
// defaulted to main: it reads as main.
func TestARepoFromBeforeItsOwnMainIsOnMain(t *testing.T) {
	db := migratedTo(t, 3)
	if _, err := db.Exec(`INSERT INTO projects (id, created_at) VALUES ('ledger', '2026-10-20T00:00:00Z');
		INSERT INTO project_repos (project_id, name, path, root_commit) VALUES ('ledger', 'app', '/h/app', 'c0ffee')`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migrationFiles); err != nil {
		t.Fatal(err)
	}
	list, err := projects{db}.List(ctx)
	if err != nil || len(list) != 1 || len(list[0].Repos) != 1 || list[0].Repos[0].Default != "main" {
		t.Errorf("%+v %v", list, err)
	}
}

func TestReviewsAndTheirComments(t *testing.T) {
	db := open(t)
	revs, comments := db.Reviews(), db.ReviewComments()
	r, err := revs.Save(ctx, Review{ProjectID: "ledger", Branch: "lead/atlas/integration"})
	if err != nil || r.ID != "rev-1" || r.CreatedAt.IsZero() || !r.ChangesRequestedAt.IsZero() {
		t.Fatalf("saved %+v %v", r, err)
	}
	r.ChangesRequestedAt = time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	r.ChangesNote = "round it"
	r.WithdrawnAt, r.WithdrawnReason, r.SupersededBy = time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC), "all of it is in lead/atlas/big", "rev-9"
	if _, err := revs.Save(ctx, r); err != nil {
		t.Fatal(err)
	}
	if got, err := revs.Get(ctx, "rev-1"); err != nil || !got.ChangesRequestedAt.Equal(r.ChangesRequestedAt) || got.Branch != r.Branch ||
		!got.WithdrawnAt.Equal(r.WithdrawnAt) || got.WithdrawnReason != r.WithdrawnReason || got.SupersededBy != "rev-9" || got.ChangesNote != "round it" {
		t.Errorf("got %+v %v", got, err)
	}
	if _, err := revs.Save(ctx, Review{ProjectID: "other", Branch: "lead/x/y"}); err != nil {
		t.Fatal(err)
	}
	if list, _ := revs.List(ctx); len(list) != 2 || list[0].ID != "rev-1" || !list[1].WithdrawnAt.IsZero() || list[1].SupersededBy != "" {
		t.Errorf("list %+v", list)
	}

	line, err := comments.Save(ctx, ReviewComment{ReviewID: "rev-1", Repo: "api", Path: "a.go", Side: "new", Line: 12, Commit: "c0ffee", Snippet: "x := 1", Body: "why 1?"})
	if err != nil || line.ID != "com-1" || line.State != CommentDraft || line.CreatedAt.IsZero() {
		t.Fatalf("saved %+v %v", line, err)
	}
	if _, err := comments.Save(ctx, ReviewComment{ReviewID: "rev-1", Body: "general"}); err != nil {
		t.Fatal(err)
	}
	if _, err := comments.Save(ctx, ReviewComment{ReviewID: "rev-2", Body: "elsewhere"}); err != nil {
		t.Fatal(err)
	}
	line.State, line.SentAt = CommentSent, time.Now().UTC()
	if _, err := comments.Save(ctx, line); err != nil {
		t.Fatal(err)
	}
	got, err := comments.ForReview(ctx, "rev-1")
	if err != nil || len(got) != 2 || got[0].State != CommentSent || got[0].SentAt.IsZero() || got[0].Line != 12 || got[0].Snippet != "x := 1" || got[1].Repo != "" {
		t.Errorf("for rev-1 %+v %v", got, err)
	}
	if err := comments.Delete(ctx, "com-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := comments.Get(ctx, "com-2"); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted: %v", err)
	}
	for _, id := range []string{"com-9", "rev-1", ""} {
		if err := comments.Delete(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("delete %q: %v", id, err)
		}
	}
	if _, err := revs.Get(ctx, "mr-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a merge request id is no review: %v", err)
	}
}

// Open merge requests from before reviews: one review per branch, their review
// requests merged into one; closed ones keep none.
func TestOpenMergeRequestsFromBeforeGetAReviewPerBranch(t *testing.T) {
	db := migratedTo(t, 4)
	const at = "2026-10-20T00:00:00Z"
	if _, err := db.Exec(`
		INSERT INTO merge_requests (project_id, author_instance, repo, branch, state, created_at) VALUES
			('ledger', 'mara@ledger', 'api', 'lead/atlas/integration', 'open', '` + at + `'),
			('ledger', 'mara@ledger', 'web', 'lead/atlas/integration', 'open', '` + at + `'),
			('ledger', 'mara@ledger', 'api', 'lead/atlas/old', 'merged', '` + at + `'),
			('ledger', 'mara@ledger', 'api', 'lead/atlas/v2', 'open', '` + at + `');
		INSERT INTO requests (kind, project_id, summary, options, source, status, created_at, merge_request_id, links) VALUES
			('review', 'ledger', 'api', '[]', 'agent', 'open', '` + at + `', 'mr-1', '[]'),
			('review', 'ledger', 'web', '[]', 'agent', 'open', '` + at + `', 'mr-2', '[]'),
			('review', 'ledger', 'old', '[]', 'agent', 'resolved', '` + at + `', 'mr-3', '[]'),
			('review', 'ledger', 'v2', '[]', 'agent', 'open', '` + at + `', 'mr-4', '[]');`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migrationFiles); err != nil {
		t.Fatal(err)
	}
	mrs := mergeRequests{db}
	for id, want := range map[string]string{"mr-1": "rev-1", "mr-2": "rev-1", "mr-3": "", "mr-4": "rev-2"} {
		if m, err := mrs.Get(ctx, id); err != nil || m.ReviewID != want {
			t.Errorf("%s in %q, want %q (%v)", id, m.ReviewID, want, err)
		}
	}
	open, err := requests{db}.Open(ctx)
	if err != nil || len(open) != 2 || open[0].ID != "req-1" || open[0].ReviewID != "rev-1" || open[1].ReviewID != "rev-2" {
		t.Errorf("open requests %+v %v", open, err)
	}
	if list, _ := (reviews{db}).List(ctx); len(list) != 2 || list[0].Branch != "lead/atlas/integration" || list[1].Branch != "lead/atlas/v2" {
		t.Errorf("reviews %+v", list)
	}
}

// A project keeps the workspace it was made in, a project made before
// workspaces has none, and a migration sets one.
func TestProjectWorkspace(t *testing.T) {
	p := open(t).Projects()
	if err := p.Create(ctx, "ledger", "/home/ceo/workspace/foreman/ledger"); err != nil {
		t.Fatal(err)
	}
	if err := p.Create(ctx, "aragon", ""); err != nil {
		t.Fatal(err)
	}
	workspaces := func() []string {
		list, err := p.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, r := range list {
			out = append(out, r.Workspace)
		}
		return out
	}
	if got := workspaces(); !slices.Equal(got, []string{"", "/home/ceo/workspace/foreman/ledger"}) {
		t.Errorf("created: %q", got)
	}
	if err := p.SetWorkspace(ctx, "aragon", "/home/ceo/workspace/foreman/aragon"); err != nil {
		t.Fatal(err)
	}
	if got := workspaces(); got[0] != "/home/ceo/workspace/foreman/aragon" {
		t.Errorf("migrated: %q", got)
	}
	if err := p.SetWorkspace(ctx, "nope", "/x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
}

// migratedTo is a database with the first n of the real migrations applied,
// to fill with rows from before the next ones.
func migratedTo(t *testing.T, n int) *sql.DB {
	t.Helper()
	names, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil || len(names) < n {
		t.Fatalf("%d migrations: %v", len(names), err)
	}
	first := fstest.MapFS{}
	for _, name := range names[:n] {
		data, err := fs.ReadFile(migrationFiles, name)
		if err != nil {
			t.Fatal(err)
		}
		first[name] = &fstest.MapFile{Data: data}
	}
	db := rawDB(t)
	if err := migrate(ctx, db, first); err != nil {
		t.Fatal(err)
	}
	return db
}

// A merge request from before targets merges into its repo's default branch
// as it was then; one on a repo the project no longer has, into main.
func TestMergeRequestsFromBeforeTargetsGoToTheDefault(t *testing.T) {
	db := migratedTo(t, 13)
	const at = "2026-10-20T00:00:00Z"
	if _, err := db.Exec(`
		INSERT INTO projects (id, created_at) VALUES ('ledger', '` + at + `');
		INSERT INTO project_repos (project_id, name, path, root_commit, main) VALUES ('ledger', 'api', '/h/api', 'a', 'master');
		INSERT INTO merge_requests (project_id, author_instance, repo, branch, state, created_at) VALUES
			('ledger', 'mara@ledger', 'API', 'lead/atlas/one', 'open', '` + at + `'),
			('ledger', 'mara@ledger', 'gone', 'lead/atlas/two', 'merged', '` + at + `');`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migrationFiles); err != nil {
		t.Fatal(err)
	}
	mrs := mergeRequests{db}
	for id, want := range map[string]string{"mr-1": "master", "mr-2": "main"} {
		if m, err := mrs.Get(ctx, id); err != nil || m.Target != want {
			t.Errorf("%s into %q, want %q (%v)", id, m.Target, want, err)
		}
	}
	if list, _ := (projects{db}).List(ctx); len(list) != 1 || list[0].Repos[0].Default != "master" {
		t.Errorf("default branch kept: %+v", list)
	}
}

// A repo's targets: one list, its default first and flagged, then oldest
// first; one per name; kept apart per project; following a rename, gone with
// the repo; the default never removed, another refused removal while a merge
// request into it is open. Changing the default moves the flag: the old one
// stays a target and merge requests keep theirs.
func TestTargets(t *testing.T) {
	db := open(t)
	p := db.Projects()
	for _, id := range []string{"ledger", "relay"} {
		if err := p.Create(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
		if err := p.AddRepo(ctx, id, RepoRecord{Name: "api", Path: "/h/" + id, RootCommit: "a", Default: "main"}); err != nil {
			t.Fatal(err)
		}
	}
	at := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	release := TargetRecord{Name: "release/2.0", MadeBy: "mara@ledger", From: "c0ffee", CreatedAt: at}
	hotfix := TargetRecord{Name: "hotfix", MadeBy: "mara@ledger", From: "beef", CreatedAt: at.Add(time.Hour)}
	for _, tg := range []TargetRecord{hotfix, release} {
		if err := p.AddTarget(ctx, "ledger", "api", tg); err != nil {
			t.Fatal(err)
		}
	}
	for name, c := range map[string]struct {
		repo string
		tg   TargetRecord
		want error
	}{
		"the same name again":     {"api", release, ErrExists},
		"the repo's default":      {"API", TargetRecord{Name: "main", CreatedAt: at}, ErrExists},
		"a repo that isn't there": {"web", release, ErrNotFound},
	} {
		if err := p.AddTarget(ctx, "ledger", c.repo, c.tg); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
	repo := func(project string) RepoRecord {
		list, _ := p.List(ctx)
		for _, r := range list {
			if r.ID == project {
				return r.Repos[0]
			}
		}
		return RepoRecord{}
	}
	if got := repo("ledger"); targetNames(got) != "main*,release/2.0,hotfix" || !reflect.DeepEqual(got.Targets[1:], []TargetRecord{release, hotfix}) || got.Targets[0].MadeBy != "" {
		t.Errorf("the default first, then oldest first: %+v", got.Targets)
	}
	if got := repo("relay"); targetNames(got) != "main*" {
		t.Errorf("another project's repo: %+v", got.Targets)
	}

	mrs := db.MergeRequests()
	into, _ := mrs.Save(ctx, MergeRequest{ProjectID: "ledger", AuthorInstance: "mara@ledger", Repo: "api", Branch: "lead/atlas/fix", Target: "hotfix"})
	if err := p.RemoveTarget(ctx, "ledger", "api", "hotfix"); !errors.Is(err, ErrInUse) {
		t.Errorf("with a merge request open into it: %v", err)
	}
	into.State = MergeMerged
	if _, err := mrs.Save(ctx, into); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveTarget(ctx, "ledger", "api", "hotfix"); err != nil {
		t.Errorf("once it merged: %v", err)
	}
	for name, target := range map[string]string{"again": "hotfix", "the default": "main"} {
		if err := p.RemoveTarget(ctx, "ledger", "api", target); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}

	// Renamed with its repo.
	if err := p.RenameRepo(ctx, "ledger", "api", "core"); err != nil {
		t.Fatal(err)
	}
	if got := repo("ledger"); targetNames(got) != "main*,release/2.0" {
		t.Errorf("after a rename: %+v", got.Targets)
	}

	// The default moves to a target, then to a branch that wasn't one.
	toMain, _ := mrs.Save(ctx, MergeRequest{ProjectID: "ledger", AuthorInstance: "mara@ledger", Repo: "core", Branch: "lead/atlas/a", Target: "main"})
	if err := p.SetRepoDefault(ctx, "ledger", "core", TargetRecord{Name: "release/2.0"}); err != nil {
		t.Fatal(err)
	}
	if got := repo("ledger"); got.Default != "release/2.0" || targetNames(got) != "release/2.0*,main" || got.Targets[0].MadeBy != "mara@ledger" {
		t.Errorf("made the default: %+v", got.Targets)
	}
	if err := p.SetRepoDefault(ctx, "ledger", "core", TargetRecord{Name: "trunk", From: "abc", CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	if got := repo("ledger"); targetNames(got) != "trunk*,release/2.0,main" || got.Targets[0].From != "abc" {
		t.Errorf("a new default: %+v", got.Targets)
	}
	if got, _ := mrs.Get(ctx, toMain.ID); got.Target != "main" {
		t.Errorf("a merge request moved with the default: into %q", got.Target)
	}

	// Gone with its repo, and with its project.
	if err := p.RemoveRepo(ctx, "ledger", "core"); err != nil {
		t.Fatal(err)
	}
	if err := p.AddRepo(ctx, "ledger", RepoRecord{Name: "core", Path: "/h/core", RootCommit: "a", Default: "main"}); err != nil {
		t.Fatal(err)
	}
	if got := repo("ledger"); targetNames(got) != "main*" {
		t.Errorf("a repo added again kept its old targets: %+v", got.Targets)
	}
	if err := p.AddTarget(ctx, "relay", "api", hotfix); err != nil {
		t.Fatal(err)
	}
	if err := p.Remove(ctx, "relay"); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := db.sql.QueryRow("SELECT count(*) FROM repo_targets WHERE project_id = 'relay'").Scan(&left); err != nil || left != 0 {
		t.Errorf("%d targets left (%v)", left, err)
	}
}

// Before 015 a repo's default branch was a column, its other targets apart,
// and comments had no target: after it, the default heads its targets, and a
// comment was read on its merge request's target.
func TestTargetsFromBeforeOneList(t *testing.T) {
	db := migratedTo(t, 14)
	const at = "2026-11-01T00:00:00Z"
	if _, err := db.Exec(`
		INSERT INTO projects (id, created_at) VALUES ('ledger', '` + at + `');
		INSERT INTO project_repos (project_id, name, path, root_commit, default_branch) VALUES ('ledger', 'api', '/h/api', 'a', 'master');
		INSERT INTO repo_targets (project_id, repo, name, made_by, from_commit, created_at) VALUES ('ledger', 'api', 'release', 'mara@ledger', 'c0ffee', '` + at + `');
		INSERT INTO merge_requests (project_id, author_instance, repo, branch, state, created_at, review_id, target) VALUES
			('ledger', 'mara@ledger', 'api', 'lead/atlas/one', 'open', '` + at + `', 'rev-1', 'release');
		INSERT INTO review_comments (review_id, repo, path, side, line, body, created_at) VALUES
			('rev-1', 'api', 'a.go', 'old', 3, 'why', '` + at + `'),
			('rev-1', '', '', '', 0, 'general', '` + at + `');`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, migrationFiles); err != nil {
		t.Fatal(err)
	}
	list, err := projects{db}.List(ctx)
	if err != nil || len(list) != 1 || list[0].Repos[0].Default != "master" || targetNames(list[0].Repos[0]) != "master*,release" {
		t.Errorf("%+v %v", list, err)
	}
	comments, err := reviewComments{db}.ForReview(ctx, "rev-1")
	if err != nil || len(comments) != 2 || comments[0].Target != "release" || comments[1].Target != "" {
		t.Errorf("%+v %v", comments, err)
	}
}

// withoutTargets are repos as added: their targets left out.
func withoutTargets(repos []RepoRecord) []RepoRecord {
	var list []RepoRecord
	for _, r := range repos {
		r.Targets = nil
		list = append(list, r)
	}
	return list
}

// targetNames are a repo's targets by name, in order, the default starred.
func targetNames(r RepoRecord) string {
	var names []string
	for _, t := range r.Targets {
		if t.Default {
			t.Name += "*"
		}
		names = append(names, t.Name)
	}
	return strings.Join(names, ",")
}
