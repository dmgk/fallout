package cache

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Directory implements filesystem Cacher.
type Directory struct {
	path string
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
		path: path,
	}, nil
}

func NewDefaultDirectory(subdir string) (Cacher, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	return NewDirectory(root, subdir)
}

func (c *Directory) Cache(builder, origin string, timestamp time.Time) (Entry, error) {
	if builder == "" {
		return nil, errors.New("empty builder")
	}
	if origin == "" {
		return nil, errors.New("empty origin")
	}
	if timestamp.IsZero() {
		return nil, errors.New("zero timestamp")
	}
	return DirectoryEntry(filepath.Join(c.path, builder, origin, timestamp.Format(timestampFormat)) + ext), nil
}

// DirectoryEntry implements filesystem Entry.
type DirectoryEntry string

const (
	timestampFormat = "2006-01-02T15:04:05"
	ext             = ".log"
)

func (e DirectoryEntry) Exists() bool {
	fi, err := os.Stat(string(e))
	return err == nil && fi.Size() > 0
}

func (e DirectoryEntry) Get() ([]byte, error) {
	f, err := os.Open(string(e))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func (e DirectoryEntry) Put(buf []byte) error {
	if err := os.MkdirAll(filepath.Dir(string(e)), 0755); err != nil {
		return err
	}

	f, err := os.Create(string(e))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(buf)
	return err
}

func (e DirectoryEntry) Info() (*EntryInfo, error) {
	parts := strings.Split(string(e), string(filepath.Separator))
	if len(parts) < 4 {
		return nil, errors.New("invalid DirectoryEntry")
	}
	ts, err := time.Parse(timestampFormat, strings.TrimSuffix(parts[len(parts)-1], ext))
	if err != nil {
		return nil, fmt.Errorf("invalid DirectoryEntry timestamp: %s", err)
	}
	return &EntryInfo{
		Builder:   parts[len(parts)-4],
		Origin:    parts[len(parts)-3] + string(filepath.Separator) + parts[len(parts)-2],
		Timestamp: ts,
	}, nil
}

func (c *Directory) Walker(filter *Filter) Walker {
	return &DirectoryWalker{
		path:   c.path,
		filter: filter,
	}
}

// DirectoryWalker implements filesystem cache Walker.
type DirectoryWalker struct {
	path   string
	filter *Filter
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
					if werr == ErrStop {
						return nil
					}
					return werr
				}
			}
		case err, eok := <-ech:
			if eok {
				if werr := wfn(nil, err); werr != nil {
					if werr == ErrStop {
						return nil
					}
					return werr
				}
			}
		}
	}

	return nil
}

func valueAllowed(value string, filter []string) bool {
	if len(filter) == 0 {
		return true
	}
	for _, s := range filter {
		if strings.Contains(value, s) {
			return true
		}
	}
	return false
}

func builderAllowed(builder string, f *Filter) bool {
	if f == nil {
		return true
	}
	return valueAllowed(builder, f.Builders)
}

func categoryAllowed(category string, f *Filter) bool {
	if f == nil {
		return true
	}
	return valueAllowed(category, f.Categories)
}

func originAllowed(origin string, f *Filter) bool {
	if f == nil {
		return true
	}
	return valueAllowed(origin, f.Origins)
}

func nameAllowed(name string, f *Filter) bool {
	if f == nil {
		return true
	}
	return valueAllowed(name, f.Names)
}

func (w *DirectoryWalker) walkCache(rch chan Entry, ech chan error) {
	defer close(rch)
	defer close(ech)

	dir, err := os.ReadDir(w.path)
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		if d.IsDir() && builderAllowed(d.Name(), w.filter) {
			w.walkBuilder(filepath.Join(w.path, d.Name()), rch, ech)
		}
	}
}

func (w *DirectoryWalker) walkBuilder(path string, rch chan Entry, ech chan error) {
	dir, err := os.ReadDir(path)
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		if d.IsDir() && categoryAllowed(d.Name(), w.filter) {
			w.walkCategory(filepath.Join(path, d.Name()), rch, ech)
		}
	}
}

func (w *DirectoryWalker) walkCategory(path string, rch chan Entry, ech chan error) {
	dir, err := os.ReadDir(path)
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		o := filepath.Base(path) + string(filepath.Separator) + d.Name()
		if d.IsDir() && originAllowed(o, w.filter) && nameAllowed(d.Name(), w.filter) {
			w.walkOrigin(filepath.Join(path, d.Name()), rch, ech)
		}
	}
}

func (w *DirectoryWalker) walkOrigin(path string, rch chan Entry, ech chan error) {
	dir, err := os.ReadDir(path)
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		if !d.IsDir() {
			rch <- DirectoryEntry(filepath.Join(path, d.Name()))
		}
	}
}
