package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileSink writes LogRecords to a local file as canonical JSON-lines, with
// optional size-based rotation per spec ADR-0001 §7.2.
//
// SECURITY NOTE: the path argument is resolved with filepath.Abs and
// opened as-is — there is no allow-list, sanitization, or path-traversal
// check, and the open follows symlinks. The host MUST treat path as a
// trusted configuration value and never accept it directly from end-user
// input or a plugin manifest. If the application supports plugin-supplied
// logging configuration, enforce an allow-list of writable directories
// before constructing the sink, and consider symlink-resistant resolution
// (openat2 RESOLVE_NO_SYMLINKS or a manual walk-up + lstat check)
// upstream of the FileSink.
//
// Phase 1 implementation uses a hand-rolled rotator (rather than the standard
// library logging package) — Go's log/slog and log packages do not expose a
// rotating file handler, and a clean implementation here keeps the
// dependency footprint minimal.
//
// Rotation rules:
//
//   - When maxBytes > 0, the file is rotated after a write that would push
//     its size beyond maxBytes.
//   - Rotation moves the current file to "<path>.1", existing ".N" files
//     shift to ".N+1"; the file at index keep is removed.
//   - When maxBytes <= 0, rotation is disabled and the file grows
//     unbounded.
type FileSink struct {
	path        string
	maxBytes    int64
	keep        int
	minSeverity int

	mu     sync.Mutex
	closed bool
	file   *os.File
	size   int64

	id string
}

// NewFileSink opens (creating if necessary) the file at path for append-only
// writes. Returns an error if the parent directory does not exist or the file
// cannot be opened.
//
// maxBytes <= 0 disables rotation; keep is the number of archived files to
// retain (e.g., keep=2 keeps .1 and .2).
func NewFileSink(path string, maxBytes int64, keep int, minSeverity int) (*FileSink, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("file_sink: resolve path %q: %w", path, err)
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("file_sink: open %q: %w", abs, err)
	}
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("file_sink: stat %q: %w", abs, err)
	}
	return &FileSink{
		path:        abs,
		maxBytes:    maxBytes,
		keep:        keep,
		minSeverity: minSeverity,
		file:        f,
		size:        stat.Size(),
		id:          "file:" + abs,
	}, nil
}

// ID returns the URI-style sink identifier.
func (s *FileSink) ID() string { return s.id }

// SupportsSeverity reports whether severityNumber meets the minimum.
func (s *FileSink) SupportsSeverity(severityNumber int) bool {
	return severityNumber >= s.minSeverity
}

// Emit writes record as a JSON line. Errors are absorbed; sink failures must
// not propagate to the caller of Logger.Info.
func (s *FileSink) Emit(record *LogRecord) {
	if record == nil {
		return
	}
	if !s.SupportsSeverity(record.SeverityNumber) {
		return
	}
	line, err := ToDagstackJSONL(record)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.file == nil {
		return
	}
	data := []byte(line + "\n")
	// Pre-rotate when this write would push us over the cap.
	if s.maxBytes > 0 && s.size+int64(len(data)) > s.maxBytes && s.size > 0 {
		if err := s.rotateLocked(); err != nil {
			return
		}
	}
	n, err := s.file.Write(data)
	if err != nil {
		return
	}
	s.size += int64(n)
}

// Flush attempts to sync the underlying file handle. timeoutSeconds is
// ignored — Phase 1 writes are synchronous.
func (s *FileSink) Flush(_ float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.file == nil {
		return nil
	}
	return s.file.Sync()
}

// Close closes the underlying file handle. Idempotent.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

// rotateLocked moves the current file to .1 (cascading existing .N → .N+1)
// and reopens a fresh file at path. Caller must hold s.mu.
func (s *FileSink) rotateLocked() error {
	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
	// Cascade: keep=2 → .2 := .1; .1 := current.
	for i := s.keep; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", s.path, i)
		if i == s.keep {
			// Drop the oldest archive.
			_ = os.Remove(src)
			continue
		}
		dst := fmt.Sprintf("%s.%d", s.path, i+1)
		_ = os.Rename(src, dst)
	}
	if s.keep > 0 {
		_ = os.Rename(s.path, s.path+".1")
	} else {
		_ = os.Remove(s.path)
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("file_sink: reopen after rotate: %w", err)
	}
	s.file = f
	s.size = 0
	return nil
}
