package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Merge request states (wiki/product/models/merge-request.md).
const (
	MergeOpen     = "open"
	MergeMerged   = "merged"
	MergeRejected = "rejected"
)

// MergeRequests is the merge_requests table.
type MergeRequests interface {
	// Save inserts a merge request without an id (giving it the next mr-N
	// and its creation time), or replaces the one with its id.
	Save(ctx context.Context, m MergeRequest) (MergeRequest, error)
	// Get returns one by id: ErrNotFound if none.
	Get(ctx context.Context, id string) (MergeRequest, error)
	// OpenFor returns the open merge request of a project's repo and branch:
	// ErrNotFound if none.
	OpenFor(ctx context.Context, projectID, repo, branch string) (MergeRequest, error)
	// ForProject lists a project's merge requests, oldest first; Open lists
	// every open one.
	ForProject(ctx context.Context, projectID string) ([]MergeRequest, error)
	Open(ctx context.Context) ([]MergeRequest, error)
}

// MergeRequests returns the merge_requests table.
func (db *DB) MergeRequests() MergeRequests { return mergeRequests{db.sql} }

type mergeRequests struct{ db *sql.DB }

const mergeRequestColumns = "seq, project_id, deployment_id, author_instance, repo, branch, entry, state, checks_status, conflicting, forge_pr_url, forge_pr_number, note, created_at, merged_at, rejected_at"

func (s mergeRequests) Save(ctx context.Context, m MergeRequest) (MergeRequest, error) {
	if m.State == "" {
		m.State = MergeOpen
	}
	if m.ID == "" {
		m.CreatedAt = time.Now().UTC()
	}
	values := []any{nullText(m.ProjectID), nullText(m.DeploymentID), m.AuthorInstance, m.Repo, m.Branch, nullText(m.Entry), m.State,
		nullText(m.ChecksStatus), m.Conflicting, nullText(m.ForgePRURL), m.ForgePRNumber, nullText(m.Note),
		timeText(m.CreatedAt), timeText(m.MergedAt), timeText(m.RejectedAt)}
	if m.ID == "" {
		res, err := s.db.ExecContext(ctx, "INSERT INTO merge_requests (project_id, deployment_id, author_instance, repo, branch, entry, state, checks_status, conflicting, forge_pr_url, forge_pr_number, note, created_at, merged_at, rejected_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", values...)
		if err != nil {
			return m, err
		}
		seq, err := res.LastInsertId()
		m.ID = "mr-" + strconv.FormatInt(seq, 10)
		return m, err
	}
	seq, err := mergeRequestSeq(m.ID)
	if err != nil {
		return m, err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE merge_requests SET project_id = ?, deployment_id = ?, author_instance = ?, repo = ?, branch = ?, entry = ?, state = ?, checks_status = ?, conflicting = ?, forge_pr_url = ?, forge_pr_number = ?, note = ?, created_at = ?, merged_at = ?, rejected_at = ? WHERE seq = ?", append(values, seq)...)
	if err != nil {
		return m, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return m, ErrNotFound
	}
	return m, nil
}

func (s mergeRequests) Get(ctx context.Context, id string) (MergeRequest, error) {
	seq, err := mergeRequestSeq(id)
	if err != nil {
		return MergeRequest{}, err
	}
	return one(s.query(ctx, "WHERE seq = ?", seq))
}

func (s mergeRequests) OpenFor(ctx context.Context, projectID, repo, branch string) (MergeRequest, error) {
	return one(s.query(ctx, "WHERE project_id = ? AND repo = ? AND branch = ? AND state = ?", projectID, repo, branch, MergeOpen))
}

func (s mergeRequests) ForProject(ctx context.Context, projectID string) ([]MergeRequest, error) {
	return s.query(ctx, "WHERE project_id = ?", projectID)
}

func (s mergeRequests) Open(ctx context.Context) ([]MergeRequest, error) {
	return s.query(ctx, "WHERE state = ?", MergeOpen)
}

func one(list []MergeRequest, err error) (MergeRequest, error) {
	if err != nil {
		return MergeRequest{}, err
	}
	if len(list) == 0 {
		return MergeRequest{}, ErrNotFound
	}
	return list[0], nil
}

func (s mergeRequests) query(ctx context.Context, where string, args ...any) ([]MergeRequest, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+mergeRequestColumns+" FROM merge_requests "+where+" ORDER BY seq", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []MergeRequest{}
	for rows.Next() {
		var m MergeRequest
		var seq int64
		var project, deployment, entry, checks, prURL, note, created, merged, rejected sql.NullString
		if err := rows.Scan(&seq, &project, &deployment, &m.AuthorInstance, &m.Repo, &m.Branch, &entry, &m.State, &checks,
			&m.Conflicting, &prURL, &m.ForgePRNumber, &note, &created, &merged, &rejected); err != nil {
			return nil, err
		}
		m.ID = "mr-" + strconv.FormatInt(seq, 10)
		m.ProjectID, m.DeploymentID, m.Entry, m.ChecksStatus, m.ForgePRURL, m.Note = project.String, deployment.String, entry.String, checks.String, prURL.String, note.String
		if m.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		if m.MergedAt, err = parseTime(merged); err != nil {
			return nil, err
		}
		if m.RejectedAt, err = parseTime(rejected); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func mergeRequestSeq(id string) (int64, error) {
	seq, err := strconv.ParseInt(strings.TrimPrefix(id, "mr-"), 10, 64)
	if err != nil || !strings.HasPrefix(id, "mr-") {
		return 0, errors.Join(ErrNotFound, fmt.Errorf("no merge request %q", id))
	}
	return seq, nil
}
