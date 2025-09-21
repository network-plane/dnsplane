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
