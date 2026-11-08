package wiki

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/agentic-wiki/wiki/index"
	"github.com/agentic-wiki/wiki/parse"

	"github.com/brickpop/foreman/internal/home"
)

// ErrConflict means the entry changed since the version the write was based on.
var ErrConflict = errors.New("wiki: the file changed since you read it")

// ErrInvalid means a write that can't apply: no checkbox on that line, a move
// to no status, a lane on a board without lanes.
var ErrInvalid = errors.New("wiki: invalid write")

// ErrRefused means the wiki library refused the write: a frontmatter it can't
// edit safely, or that wouldn't read back the same. The error says why.
var ErrRefused = errors.New("wiki: write refused")

// Git commits Foreman's writes in the project repo (implemented by git.CLI).
// Paths are relative to dir, which is inside the repo.
type Git interface {
	// Commit commits exactly these files, by the CEO's git identity.
	Commit(ctx context.Context, dir, message string, files ...string) error
	// Changed reports whether file differs from what is committed.
	Changed(ctx context.Context, dir, file string) (bool, error)
}

// SetCheckbox ticks (or unticks) the checkbox on that line of the entry,
// if the entry is still at version.
func (w *Wikis) SetCheckbox(ctx context.Context, project, p string, line int, done bool, version string) error {
	return w.write(ctx, project, p, version, func(e *index.Entry) (string, error) {
		text := ""
		for _, c := range e.Checkboxes {
			if c.Line == line {
				text = c.Text
			}
		}
		if text == "" {
			return "", fmt.Errorf("%w: no checkbox on line %d of %s", ErrInvalid, line, p)
		}
		if err := e.SetCheckbox(line, done); err != nil {
			return "", err // the line was checked above: this is the file
		}
		verb := "Tick"
		if !done {
			verb = "Untick"
		}
		return fmt.Sprintf("%s %q in %s", verb, text, p), nil
	})
}

// MoveCard sets the card's status to column and, when lane isn't empty, its
// lane field to lane, in one write, if the entry is still at version.
func (w *Wikis) MoveCard(ctx context.Context, project, board, p, column, lane, version string) error {
	idx, err := w.open(project)
	if err != nil {
		return err
	}
	decls, _, err := declarations(idx)
	if err != nil {
		return err
	}
	i := -1
	for j, d := range decls {
		if d.ID == board && board != "" {
			i = j
		}
	}
	if i < 0 {
		return fmt.Errorf("%w: no board %q", ErrNotFound, board)
	}
	d := decls[i]
	if _, ok := onBoard(idx, d, p); !ok {
		return fmt.Errorf("%w: %s isn't on the board %s", ErrNotFound, p, board)
	}
	// The column of cards with no status takes no drops: that would be
	// removing the field, another operation than a move.
	if column == "" {
		return fmt.Errorf("%w: a card can't be moved to no status", ErrInvalid)
	}
	fields := map[string]any{d.Status: column}
	if lane != "" {
		if d.Lane == "" {
			return fmt.Errorf("%w: the board %s has no lanes", ErrInvalid, board)
		}
		fields[d.Lane] = lane
	}
	return w.write(ctx, project, p, version, func(e *index.Entry) (string, error) {
		if err := e.SetFields(fields); err != nil {
			var refused *parse.EditError
			switch {
			case errors.As(err, &refused):
				return "", fmt.Errorf("%w: %v", ErrRefused, err)
			case errors.Is(err, index.ErrNoFrontmatter): // an index.md or log.md on a board over every entry
				return "", fmt.Errorf("%w: %s: %v", ErrInvalid, p, err)
			}
			return "", err
		}
		to := column
		if lane != "" {
			to += ", " + lane
		}
		return fmt.Sprintf("Move %s to %s", p, to), nil
	})
}

// write applies change to the entry at p if it is still at version, then
// commits it, one commit holding only that file, unless the file already had
// changes nobody committed (the lead's, mid-edit): then the change waits for
// their next commit rather than committing their edits as the CEO's. The
// index is rebuilt before returning, so a refetch reads the write.
func (w *Wikis) write(ctx context.Context, project, p, version string, change func(*index.Entry) (string, error)) error {
	w.writing.Lock()
	defer w.writing.Unlock()
	dir, idx, err := w.build(project)
	if err != nil {
		return err
	}
	e, err := idx.Resolve("/" + strings.TrimPrefix(p, "/"))
	if err != nil {
		return fmt.Errorf("%w: %s", ErrNotFound, p)
	}
	raw, err := e.Raw()
	if err != nil {
		return err
	}
	if home.Version([]byte(raw)) != version {
		return fmt.Errorf("%w: %s", ErrConflict, p)
	}
	file := rel(e.Path)
	pending, err := w.git.Changed(ctx, dir, file)
	if err != nil {
		return err
	}
	message, err := change(e)
	if err != nil {
		return err
	}
	if !pending {
		if err := w.git.Commit(ctx, dir, message, file); err != nil {
			return fmt.Errorf("committing %s: %w", file, err)
		}
	}
	w.refresh(project)
	return nil
}
