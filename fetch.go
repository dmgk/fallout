package main

import (
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/dmgk/getopt"
)

var fetchUsageTmpl = template.Must(template.New("usage").Parse(`
Usage: {{.progname}} fetch [-h] [-d DAYS] [-a DATE] [-n N]

Download and cache fallout logs.

Options:
  -h       show help and exit
  -d DAYS  download logs for the last DAYS days (default: {{.daysLimit}})
  -a DATE  download only logs after this DATE, in RFC-3339 format (default: {{.dateLimit.Format .dateFormat}})
  -n N     download only recent N logs
`[1:]))

var fetchCmd = command{
	Name:    "fetch",
	Summary: "download fallout logs",
	run:     runFetch,
}

const (
	defaultDaysLimit = 30
	dateFormat       = "2006-01-02"
)

var (
	countLimit int
	daysLimit  = defaultDaysLimit
	dateLimit  = time.Now().UTC().AddDate(0, 0, -daysLimit)
)

func showFetchUsage() {
	err := fetchUsageTmpl.Execute(os.Stdout, map[string]any{
		"progname":   progname,
		"daysLimit":  daysLimit,
		"dateLimit":  dateLimit,
		"dateFormat": dateFormat,
	})
	if err != nil {
		panic(fmt.Sprintf("error executing template %s: %v", fetchUsageTmpl.Name(), err))
	}
}

func runFetch(args []string) int {
	opts, err := getopt.NewArgv("hd:a:n:", args)
	if err != nil {
		panic(fmt.Sprintf("error creating options parser: %s", err))
	}

	for opts.Scan() {
		opt, err := opts.Option()
		if err != nil {
			errExit(err.Error())
		}

		switch opt.Opt {
		case 'h':
			showFetchUsage()
			os.Exit(0)
		case 'd':
			v, err := opt.Int()
			if err != nil {
				errExit(err.Error())
			}
			daysLimit = v
			dateLimit = time.Now().UTC().AddDate(0, 0, -daysLimit)
		case 'a':
			t, err := time.Parse(dateFormat, opt.String())
			if err != nil {
				errExit(err.Error())
			}
			if t.After(time.Now().UTC()) {
				errExit("date in the future: %s", t.Format(dateFormat))
			}
			dateLimit = t
			daysLimit = int(time.Now().UTC().Sub(dateLimit)/(24*time.Hour) + 1)
		case 'n':
			v, err := opt.Int()
			if err != nil {
				errExit(err.Error())
			}
			countLimit = v
		}
	}

	ch := make(chan *scrapeRes)

	go scrape(ch, 10)

	for r := range ch {
		if r.err != nil {
			fmt.Printf("!!!!!> err %#v\n", r.err)
			continue
		}
		fmt.Printf("====> fl {builder: %s, origin: %s, date: %s, text: %d}\n",
			r.fl.builder, r.fl.origin, r.fl.date, len(r.fl.text))
	}

	return 0
}
