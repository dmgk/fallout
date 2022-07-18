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
usage: {{.progname}} clean [-hx] [-D days] [-A date]

Clean log cache.

Options:
  -h          show help and exit
  -x          remove all cached data
  -D days     remove logs that are more than days old (default: {{.daysLimit}})
  -A date     remove logs that are older than date, in RFC-3339 format (default: {{.dateLimit.Format .dateFormat}})
`[1:]))

var cleanCmd = command{
	Name:    "clean",
	Summary: "clean log cache",
	run:     runClean,
}

const defaultCleanDaysLimit = 30

var (
	cleanDateLimit = time.Now().UTC().AddDate(0, 0, -defaultCleanDaysLimit)
	cleanAll       bool
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
	opts, err := getopt.NewArgv("hxD:A:", args)
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
		case 'x':
			cleanAll = true
		case 'D':
			v, err := opt.Int()
			if err != nil {
				errExit("-D: %s", err)
			}
			cleanDateLimit = time.Now().UTC().AddDate(0, 0, -v)
		case 'A':
			t, err := parseDateTime(opt.String())
			if err != nil {
				errExit("-A: %s", err)
			}
			cleanDateLimit = t
		default:
			panic("unhandled option: -" + string(opt.Opt))
		}
	}

	c, err := cache.NewDefaultDirectory(progname)
	if err != nil {
		errExit("error initializing cache: %s", err)
	}

	if cleanAll {
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
