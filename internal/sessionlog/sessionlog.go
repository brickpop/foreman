// Package sessionlog keeps the raw byte stream of each session on disk, for
// scrollback replay: ~/.foreman/logs/<instance>/<session-id>.log, each
// instance's folder kept under a size cap
// (wiki/product/architecture/persistence.md#session-logs).
package sessionlog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Dir opens session logs under one folder.
type Dir struct {
	Root string           // e.g. ~/.foreman/logs
	Now  func() time.Time // session ids are timestamps; replaceable in tests
	// Cap is the most an instance's folder holds, in bytes, as it is when
	// asked (the log_cap_mb setting); nil or 0: no cap.
	Cap func() int64
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
	l := &File{dir: dir, session: now().UTC().Format("20060102T150405.000000000Z"), cap: d.Cap}
	if err := l.openPart(); err != nil {
		return nil, err
	}
	l.prune()
	return l, nil
}

// File is one session's log: one file, or several parts once it outgrows a
// quarter of the cap, so the oldest part can go while the session goes on.
type File struct {
	mu      sync.Mutex
	dir     string
	session string
	cap     func() int64
	parts   []string // this session's files, oldest first; the last is written
	f       *os.File
	size    int64 // of the last part
}

// partName is a part's file name: the first is <session>.log, the next ones
// <session>.p000002.log and on, so names sort oldest first.
func (l *File) partName(n int) string {
	if n == 1 {
		return l.session + ".log"
	}
	return fmt.Sprintf("%s.p%06d.log", l.session, n)
}

// openPart starts the session's next part.
func (l *File) openPart() error {
	path := filepath.Join(l.dir, l.partName(len(l.parts)+1))
	// O_EXCL: two sessions never share a file, even with an equal timestamp.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	l.f, l.size, l.parts = f, 0, append(l.parts, path)
	return nil
}

func (l *File) limit() int64 {
	if l.cap == nil {
		return 0
	}
	return l.cap()
}

// Write appends raw terminal bytes. A part that reaches a quarter of the cap
// is closed and the next one started; then the folder is pruned.
func (l *File) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n, err := l.f.Write(p)
	l.size += int64(n)
	if limit := l.limit(); err == nil && limit > 0 && l.size >= max(limit/4, 1) {
		old := l.f
		// A part that can't be started leaves the current one growing; the
		// next write tries again.
		if l.openPart() == nil {
			_ = old.Close()
			l.prune()
		}
	}
	return n, err
}

// prune deletes the instance's oldest log files while the folder is over the
// cap, never the part being written. Called with l.mu held.
func (l *File) prune() {
	limit := l.limit()
	if limit <= 0 {
		return
	}
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return
	}
	type file struct {
		path string
		size int64
	}
	var files []file
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		files = append(files, file{filepath.Join(l.dir, e.Name()), info.Size()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	current := l.parts[len(l.parts)-1]
	for _, f := range files {
		if total <= limit || f.path == current {
			return
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}

// Tail returns at most the last max bytes written, across the session's
// parts still on disk.
func (l *File) Tail(max int64) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []byte
	for i := len(l.parts) - 1; i >= 0 && int64(len(out)) < max; i-- {
		size := l.size
		if i < len(l.parts)-1 {
			info, err := os.Stat(l.parts[i])
			if err != nil {
				break // pruned: nothing older is left
			}
			size = info.Size()
		}
		chunk, err := readTail(l.parts[i], size, max-int64(len(out)))
		if err != nil {
			return nil, err
		}
		out = append(chunk, out...)
	}
	return out, nil
}

// readTail reads the last want bytes of the first size bytes of path.
func readTail(path string, size, want int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	start := max(size-want, 0)
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(f, size-start))
}

// Path is the file being written.
func (l *File) Path() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.parts[len(l.parts)-1]
}

// Close closes the log for writing; Tail keeps working.
func (l *File) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}
