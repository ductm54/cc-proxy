// Package debugdump writes full request/response exchanges to size-rotated
// JSONL files so proxied traffic can be diffed against a real Claude Code
// capture.
package debugdump

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	activeName    = "dump.jsonl"
	rotatedPrefix = "dump-"
	rotatedSuffix = ".jsonl"
)

// Writer appends lines to <dir>/dump.jsonl. When the next line would push the
// file past maxSize, the file is renamed to dump-<timestamp>.jsonl and a fresh
// one is opened. Only the newest maxFiles rotated files are kept.
type Writer struct {
	mu       sync.Mutex
	dir      string
	maxSize  int64
	maxFiles int
	f        *os.File
	size     int64
}

// NewWriter opens (or creates) the active dump file in dir.
func NewWriter(dir string, maxSize int64, maxFiles int) (*Writer, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("debugdump: max size must be positive")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("debugdump: create dir: %w", err)
	}
	w := &Writer{dir: dir, maxSize: maxSize, maxFiles: maxFiles}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) open() error {
	f, err := os.OpenFile(filepath.Join(w.dir, activeName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("debugdump: open: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("debugdump: stat: %w", err)
	}
	w.f = f
	w.size = st.Size()
	return nil
}

// WriteLine appends line plus a trailing newline, rotating first if needed.
func (w *Writer) WriteLine(line []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return fmt.Errorf("debugdump: writer closed")
	}
	n := int64(len(line)) + 1
	if w.size > 0 && w.size+n > w.maxSize {
		if err := w.rotate(); err != nil {
			return err
		}
	}
	buf := make([]byte, 0, n)
	buf = append(append(buf, line...), '\n')
	written, err := w.f.Write(buf)
	w.size += int64(written)
	return err
}

func (w *Writer) rotate() error {
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("debugdump: close: %w", err)
	}
	w.f = nil
	name := rotatedPrefix + time.Now().UTC().Format("20060102T150405.000000000") + rotatedSuffix
	if err := os.Rename(filepath.Join(w.dir, activeName), filepath.Join(w.dir, name)); err != nil {
		return fmt.Errorf("debugdump: rename: %w", err)
	}
	if err := w.open(); err != nil {
		return err
	}
	return w.prune()
}

// prune deletes the oldest rotated files beyond maxFiles. maxFiles <= 0 keeps all.
func (w *Writer) prune() error {
	if w.maxFiles <= 0 {
		return nil
	}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return fmt.Errorf("debugdump: list: %w", err)
	}
	var rotated []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasPrefix(n, rotatedPrefix) && strings.HasSuffix(n, rotatedSuffix) {
			rotated = append(rotated, n)
		}
	}
	// Timestamped names sort chronologically.
	sort.Strings(rotated)
	for len(rotated) > w.maxFiles {
		if err := os.Remove(filepath.Join(w.dir, rotated[0])); err != nil {
			return fmt.Errorf("debugdump: prune: %w", err)
		}
		rotated = rotated[1:]
	}
	return nil
}

// Close flushes and closes the active file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}
