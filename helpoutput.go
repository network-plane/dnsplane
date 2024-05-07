package main

import "fmt"

// helpPrinter prints help for commands, optionally including subcommands.
func helpPrinter(commands map[string]cmdHelp, includeSubCommands bool, isSubCmd bool) {
	for _, cmd := range commands {
		if !isSubCmd {
			fmt.Printf("%-15s %s\n", cmd.Name, cmd.Description)
		}

		if includeSubCommands && len(cmd.SubCommands) > 0 {
			for _, subCmd := range cmd.SubCommands {
				fmt.Printf("  %-15s %s\n", subCmd.Name, subCmd.Description)
			}
		}
	}
}

// commonHelp prints common help commands.
func commonHelp(indent bool) {
	indentation := ""
	if indent {
		indentation = "  "
	}
	fmt.Printf("%s%-15s %s\n", indentation, "/", "- Go up one level")
	fmt.Printf("%s%-15s %s\n", indentation, "exit, quit, q", "- Shutdown the server")
	fmt.Printf("%s%-15s %s\n", indentation, "help, h, ?", "- Show help")
}

// mainHelp displays the available commands without subcommands.
func mainHelp() {
	fmt.Println("Available commands:")
	helpPrinter(loadCommands(), false, false)
	commonHelp(false) // Add common help commands
}

// checkHelp determines if the argument is for help.
func checkHelp(arg, currentSub string) bool {
	checkArgs := []string{"?", "help", "h", "ls", "l"}

	for _, cmd := range checkArgs {
		if arg == cmd {
			subCommandHelp(currentSub)
			return false
		}
	}

	return true
}

// subCommandHelp prints help for a specific context or for all contexts if none is provided.
func subCommandHelp(context string) {
	commands := loadCommands()

	if context == "" {
		// Print only top-level commands if context is empty
		helpPrinter(commands, false, false)
		commonHelp(false)
	} else if cmd, exists := commands[context]; exists {
		// Print only subcommands if the context is found
		helpPrinter(map[string]cmdHelp{context: cmd}, true, true)
		commonHelp(true)
	} else {
		fmt.Println("Unknown context:", context)
		commonHelp(false)
	}
}

// loadCommands returns a map with command information.
func loadCommands() map[string]cmdHelp {
	return map[string]cmdHelp{
		"record": {
			Name:        "record",
			Description: "- Record Management",
			SubCommands: map[string]cmdHelp{
				"add":    {"add", "- Add a new DNS record", nil},
				"remove": {"remove", "- Remove a DNS record", nil},
				"update": {"update", "- Update a DNS record", nil},
				"list":   {"list", "- List all DNS records", nil},
				"clear":  {"clear", "- Clear all DNS records", nil},
				"test":   {"test", "- Test a DNS record", nil},
				"load":   {"load", "- Load DNS records from a file", nil},
				"save":   {"save", "- Save DNS records to a file", nil},
			},
		},
		"cache": {
			Name:        "cache",
			Description: "- Cache Management",
			SubCommands: map[string]cmdHelp{
				"clear": {"clear", "- Clear the cache", nil},
				"list":  {"list", "- List all cache entries", nil},
			},
		},
		"dns": {
			Name:        "dns",
			Description: "- DNS Server Management",
			SubCommands: map[string]cmdHelp{
				"add":    {"add", "- Add a new DNS server", nil},
				"remove": {"remove", "- Remove a DNS server", nil},
				"update": {"update", "- Update a DNS server", nil},
				"list":   {"list", "- List all DNS servers", nil},
				"clear":  {"clear", "- Clear all DNS servers", nil},
				"test":   {"test", "- Test a DNS server", nil},
				"load":   {"load", "- Load DNS servers from a file", nil},
				"save":   {"save", "- Save DNS servers to a file", nil},
			},
		},
		"server": {
			Name:        "server",
			Description: "- Server Management",
			SubCommands: map[string]cmdHelp{
				"start":    {"start", "- Start the server", nil},
				"stop":     {"stop", "- Stop the server", nil},
				"status":   {"status", "- Show server status", nil},
				"fallback": {"fallback", "- Set/List the fallback server", nil},
				"timeout":  {"timeout", "- Set/List the server timeout", nil},
				"save":     {"save", "- Save the current settings", nil},
				"load":     {"load", "- Load the settings from the files", nil},
			},
		},
	}
}
