package fetch

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

const baseUrl = "https://lists.freebsd.org/archives/freebsd-pkg-fallout/"

var (
	builderAndOriginRe = regexp.MustCompile(`\[.+ - (.+)\]\[(.+)\].*`)
	dateRe             = regexp.MustCompile(`\((.*)\)`)
)

type Fetcher struct {
	userAgent string
}

func New(progname, version string) *Fetcher {
	return &Fetcher{
		userAgent: fmt.Sprintf("%s/%s", progname, version),
	}
}

type result struct {
	l   *log
	err error
}

type log struct {
	builder string
	origin  string
	date    time.Time
	text    string
}

func (f *Fetcher) Fetch(ch chan *result, limit int) {
	var (
		mu     sync.Mutex
		logMap = map[string]*log{}
		count  int
		stop   bool
	)

	c := colly.NewCollector(
		colly.UserAgent(f.userAgent),
	)

	c.OnHTML("tr td:nth-of-type(1) a", func(e *colly.HTMLElement) {
		// pkg-fallout archive page: https://lists.freebsd.org/archives/freebsd-pkg-fallout/
		// example entry:
		// <tr>
		//     <td class="ml"><a href="2022-July/">July 2022</a></td>
		//     ...
		// </tr>

		// fmt.Println(path.Join(baseUrl, e.Attr("href")))
		if !stop {
			e.Request.Visit(e.Attr("href"))
		}
	})

	c.OnHTML("li", func(e *colly.HTMLElement) {
		var currentUrl = e.Request.URL.String()
		if !stop && !strings.HasSuffix(currentUrl, ".html") {
			// monthly index page: https://lists.freebsd.org/archives/freebsd-pkg-fallout/2022-July/
			// example entry:
			// <li><a href="240803.html">[package - 130arm64-quarterly][lang/polyml] Failed for polyml-5.9 in build</a>: <i>pkg-fallout_at_FreeBSD.org (Fri, 01 Jul 2022 00:08:34 UTC)</i></li>

			var (
				builder, origin, logUrl string
				date                    time.Time
				err                     error
			)

			// extract builder and origin name from "a" text
			m := builderAndOriginRe.FindAllStringSubmatch(e.ChildText("a"), -1)
			if len(m) == 0 {
				return // wrong "li", skip
			}
			builder = m[0][1]
			origin = m[0][2]

			// extract fallout log URL
			u := *e.Request.URL
			u.Path = path.Join(u.Path, e.ChildAttr("a", "href"))
			logUrl = u.String()

			// extract log date from "i" text
			m = dateRe.FindAllStringSubmatch(e.ChildText("i"), -1)
			if len(m) == 0 {
				return // skip
			}
			date, err = time.Parse(time.RFC1123, m[0][1])
			if err != nil {
				ch <- &result{err: err}
			}

			mu.Lock()
			if _, ok := logMap[logUrl]; ok {
				ch <- &result{err: fmt.Errorf("duplicate log: %s", logUrl)}
			} else {
				logMap[logUrl] = &falloutLog{
					builder: builder,
					origin:  origin,
					date:    date,
				}
			}
			mu.Unlock()

			e.Request.Visit(logUrl)
		}
	})

	c.OnHTML("pre", func(e *colly.HTMLElement) {
		var currentUrl = e.Request.URL.String()
		if !stop && strings.HasSuffix(currentUrl, ".html") {
			// fallout log page
			// example HTML:
			// <!DOCTYPE html>
			// <html>
			//      ...
			// <body id="body">
			//      ...
			//      <pre class="main">You are receiving this mail as a port that you maintain
			//      ...

			mu.Lock()
			if l, ok := logMap[currentUrl]; ok {
				l.text = e.Text
				ch <- &result{l: l}
				delete(logMap, currentUrl)
				count++
				if count >= limit {
					stop = true
				}
			} else {
				ch <- &result{err: fmt.Errorf("unexpected log: %s", currentUrl)}
			}
			mu.Unlock()
		}
	})

	c.OnError(func(resp *colly.Response, err error) {
		ch <- &result{err: err}
	})

	c.Visit(baseUrl)
	close(ch)
}
