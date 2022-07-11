package cache

import "time"

type Cacher interface {
	Entry(builder, origin string, timestamp time.Time) Entry
}

type Entry interface {
	Exists() bool
	Get() (string, error)
	Put(content string) error
}
