package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Request statuses (wiki/product/models/request.md).
const (
	RequestOpen     = "open"
	RequestResolved = "resolved"
)

// Requests is the requests table.
type Requests interface {
	// Save inserts a request without an id (giving it the next req-N and its
	// creation time), or replaces the one with its id.
	Save(ctx context.Context, r Request) (Request, error)
	// Open lists the open requests, oldest first.
	Open(ctx context.Context) ([]Request, error)
	// ForReview returns a review's open review request: ErrNotFound if none.
	ForReview(ctx context.Context, reviewID string) (Request, error)
}

// Requests returns the requests table.
func (db *DB) Requests() Requests { return requests{db.sql} }

type requests struct{ db *sql.DB }

const requestColumns = "seq, kind, project_id, deployment_id, instance_id, summary, options, source, status, answer, created_at, resolved_at, review_id, links"

func (s requests) Save(ctx context.Context, r Request) (Request, error) {
	if r.Status == "" {
		r.Status = RequestOpen
	}
	if r.Options == nil {
		r.Options = []string{}
	}
	if r.Links == nil {
		r.Links = []Link{}
	}
	if r.ID == "" {
		r.CreatedAt = time.Now().UTC()
	}
	options, err := json.Marshal(r.Options)
	if err != nil {
		return r, err
	}
	links, err := json.Marshal(r.Links)
	if err != nil {
		return r, err
	}
	values := []any{r.Kind, nullText(r.ProjectID), nullText(r.DeploymentID), nullText(r.InstanceID), r.Summary, string(options), r.Source,
		r.Status, nullText(r.Answer), timeText(r.CreatedAt), timeText(r.ResolvedAt), nullText(r.ReviewID), string(links)}
	if r.ID == "" {
		res, err := s.db.ExecContext(ctx, "INSERT INTO requests (kind, project_id, deployment_id, instance_id, summary, options, source, status, answer, created_at, resolved_at, review_id, links) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", values...)
		if err != nil {
			return r, err
		}
		seq, err := res.LastInsertId()
		r.ID = "req-" + strconv.FormatInt(seq, 10)
		return r, err
	}
	seq, err := strconv.ParseInt(strings.TrimPrefix(r.ID, "req-"), 10, 64)
	if err != nil || !strings.HasPrefix(r.ID, "req-") {
		return r, errors.Join(ErrNotFound, fmt.Errorf("no request %q", r.ID))
	}
	res, err := s.db.ExecContext(ctx, "UPDATE requests SET kind = ?, project_id = ?, deployment_id = ?, instance_id = ?, summary = ?, options = ?, source = ?, status = ?, answer = ?, created_at = ?, resolved_at = ?, review_id = ?, links = ? WHERE seq = ?", append(values, seq)...)
	return r, touched(res, err)
}

func (s requests) Open(ctx context.Context) ([]Request, error) {
	return s.query(ctx, "WHERE status = ?", RequestOpen)
}

func (s requests) ForReview(ctx context.Context, reviewID string) (Request, error) {
	list, err := s.query(ctx, "WHERE status = ? AND kind = 'review' AND review_id = ?", RequestOpen, reviewID)
	if err != nil {
		return Request{}, err
	}
	if len(list) == 0 {
		return Request{}, ErrNotFound
	}
	return list[0], nil
}

func (s requests) query(ctx context.Context, where string, args ...any) ([]Request, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+requestColumns+" FROM requests "+where+" ORDER BY seq", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Request{}
	for rows.Next() {
		var r Request
		var seq int64
		var project, deployment, instance, answer, created, resolved, review sql.NullString
		var options, links string
		if err := rows.Scan(&seq, &r.Kind, &project, &deployment, &instance, &r.Summary, &options, &r.Source, &r.Status, &answer, &created, &resolved, &review, &links); err != nil {
			return nil, err
		}
		r.ID = "req-" + strconv.FormatInt(seq, 10)
		r.ProjectID, r.DeploymentID, r.InstanceID, r.Answer, r.ReviewID = project.String, deployment.String, instance.String, answer.String, review.String
		if err := json.Unmarshal([]byte(options), &r.Options); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(links), &r.Links); err != nil {
			return nil, err
		}
		if r.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		if r.ResolvedAt, err = parseTime(resolved); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}
