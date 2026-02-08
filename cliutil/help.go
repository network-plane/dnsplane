package cliutil

import "strings"

var helpTokens = map[string]struct{}{
	"?":    {},
	"help": {},
	"h":    {},
}

// IsHelpToken reports whether the provided token is a recognised help alias.
func IsHelpToken(token string) bool {
	token = strings.TrimSpace(strings.ToLower(token))
	_, ok := helpTokens[token]
	return ok
}

// IsHelpRequest reports whether the first argument in args is a help alias.
func IsHelpRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return IsHelpToken(args[0])
}

// ContainsHelpToken reports whether any argument in args is a help alias (e.g. for "dns add ?" where args are ["add", "?"]).
func ContainsHelpToken(args []string) bool {
	for _, a := range args {
		if IsHelpToken(a) {
			return true
		}
	}
	return false
}
