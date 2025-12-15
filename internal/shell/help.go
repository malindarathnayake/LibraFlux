package shell

import (
	"fmt"
)

type helpEntry struct {
	cmd  string
	desc string
}

var helpRoot = []helpEntry{
	{"configure", "Enter configuration mode"},
	{"show", "Display running state and configuration"},
	{"doctor", "Run system diagnostics"},
	{"reload", "Reload configuration from disk"},
	{"lock", "Manage configuration lock"},
	{"exit", "Exit shell"},
	{"help", "Show this help"},
}

var helpConfig = []helpEntry{
	{"service <name>", "Add or modify a service"},
	{"delete <name>", "Delete a service"},
	{"commit", "Write changes to disk"},
	{"abort", "Discard uncommitted changes"},
	{"show", "Show pending changes"},
	{"exit", "Exit configuration mode"},
	{"help", "Show this help"},
}

var helpService = []helpEntry{
	{"protocol <tcp|udp>", "Set service protocol"},
	{"ports <p1,p2,...>", "Set discrete ports"},
	{"port-range <start-end>", "Add a port range"},
	{"scheduler <rr|wrr|sh>", "Set scheduler"},
	{"backend <ip> [weight]", "Add backend"},
	{"no backend <ip>", "Remove backend"},
	{"health tcp port <p> interval <ms> timeout <ms>", "Enable health check"},
	{"no health", "Disable health check"},
	{"show", "Show current service"},
	{"exit", "Exit to configure mode"},
	{"help", "Show this help"},
}

func PrintHelp(s *Shell, mode Mode) error {
	var entries []helpEntry
	switch mode {
	case ModeConfig:
		entries = helpConfig
	case ModeService:
		entries = helpService
	default:
		entries = helpRoot
	}

	fmt.Fprintln(s.out, "Commands")
	for _, e := range entries {
		fmt.Fprintf(s.out, "  %-18s %s\n", e.cmd, e.desc)
	}
	return nil
}

