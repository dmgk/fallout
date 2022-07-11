package main

import (
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/dmgk/fallout/cache"
	"github.com/dmgk/fallout/fetch"
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
	defaultDaysLimit = 7
	dateFormat       = "2006-01-02"
)

var (
	countLimit int
	dateLimit  = time.Now().UTC().AddDate(0, 0, -defaultDaysLimit)
)

func showFetchUsage() {
	err := fetchUsageTmpl.Execute(os.Stdout, map[string]any{
		"progname":   progname,
		"daysLimit":  defaultDaysLimit,
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
			dateLimit = time.Now().UTC().AddDate(0, 0, -v)
		case 'a':
			t, err := time.Parse(dateFormat, opt.String())
			if err != nil {
				errExit(err.Error())
			}
			if t.After(time.Now().UTC()) {
				errExit("date in the future: %s", t.Format(dateFormat))
			}
			dateLimit = t
		case 'n':
			v, err := opt.Int()
			if err != nil {
				errExit(err.Error())
			}
			countLimit = v
		}
	}

	c, err := cache.DefaultDirectory(progname)
	if err != nil {
		errExit("error initializing cache: %s", err)
	}

	qfn := func(res *fetch.Result) bool {
		e := c.Entry(res.Builder, res.Origin, res.Timestamp)
		if e.Exists() {
			fmt.Fprintf(os.Stdout, "%s (cached)\n", res)
			return true
		}
		return false
	}

	rfn := func(res *fetch.Result, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			return err
		}

		fmt.Fprintf(os.Stdout, "%s : %d bytes\n", res, len(res.Content))
		e := c.Entry(res.Builder, res.Origin, res.Timestamp)
		return e.Put(res.Content)
	}

	f := fetch.NewMaillist(fmt.Sprintf("%s/%s", progname, version))
	f.Fetch(&fetch.Options{
		After: dateLimit,
		Limit: countLimit,
	}, qfn, rfn)

	return 0
}
