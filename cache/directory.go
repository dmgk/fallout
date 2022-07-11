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

func DefaultDirectory(subdir string) (Cacher, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	return NewDirectory(root, subdir)
}

func (c *Directory) Path() string {
	return c.path
}

var ErrStop = errors.New("stop")

func (c *Directory) Walk(f *Filter, fn WalkFunc) error {
	pch := make(chan string)
	ech := make(chan error)

	go c.walkCache(f, pch, ech)

	pok := true
	for pok {
		var p string
		select {
		case p, pok = <-pch:
			if pok {
				if ferr := fn(p, nil); ferr != nil {
					if ferr == ErrStop {
						return nil
					}
					return ferr
				}
			}
		case err, eok := <-ech:
			if eok {
				if ferr := fn("", err); ferr != nil {
					if ferr == ErrStop {
						return nil
					}
					return ferr
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

func (f *Filter) builderAllowed(builder string) bool {
	return valueAllowed(builder, f.Builders)
}

func (f *Filter) categoryAllowed(category string) bool {
	return valueAllowed(category, f.Categories)
}

func (f *Filter) originAllowed(origin string) bool {
	return valueAllowed(origin, f.Origins)
}

func (f *Filter) nameAllowed(name string) bool {
	return valueAllowed(name, f.Names)
}

func (c *Directory) walkCache(f *Filter, pch chan string, ech chan error) {
	defer close(pch)
	defer close(ech)

	dir, err := os.ReadDir(c.path)
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		if d.IsDir() && f.builderAllowed(d.Name()) {
			c.walkBuilder(filepath.Join(c.path, d.Name()), f, pch, ech)
		}
	}
}

func (c *Directory) walkBuilder(path string, f *Filter, pch chan string, ech chan error) {
	dir, err := os.ReadDir(path)
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		if d.IsDir() && f.categoryAllowed(d.Name()) {
			c.walkCategory(filepath.Join(path, d.Name()), f, pch, ech)
		}
	}
}

func (c *Directory) walkCategory(path string, f *Filter, pch chan string, ech chan error) {
	dir, err := os.ReadDir(path)
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		o := filepath.Base(path) + string(filepath.Separator) + d.Name()
		if d.IsDir() && f.originAllowed(o) && f.nameAllowed(d.Name()) {
			c.walkOrigin(filepath.Join(path, d.Name()), f, pch, ech)
		}
	}
}

func (c *Directory) walkOrigin(path string, f *Filter, pch chan string, ech chan error) {
	dir, err := os.ReadDir(path)
	if err != nil {
		ech <- err
		return
	}
	for _, d := range dir {
		if !d.IsDir() {
			pch <- filepath.Join(path, d.Name())
		}
	}
}

const (
	timestampFormat = "2006-01-02T15:04:05"
	ext             = ".log"
)

type DirectoryEntry string

func (c *Directory) Entry(builder, origin string, timestamp time.Time) Entry {
	return DirectoryEntry(filepath.Join(c.path, builder, origin, timestamp.Format(timestampFormat)) + ext)
}

func (e DirectoryEntry) Exists() bool {
	fi, err := os.Stat(string(e))
	return err == nil && fi.Size() > 0
}

func (e DirectoryEntry) Get() (string, error) {
	f, err := os.Open(string(e))
	if err != nil {
		return "", err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", err
	}

	buf := bufGet()
	buf.Grow(int(fi.Size()) + bytes.MinRead)
	_, err = buf.ReadFrom(f)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (e DirectoryEntry) Put(content string) error {
	if err := os.MkdirAll(filepath.Dir(string(e)), 0755); err != nil {
		return err
	}

	f, err := os.Create(string(e))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.WriteString(f, content)
	return err
}

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func bufGet() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

func bufPut(b *bytes.Buffer) {
	b.Reset()
	bufPool.Put(b)
}
