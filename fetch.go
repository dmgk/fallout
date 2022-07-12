package main

import (
	"fmt"
	"html/template"
	"os"
	"sync/atomic"
	"time"

	"github.com/dmgk/fallout/cache"
	"github.com/dmgk/fallout/fetch"
	"github.com/dmgk/getopt"
)

var fetchUsageTmpl = template.Must(template.New("usage-fetch").Parse(`
usage: {{.progname}} fetch [-h] [-d days] [-a date] [-n count]

Download and cache fallout logs.

Options:
  -h          show help and exit
  -d days     download logs for the last days (default: {{.daysLimit}})
  -a date     download only logs after this date, in RFC-3339 format (default: {{.dateLimit.Format .dateFormat}})
  -n count    download only recent count logs
`[1:]))

var fetchCmd = command{
	Name:    "fetch",
	Summary: "download fallout logs",
	run:     runFetch,
}

const defaultFetchDaysLimit = 7

var (
	fetchCountLimit int
	fetchDateLimit  = time.Now().UTC().AddDate(0, 0, -defaultFetchDaysLimit)
)

func showFetchUsage() {
	err := fetchUsageTmpl.Execute(os.Stdout, map[string]any{
		"progname":   progname,
		"daysLimit":  defaultFetchDaysLimit,
		"dateLimit":  fetchDateLimit,
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
			fetchDateLimit = time.Now().UTC().AddDate(0, 0, -v)
		case 'a':
			t, err := time.Parse(dateFormat, opt.String())
			if err != nil {
				errExit(err.Error())
			}
			if t.After(time.Now().UTC()) {
				errExit("date in the future: %s", t.Format(dateFormat))
			}
			fetchDateLimit = t
		case 'n':
			v, err := opt.Int()
			if err != nil {
				errExit(err.Error())
			}
			fetchCountLimit = v
		}
	}

	var totalCount, newCount uint32
	c, err := cache.NewDefaultDirectory(progname)
	if err != nil {
		errExit("error initializing cache: %s", err)
	}

	f := fetch.NewMaillist(fmt.Sprintf("%s/%s", progname, version))
	o := &fetch.Options{
		After: fetchDateLimit,
		Limit: fetchCountLimit,
	}

	qfn := func(res *fetch.Result) (bool, error) {
		e, err := c.Cache(res.Builder, res.Origin, res.Timestamp)
		if err != nil {
			return false, err
		}
		atomic.AddUint32(&totalCount, 1)
		if e.Exists() {
			fmt.Fprintf(os.Stdout, "%s (cached)\n", res)
			return true, nil
		}
		return false, nil
	}

	rfn := func(res *fetch.Result, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			return err
		}

		fmt.Fprintf(os.Stdout, "%s : %d bytes\n", res, len(res.Content))
		e, err := c.Cache(res.Builder, res.Origin, res.Timestamp)
		if err != nil {
			return err
		}
		if err := e.Put(res.Content); err != nil {
			return err
		}
		atomic.AddUint32(&newCount, 1)

		return nil
	}

	if err := f.Fetch(o, qfn, rfn); err != nil {
		errExit("fetch error: %s", err)
		return 1
	}
	fmt.Printf("Processed %d logs, %d new.\n", totalCount, newCount)

	return 0
}
