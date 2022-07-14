package main

import (
	"fmt"
	"html/template"
	"os"
	"strings"
	"unicode"

	"github.com/dmgk/fallout/format"
	"github.com/dmgk/getopt"
)

const (
	dateFormat = "2006-01-02"
)

var usageTmpl = template.Must(template.New("usage").Parse(`
usage: {{.progname}} [-hV] [-M mode] [-G colors] command [options]

Download and search fallout logs.

Options:
  -h              show help and exit
  -V              show version and exit
  -M mode         color mode [auto|never|always] (default: {{.colorMode}})
  -G colors       set colors (default: "{{.colors}}")
                  the order is query,match,path,separator; see ls(1) for color codes

Commands (pass -h for command help):{{range .cmds}}
  {{.Name | printf "%-15s"}} {{.Summary}}{{end}}
`[1:]))

var (
	progname   string
	version    = "devel"
	colorMode  = colorModeAuto
	colors     = format.DefaultColors
	builders   []string
	categories []string
	origins    []string
	names      []string
)

const (
	colorModeAuto   = "auto"
	colorModeAlways = "always"
	colorModeNever  = "never"
)

func showUsage() {
	err := usageTmpl.Execute(os.Stdout, map[string]any{
		"progname":  progname,
		"colorMode": colorMode,
		"colors":    colors,
		"cmds":      cmds,
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

type command struct {
	Name    string
	Summary string
	run     func(args []string) int
}

var cmds = []*command{
	&fetchCmd,
	&grepCmd,
	&cleanCmd,
}

func main() {
	if v, ok := os.LookupEnv("FALLOUT_COLORS"); ok && v != "" {
		colors = v
	}

	opts, err := getopt.NewArgv("hVM:G:", argsWithDefaults(os.Args, "FALLOUT_OPTS"))
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
		case 'V':
			showVersion()
			os.Exit(0)
		case 'M':
			switch opt.String() {
			case colorModeAuto, colorModeNever, colorModeAlways:
				colorMode = opt.String()
			default:
				errExit("invalid color mode: %s", opt.String())
			}
		case 'G':
			colors = opt.String()
		default:
			panic("unhandled option: -" + string(opt.Opt))
		}
	}

	args := opts.Args()
	if len(args) == 0 {
		showUsage()
		os.Exit(1)
	}

	var cmd *command
	for _, c := range cmds {
		if c.Name == args[0] {
			cmd = c
			break
		}
	}
	if cmd == nil {
		showUsage()
		os.Exit(1)
	}

	os.Exit(cmd.run(args))
}

func argsWithDefaults(argv []string, env string) []string {
	args := argv[1:]
	if v, ok := os.LookupEnv(env); ok && v != "" {
		args = append(splitOptions(v), args...)
	}
	return append([]string{argv[0]}, args...)
}

func splitOptions(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == ','
	})
}
