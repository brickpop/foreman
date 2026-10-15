package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"
)

// Message states (wiki/product/architecture/protocol.md#messages).
const (
	MessageQueued    = "queued"
	MessageDelivered = "delivered"
	MessageExpired   = "expired"
	MessageFailed    = "failed"
)

// Messages is the team_messages table.
type Messages interface {
	// Add stores a new message, queued, giving it the deployment's next msg-N.
	Add(ctx context.Context, m TeamMessage) (TeamMessage, error)
	// Get returns a deployment's message by its msg-N.
	Get(ctx context.Context, deploymentID, ref string) (TeamMessage, error)
	// ReplyTo returns the reply to an ask, if there is one.
	ReplyTo(ctx context.Context, deploymentID, ask string) (TeamMessage, error)
	// List returns a deployment's messages, oldest first.
	List(ctx context.Context, deploymentID string) ([]TeamMessage, error)
	// Queued returns what waits for an instance, oldest first.
	Queued(ctx context.Context, to string) ([]TeamMessage, error)
	// SetState moves messages to a state; delivered also stamps the time.
	SetState(ctx context.Context, ids []int64, state, reason string) error
	// Expire ends what still waits in a deployment; it returns those messages.
	Expire(ctx context.Context, deploymentID, reason string) ([]TeamMessage, error)
}

// Messages returns the team_messages table.
func (db *DB) Messages() Messages { return messages{db.sql} }

type messages struct{ db *sql.DB }

const messageColumns = "id, deployment_id, ref, from_instance, to_instance, kind, in_reply_to, body, artifact_ref, state, reason, created_at, delivered_at"

func (s messages) Add(ctx context.Context, m TeamMessage) (TeamMessage, error) {
	tx, err := s.db.BeginTx(ctx, nil) // immediate: the numbering can't race
	if err != nil {
		return TeamMessage{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var last int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(CAST(substr(ref, 5) AS INTEGER)), 0) FROM team_messages WHERE deployment_id = ?", m.DeploymentID).Scan(&last); err != nil {
		return TeamMessage{}, err
	}
	m.Ref, m.State, m.Reason, m.CreatedAt, m.DeliveredAt = "msg-"+strconv.Itoa(last+1), MessageQueued, "", time.Now().UTC(), time.Time{}
	res, err := tx.ExecContext(ctx, "INSERT INTO team_messages (deployment_id, ref, from_instance, to_instance, kind, in_reply_to, body, artifact_ref, state, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.DeploymentID, m.Ref, m.FromInstance, m.ToInstance, m.Kind, nullText(m.InReplyTo), m.Body, nullText(m.ArtifactRef), m.State, timeText(m.CreatedAt))
	if err != nil {
		return TeamMessage{}, err
	}
	if m.ID, err = res.LastInsertId(); err != nil {
		return TeamMessage{}, err
	}
	return m, tx.Commit()
}

func (s messages) Get(ctx context.Context, deploymentID, ref string) (TeamMessage, error) {
	return scanMessage(s.db.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM team_messages WHERE deployment_id = ? AND ref = ?", deploymentID, ref))
}

func (s messages) ReplyTo(ctx context.Context, deploymentID, ask string) (TeamMessage, error) {
	return scanMessage(s.db.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM team_messages WHERE deployment_id = ? AND kind = 'reply' AND in_reply_to = ?", deploymentID, ask))
}

func (s messages) List(ctx context.Context, deploymentID string) ([]TeamMessage, error) {
	return s.query(ctx, "SELECT "+messageColumns+" FROM team_messages WHERE deployment_id = ? ORDER BY id", deploymentID)
}

func (s messages) Queued(ctx context.Context, to string) ([]TeamMessage, error) {
	return s.query(ctx, "SELECT "+messageColumns+" FROM team_messages WHERE to_instance = ? AND state = 'queued' ORDER BY id", to)
}

func (s messages) SetState(ctx context.Context, ids []int64, state, reason string) error {
	var delivered any
	if state == MessageDelivered {
		delivered = timeText(time.Now())
	}
	for _, id := range ids {
		if _, err := s.db.ExecContext(ctx, "UPDATE team_messages SET state = ?, reason = ?, delivered_at = COALESCE(?, delivered_at) WHERE id = ?",
			state, nullText(reason), delivered, id); err != nil {
			return err
		}
	}
	return nil
}

func (s messages) Expire(ctx context.Context, deploymentID, reason string) ([]TeamMessage, error) {
	expired, err := s.query(ctx, "UPDATE team_messages SET state = 'expired', reason = ? WHERE deployment_id = ? AND state = 'queued' RETURNING "+messageColumns, reason, deploymentID)
	return expired, err
}

func (s messages) query(ctx context.Context, q string, args ...any) ([]TeamMessage, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []TeamMessage{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func scanMessage(row interface{ Scan(...any) error }) (TeamMessage, error) {
	var m TeamMessage
	var inReplyTo, artifact, reason, created, delivered sql.NullString
	err := row.Scan(&m.ID, &m.DeploymentID, &m.Ref, &m.FromInstance, &m.ToInstance, &m.Kind, &inReplyTo, &m.Body, &artifact,
		&m.State, &reason, &created, &delivered)
	if errors.Is(err, sql.ErrNoRows) {
		return TeamMessage{}, ErrNotFound
	}
	if err != nil {
		return TeamMessage{}, err
	}
	m.InReplyTo, m.ArtifactRef, m.Reason = inReplyTo.String, artifact.String, reason.String
	if m.CreatedAt, err = parseTime(created); err != nil {
		return TeamMessage{}, err
	}
	m.DeliveredAt, err = parseTime(delivered)
	return m, err
}
