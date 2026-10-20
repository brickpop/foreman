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

// ErrActive means the team is already deployed on the project.
var ErrActive = errors.New("store: already deployed")

// Deployments is the deployments table.
type Deployments interface {
	// Deploy creates the deployment of team on project, or reactivates the
	// stopped one with the new goal (one row per pair, keeping its history).
	// ErrActive if it is already active.
	Deploy(ctx context.Context, team, project, goal string) (Deployment, error)
	// Stop marks it stopped; stopping a stopped deployment changes nothing.
	Stop(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (Deployment, error)
	// List returns every deployment, active and stopped, by id.
	List(ctx context.Context) ([]Deployment, error)
}

// Deployments returns the deployments table.
func (db *DB) Deployments() Deployments { return deployments{db.sql} }

type deployments struct{ db *sql.DB }

const deploymentColumns = "id, team_id, project_id, goal, status, created_at, stopped_at"

func (d deployments) Deploy(ctx context.Context, team, project, goal string) (Deployment, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return Deployment{}, err
	}
	defer func() { _ = tx.Rollback() }()
	id := team + "@" + project
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
		_, err = tx.ExecContext(ctx, "UPDATE deployments SET goal = ?, status = 'active', stopped_at = NULL WHERE id = ?", goal, id)
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

func (d deployments) Stop(ctx context.Context, id string) error {
	return touched(d.db.ExecContext(ctx, "UPDATE deployments SET status = 'stopped', stopped_at = coalesce(stopped_at, ?) WHERE id = ?",
		timeText(time.Now()), id))
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
	err := row.Scan(&dep.ID, &dep.TeamID, &dep.ProjectID, &dep.Goal, &dep.Status, &created, &stopped)
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
