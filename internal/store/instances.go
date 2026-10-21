package store

import (
	"context"
	"database/sql"
	"errors"
)

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

const instanceColumns = "id, deployment_id, agent_id, status, task, pid, pty_id, sandbox_path, harness, profile, harness_session_id, started_at, restarts, stop_reason"

func (s instances) Put(ctx context.Context, i Instance) error {
	_, err := s.db.ExecContext(ctx, "INSERT OR REPLACE INTO instances ("+instanceColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		i.ID, i.DeploymentID, i.AgentID, i.Status, nullText(i.Task), nullInt(i.PID), nullText(i.PtyID), i.SandboxPath, i.Harness, i.Profile,
		nullText(i.HarnessSessionID), timeText(i.StartedAt), i.Restarts, i.StopReason)
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
	var task, ptyID, sessionID, started sql.NullString
	var pid sql.NullInt64
	err := row.Scan(&i.ID, &i.DeploymentID, &i.AgentID, &i.Status, &task, &pid, &ptyID, &i.SandboxPath, &i.Harness, &i.Profile,
		&sessionID, &started, &i.Restarts, &i.StopReason)
	if errors.Is(err, sql.ErrNoRows) {
		return Instance{}, ErrNotFound
	}
	if err != nil {
		return Instance{}, err
	}
	i.Task, i.PID, i.PtyID, i.HarnessSessionID = task.String, int(pid.Int64), ptyID.String, sessionID.String
	i.StartedAt, err = parseTime(started)
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
