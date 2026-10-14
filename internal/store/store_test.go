package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
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
	// Workspace folders may have spaces and characters a SQLite URI treats specially.
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
	if v := version(t, db.sql); v != 1 {
		t.Errorf("version %d after the first open", v)
	}
	var mode string
	_ = db.sql.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Errorf("journal mode %q, want wal", mode)
	}
	if _, err := db.Deployments().Deploy(ctx, "atlas", "ledger", "ship it"); err != nil {
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
	d, err := deps.Deploy(ctx, "atlas", "ledger", "ship invoices")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "atlas@ledger" || d.Status != DeploymentActive || d.Goal != "ship invoices" || d.CreatedAt.Before(before) || !d.StoppedAt.IsZero() {
		t.Errorf("deployed %+v", d)
	}
	if _, err := deps.Deploy(ctx, "atlas", "ledger", "again"); !errors.Is(err, ErrActive) {
		t.Errorf("second deploy while active: %v", err)
	}

	if err := deps.Stop(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	stopped, _ := deps.Get(ctx, d.ID)
	if stopped.Status != DeploymentStopped || stopped.StoppedAt.IsZero() {
		t.Errorf("stopped %+v", stopped)
	}
	if err := deps.Stop(ctx, d.ID); err != nil {
		t.Errorf("stopping twice: %v", err)
	}
	if again, _ := deps.Get(ctx, d.ID); !again.StoppedAt.Equal(stopped.StoppedAt) {
		t.Error("stopping twice moved stopped_at")
	}

	// Re-deploying reactivates the same row: new goal, same id and creation.
	r, err := deps.Deploy(ctx, "atlas", "ledger", "ship payments")
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != d.ID || r.Status != DeploymentActive || r.Goal != "ship payments" || !r.CreatedAt.Equal(d.CreatedAt) || !r.StoppedAt.IsZero() {
		t.Errorf("redeployed %+v, first %+v", r, d)
	}
}

func TestDeploymentLookupsAndRules(t *testing.T) {
	deps := open(t).Deployments()
	if _, err := deps.Get(ctx, "nobody@nowhere"); !errors.Is(err, ErrNotFound) {
		t.Errorf("get unknown: %v", err)
	}
	if err := deps.Stop(ctx, "nobody@nowhere"); !errors.Is(err, ErrNotFound) {
		t.Errorf("stop unknown: %v", err)
	}
	if _, err := deps.Deploy(ctx, "atlas", "ledger", "   "); err == nil {
		t.Error("a blank goal was accepted")
	}
	for _, pair := range [][2]string{{"atlas", "relay"}, {"atlas", "ledger"}, {"build", "ledger"}} {
		if _, err := deps.Deploy(ctx, pair[0], pair[1], "goal"); err != nil {
			t.Fatal(err)
		}
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
			_, err := deps.Deploy(ctx, "atlas", "ledger", "goal")
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
	if _, err := db.Deployments().Deploy(ctx, team, project, "goal"); err != nil {
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
		Harness: "claude-code", HarnessSessionID: "s-1", StartedAt: at, LastOutputAt: at.Add(time.Minute), Restarts: 2,
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
		if !got.StartedAt.Equal(want.StartedAt) || !got.LastOutputAt.Equal(want.LastOutputAt) {
			t.Errorf("times %v %v, want %v %v", got.StartedAt, got.LastOutputAt, want.StartedAt, want.LastOutputAt)
		}
		got.StartedAt, got.LastOutputAt, want.StartedAt, want.LastOutputAt = time.Time{}, time.Time{}, time.Time{}, time.Time{}
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
	if _, err := tx.Exec("UPDATE deployments SET goal = 'changing' WHERE id = 'atlas@ledger'"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		d, err := db.Deployments().Get(ctx, "atlas@ledger")
		if err == nil && d.Goal != "goal" {
			err = errors.New("read saw an uncommitted write: " + d.Goal)
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
