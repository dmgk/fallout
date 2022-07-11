package grep

import (
	"errors"
	"fmt"

	"github.com/dmgk/fallout/cache"
)

type Options struct {
	QueryIsRegexp bool
}

// Stop is a special value that can be returned by GrepFunc to indicate that
// search needs to be terminated early.
var Stop = errors.New("stop")

// Result describes one search match result.
type Result struct {
	// Text holds the match as a byte slice
	Text []byte

	// QuerySubmatch is a byte index pair identifying the query submatch in Text
	QuerySubmatch []int

	// QuerySubmatch is a byte index pair identifying the result submatch in Text
	ResultSubmatch []int
}

type GrepFunc func(path string, res []*Result, err error) error

func Grep(c cache.Cacher, f *cache.Filter, o *Options, fn GrepFunc) error {
	c.Walk(f, func(path string, err error) error {
		if err != nil {
			fmt.Printf("++++> %v\n", err)
			return err
		}
		fmt.Printf("====> %s\n", path)
		return nil
	})

	return nil
}
