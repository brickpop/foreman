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

// Projects is the projects and their repos (wiki/product/models/project-config.md#the-project-record):
// what makes a project, on this machine. Its rules are in its project repo.
type Projects interface {
	// List returns every project by id, each with its repos by name.
	List(ctx context.Context) ([]ProjectRecord, error)
	// Create adds a project with no repos. ErrExists if the id is taken.
	Create(ctx context.Context, id string) error
	// SetProfile names the profile its agents run on ("" for the default).
	SetProfile(ctx context.Context, id, profile string) error
	// AddRepo adds a repo to the project. ErrExists if the project has that
	// name or that path already.
	AddRepo(ctx context.Context, project string, r RepoRecord) error
	// MoveRepo points a repo at a new path (Locate). ErrExists if another
	// repo of the project has that path.
	MoveRepo(ctx context.Context, project, name, path string) error
	// SetRepoMain sets a repo's main branch.
	SetRepoMain(ctx context.Context, project, name, main string) error
	// RemoveRepo takes a repo out of the project.
	RemoveRepo(ctx context.Context, project, name string) error
	// RenameRepo renames a repo, and moves the project's open merge requests
	// on it to the new name; closed ones keep the old name. ErrExists if the
	// project has a repo of that name.
	RenameRepo(ctx context.Context, project, name, newName string) error
	// Remove deletes the project, its repos and its whole history: its
	// deployments and everything recorded for them (instances, messages,
	// actions, work items, merge requests, requests), so a project added
	// again under that name starts clean (ids are built from the name).
	Remove(ctx context.Context, id string) error
}

// ProjectRecord is a project as the database keeps it.
type ProjectRecord struct {
	ID        string
	Profile   string
	CreatedAt time.Time
	Repos     []RepoRecord
}

// RepoRecord is one of a project's repos: a name unique in the project, its
// folder, its first commit, which only that repo holds, and its main branch:
// the one agents start from and merges go to.
type RepoRecord struct {
	Name       string
	Path       string
	RootCommit string
	Main       string
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
	rows, err = p.db.QueryContext(ctx, "SELECT project_id, name, path, root_commit, main FROM project_repos ORDER BY project_id, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var project string
		var r RepoRecord
		if err := rows.Scan(&project, &r.Name, &r.Path, &r.RootCommit, &r.Main); err != nil {
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

func (p projects) SetProfile(ctx context.Context, id, profile string) error {
	return touched(p.db.ExecContext(ctx, "UPDATE projects SET profile = ? WHERE id = ?", profile, id))
}

func (p projects) AddRepo(ctx context.Context, project string, r RepoRecord) error {
	// No row inserted: no such project.
	res, err := p.db.ExecContext(ctx, "INSERT INTO project_repos (project_id, name, path, root_commit, main) SELECT id, ?, ?, ?, ? FROM projects WHERE id = ?",
		r.Name, r.Path, r.RootCommit, r.Main, project)
	if err != nil {
		return unique(err)
	}
	return touched(res, nil)
}

func (p projects) MoveRepo(ctx context.Context, project, name, path string) error {
	res, err := p.db.ExecContext(ctx, "UPDATE project_repos SET path = ? WHERE project_id = ? AND name = ?", path, project, name)
	if err != nil {
		return unique(err)
	}
	return touched(res, nil)
}

func (p projects) SetRepoMain(ctx context.Context, project, name, main string) error {
	return touched(p.db.ExecContext(ctx, "UPDATE project_repos SET main = ? WHERE project_id = ? AND name = ?", main, project, name))
}

func (p projects) RemoveRepo(ctx context.Context, project, name string) error {
	return touched(p.db.ExecContext(ctx, "DELETE FROM project_repos WHERE project_id = ? AND name = ?", project, name))
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
