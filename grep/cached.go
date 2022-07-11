package grep

import (
	"fmt"
	"regexp"

	"github.com/dmgk/fallout/cache"
)

func NewCached(walker cache.Walker) Grepper {
	return &CachedGrepper{
		w: walker,
	}
}

type CachedGrepper struct {
	w cache.Walker
}

func (g *CachedGrepper) Grep(options *Options, queries []string, gfn GrepFunc) error {
	var ms []*matcher
	for _, q := range queries {
		m, err := newMatcher(options, q)
		if err != nil {
			return err
		}
		ms = append(ms, m)
	}

	g.w.Walk(func(entry cache.Entry, err error) error {
		if err != nil {
			fmt.Printf("++++> %v\n", err)
			return err
		}
		fmt.Printf("====> %s\n", entry)
		return nil
	})

	return nil
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
func (m *matcher) match(content []byte) (*Result, error) {
	smi := m.rx.FindSubmatchIndex(content)
	if smi == nil {
		return nil, nil
	}
	if len(smi) <= m.rsi {
		return nil, fmt.Errorf("unexpected number of subexpressions %d in %v", len(smi), m)
	}

	res := &Result{
		Match: content[smi[0]:smi[1]],
	}
	if m.rsi >= 0 {
		res.ResultSubmatch = []int{smi[2*m.rsi] - smi[0], smi[2*m.rsi+1] - smi[0]}
	}

	return res, nil
}
