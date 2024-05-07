package main

import "fmt"

// helpPrinter prints help for commands.
func helpPrinter(commands map[string]cmdHelp) {
	for _, cmd := range commands {
		fmt.Printf("%-15s %s\n", cmd.Name, cmd.Description)
		if len(cmd.SubCommands) > 0 {
			for _, subCmd := range cmd.SubCommands {
				fmt.Printf("  %-15s %s\n", subCmd.Name, subCmd.Description)
			}
		}
	}
	commonHelp()
}

// commonHelp prints common help commands.
func commonHelp() {
	fmt.Printf("%-15s %s\n", "/", "- Go up one level")
	fmt.Printf("%-15s %s\n", "exit, quit, q", "- Shutdown the server")
	fmt.Printf("%-15s %s\n", "help, h, ?", "- Show help")
}

func mainHelp() {
	fmt.Println("Available commands:")
	helpPrinter(loadCommands())
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

func subCommandHelp(context string) {
	if cmd, exists := loadCommands()[context]; exists {
		helpPrinter(map[string]cmdHelp{context: cmd})
	} else {
		fmt.Println("Unknown context:", context)
	}
}

// Load the command data from a JSON file or a static structure.
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
