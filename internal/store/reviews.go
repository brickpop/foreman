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

// Review comment states (wiki/product/models/review.md).
const (
	CommentDraft = "draft"
	CommentSent  = "sent"
)

// Reviews is the reviews table.
type Reviews interface {
	// Save inserts a review without an id (giving it the next rev-N and its
	// creation time), or replaces the one with its id.
	Save(ctx context.Context, r Review) (Review, error)
	// Get returns one by id: ErrNotFound if none.
	Get(ctx context.Context, id string) (Review, error)
	// List lists every review, oldest first.
	List(ctx context.Context) ([]Review, error)
}

// ReviewComments is the review_comments table.
type ReviewComments interface {
	// Save inserts a comment without an id (giving it the next com-N and its
	// creation time), or replaces the one with its id.
	Save(ctx context.Context, c ReviewComment) (ReviewComment, error)
	// Get returns one by id: ErrNotFound if none.
	Get(ctx context.Context, id string) (ReviewComment, error)
	// ForReview lists a review's comments, oldest first.
	ForReview(ctx context.Context, reviewID string) ([]ReviewComment, error)
	// Delete removes one: ErrNotFound if none.
	Delete(ctx context.Context, id string) error
}

// Reviews returns the reviews table.
func (db *DB) Reviews() Reviews { return reviews{db.sql} }

// ReviewComments returns the review_comments table.
func (db *DB) ReviewComments() ReviewComments { return reviewComments{db.sql} }

type reviews struct{ db *sql.DB }

func (s reviews) Save(ctx context.Context, r Review) (Review, error) {
	if r.ID == "" {
		r.CreatedAt = time.Now().UTC()
		res, err := s.db.ExecContext(ctx, "INSERT INTO reviews (project_id, branch, created_at, changes_requested_at, changes_note, withdrawn_at, withdrawn_reason, superseded_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			r.ProjectID, r.Branch, timeText(r.CreatedAt), timeText(r.ChangesRequestedAt), r.ChangesNote, timeText(r.WithdrawnAt), r.WithdrawnReason, r.SupersededBy)
		if err != nil {
			return r, err
		}
		seq, err := res.LastInsertId()
		r.ID = "rev-" + strconv.FormatInt(seq, 10)
		return r, err
	}
	seq, err := seqOf(r.ID, "rev-", "review")
	if err != nil {
		return r, err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE reviews SET project_id = ?, branch = ?, created_at = ?, changes_requested_at = ?, changes_note = ?, withdrawn_at = ?, withdrawn_reason = ?, superseded_by = ? WHERE seq = ?",
		r.ProjectID, r.Branch, timeText(r.CreatedAt), timeText(r.ChangesRequestedAt), r.ChangesNote, timeText(r.WithdrawnAt), r.WithdrawnReason, r.SupersededBy, seq)
	return r, touched(res, err)
}

func (s reviews) Get(ctx context.Context, id string) (Review, error) {
	seq, err := seqOf(id, "rev-", "review")
	if err != nil {
		return Review{}, err
	}
	list, err := s.query(ctx, "WHERE seq = ?", seq)
	if err != nil {
		return Review{}, err
	}
	if len(list) == 0 {
		return Review{}, ErrNotFound
	}
	return list[0], nil
}

func (s reviews) List(ctx context.Context) ([]Review, error) { return s.query(ctx, "") }

func (s reviews) query(ctx context.Context, where string, args ...any) ([]Review, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT seq, project_id, branch, created_at, changes_requested_at, changes_note, withdrawn_at, withdrawn_reason, superseded_by FROM reviews "+where+" ORDER BY seq", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Review{}
	for rows.Next() {
		var r Review
		var seq int64
		var created, requested, withdrawn sql.NullString
		if err := rows.Scan(&seq, &r.ProjectID, &r.Branch, &created, &requested, &r.ChangesNote, &withdrawn, &r.WithdrawnReason, &r.SupersededBy); err != nil {
			return nil, err
		}
		r.ID = "rev-" + strconv.FormatInt(seq, 10)
		if r.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		if r.ChangesRequestedAt, err = parseTime(requested); err != nil {
			return nil, err
		}
		if r.WithdrawnAt, err = parseTime(withdrawn); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

type reviewComments struct{ db *sql.DB }

func (s reviewComments) Save(ctx context.Context, c ReviewComment) (ReviewComment, error) {
	if c.State == "" {
		c.State = CommentDraft
	}
	if c.ID == "" {
		c.CreatedAt = time.Now().UTC()
	}
	values := []any{c.ReviewID, c.Repo, c.Path, c.Side, c.Line, c.Commit, c.Snippet, c.Body, c.State, timeText(c.CreatedAt), timeText(c.SentAt)}
	if c.ID == "" {
		res, err := s.db.ExecContext(ctx, "INSERT INTO review_comments (review_id, repo, path, side, line, commit_sha, snippet, body, state, created_at, sent_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", values...)
		if err != nil {
			return c, err
		}
		seq, err := res.LastInsertId()
		c.ID = "com-" + strconv.FormatInt(seq, 10)
		return c, err
	}
	seq, err := seqOf(c.ID, "com-", "comment")
	if err != nil {
		return c, err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE review_comments SET review_id = ?, repo = ?, path = ?, side = ?, line = ?, commit_sha = ?, snippet = ?, body = ?, state = ?, created_at = ?, sent_at = ? WHERE seq = ?", append(values, seq)...)
	return c, touched(res, err)
}

func (s reviewComments) Get(ctx context.Context, id string) (ReviewComment, error) {
	seq, err := seqOf(id, "com-", "comment")
	if err != nil {
		return ReviewComment{}, err
	}
	list, err := s.query(ctx, "WHERE seq = ?", seq)
	if err != nil {
		return ReviewComment{}, err
	}
	if len(list) == 0 {
		return ReviewComment{}, ErrNotFound
	}
	return list[0], nil
}

func (s reviewComments) ForReview(ctx context.Context, reviewID string) ([]ReviewComment, error) {
	return s.query(ctx, "WHERE review_id = ?", reviewID)
}

func (s reviewComments) Delete(ctx context.Context, id string) error {
	seq, err := seqOf(id, "com-", "comment")
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, "DELETE FROM review_comments WHERE seq = ?", seq)
	return touched(res, err)
}

func (s reviewComments) query(ctx context.Context, where string, args ...any) ([]ReviewComment, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT seq, review_id, repo, path, side, line, commit_sha, snippet, body, state, created_at, sent_at FROM review_comments "+where+" ORDER BY seq", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []ReviewComment{}
	for rows.Next() {
		var c ReviewComment
		var seq int64
		var created, sent sql.NullString
		if err := rows.Scan(&seq, &c.ReviewID, &c.Repo, &c.Path, &c.Side, &c.Line, &c.Commit, &c.Snippet, &c.Body, &c.State, &created, &sent); err != nil {
			return nil, err
		}
		c.ID = "com-" + strconv.FormatInt(seq, 10)
		if c.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		if c.SentAt, err = parseTime(sent); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// seqOf is the number in an id such as rev-3; ErrNotFound for anything else.
func seqOf(id, prefix, what string) (int64, error) {
	seq, err := strconv.ParseInt(strings.TrimPrefix(id, prefix), 10, 64)
	if err != nil || !strings.HasPrefix(id, prefix) {
		return 0, errors.Join(ErrNotFound, fmt.Errorf("no %s %q", what, id))
	}
	return seq, nil
}
