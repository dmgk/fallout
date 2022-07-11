package grep

import (
	"errors"

	"github.com/dmgk/fallout/cache"
)

type Grepper interface {
	Grep(options *Options, queries []string, gfn GrepFunc) error
}

type GrepFunc func(entry cache.Entry, res []*Result, err error) error

type Options struct {
	ContextAfter  int
	ContextBefore int
	QueryIsRegexp bool
}

// Result describes one search match result.
type Result struct {
	// Match holds the match as a byte string
	Match []byte

	// QuerySubmatch is a byte index pair identifying the result submatch in Text
	ResultSubmatch []int
}

// Stop is a special value that can be returned by GrepFunc to indicate that
// search needs to be terminated early.
var Stop = errors.New("stop")
