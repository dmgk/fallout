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
	c *colly.Collector
}

func NewMaillist(userAgent string) Fetcher {
	c := colly.NewCollector(
		colly.UserAgent(userAgent),
	)

	return &Maillist{c: c}
}

func (f *Maillist) Fetch(options *Options, qfn QueryFunc, rfn ResultFunc) error {
	rch := make(chan *Result)
	ech := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go f.fetchMaillist(ctx, options, qfn, rch, ech)

	rok := true
	for rok {
		var res *Result
		select {
		case res, rok = <-rch:
			if rok {
				if rerr := rfn(res, nil); rerr != nil {
					if rerr == ErrStop {
						return nil
					}
					return rerr
				}
			}
		case err, eok := <-ech:
			if eok {
				if rerr := rfn(nil, err); rerr != nil {
					if rerr == ErrStop {
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
func (f *Maillist) fetchMaillist(ctx context.Context, options *Options, qfn QueryFunc, rch chan *Result, ech chan error) {
	var (
		resMap = map[string]*Result{}
		count  int
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(rch)
	defer close(ech)

	f.c.OnHTML("tr td:nth-of-type(1) a", func(e *colly.HTMLElement) {
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
			ma := options.After.UTC().Year()*100 + int(options.After.UTC().Month())
			if mi < ma {
				cancel() // link is to the month before "After", stop
				return
			}

			// extract month page URL
			u := *e.Request.URL
			u.Path = path.Join(u.Path, e.Attr("href"))

			// visit month page and collect fallout log links
			f.c.Visit(u.String())

			// process collected partial results
			var sres []*Result
			for _, r := range resMap {
				sres = append(sres, r)
			}
			sort.Slice(sres, func(i, j int) bool {
				// by descending Timestamp
				return sres[i].Timestamp.After(sres[j].Timestamp)
			})
			for _, r := range sres {
				if options.Limit > 0 && count > options.Limit {
					break // limit is reached
				}
				// fetch fallout log, unless it was already cached
				if !qfn(r) {
					f.c.Visit(r.URL)
				}
				count++
			}
		}
	})

	f.c.OnHTML("li", func(e *colly.HTMLElement) {
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

				var res Result

				// extract builder and origin name from the "a" text
				m := builderAndOriginRe.FindAllStringSubmatch(e.ChildText("a"), -1)
				if len(m) == 0 {
					return // wrong "li", skip
				}
				res.Builder = m[0][1]
				res.Origin = m[0][2]

				// extract log timestamp from the "i" text
				m = timestampRe.FindAllStringSubmatch(e.ChildText("i"), -1)
				if len(m) == 0 {
					ech <- fmt.Errorf("no timestamp in message title: %s", e.ChildText("i"))
					return
				}
				ts, err := time.Parse(time.RFC1123, m[0][1])
				if err != nil {
					ech <- err
					return
				}
				if ts.UTC().Before(options.After.UTC()) {
					return // timestamp is before "After", skip
				}
				res.Timestamp = ts

				// extract log page URL
				u := *e.Request.URL
				u.Path = path.Join(u.Path, e.ChildAttr("a", "href"))
				res.URL = u.String()

				// stash partial result, Content will be filled later by the archive page handler
				if _, ok := resMap[res.URL]; ok {
					ech <- fmt.Errorf("duplicate log: %s", res.URL)
				} else {
					resMap[res.URL] = &res
				}
			}
		}
	})

	f.c.OnHTML("pre", func(e *colly.HTMLElement) {
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
					res.Content = e.Text
					rch <- res
				} else {
					ech <- fmt.Errorf("unexpected log: %s", currentUrl)
				}
			}
		}
	})

	f.c.OnError(func(resp *colly.Response, err error) {
		ech <- err
	})

	f.c.Visit(baseUrl)
}
