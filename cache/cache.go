package cache

import (
	"errors"
	"time"
)

// Cacher is the cache interface.
type Cacher interface {
	// Path returns this cache path (implementation-specific).
	Path() string
	// Timestamp of the most recent entry
	Timestamp() time.Time
	// Cache returns (a possibly not yet existing or empty) cache entry with given attributes.
	Entry(builder, origin string, timestamp time.Time) (Entry, error)
	// Walker returns cache walking interface.
	Walker(filter *Filter) Walker
	// Remove completely removes all cached data.
	Remove() error
}

// Entry is the cache entry interface.
type Entry interface {
	// Path returns this entry path (implementation-specific).
	Path() string
	// Exists return true if this entry is present in the cache.
	Exists() bool
	// Get returns entry contents.
	Read() ([]byte, error)
	// Put saves buf as the entry contents in the cache.
	Write(buf []byte) error
	// Remove removes this entry from the cache.
	Remove() error
	// With calls wfn with this entry contents as a byte slice.
	// Underlying buffer is taken from the buffer pool and reused.
	With(wfn WithFunc) error
	// Info return entry attributes.
	Info() EntryInfo
}

type WithFunc func(buf []byte) error

// EntryInfo holds entry attributes.
type EntryInfo struct {
	// Builder name.
	Builder string
	// Port origin.
	Origin string
	// Fallout log timestamp.
	Timestamp time.Time
}

// Filter describes what walked is allowed to walk.
type Filter struct {
	// Allowed builder names, partial names are ok.
	Builders []string
	// Allowed categories, partial names are ok.
	Categories []string
	// Allowed origins, partial names are ok.
	Origins []string
	// Allowed port names, partial names are ok.
	Names []string
}

// Walker is the cache walker interface.
type Walker interface {
	// Walk walks the cache and calls wfn for each entry that made it through Filter.
	Walk(wfn WalkFunc) error
}

type WalkFunc func(entry Entry, err error) error

// Stop is a special value that can be returned by WalkFunc to indicate that
// walking needs to be terminated early.
var Stop = errors.New("stop")
