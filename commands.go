package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chzyer/readline"
)

// Map to exclude commands from saving to history
var excludedCommands = map[string]bool{
	"q": true, "quit": true, "exit": true, "h": true, "help": true, "ls": true, "l": true, "/": true,
}

// Handle the command loop for reading and processing user input
func handleCommandLoop(rl *readline.Instance) {
	var currentContext string
	setupAutocomplete(rl, currentContext)

	for {
		updatePrompt(rl, currentContext)
		command, err := rl.Readline()
		if err != nil {
			break
		}

		command = strings.TrimSpace(command)
		args := strings.Fields(command)
		if len(args) == 0 {
			continue
		}

		if isExitCommand(args[0]) {
			fmt.Println("Shutting down.")
			os.Exit(1)
		}

		if !excludedCommands[args[0]] {
			if err := rl.SaveHistory(command); err != nil {
				fmt.Println("Error saving history:", err)
			}
		}

		if currentContext == "" {
			handleGlobalCommands(args, rl, &currentContext)
		} else {
			handleSubcommands(args, rl, &currentContext)
		}
	}
}

// Check if the command is an exit command
func isExitCommand(cmd string) bool {
	return cmd == "q" || cmd == "quit" || cmd == "exit"
}

// Handle global commands
func handleGlobalCommands(args []string, rl *readline.Instance, currentContext *string) {
	switch args[0] {
	case "stats":
		handleStats()
	case "record", "cache", "dns":
		handleContextCommand(args[0], args, rl, currentContext)
	case "server":
		if len(args) > 1 {
			switch args[1] {
			case "start", "stop", "status", "configure":
				handleContextCommand(args[1], args, rl, currentContext)
			}
		}
		handleContextCommand(args[0], args, rl, currentContext)
	case "help", "h", "?", "ls", "l":
		mainHelp()
	default:
		fmt.Println("Unknown command:", args[0])
	}
}

// Handle context-based commands
func handleContextCommand(command string, args []string, rl *readline.Instance, currentContext *string) {
	if len(args) > 1 {
		handleSubcommand(command, args, *currentContext)
	} else {
		*currentContext = command
		setupAutocomplete(rl, *currentContext)
	}
}

// Handle subcommands based on the current context
func handleSubcommands(args []string, rl *readline.Instance, currentContext *string) {
	if args[0] == "/" {
		*currentContext = "" // Change context back to global
		setupAutocomplete(rl, *currentContext)
		return
	}

	switch *currentContext {
	case "record", "cache", "dns", "server":
		handleSubcommand(*currentContext, args, *currentContext)
	default:
		fmt.Println("Unknown subcommand:", args[0])
	}
}

// Dispatch subcommands to the appropriate handlers
func handleSubcommand(command string, args []string, context string) {
	switch command {
	case "record":
		handleRecord(args, context)
	case "cache":
		handleCache(args, context)
	case "dns":
		handleDNS(args, context)
	case "server":
		handleServer(args, context)
	case "start":
		handleServerStart(args, context)
	case "stop":
		handleServerStop(args, context)
	case "status":
		handleServerStatus(args, context)
	case "configure":
		handleServerConfigure(args, context)
	default:
		fmt.Println("Unknown subcommand:", args[0])
	}
}
