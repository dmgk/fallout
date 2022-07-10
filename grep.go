package main

var grepCmd = cmd{
	Name:    "grep",
	Summary: "search fallout logs",
	run:     runGrep,
}

func runGrep(args []string) int {
	return 0
}
