package cache

import (
	"errors"
	"time"
)

type Cacher interface {
	Cache(builder, origin string, timestamp time.Time) (Entry, error)
	Walker(filter *Filter) Walker
}

type Entry interface {
	Exists() bool
	Get() ([]byte, error)
	Put(buf []byte) error
	Info() (*EntryInfo, error)
}

type EntryInfo struct {
	Builder   string
	Origin    string
	Timestamp time.Time
}

type Filter struct {
	Builders   []string
	Categories []string
	Origins    []string
	Names      []string
}

type Walker interface {
	Walk(wfn WalkFunc) error
}

type WalkFunc func(entry Entry, err error) error

var ErrStop = errors.New("stop")
