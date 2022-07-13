package main

import (
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/dmgk/fallout/cache"
	"github.com/dmgk/getopt"
)

var cleanUsageTmpl = template.Must(template.New("usage-clean").Parse(`
usage: {{.progname}} clean [-hx] [-d days] [-a date]

Clean log cache.

Options:
  -h          show help and exit
  -d days     remove logs that are more than days old (default: {{.daysLimit}})
  -a date     remove logs that are older than date, in RFC-3339 format (default: {{.dateLimit.Format .dateFormat}})
  -x          remove all cached data
`[1:]))

var cleanCmd = command{
	Name:    "clean",
	Summary: "clean log cache",
	run:     runClean,
}

const defaultCleanDaysLimit = 30

var (
	cleanDateLimit = time.Now().UTC().AddDate(0, 0, -defaultCleanDaysLimit)
	allClean       bool
)

func showCleanUsage() {
	err := cleanUsageTmpl.Execute(os.Stdout, map[string]any{
		"progname":   progname,
		"daysLimit":  defaultCleanDaysLimit,
		"dateLimit":  cleanDateLimit,
		"dateFormat": dateFormat,
	})
	if err != nil {
		panic(fmt.Sprintf("error executing template %s: %v", cleanUsageTmpl.Name(), err))
	}
}

func runClean(args []string) int {
	opts, err := getopt.NewArgv("hd:a:x", args)
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
			showCleanUsage()
			os.Exit(0)
		case 'd':
			v, err := opt.Int()
			if err != nil {
				errExit(err.Error())
			}
			cleanDateLimit = time.Now().UTC().AddDate(0, 0, -v)
		case 'a':
			t, err := time.Parse(dateFormat, opt.String())
			if err != nil {
				errExit(err.Error())
			}
			if t.After(time.Now().UTC()) {
				errExit("date in the future: %s", t.Format(dateFormat))
			}
			cleanDateLimit = t
		case 'x':
			allClean = true
		}
	}

	c, err := cache.NewDefaultDirectory(progname)
	if err != nil {
		errExit("error initializing cache: %s", err)
	}

	if allClean {
		fmt.Printf("Removing %s\n", c.Path())
		if err := c.Remove(); err != nil {
			errExit("error removing cache: %s", err)
		}
		return 0
	}

	w := c.Walker(nil)
	err = w.Walk(func(entry cache.Entry, err error) error {
		if err != nil {
			return err
		}

		inf := entry.Info()
		if inf.Timestamp.Before(cleanDateLimit) {
			fmt.Printf("Removing %s\n", entry)
			entry.Remove()
		}

		return nil
	})
	if err != nil {
		errExit("error: %s", err)
	}

	return 0
}
