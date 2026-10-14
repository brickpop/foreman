package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Instance is one agent running in one deployment
// (wiki/product/models/instance.md). Empty strings and zero values are
// stored as NULL.
type Instance struct {
	ID               string // agent@project
	DeploymentID     string
	AgentID          string
	Status           string // queued, starting, running, idle, awaiting, done or stopped
	Task             string // the current task note, for a restart
	PID              int
	PtyID            string // the engine's handle (e.g. the container id)
	SandboxPath      string
	Harness          string
	HarnessSessionID string // for the harness's own resume
	StartedAt        time.Time
	LastOutputAt     time.Time
	Restarts         int
}

// Instances is the instances table.
type Instances interface {
	// Put writes the whole instance, inserting or replacing it. Its deployment
	// must exist.
	Put(ctx context.Context, inst Instance) error
	Get(ctx context.Context, id string) (Instance, error)
	// List returns a deployment's instances, by id.
	List(ctx context.Context, deploymentID string) ([]Instance, error)
}

// Instances returns the instances table.
func (db *DB) Instances() Instances { return instances{db.sql} }

type instances struct{ db *sql.DB }

const instanceColumns = "id, deployment_id, agent_id, status, task, pid, pty_id, sandbox_path, harness, harness_session_id, started_at, last_output_at, restarts"

func (s instances) Put(ctx context.Context, i Instance) error {
	_, err := s.db.ExecContext(ctx, "INSERT OR REPLACE INTO instances ("+instanceColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		i.ID, i.DeploymentID, i.AgentID, i.Status, nullText(i.Task), nullInt(i.PID), nullText(i.PtyID), i.SandboxPath, i.Harness,
		nullText(i.HarnessSessionID), timeText(i.StartedAt), timeText(i.LastOutputAt), i.Restarts)
	return err
}

func (s instances) Get(ctx context.Context, id string) (Instance, error) {
	return scanInstance(s.db.QueryRowContext(ctx, "SELECT "+instanceColumns+" FROM instances WHERE id = ?", id))
}

func (s instances) List(ctx context.Context, deploymentID string) ([]Instance, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+instanceColumns+" FROM instances WHERE deployment_id = ? ORDER BY id", deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Instance
	for rows.Next() {
		inst, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, inst)
	}
	return list, rows.Err()
}

func scanInstance(row interface{ Scan(...any) error }) (Instance, error) {
	var i Instance
	var task, ptyID, sessionID, started, lastOutput sql.NullString
	var pid sql.NullInt64
	err := row.Scan(&i.ID, &i.DeploymentID, &i.AgentID, &i.Status, &task, &pid, &ptyID, &i.SandboxPath, &i.Harness,
		&sessionID, &started, &lastOutput, &i.Restarts)
	if errors.Is(err, sql.ErrNoRows) {
		return Instance{}, ErrNotFound
	}
	if err != nil {
		return Instance{}, err
	}
	i.Task, i.PID, i.PtyID, i.HarnessSessionID = task.String, int(pid.Int64), ptyID.String, sessionID.String
	if i.StartedAt, err = parseTime(started); err != nil {
		return Instance{}, err
	}
	i.LastOutputAt, err = parseTime(lastOutput)
	return i, err
}

func nullText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}
