package main

import (
	"fmt"
	"os"
	"strings"

	// "github.com/bettercap/readline"
	"github.com/chzyer/readline"
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

		// We don't want to save these commands in history
		excludedCommands := []string{"q", "quit", "exit", "?", "h", "help", "ls", "l", "/"}

		shouldSave := true
		for _, cmd := range excludedCommands {
			if command == cmd {
				shouldSave = false
				break
			}
		}

		if shouldSave {
			err := rl.SaveHistory(command) // Save command history
			if err != nil {
				fmt.Println("Error saving history:", err)
			}
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
	case "help", "h", "?", "ls", "l":
		mainHelp()
	default:
		fmt.Println("Unknown command:", args[0])
	}
}

// Handle subcommands based on the current context
func handleSubcommands(args []string, rl *readline.Instance, currentContext *string) {
	// Special Handlers
	switch args[0] {
	case "/":
		*currentContext = "" // Change context back to global
		setupAutocomplete(rl, *currentContext)
		return
	case "quit", "q", "exit":
		fmt.Println("Shutting down.")
		os.Exit(1)
	}

	switch *currentContext {
	case "record":
		handleRecord(args, *currentContext) // Process record subcommands
	case "cache":
		handleCache(args, *currentContext)
	case "dns":
		handleDNS(args, *currentContext)
	case "server":
		handleServer(args, *currentContext)
	default:
		fmt.Println("Unknown server subcommand:", args[0])
	}
}
