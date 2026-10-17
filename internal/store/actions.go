package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Actions is the team_actions table.
type Actions interface {
	// Add stores what a git tool did.
	Add(ctx context.Context, a TeamAction) (TeamAction, error)
	// List returns a deployment's actions, oldest first.
	List(ctx context.Context, deploymentID string) ([]TeamAction, error)
}

// Actions returns the team_actions table.
func (db *DB) Actions() Actions { return actions{db.sql} }

type actions struct{ db *sql.DB }

func (s actions) Add(ctx context.Context, a TeamAction) (TeamAction, error) {
	if a.Repos == nil {
		a.Repos = []ActionRepo{}
	}
	repos, err := json.Marshal(a.Repos)
	if err != nil {
		return a, err
	}
	a.CreatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx, "INSERT INTO team_actions (deployment_id, instance, tool, branch, message, repos, error, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		a.DeploymentID, a.Instance, a.Tool, nullText(a.Branch), nullText(a.Message), string(repos), nullText(a.Error), timeText(a.CreatedAt))
	if err != nil {
		return a, err
	}
	a.ID, err = res.LastInsertId()
	return a, err
}

func (s actions) List(ctx context.Context, deploymentID string) ([]TeamAction, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+actionColumns+" FROM team_actions WHERE deployment_id = ? ORDER BY id", deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []TeamAction{}
	for rows.Next() {
		a, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

const actionColumns = "id, deployment_id, instance, tool, branch, message, repos, error, created_at"

func scanAction(row interface{ Scan(...any) error }) (TeamAction, error) {
	var a TeamAction
	var branch, message, errText, created sql.NullString
	var repos string
	if err := row.Scan(&a.ID, &a.DeploymentID, &a.Instance, &a.Tool, &branch, &message, &repos, &errText, &created); err != nil {
		return TeamAction{}, err
	}
	a.Branch, a.Message, a.Error = branch.String, message.String, errText.String
	if err := json.Unmarshal([]byte(repos), &a.Repos); err != nil {
		return TeamAction{}, err
	}
	var err error
	a.CreatedAt, err = parseTime(created)
	return a, err
}
