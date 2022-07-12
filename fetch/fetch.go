package fetch

import (
	"errors"
	"fmt"
	"time"
)

type Fetcher interface {
	Fetch(options *Options, qfn QueryFunc, rfn ResultFunc) error
}

type Options struct {
	After time.Time
	Limit int
}

type QueryFunc func(res *Result) (bool, error)
type ResultFunc func(res *Result, err error) error

type Result struct {
	Builder   string
	Origin    string
	Timestamp time.Time
	URL       string
	Content   []byte
}

func (r *Result) String() string {
	return fmt.Sprintf("%s %32s %s", r.Timestamp.Format("2006-01-02 15:04:05"), r.Builder, r.Origin)
}

var Stop = errors.New("stop")
