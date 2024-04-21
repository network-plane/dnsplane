package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/bettercap/readline"
)

// Handle the command loop for reading and processing user input
func handleCommandLoop(rl *readline.Instance) {
	var currentContext string
	setupAutocomplete(rl, currentContext)

	for {
		updatePrompt(rl, currentContext) // Update the prompt based on context
		command, err := rl.Readline()
		if err != nil { // Handle EOF or interrupt
			break
		}

		command = strings.TrimSpace(command)
		args := strings.Fields(command)

		if len(args) == 0 {
			continue
		}

		if currentContext == "" {
			handleGlobalCommands(args, rl, &currentContext) // Handle global commands
		} else {
			handleSubcommands(args, rl, &currentContext) // Handle subcommands based on the context
		}
	}
}

// Handle global commands
func handleGlobalCommands(args []string, rl *readline.Instance, currentContext *string) {
	switch args[0] {
	case "stats":
		handleStats()
	case "record":
		if len(args) > 1 {
			handleRecord(args, *currentContext)
		} else {
			*currentContext = "record"
			setupAutocomplete(rl, *currentContext)
		}
	case "cache":
		if len(args) > 1 {
			handleCache(args, *currentContext)
		} else {
			*currentContext = "cache"
			setupAutocomplete(rl, *currentContext)
		}
	case "dns":
		if len(args) > 1 {
			handleDNS(args, *currentContext)
		} else {
			*currentContext = "dns"
			setupAutocomplete(rl, *currentContext)
		}
	case "server":
		if len(args) > 1 {
			handleServer(args, *currentContext)
		} else {
			*currentContext = "server"
			setupAutocomplete(rl, *currentContext)
		}
	case "exit", "quit", "q":
		fmt.Println("Shutting down.")
		os.Exit(1)
	case "help", "h", "?":
		mainHelp()
	default:
		fmt.Println("Unknown command:", args[0])
	}
}

// Handle subcommands based on the current context
func handleSubcommands(args []string, rl *readline.Instance, currentContext *string) {
	switch *currentContext {
	case "record":
		if args[0] == "/" {
			*currentContext = ""
			setupAutocomplete(rl, *currentContext) // Change context back to global
		} else {
			handleRecord(args, *currentContext) // Process record subcommands
		}
	case "cache":
		if args[0] == "/" {
			*currentContext = ""
			setupAutocomplete(rl, *currentContext) // Change context back to global
		} else {
			handleCache(args, *currentContext)
		}
	case "dns":
		if args[0] == "/" {
			*currentContext = ""
			setupAutocomplete(rl, *currentContext) // Change context back to global
		} else {
			handleDNS(args, *currentContext)
		}
	case "server":
		if args[0] == "/" {
			*currentContext = ""
			setupAutocomplete(rl, *currentContext) // Change context back to global
		} else {
			handleServer(args, *currentContext)
		}
	default:
		fmt.Println("Unknown server subcommand:", args[0])
	}
}
