package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrExists means a project, or a repo of the project, already has that
// name (in any case) or that path; or a repo, that target.
var ErrExists = errors.New("store: already exists")

// ErrInUse means a target can't go: a merge request into it is open.
var ErrInUse = errors.New("store: in use")

// Projects is the projects and their repos (wiki/product/models/project-config.md#the-project-record):
// what makes a project, on this machine. Its rules are in its project repo.
type Projects interface {
	// List returns every project by id, each with its repos by name.
	List(ctx context.Context) ([]ProjectRecord, error)
	// Create adds a project with no repos, in its workspace. ErrExists if the
	// id is taken.
	Create(ctx context.Context, id, workspace string) error
	// SetWorkspace moves the project to a workspace (a migration).
	SetWorkspace(ctx context.Context, id, workspace string) error
	// SetProfile names the profile its agents run on ("" for the default).
	SetProfile(ctx context.Context, id, profile string) error
	// SetArchived archives a project, or unarchives it.
	SetArchived(ctx context.Context, id string, archived bool) error
	// AddRepo adds a repo to the project, with r.Default as its default
	// target. ErrExists if the project has that name or that path already.
	AddRepo(ctx context.Context, project string, r RepoRecord) error
	// MoveRepo points a repo at a new path (Locate). ErrExists if another
	// repo of the project has that path.
	MoveRepo(ctx context.Context, project, name, path string) error
	// SetRepoDefault makes t the repo's default target, recording it when the
	// repo has no target of that name; the old default stays a target, and
	// merge requests keep theirs. ErrNotFound if there is no such repo.
	SetRepoDefault(ctx context.Context, project, name string, t TargetRecord) error
	// AddTarget records another target of a repo. ErrExists if the repo has
	// one of that name, ErrNotFound if there is no such repo.
	AddTarget(ctx context.Context, project, repo string, t TargetRecord) error
	// RemoveTarget forgets one of a repo's targets but its default:
	// ErrNotFound if it has no other of that name, ErrInUse while a merge
	// request into it is open.
	RemoveTarget(ctx context.Context, project, repo, name string) error
	// RemoveRepo takes a repo out of the project, with its targets.
	RemoveRepo(ctx context.Context, project, name string) error
	// RenameRepo renames a repo, with its targets, and moves the project's
	// open merge requests on it to the new name; closed ones keep the old name. ErrExists if the
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
	Archived  bool
	Workspace string // "": in Foreman's home, made before workspaces
	CreatedAt time.Time
	Repos     []RepoRecord
}

// RepoRecord is one of a project's repos: a name unique in the project, its
// folder, its first commit, which only that repo holds, and its targets, the
// default first, then oldest first. Default names the default one (read from
// Targets; on AddRepo, the branch recorded as it).
type RepoRecord struct {
	Name       string
	Path       string
	RootCommit string
	Default    string
	Targets    []TargetRecord
}

// TargetRecord is a branch of a repo that merges may go to: whether it is
// the default, who made it ("": the CEO) at which commit ("" for one set by
// the CEO) and when.
type TargetRecord struct {
	Name      string
	Default   bool
	MadeBy    string
	From      string
	CreatedAt time.Time
}

// Projects returns the projects table.
func (db *DB) Projects() Projects { return projects{db.sql} }

type projects struct{ db *sql.DB }

func (p projects) List(ctx context.Context) ([]ProjectRecord, error) {
	rows, err := p.db.QueryContext(ctx, "SELECT id, profile, archived, workspace, created_at FROM projects ORDER BY id")
	if err != nil {
		return nil, err
	}
	var list []ProjectRecord
	byID := map[string]int{}
	for rows.Next() {
		var r ProjectRecord
		var created sql.NullString
		if err := rows.Scan(&r.ID, &r.Profile, &r.Archived, &r.Workspace, &created); err != nil {
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
	targets, err := p.targets(ctx)
	if err != nil {
		return nil, err
	}
	rows, err = p.db.QueryContext(ctx, "SELECT project_id, name, path, root_commit FROM project_repos ORDER BY project_id, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var project string
		var r RepoRecord
		if err := rows.Scan(&project, &r.Name, &r.Path, &r.RootCommit); err != nil {
			return nil, err
		}
		r.Targets = targets[targetKey(project, r.Name)]
		if len(r.Targets) > 0 && r.Targets[0].Default {
			r.Default = r.Targets[0].Name
		}
		if i, ok := byID[strings.ToLower(project)]; ok {
			list[i].Repos = append(list[i].Repos, r)
		}
	}
	return list, rows.Err()
}

func (p projects) Create(ctx context.Context, id, workspace string) error {
	_, err := p.db.ExecContext(ctx, "INSERT INTO projects (id, workspace, created_at) VALUES (?, ?, ?)", id, workspace, timeText(time.Now()))
	return unique(err)
}

func (p projects) SetProfile(ctx context.Context, id, profile string) error {
	return touched(p.db.ExecContext(ctx, "UPDATE projects SET profile = ? WHERE id = ?", profile, id))
}

func (p projects) SetWorkspace(ctx context.Context, id, workspace string) error {
	return touched(p.db.ExecContext(ctx, "UPDATE projects SET workspace = ? WHERE id = ?", workspace, id))
}

func (p projects) SetArchived(ctx context.Context, id string, archived bool) error {
	return touched(p.db.ExecContext(ctx, "UPDATE projects SET archived = ? WHERE id = ?", archived, id))
}

func (p projects) AddRepo(ctx context.Context, project string, r RepoRecord) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// No row inserted: no such project.
	res, err := tx.ExecContext(ctx, "INSERT INTO project_repos (project_id, name, path, root_commit) SELECT id, ?, ?, ? FROM projects WHERE id = ?",
		r.Name, r.Path, r.RootCommit, project)
	if err != nil {
		return unique(err)
	}
	if err := touched(res, nil); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO repo_targets (project_id, repo, name, made_by, from_commit, created_at, is_default) VALUES (?, ?, ?, '', '', ?, 1)",
		project, r.Name, r.Default, timeText(time.Now())); err != nil {
		return err
	}
	return tx.Commit()
}

func (p projects) MoveRepo(ctx context.Context, project, name, path string) error {
	res, err := p.db.ExecContext(ctx, "UPDATE project_repos SET path = ? WHERE project_id = ? AND name = ?", path, project, name)
	if err != nil {
		return unique(err)
	}
	return touched(res, nil)
}

func (p projects) SetRepoDefault(ctx context.Context, project, name string, t TargetRecord) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := touched(tx.ExecContext(ctx, "UPDATE repo_targets SET is_default = 0 WHERE project_id = ? AND repo = ? AND is_default = 1", project, name)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO repo_targets (project_id, repo, name, made_by, from_commit, created_at, is_default) VALUES (?, ?, ?, ?, ?, ?, 1) ON CONFLICT (project_id, repo, name) DO UPDATE SET is_default = 1",
		project, name, t.Name, t.MadeBy, t.From, timeText(t.CreatedAt)); err != nil {
		return err
	}
	return tx.Commit()
}

func (p projects) AddTarget(ctx context.Context, project, repo string, t TargetRecord) error {
	// No row inserted: no such repo.
	res, err := p.db.ExecContext(ctx, "INSERT INTO repo_targets (project_id, repo, name, made_by, from_commit, created_at) SELECT project_id, name, ?, ?, ?, ? FROM project_repos WHERE project_id = ? AND name = ?",
		t.Name, t.MadeBy, t.From, timeText(t.CreatedAt), project, repo)
	if err != nil {
		return unique(err)
	}
	return touched(res, nil)
}

func (p projects) RemoveTarget(ctx context.Context, project, repo, name string) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var open int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM merge_requests WHERE project_id = ? AND repo = ? AND target = ? AND state = ?", project, repo, name, MergeOpen).Scan(&open); err != nil {
		return err
	}
	if open > 0 {
		return ErrInUse
	}
	if err := touched(tx.ExecContext(ctx, "DELETE FROM repo_targets WHERE project_id = ? AND repo = ? AND name = ? AND is_default = 0", project, repo, name)); err != nil {
		return err
	}
	return tx.Commit()
}

// targets are every repo's targets, by targetKey: the default first, then
// oldest first.
func (p projects) targets(ctx context.Context) (map[string][]TargetRecord, error) {
	rows, err := p.db.QueryContext(ctx, "SELECT project_id, repo, name, is_default, made_by, from_commit, created_at FROM repo_targets ORDER BY is_default DESC, created_at, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := map[string][]TargetRecord{}
	for rows.Next() {
		var project, repo string
		var t TargetRecord
		var created sql.NullString
		if err := rows.Scan(&project, &repo, &t.Name, &t.Default, &t.MadeBy, &t.From, &created); err != nil {
			return nil, err
		}
		if t.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		all[targetKey(project, repo)] = append(all[targetKey(project, repo)], t)
	}
	return all, rows.Err()
}

// targetKey names a project's repo as both match, in any case.
func targetKey(project, repo string) string { return strings.ToLower(project + "\x00" + repo) }

func (p projects) RemoveRepo(ctx context.Context, project, name string) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := touched(tx.ExecContext(ctx, "DELETE FROM project_repos WHERE project_id = ? AND name = ?", project, name)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM repo_targets WHERE project_id = ? AND repo = ?", project, name); err != nil {
		return err
	}
	return tx.Commit()
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
	if _, err := tx.ExecContext(ctx, "UPDATE repo_targets SET repo = ? WHERE project_id = ? AND repo = ?", newName, project, name); err != nil {
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
		"DELETE FROM repo_targets WHERE project_id = ?",
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
