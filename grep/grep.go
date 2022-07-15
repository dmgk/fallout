package grep

import (
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/dmgk/fallout/cache"
)

// Grepper searches cached fallout logs.
type Grepper struct {
	walker cache.Walker
}

// New creates a new Grepper instance.
func New(walker cache.Walker) *Grepper {
	return &Grepper{
		walker: walker,
	}
}

type Options struct {
	// Number of context lines before the match.
	ContextAfter int
	// Number of context lines after the match.
	ContextBefore int
	// Treat queries as a regular expressions, not as a plain text.
	QueryIsRegexp bool
	// At least one query needs to match, not all of them.
	Ored bool
}

type GrepFunc func(entry cache.Entry, res []*Match, err error) error

// Match describes one search match result.
type Match struct {
	// Text holds the match as a byte string.
	Text []byte
	// ResultSubmatch is a byte index pair identifying the result submatch in Text.
	ResultSubmatch []int
}

// Stop is a special value that can be returned by GrepFunc to indicate that
// search needs to be terminated early.
var Stop = errors.New("stop")

// grepResult holds entry matching results.
type grepResult struct {
	entry cache.Entry
	mm    []*Match
}

// Grep searches cached logs and calls gfn for each found match.
func (g *Grepper) Grep(options *Options, queries []string, gfn GrepFunc, jobs int) error {
	var mrs []*matcher
	for _, q := range queries {
		m, err := newMatcher(options, q)
		if err != nil {
			return err
		}
		mrs = append(mrs, m)
	}

	rch := make(chan *grepResult)
	ech := make(chan error)

	go g.walkCache(mrs, options.Ored, rch, ech, jobs)

	rok := true
	for rok {
		var r *grepResult
		select {
		case r, rok = <-rch:
			if rok {
				if gerr := gfn(r.entry, r.mm, nil); gerr != nil {
					if gerr == Stop {
						return nil
					}
					return gerr
				}
			}
		case err, eok := <-ech:
			if eok {
				if gerr := gfn(nil, nil, err); gerr != nil {
					if gerr == Stop {
						return nil
					}
					return gerr
				}
			}
		}
	}
	return nil
}

// walkCache does matching against cached logs.
func (g *Grepper) walkCache(mrs []*matcher, ored bool, rch chan *grepResult, ech chan error, jobs int) {
	defer close(rch)
	defer close(ech)

	var wg sync.WaitGroup
	sem := make(chan int, jobs)

	err := g.walker.Walk(func(entry cache.Entry, err error) error {
		if err != nil {
			return err
		}

		sem <- 1
		wg.Add(1)

		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()

			err := entry.With(func(buf []byte) error {
				res := &grepResult{
					entry: entry,
				}

				// no queries were provided, return the whole text
				if len(mrs) == 0 {
					res.mm = []*Match{
						{Text: buf},
					}
					rch <- res
					return nil
				}

				for _, mr := range mrs {
					m, err := mr.match(buf)
					if err != nil {
						return err
					}
					if !ored && m == nil {
						return nil // results are ANDed and the current matcher doesn't match
					}
					if m != nil {
						res.mm = append(res.mm, m)
					}
				}
				if len(res.mm) > 0 {
					rch <- res
				}

				return nil
			})
			if err != nil {
				ech <- err
			}
		}()

		return nil
	})
	if err != nil {
		ech <- err
	}

	wg.Wait()
}

// matcher describes a compiled regular expression query
type matcher struct {
	// Compiled regexp.
	rx *regexp.Regexp
	// Result subexpression index.
	rsi int
}

const (
	// Result subexpression name.
	rsn = "r"
	// Query rx pattern.
	queryPat = `(?:.*\n){0,%d}.*(?P<r>%s).*(\n|\z)(?:.*\n){0,%d}`
)

// newMatcher returns compiled matcher for the given query.
func newMatcher(options *Options, query string) (*matcher, error) {
	q := query
	if !options.QueryIsRegexp {
		q = regexp.QuoteMeta(q)
	}
	rx, err := regexp.Compile(fmt.Sprintf(queryPat, options.ContextBefore, q, options.ContextAfter))
	if err != nil {
		return nil, err
	}

	rsi := -1
	for i, n := range rx.SubexpNames() {
		if n == rsn {
			rsi = i
		}
	}
	if rsi < 0 {
		return nil, fmt.Errorf("invalid subexpressions: %s", rx)
	}

	return &matcher{
		rx:  rx,
		rsi: rsi,
	}, nil
}

// match performs matching against the given content.
func (m *matcher) match(content []byte) (*Match, error) {
	smi := m.rx.FindSubmatchIndex(content)
	if smi == nil {
		return nil, nil
	}
	if len(smi) <= m.rsi {
		return nil, fmt.Errorf("unexpected number of subexpressions %d in %v", len(smi), m)
	}

	res := &Match{
		Text: content[smi[0]:smi[1]],
	}
	if m.rsi >= 0 {
		res.ResultSubmatch = []int{smi[2*m.rsi] - smi[0], smi[2*m.rsi+1] - smi[0]}
	}

	return res, nil
}
