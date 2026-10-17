package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Work item stages (wiki/product/features/pipeline-and-merges.md#work-item-stages).
const (
	StageInWorktree = iota
	StageHanded
	StageReviewing
	StageIntegrated
)

// WorkItems is the work_items table.
type WorkItems interface {
	// Save inserts a work item without an id (giving it the next wi-N), or
	// replaces the one with its id.
	Save(ctx context.Context, w WorkItem) (WorkItem, error)
	// Find returns a deployment's work item for a branch: ErrNotFound if none.
	Find(ctx context.Context, deploymentID, branch string) (WorkItem, error)
	// ForDeployment and ForProject list work items, oldest first.
	ForDeployment(ctx context.Context, deploymentID string) ([]WorkItem, error)
	ForProject(ctx context.Context, projectID string) ([]WorkItem, error)
}

// WorkItems returns the work_items table.
func (db *DB) WorkItems() WorkItems { return workItems{db.sql} }

type workItems struct{ db *sql.DB }

const workItemColumns = "w.seq, w.deployment_id, w.author_instance, w.branch, w.card_ref, w.stage, w.note, w.checks_status, w.merge_request_id"

func (s workItems) Save(ctx context.Context, w WorkItem) (WorkItem, error) {
	values := []any{w.DeploymentID, w.AuthorInstance, w.Branch, nullText(w.CardRef), w.Stage, nullText(w.Note), nullText(w.ChecksStatus), nullText(w.MergeRequestID)}
	if w.ID == "" {
		res, err := s.db.ExecContext(ctx, "INSERT INTO work_items (deployment_id, author_instance, branch, card_ref, stage, note, checks_status, merge_request_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", values...)
		if err != nil {
			return w, err
		}
		seq, err := res.LastInsertId()
		w.ID = "wi-" + strconv.FormatInt(seq, 10)
		return w, err
	}
	seq, err := workItemSeq(w.ID)
	if err != nil {
		return w, err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE work_items SET deployment_id = ?, author_instance = ?, branch = ?, card_ref = ?, stage = ?, note = ?, checks_status = ?, merge_request_id = ? WHERE seq = ?", append(values, seq)...)
	if err != nil {
		return w, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return w, ErrNotFound
	}
	return w, nil
}

func (s workItems) Find(ctx context.Context, deploymentID, branch string) (WorkItem, error) {
	list, err := s.query(ctx, "WHERE w.deployment_id = ? AND w.branch = ?", deploymentID, branch)
	if err != nil {
		return WorkItem{}, err
	}
	if len(list) == 0 {
		return WorkItem{}, ErrNotFound
	}
	return list[0], nil
}

func (s workItems) ForDeployment(ctx context.Context, deploymentID string) ([]WorkItem, error) {
	return s.query(ctx, "WHERE w.deployment_id = ?", deploymentID)
}

func (s workItems) ForProject(ctx context.Context, projectID string) ([]WorkItem, error) {
	return s.query(ctx, "JOIN deployments d ON d.id = w.deployment_id WHERE d.project_id = ?", projectID)
}

func (s workItems) query(ctx context.Context, where string, args ...any) ([]WorkItem, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+workItemColumns+" FROM work_items w "+where+" ORDER BY w.seq", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []WorkItem{}
	for rows.Next() {
		var w WorkItem
		var seq int64
		var card, note, checks, mr sql.NullString
		if err := rows.Scan(&seq, &w.DeploymentID, &w.AuthorInstance, &w.Branch, &card, &w.Stage, &note, &checks, &mr); err != nil {
			return nil, err
		}
		w.ID = "wi-" + strconv.FormatInt(seq, 10)
		w.CardRef, w.Note, w.ChecksStatus, w.MergeRequestID = card.String, note.String, checks.String, mr.String
		list = append(list, w)
	}
	return list, rows.Err()
}

func workItemSeq(id string) (int64, error) {
	seq, err := strconv.ParseInt(strings.TrimPrefix(id, "wi-"), 10, 64)
	if err != nil || !strings.HasPrefix(id, "wi-") {
		return 0, errors.Join(ErrNotFound, fmt.Errorf("no work item %q", id))
	}
	return seq, nil
}
