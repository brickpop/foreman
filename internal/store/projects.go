package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrExists means a project, or a repo of the project, already has that
// name (in any case) or that path.
var ErrExists = errors.New("store: already exists")

// Projects is this machine's part of each project
// (wiki/product/models/project-config.md#the-project-record): its profile,
// each repo's path, and its history. The rest of its record is in
// projects.toml, in Foreman's HQ, which travels; its rules are in its HQ.
type Projects interface {
	// List returns every project this machine has a row for, by id, each
	// with its repos' paths by name.
	List(ctx context.Context) ([]ProjectRecord, error)
	// Create adds a project's row. ErrExists if the id is taken.
	Create(ctx context.Context, id string) error
	// SetProfile names the profile its agents run on ("" for the default),
	// making its row if this machine has none yet.
	SetProfile(ctx context.Context, id, profile string) error
	// SetRepoPath records where a repo of the project is on this machine
	// (added, or located), making the project's row if there is none yet.
	// ErrExists if another repo of the project has that path.
	SetRepoPath(ctx context.Context, project, name, path string) error
	// RemoveRepo forgets a repo's path.
	RemoveRepo(ctx context.Context, project, name string) error
	// RenameRepo renames a repo, and moves the project's
	// open merge requests on it to the new name; closed ones keep the old name. ErrExists if the
	// project has a repo of that name.
	RenameRepo(ctx context.Context, project, name, newName string) error
	// Remove deletes the project, its repos and its whole history: its
	// deployments and everything recorded for them (instances, messages,
	// actions, work items, merge requests, requests), so a project added
	// again under that name starts clean (ids are built from the name).
	Remove(ctx context.Context, id string) error
}

// ProjectRecord is a project as this machine keeps it.
type ProjectRecord struct {
	ID        string
	Profile   string
	CreatedAt time.Time
	Repos     []RepoRecord
}

// RepoRecord is where one of a project's repos is on this machine.
type RepoRecord struct {
	Name string
	Path string
}

// Projects returns the projects table.
func (db *DB) Projects() Projects { return projects{db.sql} }

type projects struct{ db *sql.DB }

func (p projects) List(ctx context.Context) ([]ProjectRecord, error) {
	rows, err := p.db.QueryContext(ctx, "SELECT id, profile, created_at FROM projects ORDER BY id")
	if err != nil {
		return nil, err
	}
	var list []ProjectRecord
	byID := map[string]int{}
	for rows.Next() {
		var r ProjectRecord
		var created sql.NullString
		if err := rows.Scan(&r.ID, &r.Profile, &created); err != nil {
			rows.Close()
			return nil, err
		}
		if r.CreatedAt, err = parseTime(created); err != nil {
			rows.Close()
			return nil, err
		}
		byID[strings.ToLower(r.ID)] = len(list)
		list = append(list, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = p.db.QueryContext(ctx, "SELECT project_id, name, path FROM project_repos ORDER BY project_id, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var project string
		var r RepoRecord
		if err := rows.Scan(&project, &r.Name, &r.Path); err != nil {
			return nil, err
		}
		if i, ok := byID[strings.ToLower(project)]; ok {
			list[i].Repos = append(list[i].Repos, r)
		}
	}
	return list, rows.Err()
}

func (p projects) Create(ctx context.Context, id string) error {
	_, err := p.db.ExecContext(ctx, "INSERT INTO projects (id, created_at) VALUES (?, ?)", id, timeText(time.Now()))
	return unique(err)
}

// ensure makes the project's row if this machine has none: a project from
// projects.toml, made on another machine.
func ensure(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO projects (id, created_at) VALUES (?, ?)", id, timeText(time.Now()))
	return err
}

func (p projects) SetProfile(ctx context.Context, id, profile string) error {
	return p.inTx(ctx, func(tx *sql.Tx) error {
		if err := ensure(ctx, tx, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE projects SET profile = ? WHERE id = ?", profile, id)
		return err
	})
}

func (p projects) SetRepoPath(ctx context.Context, project, name, path string) error {
	return p.inTx(ctx, func(tx *sql.Tx) error {
		if err := ensure(ctx, tx, project); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO project_repos (project_id, name, path) VALUES (?, ?, ?) ON CONFLICT (project_id, name) DO UPDATE SET path = excluded.path",
			project, name, path)
		return unique(err)
	})
}

// inTx runs do in a transaction, committed when it returns nil.
func (p projects) inTx(ctx context.Context, do func(*sql.Tx) error) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := do(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (p projects) RemoveRepo(ctx context.Context, project, name string) error {
	_, err := p.db.ExecContext(ctx, "DELETE FROM project_repos WHERE project_id = ? AND name = ?", project, name)
	return err
}

func (p projects) RenameRepo(ctx context.Context, project, name, newName string) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, "UPDATE project_repos SET name = ? WHERE project_id = ? AND name = ?", newName, project, name)
	if err != nil {
		return unique(err)
	}
	if err := touched(res, nil); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE merge_requests SET repo = ? WHERE project_id = ? AND repo = ? AND state = ?",
		newName, project, name, MergeOpen); err != nil {
		return err
	}
	return tx.Commit()
}

func (p projects) Remove(ctx context.Context, id string) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Children first: they reference the deployments.
	const ofProject = "deployment_id IN (SELECT id FROM deployments WHERE project_id = ?)"
	for _, q := range []string{
		"DELETE FROM instances WHERE " + ofProject,
		"DELETE FROM team_messages WHERE " + ofProject,
		"DELETE FROM team_actions WHERE " + ofProject,
		"DELETE FROM work_items WHERE " + ofProject,
		"DELETE FROM review_comments WHERE review_id IN (SELECT 'rev-' || seq FROM reviews WHERE project_id = ?1)",
		"DELETE FROM reviews WHERE project_id = ?1",
		"DELETE FROM merge_requests WHERE project_id = ?1 OR deployment_id IN (SELECT id FROM deployments WHERE project_id = ?1)",
		"DELETE FROM requests WHERE project_id = ?1 OR deployment_id IN (SELECT id FROM deployments WHERE project_id = ?1)",
		"DELETE FROM deployments WHERE project_id = ?",
		"DELETE FROM project_repos WHERE project_id = ?",
	} {
		if _, err := tx.ExecContext(ctx, q, id); err != nil {
			return err
		}
	}
	res, err := tx.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	if err := touched(res, err); err != nil {
		return err
	}
	return tx.Commit()
}
