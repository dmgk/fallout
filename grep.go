package main

var grepCmd = command{
	Name:    "grep",
	Summary: "search fallout logs",
	run:     runGrep,
}

func runGrep(args []string) int {
	return 0
}
