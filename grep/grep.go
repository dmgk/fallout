package grep

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/dmgk/fallout/cache"
)

type Grepper struct {
	walker cache.Walker
}

func New(walker cache.Walker) *Grepper {
	return &Grepper{
		walker: walker,
	}
}

type Options struct {
	ContextAfter  int
	ContextBefore int
	QueryIsRegexp bool
}

type GrepFunc func(entry cache.Entry, res []*Match, err error) error

// Match describes one search match result.
type Match struct {
	// Text holds the match as a byte string
	Text []byte

	// QuerySubmatch is a byte index pair identifying the result submatch in Text
	ResultSubmatch []int
}

// Stop is a special value that can be returned by GrepFunc to indicate that
// search needs to be terminated early.
var Stop = errors.New("stop")

type grepResult struct {
	entry cache.Entry
	mm    []*Match
}

func (g *Grepper) Grep(options *Options, queries []string, gfn GrepFunc) error {
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

	go g.walkCache(mrs, rch, ech)

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

func (g *Grepper) walkCache(mrs []*matcher, rch chan *grepResult, ech chan error) {
	defer close(rch)
	defer close(ech)

	g.walker.Walk(func(entry cache.Entry, err error) error {
		if err != nil {
			return err
		}

		buf, err := entry.Get()
		if err != nil {
			return err
		}

		res := &grepResult{
			entry: entry,
		}
		for _, mr := range mrs {
			m, err := mr.match(buf)
			if err != nil {
				return err
			}
			// if !rxsOr && m == nil {
			//     return // results are ANDed and the current rx doesn't match
			// }
			if m != nil {
				res.mm = append(res.mm, m)
			}
		}

		if len(res.mm) > 0 {
			rch <- res
		}
		return nil
	})
}

type matcher struct {
	rx  *regexp.Regexp // compiled regexp
	rsi int            // result subexpression index
}

const (
	// result subexpression name
	rsn = "r"
	// query rx pattern
	queryPat = `(?:.*\n){0,%d}.*(?P<r>%s).*(\n|\z)(?:.*\n){0,%d}`
)

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
