// Package store is Foreman's runtime state: state.db, a SQLite database
// (wiki/product/architecture/persistence.md#statedb). Each block owns its
// tables and reaches them only through its own interface (Deployments,
// Instances, …), never through the database itself.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // the "sqlite" driver: pure Go, no C compiler
)

var (
	// ErrNotFound means no row has that id.
	ErrNotFound = errors.New("store: not found")
	// ErrTooNew means state.db was written by a newer Foreman.
	ErrTooNew = errors.New("store: state.db is newer than this Foreman")
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// DB is an open state.db.
type DB struct{ sql *sql.DB }

// Open opens (or creates) the database at path and brings its schema up to
// date.
func Open(ctx context.Context, path string) (*DB, error) {
	// WAL lets readers run during a write; the busy timeout makes a second
	// writer wait instead of failing at once; immediate transactions take the
	// write lock up front, so two read-then-write transactions can't deadlock.
	dsn := "file:" + uriPath.Replace(path) + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := migrate(ctx, db, migrationFiles); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &DB{sql: db}, nil
}

// uriPath escapes what a SQLite URI would read as its query or fragment.
var uriPath = strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23")

// Close closes the database.
func (db *DB) Close() error { return db.sql.Close() }

// migrate applies, in order and each in its own transaction, the migrations
// past the database's version (PRAGMA user_version). Migration files are
// named NNN_what.sql, numbered from 001 without gaps.
func migrate(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	names, err := fs.Glob(fsys, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > len(names) {
		return fmt.Errorf("%w (schema version %d, this one knows %d)", ErrTooNew, version, len(names))
	}
	for i := version; i < len(names); i++ {
		if want := fmt.Sprintf("migrations/%03d_", i+1); !strings.HasPrefix(names[i], want) {
			return fmt.Errorf("migration %s out of sequence: expected %s*", names[i], want)
		}
		body, err := fs.ReadFile(fsys, names[i])
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("%s: %w", names[i], err)
		}
		// PRAGMA takes no parameters; i+1 is a number we computed.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Timestamps are stored as UTC RFC 3339 text, readable with the sqlite3 shell.

func timeText(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s sql.NullString) (time.Time, error) {
	if !s.Valid {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s.String)
}

// unique turns a uniqueness violation into ErrExists.
func unique(err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return ErrExists
	}
	return err
}

// touched is ErrNotFound when a statement touched no row.
func touched(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
