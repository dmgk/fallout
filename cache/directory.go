package cache

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Directory implements filesystem Cacher.
type Directory struct {
	// cache directory absolute path
	path string
	// timestamp of the most recent entry
	timestamp time.Time
}

func NewDirectory(root, subdir string) (Cacher, error) {
	path, err := filepath.Abs(filepath.Join(root, subdir))
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}
	return &Directory{
		path:      path,
		timestamp: loadTimestamp(path),
	}, nil
}

func NewDefaultDirectory(subdir string) (Cacher, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	return NewDirectory(root, subdir)
}

func (c *Directory) Path() string {
	return c.path
}

func (c *Directory) Timestamp() time.Time {
	return c.timestamp
}

func (c *Directory) Entry(builder, origin string, timestamp time.Time) (Entry, error) {
	return newEntry(c, builder, origin, timestamp)
}

// DirectoryEntry implements filesystem Entry.
type DirectoryEntry struct {
	// directory cache that owns this entry
	cache *Directory
	// entry absolute path
	path      string
	builder   string
	origin    string
	timestamp time.Time
}

const (
	timestampFormat = "2006-01-02T15:04:05"
	ext             = ".log"
)

func newEntry(c *Directory, builder, origin string, timestamp time.Time) (*DirectoryEntry, error) {
	if builder == "" {
		return nil, errors.New("empty builder")
	}
	if origin == "" {
		return nil, errors.New("empty origin")
	}
	if timestamp.IsZero() {
		return nil, errors.New("zero timestamp")
	}
	return &DirectoryEntry{
		cache:     c,
		path:      filepath.Join(c.path, builder, origin, timestamp.UTC().Format(timestampFormat)) + ext,
		builder:   builder,
		origin:    origin,
		timestamp: timestamp.UTC(),
	}, nil
}

func (e *DirectoryEntry) Path() string {
	return e.path
}

func (e *DirectoryEntry) Exists() bool {
	fi, err := os.Stat(e.path)
	return err == nil && fi.Size() > 0
}

func (e *DirectoryEntry) Read() ([]byte, error) {
	f, err := os.Open(e.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}

func (e *DirectoryEntry) Write(buf []byte) error {
	if err := os.MkdirAll(filepath.Dir(e.path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(e.path, buf, 0644); err != nil {
		return err
	}
	e.cache.updateTimestamp(e.timestamp)
	return nil
}

func (e *DirectoryEntry) Remove() error {
	return os.Remove(e.path)
}

func (e *DirectoryEntry) With(wfn WithFunc) error {
	f, err := os.Open(e.path)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	buf := bufGet()
	defer bufPut(buf)

	buf.Grow(int(fi.Size()) + bytes.MinRead)
	_, err = buf.ReadFrom(f)
	if err != nil {
		return err
	}

	return wfn(buf.Bytes())
}

func (e *DirectoryEntry) Info() EntryInfo {
	return EntryInfo{
		Builder:   e.builder,
		Origin:    e.origin,
		Timestamp: e.timestamp,
	}
}

func (e *DirectoryEntry) String() string {
	return e.Path()
}

func (c *Directory) Walker(filter *Filter) Walker {
	w := &DirectoryWalker{
		cache: c,
	}
	if filter != nil {
		w.filter = *filter
	}
	return w
}

// DirectoryWalker implements filesystem cache Walker.
type DirectoryWalker struct {
	filter Filter
	cache  *Directory
}

func (w *DirectoryWalker) Walk(wfn WalkFunc) error {
	rch := make(chan Entry)
	ech := make(chan error)

	go w.walkCache(rch, ech)

	rok := true
	for rok {
		var r Entry
		select {
		case r, rok = <-rch:
			if rok {
				if werr := wfn(r, nil); werr != nil {
					if werr == Stop {
						return nil
					}
					return werr
				}
			}
		case err, eok := <-ech:
			if eok {
				if werr := wfn(nil, err); werr != nil {
					if werr == Stop {
						return nil
					}
					return werr
				}
			}
		}
	}

	return nil
}

func (w *DirectoryWalker) builderAllowed(builder string) bool {
	return valueAllowed(builder, w.filter.Builders, false)
}

func (w *DirectoryWalker) categoryAllowed(category string) bool {
	return valueAllowed(category, w.filter.Categories, false)
}

func (w *DirectoryWalker) originAllowed(origin string) bool {
	return valueAllowed(origin, w.filter.Origins, true)
}

func (w *DirectoryWalker) nameAllowed(name string) bool {
	return valueAllowed(name, w.filter.Names, false)
}

func valueAllowed(value string, filter []string, exact bool) bool {
	if len(filter) == 0 {
		return true
	}
	for _, s := range filter {
		if exact && value == s || !exact && strings.Contains(value, s) {
			return true
		}
	}
	return false
}

func (w *DirectoryWalker) walkCache(rch chan Entry, ech chan error) {
	defer close(rch)
	defer close(ech)

	dir, err := os.ReadDir(w.cache.path)
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		if d.IsDir() && w.builderAllowed(d.Name()) {
			w.walkBuilder(d.Name(), rch, ech)
		}
	}
}

func (w *DirectoryWalker) walkBuilder(builder string, rch chan Entry, ech chan error) {
	dir, err := os.ReadDir(filepath.Join(w.cache.path, builder))
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		if d.IsDir() && w.categoryAllowed(d.Name()) {
			w.walkCategory(builder, d.Name(), rch, ech)
		}
	}
}

func (w *DirectoryWalker) walkCategory(builder, category string, rch chan Entry, ech chan error) {
	dir, err := os.ReadDir(filepath.Join(w.cache.path, builder, category))
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		origin := category + string(filepath.Separator) + d.Name()
		if d.IsDir() && w.originAllowed(origin) && w.nameAllowed(d.Name()) {
			w.walkOrigin(builder, origin, rch, ech)
		}
	}
}

func (w *DirectoryWalker) walkOrigin(builder, origin string, rch chan Entry, ech chan error) {
	dir, err := os.ReadDir(filepath.Join(w.cache.path, builder, origin))
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		if !d.IsDir() {
			ts, err := time.Parse(timestampFormat, strings.TrimSuffix(d.Name(), ext))
			if err != nil {
				ech <- err
				continue
			}
			if ts.Before(w.filter.Since) || !w.filter.Before.IsZero() && ts.After(w.filter.Before) {
				continue
			}
			e, err := newEntry(w.cache, builder, origin, ts)
			if err != nil {
				ech <- err
				continue
			}
			rch <- e
		}
	}
}

const (
	cacheTimestampName   = ".timestamp"
	cacheTimestampFormat = time.RFC3339
)

func loadTimestamp(path string) time.Time {
	var zero time.Time
	if buf, err := os.ReadFile(filepath.Join(path, cacheTimestampName)); err == nil {
		if ts, err := time.Parse(cacheTimestampFormat, string(buf)); err == nil {
			return ts
		}
	}
	return zero
}

func (c *Directory) updateTimestamp(ts time.Time) {
	if ts.After(c.timestamp) {
		c.timestamp = ts
		_ = os.WriteFile(filepath.Join(c.path, cacheTimestampName), []byte(ts.Format(cacheTimestampFormat)), 0664)
	}
}

func (c *Directory) Remove() error {
	return os.RemoveAll(c.path)
}

var bufPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

func bufGet() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

func bufPut(buf *bytes.Buffer) {
	buf.Reset()
	bufPool.Put(buf)
}
