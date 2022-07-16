package main

import (
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/dmgk/fallout/cache"
	"github.com/dmgk/getopt"
)

var statsUsageTmpl = template.Must(template.New("usage-stats").Parse(`
usage: {{.progname}} stats [-h]

Show cached logs statistics.

Options:
  -h              show help and exit
`[1:]))

var statsCmd = command{
	Name:    "stats",
	Summary: "show cache statistics",
	run:     runStats,
}

func showStatsUsage() {
	err := statsUsageTmpl.Execute(os.Stdout, map[string]any{})
	if err != nil {
		panic(fmt.Sprintf("error executing template %s: %v", statsUsageTmpl.Name(), err))
	}
}

var statsTmpl = template.Must(template.New("stats-output").Parse(`
Cache size:         {{.logsSize}}
Latest log:         {{.latestTimestamp}}
Earliest log:       {{.earliestTimestamp}}
Builders:           {{.buildersCount}}
Failing ports:      {{.originsCount}}
Logs:               {{.logsCount}}
Most failures:      {{.topBuilderName}} ({{.topBuilderCount}} ports)
`[1:]))

func runStats(args []string) int {
	opts, err := getopt.NewArgv("h", args)
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
			showStatsUsage()
			os.Exit(0)
		}
	}

	const tsFormat = "2006-01-02 15:04:05 UTC"
	var (
		latestTs        time.Time
		earliestTs      time.Time
		buildersMap     = map[string]map[string]struct{}{}
		originsSet      = map[string]struct{}{}
		logTotalCount   int
		logTotalSize    int64
		topBuilderName  string
		topBuilderCount int
	)

	c, err := cache.NewDefaultDirectory(progname)
	if err != nil {
		errExit("error initializing cache: %s", err)
	}

	w := c.Walker(nil)
	err = w.Walk(func(entry cache.Entry, err error) error {
		if err != nil {
			return err
		}

		inf := entry.Info()

		if inf.Timestamp.Before(earliestTs) || earliestTs.IsZero() {
			earliestTs = inf.Timestamp
		}
		if inf.Timestamp.After(latestTs) {
			latestTs = inf.Timestamp
		}
		if _, ok := buildersMap[inf.Builder]; !ok {
			buildersMap[inf.Builder] = map[string]struct{}{}
		}
		buildersMap[inf.Builder][inf.Origin] = struct{}{}
		originsSet[inf.Origin] = struct{}{}
		logTotalCount += 1

		f, err := os.Open(entry.Path())
		if err != nil {
			return err
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		logTotalSize += fi.Size()

		return nil
	})
	if err != nil {
		errExit("error: %s", err)
	}

	if logTotalCount == 0 {
		fmt.Println("No logs in cache.")
		return 0
	}

	for n, bo := range buildersMap {
		if len(bo) > topBuilderCount {
			topBuilderName = n
			topBuilderCount = len(bo)
		}
	}

	err = statsTmpl.Execute(os.Stdout, map[string]any{
		"latestTimestamp":   latestTs.Format(tsFormat),
		"earliestTimestamp": earliestTs.Format(tsFormat),
		"buildersCount":     len(buildersMap),
		"topBuilderName":    topBuilderName,
		"topBuilderCount":   topBuilderCount,
		"originsCount":      len(originsSet),
		"logsCount":         logTotalCount,
		"logsSize":          formatSize(logTotalSize),
	})
	if err != nil {
		errExit("error: %s", err)
	}

	return 0
}
