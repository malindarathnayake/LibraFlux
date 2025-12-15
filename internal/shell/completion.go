package shell

import (
	"sort"
	"strings"
)

func (s *Shell) Complete(line string) []string {
	line = strings.TrimLeft(line, " ")
	if line == "" {
		return s.completeForMode([]string{}, true)
	}

	hasTrailingSpace := strings.HasSuffix(line, " ")
	tokens := strings.Fields(line)
	return s.completeForMode(tokens, hasTrailingSpace)
}

func (s *Shell) completeForMode(tokens []string, hasTrailingSpace bool) []string {
	var words []string
	switch s.mode {
	case ModeConfig:
		words = []string{"service", "delete", "commit", "abort", "show", "exit", "help", "?"}
	case ModeService:
		words = []string{"protocol", "ports", "port-range", "scheduler", "backend", "no", "health", "show", "exit", "help", "?"}
	default:
		words = []string{"configure", "show", "doctor", "reload", "lock", "exit", "help", "?"}
	}

	prefix := ""
	if len(tokens) > 0 && !hasTrailingSpace {
		prefix = tokens[len(tokens)-1]
	}

	var out []string
	for _, w := range words {
		if prefix == "" || strings.HasPrefix(w, prefix) {
			out = append(out, w)
		}
	}
	sort.Strings(out)
	return out
}

