package main

import (
	"fmt"
	"html/template"
	"io"
	"os"

	"github.com/dmgk/fallout/cache"
	"github.com/dmgk/fallout/format"
	"github.com/dmgk/fallout/grep"
	"github.com/dmgk/getopt"
	"github.com/mattn/go-isatty"
)

var grepUsageTmpl = template.Must(template.New("usage-grep").Parse(`
usage: {{.progname}} grep [-hx] [-A count] [-B count] [-C count] [-b builder[,builder]] [-c category[,category]] [-o origin[,origin]] [-n name[,name]] query [query ...]

Search cached fallout logs.

Options:
  -h          show help and exit
  -A count    show count lines of context after match
  -B count    show count lines of context before match
  -C count    show count lines of context around match
  -b builder  limit search only to this builder
  -c category limit search only to this category
  -o origin   limit search only to this origin
  -n name     limit search only to this port name
  -x          treat query as a regular expression
  -O          multiple queries are OR-ed (default: AND-ed)
  -l          print only matching log filenames
  -M          color mode [auto|never|always] (default: {{.colorMode}})
  -G colors   set colors (default: "{{.colors}}")
              the order is query,match,path,separator; see ls(1) for color codes
`[1:]))

var grepCmd = command{
	Name:    "grep",
	Summary: "search fallout logs",
	run:     runGrep,
}

var (
	contextAfter  int
	contextBefore int
	builders      []string
	categories    []string
	origins       []string
	names         []string
	queryIsRegexp bool
	ored          bool
	filenamesOnly bool
	colorMode     = colorModeAuto
	colors        = format.DefaultColors
)

const (
	colorModeAuto   = "auto"
	colorModeAlways = "always"
	colorModeNever  = "never"
)

func showGrepUsage() {
	err := grepUsageTmpl.Execute(os.Stdout, map[string]any{
		"progname":  progname,
		"colorMode": colorMode,
		"colors":    colors,
	})
	if err != nil {
		panic(fmt.Sprintf("error executing template %s: %v", grepUsageTmpl.Name(), err))
	}
}

func runGrep(args []string) int {
	if val, ok := os.LookupEnv("FALLOUT_COLORS"); ok && val != "" {
		colors = val
	}

	opts, err := getopt.NewArgv("hA:B:C:b:c:o:n:xOlM:G:", args)
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
		case 'b':
			builders = splitOptions(opt.String())
		case 'c':
			categories = splitOptions(opt.String())
		case 'o':
			origins = splitOptions(opt.String())
		case 'n':
			names = splitOptions(opt.String())
		case 'x':
			queryIsRegexp = true
		case 'O':
			ored = true
		case 'l':
			filenamesOnly = true
		case 'M':
			switch opt.String() {
			case colorModeAuto:
			case colorModeNever:
			case colorModeAlways:
				colorMode = opt.String()
			default:
				errExit("invalid color mode: %s", opt.String())
			}
		case 'G':
			colors = opt.String()
		}
	}

	c, err := cache.NewDefaultDirectory(progname)
	if err != nil {
		errExit("error initializing cache: %s", err)
	}

	f := &cache.Filter{
		Builders:   builders,
		Categories: categories,
		Origins:    origins,
		Names:      names,
	}
	w := c.Walker(f)
	g := grep.New(w)

	fm := initFormatter()

	o := &grep.Options{
		ContextAfter:  contextAfter,
		ContextBefore: contextBefore,
		QueryIsRegexp: queryIsRegexp,
	}
	gfn := func(entry cache.Entry, res []*grep.Match, err error) error {
		if err != nil {
			return err
		}
		return fm.Format(entry, res)
	}

	if err := g.Grep(o, opts.Args(), gfn); err != nil {
		errExit("grep error: %s", err)
		return 1
	}

	return 0
}

func initFormatter() format.Formatter {
	var w io.Writer = os.Stdout
	flags := format.Fdefaults
	term := isatty.IsTerminal(os.Stdout.Fd())

	if colorMode == colorModeAlways || (term && colorMode == colorModeAuto) {
		flags |= format.Fcolor
		if colors != "" {
			format.SetColors(colors)
		}
	}
	if filenamesOnly {
		flags |= format.FfilenamesOnly
	}

	return format.NewText(w, flags)
}
