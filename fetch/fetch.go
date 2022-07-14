package fetch

import (
	"errors"
	"fmt"
	"time"
)

// Fetcher is the log downloader interface.
type Fetcher interface {
	// Fetch logs and download logs for which qfn returns false.
	// If qfn returns true, then the log is assumed to be already cached.
	// Call rfn for each downloaded log.
	Fetch(filter *Filter, qfn QueryFunc, rfn ResultFunc) error
}

// Filter holds fetcher filter options.
type Filter struct {
	// Download only logs created after this date.
	After time.Time
	// Download only this many most recent logs.
	Limit int
	// Allowed builder names, partial names are ok.
	Builders []string
	// Allowed categories, partial names are ok.
	Categories []string
	// Allowed origins, partial names are ok.
	Origins []string
	// Allowed port names, partial names are ok.
	Names []string
}

type QueryFunc func(res *Result) (bool, error)
type ResultFunc func(res *Result, err error) error

// Result is the fetch result.
type Result struct {
	// Log builder name.
	Builder string
	// Log port origin.
	Origin string
	// Log timestamp.
	Timestamp time.Time
	// Log content URL.
	URL string
	// Log content.
	Content []byte
}

func (r *Result) String() string {
	return fmt.Sprintf("%s %32s %s", r.Timestamp.Format("2006-01-02 15:04:05"), r.Builder, r.Origin)
}

// Stop is a special value that can be returned by ResultFunc to indicate that
// fetching needs to be terminated early.
var Stop = errors.New("stop")
