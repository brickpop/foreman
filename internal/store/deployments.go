package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const (
	DeploymentActive  = "active"
	DeploymentStopped = "stopped"
)

var (
	// ErrActive means the team is already deployed on the project.
	ErrActive = errors.New("store: already deployed")
	// ErrProjectTaken means another team is deployed on the project: a project
	// has one team at most.
	ErrProjectTaken = errors.New("store: another team is deployed on the project")
)

// Deployments is the deployments table.
type Deployments interface {
	// Deploy creates the deployment of team on project, or reactivates the
	// stopped one with the new goal (one row per pair, keeping its history).
	// ErrActive if it is already active, ErrProjectTaken if another team is.
	Deploy(ctx context.Context, team, project, goal string) (Deployment, error)
	// Stop marks it stopped, with why when Foreman stopped it on its own (""
	// for the CEO's Stop); stopping a stopped deployment changes nothing.
	Stop(ctx context.Context, id, reason string) error
	Get(ctx context.Context, id string) (Deployment, error)
	// List returns every deployment, active and stopped, by id.
	List(ctx context.Context) ([]Deployment, error)
}

// Deployments returns the deployments table.
func (db *DB) Deployments() Deployments { return deployments{db.sql} }

type deployments struct{ db *sql.DB }

const deploymentColumns = "id, team_id, project_id, goal, status, created_at, stopped_at, stop_reason"

func (d deployments) Deploy(ctx context.Context, team, project, goal string) (Deployment, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return Deployment{}, err
	}
	defer func() { _ = tx.Rollback() }()
	id := team + "@" + project
	var other string
	err = tx.QueryRowContext(ctx, "SELECT id FROM deployments WHERE project_id = ? AND status = 'active' AND team_id != ?", project, team).Scan(&other)
	switch {
	case err == nil:
		return Deployment{}, ErrProjectTaken
	case !errors.Is(err, sql.ErrNoRows):
		return Deployment{}, err
	}
	var status string
	err = tx.QueryRowContext(ctx, "SELECT status FROM deployments WHERE team_id = ? AND project_id = ?", team, project).Scan(&status)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, "INSERT INTO deployments (id, team_id, project_id, goal, created_at) VALUES (?, ?, ?, ?, ?)",
			id, team, project, goal, timeText(time.Now()))
	case err != nil:
	case status == DeploymentActive:
		return Deployment{}, ErrActive
	default:
		_, err = tx.ExecContext(ctx, "UPDATE deployments SET goal = ?, status = 'active', stopped_at = NULL, stop_reason = '' WHERE id = ?", goal, id)
	}
	if err != nil {
		return Deployment{}, err
	}
	dep, err := scanDeployment(tx.QueryRowContext(ctx, "SELECT "+deploymentColumns+" FROM deployments WHERE id = ?", id))
	if err != nil {
		return Deployment{}, err
	}
	return dep, tx.Commit()
}

func (d deployments) Stop(ctx context.Context, id, reason string) error {
	// A stopped one keeps when and why it stopped.
	return touched(d.db.ExecContext(ctx, "UPDATE deployments SET stop_reason = CASE WHEN status = 'active' THEN ? ELSE stop_reason END, status = 'stopped', stopped_at = coalesce(stopped_at, ?) WHERE id = ?",
		reason, timeText(time.Now()), id))
}

func (d deployments) Get(ctx context.Context, id string) (Deployment, error) {
	return scanDeployment(d.db.QueryRowContext(ctx, "SELECT "+deploymentColumns+" FROM deployments WHERE id = ?", id))
}

func (d deployments) List(ctx context.Context) ([]Deployment, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT "+deploymentColumns+" FROM deployments ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Deployment
	for rows.Next() {
		dep, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, dep)
	}
	return list, rows.Err()
}

func scanDeployment(row interface{ Scan(...any) error }) (Deployment, error) {
	var dep Deployment
	var created, stopped sql.NullString
	err := row.Scan(&dep.ID, &dep.TeamID, &dep.ProjectID, &dep.Goal, &dep.Status, &created, &stopped, &dep.StopReason)
	if errors.Is(err, sql.ErrNoRows) {
		return Deployment{}, ErrNotFound
	}
	if err != nil {
		return Deployment{}, err
	}
	if dep.CreatedAt, err = parseTime(created); err != nil {
		return Deployment{}, err
	}
	dep.StoppedAt, err = parseTime(stopped)
	return dep, err
}
