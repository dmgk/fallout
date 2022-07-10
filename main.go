package main

import (
	"fmt"
	"html/template"
	"os"

	"github.com/dmgk/getopt"
)

type cmd struct {
	Name    string
	Summary string
	run     func(args []string) int
}

var cmds = []cmd{
	fetchCmd,
	grepCmd,
}

var usageTmpl = template.Must(template.New("usage").Parse(`
Usage: {{.progname}} [-hv] command [options]

Download and search fallout logs.

Commands (pass -h for command help) :{{range .cmds}}
  {{.Name | printf "%-8s"}} {{.Summary}}{{end}}
`[1:]))

var (
	progname string
	version  = "devel"
)

func showUsage() {
	err := usageTmpl.Execute(os.Stdout, map[string]any{
		"progname": progname,
		"cmds":     cmds,
	})
	if err != nil {
		panic(fmt.Sprintf("error executing template %s: %v", usageTmpl.Name(), err))
	}
}

func showVersion() {
	fmt.Printf("%s %s\n", progname, version)
}

func errExit(format string, v ...any) {
	fmt.Fprint(os.Stderr, progname, ": ")
	fmt.Fprintf(os.Stderr, format, v...)
	fmt.Fprintln(os.Stderr)
	os.Exit(1)
}

func main() {
	opts, err := getopt.New("hv")
	if err != nil {
		panic(fmt.Sprintf("error creating options parser: %s", err))
	}
	progname = opts.ProgramName()

	for opts.Scan() {
		opt, err := opts.Option()
		if err != nil {
			errExit(err.Error())
		}

		switch opt.Opt {
		case 'h':
			showUsage()
			os.Exit(0)
		case 'v':
			showVersion()
			os.Exit(0)
		default:
			panic("unhandled option: -" + string(opt.Opt))
		}
	}

	args := opts.Args()

	if len(args) == 0 {
		showUsage()
		os.Exit(1)
	}
}