package cache

import "time"

type Cacher interface {
	Entry(builder, origin string, timestamp time.Time) Entry
	Walk(f *Filter, fn WalkFunc) error
}

type Filter struct {
	Builders   []string
	Categories []string
	Origins    []string
	Names      []string
}

type WalkFunc func(path string, err error) error

type Entry interface {
	Exists() bool
	Get() (string, error)
	Put(content string) error
}
