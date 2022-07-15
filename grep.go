package main

import (
	"bufio"
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
usage: {{.progname}} grep [-hxOl] [-A count] [-B count] [-C count] [-b builder[,builder]] [-c category[,category]] [-o origin[,origin]] [-n name[,name]] query [query ...]

Search cached fallout logs.

Options:
  -h              show help and exit
  -F              interpret query as a plain text, not regular expression
  -O              multiple queries are OR-ed (default: AND-ed)
  -l              print only matching log filenames
  -A count        show count lines of context after match
  -B count        show count lines of context before match
  -C count        show count lines of context around match
  -b builder,...  limit search only to these builders
  -c category,... limit search only to these categories
  -o origin,...   limit search only to these origins
  -n name,...     limit search only to these port names
`[1:]))

var grepCmd = command{
	Name:    "grep",
	Summary: "search fallout logs",
	run:     runGrep,
}

var (
	queryIsRegexp = true
	ored          bool
	filenamesOnly bool
	contextAfter  int
	contextBefore int
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
	opts, err := getopt.NewArgv("hFOlA:B:C:b:c:o:n:", argsWithDefaults(args, "FALLOUT_GREP_OPTS"))
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
		case 'F':
			queryIsRegexp = false
		case 'O':
			ored = true
		case 'l':
			filenamesOnly = true
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
		default:
			panic("unhandled option: -" + string(opt.Opt))
		}
	}

	// read origins from stdin if it's not a tty
	// this allows easy feeding origins from e.g. portgrep: `portgrep -u go -1 | fallout grep -C2 error:`
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		sc := bufio.NewScanner(os.Stdin)
		sc.Split(bufio.ScanWords)
		for sc.Scan() {
			origins = append(origins, sc.Text())
		}
	}

	c, err := cache.NewDefaultDirectory(progname)
	if err != nil {
		errExit("error initializing cache: %s", err)
	}

	cflt := &cache.Filter{
		Builders:   builders,
		Categories: categories,
		Origins:    origins,
		Names:      names,
	}
	w := c.Walker(cflt)
	g := grep.New(w)

	fm := initFormatter()

	gopt := &grep.Options{
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

	if err := g.Grep(gopt, opts.Args(), gfn); err != nil {
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
