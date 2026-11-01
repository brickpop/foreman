package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
)

// A project's Activity section (wiki/product/features/project-activity.md): the
// team traffic of a deployment, messages and git actions together,
// newest first, a page at a time.

// Activity reads team traffic across the messages and actions tables.
type Activity interface {
	// Page returns up to limit items of the deployment, newest first,
	// older than the cursor before ("": the newest). Its Next is the cursor
	// of the following page, "" when there is none.
	Page(ctx context.Context, deployment, before string, limit int) (ActivityPage, error)
}

// ErrBadCursor means a page's cursor isn't one Page gave.
var ErrBadCursor = errors.New("store: bad cursor")

// Activity returns the team traffic reader.
func (db *DB) Activity() Activity { return activity{db.sql} }

type activity struct{ db *sql.DB }

// position is where an item sits in the timeline: its time (julianday, since
// the stored text doesn't sort as text below a second), then its table, then
// its id, so no two items tie.
type position struct {
	at    float64
	table string // "a" (actions) or "m" (messages)
	id    int64
}

func (p position) cursor() string {
	return strconv.FormatFloat(p.at, 'g', -1, 64) + ":" + p.table + ":" + strconv.FormatInt(p.id, 10)
}

func parseCursor(s string) (position, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 || (parts[1] != "a" && parts[1] != "m") {
		return position{}, ErrBadCursor
	}
	at, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return position{}, ErrBadCursor
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return position{}, ErrBadCursor
	}
	return position{at, parts[1], id}, nil
}

func (s activity) Page(ctx context.Context, deployment, before string, limit int) (ActivityPage, error) {
	page := ActivityPage{Items: []ActivityItem{}}
	if limit <= 0 {
		return page, nil
	}
	args := []any{deployment, deployment}
	q := `SELECT at, tbl, id FROM (
		SELECT julianday(created_at) AS at, 'a' AS tbl, id FROM team_actions WHERE deployment_id = ?
		UNION ALL
		SELECT julianday(created_at), 'm', id FROM team_messages WHERE deployment_id = ?)`
	if before != "" {
		p, err := parseCursor(before)
		if err != nil {
			return ActivityPage{}, err
		}
		q += ` WHERE (at, tbl, id) < (?, ?, ?)`
		args = append(args, p.at, p.table, p.id)
	}
	// One more than asked: whether there is a next page.
	q += ` ORDER BY at DESC, tbl DESC, id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return ActivityPage{}, err
	}
	var order []position
	for rows.Next() {
		var p position
		if err := rows.Scan(&p.at, &p.table, &p.id); err != nil {
			rows.Close()
			return ActivityPage{}, err
		}
		order = append(order, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return ActivityPage{}, err
	}
	if len(order) > limit {
		order = order[:limit]
		page.Next = order[limit-1].cursor()
	}
	var msgIDs, actIDs []int64
	for _, p := range order {
		if p.table == "m" {
			msgIDs = append(msgIDs, p.id)
		} else {
			actIDs = append(actIDs, p.id)
		}
	}
	msgs, acts := map[int64]*TeamMessage{}, map[int64]*TeamAction{}
	if err := s.load(ctx, "SELECT "+messageColumns+" FROM team_messages", msgIDs, func(row *sql.Rows) error {
		m, err := scanMessage(row)
		msgs[m.ID] = &m
		return err
	}); err != nil {
		return ActivityPage{}, err
	}
	if err := s.load(ctx, "SELECT "+actionColumns+" FROM team_actions", actIDs, func(row *sql.Rows) error {
		a, err := scanAction(row)
		acts[a.ID] = &a
		return err
	}); err != nil {
		return ActivityPage{}, err
	}
	for _, p := range order {
		if p.table == "m" {
			page.Items = append(page.Items, ActivityItem{Message: msgs[p.id]})
		} else {
			page.Items = append(page.Items, ActivityItem{Action: acts[p.id]})
		}
	}
	return page, nil
}

// load reads the rows of ids from selectFrom, handing each to scan.
func (s activity) load(ctx context.Context, selectFrom string, ids []int64, scan func(*sql.Rows) error) error {
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, selectFrom+" WHERE id IN ("+strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")+")", args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
