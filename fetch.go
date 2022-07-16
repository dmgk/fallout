package main

import (
	"bufio"
	"fmt"
	"html/template"
	"os"
	"sync/atomic"
	"time"

	"github.com/dmgk/fallout/cache"
	"github.com/dmgk/fallout/fetch"
	"github.com/dmgk/getopt"
	"github.com/mattn/go-isatty"
)

var fetchUsageTmpl = template.Must(template.New("usage-fetch").Parse(`
usage: {{.progname}} fetch [-h] [-D days] [-A date] [-N count] [-b builder[,builder]] [-c category[,category]] [-o origin[,origin]] [-n name[,name]]

Download and cache fallout logs.

Options:
  -h              show help and exit
  -D days         download logs for the last days (default: {{.daysLimit}})
  -A date         download only logs after this date, in RFC-3339 format (default: {{.dateLimit.Format .dateFormat}})
  -N count        download only recent count logs
  -b builder,...  download only logs from these builders
  -c category,... download only logs for these categories
  -o origin,...   download only logs for these origins
  -n name,...     download only logs for these port names
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
	onlyNew         = true
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
	opts, err := getopt.NewArgv("hD:A:N:b:c:o:n:", argsWithDefaults(args, "FALLOUT_FETCH_OPTS"))
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
		case 'D':
			v, err := opt.Int()
			if err != nil {
				errExit(err.Error())
			}
			fetchDateLimit = time.Now().UTC().AddDate(0, 0, -v)
			onlyNew = false
		case 'A':
			t, err := time.Parse(dateFormat, opt.String())
			if err != nil {
				errExit(err.Error())
			}
			if t.After(time.Now().UTC()) {
				errExit("date in the future: %s", t.Format(dateFormat))
			}
			fetchDateLimit = t
			onlyNew = false
		case 'N':
			v, err := opt.Int()
			if err != nil {
				errExit(err.Error())
			}
			fetchCountLimit = v
		case 'b':
			builders = splitOptions(opt.String())
		case 'c':
			categories = splitOptions(opt.String())
		case 'o':
			origins = splitOptions(opt.String())
		case 'n':
			names = splitOptions(opt.String())
		default:
			panic("unhandled option: -" + string(opt.Opt))
		}
	}

	// read origins from stdin if it's not a tty
	// this allows easy feeding origins from e.g. portgrep: `portgrep -u go -1 | fallout fetch -D14`
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		sc := bufio.NewScanner(os.Stdin)
		sc.Split(bufio.ScanWords)
		for sc.Scan() {
			origins = append(origins, sc.Text())
		}
	}

	var count uint32
	c, err := cache.NewDefaultDirectory(progname)
	if err != nil {
		errExit("error initializing cache: %s", err)
	}

	f := fetch.NewMaillist(fmt.Sprintf("%s/%s", progname, version))
	fflt := &fetch.Filter{
		After:      fetchDateLimit,
		Limit:      fetchCountLimit,
		Builders:   builders,
		Categories: categories,
		Origins:    origins,
		Names:      names,
	}
	if onlyNew && !c.Timestamp().IsZero() {
		fflt.After = c.Timestamp()
	}

	qfn := func(res *fetch.Result) (bool, error) {
		e, err := c.Entry(res.Builder, res.Origin, res.Timestamp)
		if err != nil {
			return false, err
		}
		if e.Exists() {
			if !onlyNew {
				fmt.Fprintf(os.Stdout, "%s (cached)\n", res)
			}
			return true, nil
		}
		return false, nil
	}

	rfn := func(res *fetch.Result, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			return err
		}

		fmt.Fprintf(os.Stdout, "%s : %s\n", res, formatSize(int64(len(res.Content))))
		e, err := c.Entry(res.Builder, res.Origin, res.Timestamp)
		if err != nil {
			return err
		}
		if err := e.Write(res.Content); err != nil {
			return err
		}
		atomic.AddUint32(&count, 1)

		return nil
	}

	if err := f.Fetch(fflt, qfn, rfn); err != nil {
		errExit("fetch error: %s", err)
		return 1
	}
	if count > 0 {
		fmt.Printf("Downloaded %d new log(s).\n", count)
	} else {
		fmt.Println("No new logs.")
	}

	return 0
}
