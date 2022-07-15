package fetch

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

// Maillist implements Fetcher that scrapes logs from pkg-fallout mail list archives.
type Maillist struct {
	filter    Filter
	collector *colly.Collector
}

func NewMaillist(userAgent string) Fetcher {
	c := colly.NewCollector(
		colly.UserAgent(userAgent),
	)
	return &Maillist{
		collector: c,
	}
}

func (f *Maillist) Fetch(filter *Filter, qfn QueryFunc, rfn ResultFunc) error {
	if filter != nil {
		f.filter = *filter
	}
	rch := make(chan *Result)
	ech := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go f.fetchMaillist(ctx, qfn, rch, ech)

	rok := true
	for rok {
		var res *Result
		select {
		case res, rok = <-rch:
			if rok {
				if rerr := rfn(res, nil); rerr != nil {
					if rerr == Stop {
						return nil
					}
					return rerr
				}
			}
		case err, eok := <-ech:
			if eok {
				if rerr := rfn(nil, err); rerr != nil {
					if rerr == Stop {
						return nil
					}
					return rerr
				}
			}
		}
	}

	return nil
}

const baseUrl = "https://lists.freebsd.org/archives/freebsd-pkg-fallout/"

var (
	builderAndOriginRe = regexp.MustCompile(`\[.+ - (.+)\]\[(.+)\].*`)
	timestampRe        = regexp.MustCompile(`\((.*)\)`)
)

// fetchMaillists scrapes fallout logs from pkg-fallout mail list archive pages.
// NOTE: keep this code non-parallel to avoid spurious 503 Service Unavailable from lists.freebsd.org.
func (f *Maillist) fetchMaillist(ctx context.Context, qfn QueryFunc, rch chan *Result, ech chan error) {
	resMap := make(map[string]*Result)
	count := 0

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(rch)
	defer close(ech)

	f.collector.OnHTML("tr td:nth-of-type(1) a", func(e *colly.HTMLElement) {
		select {
		case <-ctx.Done():
			return
		default:
			// Archive page: https://lists.freebsd.org/archives/freebsd-pkg-fallout/
			// Example:
			//
			//   <tr>
			//     <td class="ml"><a href="2022-July/">July 2022</a></td>
			//     ...
			//   </tr>
			//
			// We're assuming this page is a month index, in the descending order of months e.g.
			//   July 2022
			//   June 2022
			//   May 2022
			//   ...

			ts, err := time.Parse("January 2006", e.Text)
			if err != nil {
				ech <- err
				return
			}
			mi := ts.UTC().Year()*100 + int(ts.UTC().Month())
			ma := f.filter.After.UTC().Year()*100 + int(f.filter.After.UTC().Month())
			if mi < ma {
				cancel() // link is to the month before "After", stop
				return
			}

			// extract month page URL
			u := *e.Request.URL
			u.Path = path.Join(u.Path, e.Attr("href"))

			// visit month page and collect fallout log links
			f.collector.Visit(u.String())

			// process collected partial results
			var resSlice []*Result
			for _, r := range resMap {
				resSlice = append(resSlice, r)
			}
			sort.Slice(resSlice, func(i, j int) bool {
				// by descending Timestamp
				return resSlice[i].Timestamp.After(resSlice[j].Timestamp)
			})
			for _, r := range resSlice {
				count++
				if f.filter.Limit > 0 && count > f.filter.Limit {
					break // limit is reached
				}
				// fetch fallout log, unless it was already cached
				cached, err := qfn(r)
				if err != nil {
					ech <- err
					continue
				}
				if !cached {
					f.collector.Visit(r.URL)
				}
			}
		}
	})

	f.collector.OnHTML("li", func(e *colly.HTMLElement) {
		select {
		case <-ctx.Done():
			return
		default:
			var currentUrl = e.Request.URL.String()
			if !strings.HasSuffix(currentUrl, ".html") {
				// Monthly index page, e.g. https://lists.freebsd.org/archives/freebsd-pkg-fallout/2022-July/
				// Example:
				//
				//   <li>
				//     <a href="240803.html">[package - 130arm64-quarterly][lang/polyml] Failed for polyml-5.9 in build</a>:
				//     <i>pkg-fallout_at_FreeBSD.org (Fri, 01 Jul 2022 00:08:34 UTC)</i>
				//   </li>
				//
				// We're assuming this page is a message index, in the ascending order by the message date.

				var builder, origin, category, name, url string
				var ts time.Time
				var err error

				// extract builder and origin name from the "a" text
				m := builderAndOriginRe.FindAllStringSubmatch(e.ChildText("a"), -1)
				if len(m) == 0 {
					return // wrong "li", skip
				}
				builder = m[0][1]
				origin = m[0][2]
				if cn := strings.Split(origin, "/"); len(cn) == 2 {
					category, name = cn[0], cn[1]
				}

				if !(f.builderAllowed(builder) && f.originAllowed(origin) && f.categoryAllowed(category) && f.nameAllowed(name)) {
					return // did not pass the filter
				}

				// extract log timestamp from the "i" text
				m = timestampRe.FindAllStringSubmatch(e.ChildText("i"), -1)
				if len(m) == 0 {
					ech <- fmt.Errorf("no timestamp in message title: %s", e.ChildText("i"))
					return
				}
				ts, err = time.Parse(time.RFC1123, m[0][1])
				if err != nil {
					ech <- err
					return
				}
				if ts.UTC().Before(f.filter.After.UTC()) {
					return // timestamp is before "After", skip
				}

				// extract log page URL
				u := *e.Request.URL
				u.Path = path.Join(u.Path, e.ChildAttr("a", "href"))
				url = u.String()

				// stash partial result, Content will be filled later by the archive page handler
				if _, ok := resMap[url]; ok {
					ech <- fmt.Errorf("duplicate log: %s", url)
				} else {
					resMap[url] = &Result{
						Builder:   builder,
						Origin:    origin,
						Timestamp: ts.UTC(),
						URL:       url,
					}
				}
			}
		}
	})

	f.collector.OnHTML("pre", func(e *colly.HTMLElement) {
		select {
		case <-ctx.Done():
			return
		default:
			var currentUrl = e.Request.URL.String()
			if strings.HasSuffix(currentUrl, ".html") {
				// Log page, e.g. https://lists.freebsd.org/archives/freebsd-pkg-fallout/2022-July/240803.html
				// Example:
				//
				//   <!DOCTYPE html>
				//   <html>
				//      ...
				//   <body id="body">
				//      ...
				//      <pre class="main">You are receiving this mail as a port that you maintain
				//      ...

				// fill result Content
				if res, ok := resMap[currentUrl]; ok {
					res.Content = []byte([]byte(e.Text))
					rch <- res
				} else {
					ech <- fmt.Errorf("unexpected log: %s", currentUrl)
				}
			}
		}
	})

	f.collector.OnError(func(resp *colly.Response, err error) {
		ech <- err
	})

	f.collector.Visit(baseUrl)
}

func (f *Maillist) builderAllowed(builder string) bool {
	return valueAllowed(builder, f.filter.Builders, false)
}

func (f *Maillist) categoryAllowed(category string) bool {
	return valueAllowed(category, f.filter.Categories, false)
}

func (f *Maillist) originAllowed(origin string) bool {
	return valueAllowed(origin, f.filter.Origins, true)
}

func (f *Maillist) nameAllowed(name string) bool {
	return valueAllowed(name, f.filter.Names, false)
}

func valueAllowed(value string, filter []string, exact bool) bool {
	if len(filter) == 0 {
		return true
	}
	for _, s := range filter {
		if exact && value == s || !exact && strings.Contains(value, s) {
			return true
		}
	}
	return false
}
