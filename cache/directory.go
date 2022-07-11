package cache

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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
