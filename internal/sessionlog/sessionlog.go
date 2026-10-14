// Package sessionlog keeps the raw byte stream of each session on disk, for
// scrollback replay: .foreman/logs/<instance>/<session-id>.log
// (wiki/product/architecture/persistence.md).
package sessionlog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Dir opens session logs under one folder.
type Dir struct {
	Root string           // e.g. <workspace>/.foreman/logs
	Now  func() time.Time // session ids are timestamps; replaceable in tests
}

// Open creates the log of a new session of instance.
func (d Dir) Open(instance string) (*File, error) {
	if instance == "" || strings.ContainsAny(instance, `/\`) || instance == "." || instance == ".." {
		return nil, fmt.Errorf("sessionlog: invalid instance id %q", instance)
	}
	now := time.Now
	if d.Now != nil {
		now = d.Now
	}
	dir := filepath.Join(d.Root, instance)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	name := now().UTC().Format("20060102T150405.000000000Z") + ".log"
	// O_EXCL: two sessions never share a file, even with an equal timestamp.
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &File{f: f, path: f.Name()}, nil
}

// File is one session's log.
type File struct {
	f    *os.File
	path string
	size int64
}

// Write appends raw terminal bytes.
func (l *File) Write(p []byte) (int, error) {
	n, err := l.f.Write(p)
	l.size += int64(n)
	return n, err
}

// Tail returns at most the last max bytes written.
func (l *File) Tail(max int64) ([]byte, error) {
	f, err := os.Open(l.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	start := l.size - max
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(f, l.size-start))
}

// Path is the log's file path.
func (l *File) Path() string { return l.path }

// Close closes the log for writing; Tail keeps working.
func (l *File) Close() error { return l.f.Close() }
