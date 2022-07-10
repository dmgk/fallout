package main

var fetchCmd = cmd{
	Name:    "fetch",
	Summary: "download fallout logs",
	run:     runFetch,
}

func runFetch(args []string) int {
	return 0
}
