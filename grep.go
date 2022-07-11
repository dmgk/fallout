package main

import (
	"fmt"
	"html/template"
	"os"
	"strings"

	"github.com/dmgk/fallout/cache"
	"github.com/dmgk/fallout/grep"
	"github.com/dmgk/getopt"
)

var grepUsageTmpl = template.Must(template.New("usage-grep").Parse(`
usage: {{.progname}} grep [-hx] [-A count] [-B count] [-C count] [-b builder[,builder]] [-c category[,category]] [-o origin[,origin]] [-n name[,name]] query [query ...]

Search cached fallout logs.

Options:
  -h          show help and exit
  -A count    show count lines of context after match
  -B count    show count lines of context before match
  -C count    show count lines of context around match
  -x          treat query as a regular expression
  -b builder  limit search only to this builder
  -c category limit search only to this category
  -o origin   limit search only to this origin
  -n name     limit search only to this port name
`[1:]))

var grepCmd = command{
	Name:    "grep",
	Summary: "search fallout logs",
	run:     runGrep,
}

var (
	contextAfter  int
	contextBefore int
	queryIsRegexp bool
	builders      []string
	categories    []string
	origins       []string
	names         []string
)

func showGrepUsage() {
	err := grepUsageTmpl.Execute(os.Stdout, map[string]any{
		"progname": progname,
	})
	if err != nil {
		panic(fmt.Sprintf("error executing template %s: %v", grepUsageTmpl.Name(), err))
	}
}

func runGrep(args []string) int {
	opts, err := getopt.NewArgv("hA:B:C:xb:c:o:n:", args)
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
			showGrepUsage()
			os.Exit(0)
		case 'A':
			v, err := opt.Int()
			if err != nil {
				errExit(err.Error())
			}
			contextAfter = v
		case 'B':
			v, err := opt.Int()
			if err != nil {
				errExit(err.Error())
			}
			contextBefore = v
		case 'C':
			v, err := opt.Int()
			if err != nil {
				errExit(err.Error())
			}
			contextBefore = v
			contextAfter = v
		case 'x':
			queryIsRegexp = true
		case 'b':
			builders = strings.Split(opt.String(), ",")
		case 'c':
			categories = strings.Split(opt.String(), ",")
		case 'o':
			origins = strings.Split(opt.String(), ",")
		case 'n':
			names = strings.Split(opt.String(), ",")
		}
	}

	c, err := cache.DefaultDirectory(progname)
	if err != nil {
		errExit("error initializing cache: %s", err)
	}

	f := &cache.Filter{
		Builders:   builders,
		Categories: categories,
		Origins:    origins,
		Names:      names,
	}
	o := &grep.Options{
		QueryIsRegexp: queryIsRegexp,
	}
	fn := func(path string, res []*grep.Result, err error) error {
		return nil
	}

	grep.Grep(c, f, o, fn)

	return 0
}
