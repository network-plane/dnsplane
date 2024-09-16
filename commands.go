package main

import (
	"dnsresolver/data"
	"dnsresolver/dnsrecordcache"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsserver"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/chzyer/readline"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
			os.Exit(0)
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
	case "record", "cache", "dns", "server":
		handleContextCommand(args[0], args, rl, currentContext)
	case "help", "h", "?", "ls", "l":
		showHelp("")
	case "exit", "quit", "q":
		fmt.Println("Shutting down.")
		os.Exit(0)
	default:
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", args[0])
	}
}

// Handle server-specific commands
func handleServerCommand(args []string, rl *readline.Instance, currentContext *string) {
	if len(args) > 1 {
		switch args[1] {
		case "start", "stop", "status", "configure":
			handleContextCommand(args[1], args[1:], rl, currentContext)
			return
		}
	}
	handleContextCommand(args[0], args, rl, currentContext)
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

	if args[0] == "help" || args[0] == "?" {
		showHelp(*currentContext)
		return
	}

	handleSubcommand(*currentContext, args, *currentContext)
}

// Dispatch subcommands to the appropriate handlers
func handleSubcommand(command string, args []string, context string) {
	handlers := map[string]func([]string){
		"record": handleRecord,
		"cache":  handleCache,
		"dns":    handleDNS,
		"server": handleServer,
	}

	if handler, ok := handlers[command]; ok {
		handler(args[1:]) // Pass all arguments except the first (command name)
	} else {
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", command)
	}
}

// Handlers for the commands
func handleStats() {
	showStats()
}

func handleCommand(args []string, context string, commands map[string]func([]string)) {
	if len(args) == 0 {
		fmt.Printf("%s subcommand required. Use '%s ?' for help.\n", context, context)
		return
	}

	subCmd := args[0]
	if !checkHelp(subCmd, context) {
		// checkHelp returns false if it's a help command and has already displayed help
		return
	}

	if handler, ok := commands[subCmd]; ok {
		// Pass the entire args slice to the handler
		// The handler should handle cases where no additional arguments are provided
		handler(args)
	} else {
		fmt.Printf("Unknown %s subcommand: %s. Use '%s ?' for help.\n", context, subCmd, context)
	}
}

func handleRecord(args []string) {
	commands := map[string]func([]string){
		"add":    func(args []string) { gDNSRecords = dnsrecords.Add(args, gDNSRecords) },
		"remove": func(args []string) { gDNSRecords = dnsrecords.Remove(args, gDNSRecords) },
		"update": func(args []string) { dnsrecords.Update(args, gDNSRecords) },
		"list":   func(args []string) { dnsrecords.List(gDNSRecords) },
		"clear":  func(args []string) { gDNSRecords = []dnsrecords.DNSRecord{} },
		"load":   func(args []string) { data.LoadDNSRecords() },
		"save":   func(args []string) { data.SaveDNSRecords(gDNSRecords) },
	}
	handleCommand(args, "record", commands)
}

func handleCache(args []string) {
	commands := map[string]func([]string){
		"list":   func(args []string) { dnsrecordcache.List(cacheRecordsData) },
		"remove": func(args []string) { cacheRecordsData = dnsrecordcache.Remove(args, cacheRecordsData) },
		"clear":  func(args []string) { cacheRecordsData = []dnsrecordcache.CacheRecord{} },
		"load":   func(args []string) { data.LoadCacheRecords() },
		"save":   func(args []string) { data.SaveCacheRecords(cacheRecordsData) },
	}
	handleCommand(args, "cache", commands)
}

func handleDNS(args []string) {
	dnsData := data.GetInstance()
	dnsServers := dnsData.DNSServers
	commands := map[string]func([]string){
		"add":    func(args []string) { dnsServers = dnsserver.Add(args, dnsServers) },
		"remove": func(args []string) { dnsServers = dnsserver.Remove(args, dnsServers) },
		"update": func(args []string) { dnsServers = dnsserver.Update(args, dnsServers) },
		"list":   func(args []string) { dnsserver.List(dnsServers) },
		"clear":  func(args []string) { dnsServers = []dnsserver.DNSServer{} },
		"load":   func(args []string) { data.LoadDNSServers() },
		"save":   func(args []string) { data.SaveDNSServers(dnsServers) },
	}
	handleCommand(args, "dns", commands)
}

func handleServer(args []string) {
	if len(args) == 0 {
		fmt.Println("Server subcommand required. Use 'server ?' for help.")
		return
	}

	dnsData := data.GetInstance()
	dnsServerSettings := dnsData.Settings

	serverCommands := map[string]func([]string){
		"start":     handleServerStart,
		"stop":      handleServerStop,
		"status":    handleServerStatus,
		"configure": handleServerConfigure,
		"load": func(args []string) {
			dnsServerSettings = data.LoadSettings()
			dnsData.UpdateSettings(dnsServerSettings)
		},
		"save": func(args []string) { data.SaveSettings(dnsServerSettings) },
	}

	if cmd, ok := serverCommands[args[0]]; ok {
		cmd(args[1:])
	} else {
		fmt.Printf("Unknown server subcommand: %s. Use 'server ?' for help.\n", args[0])
	}
}

func handleServerStart(args []string) {
	if len(args) == 0 {
		fmt.Println("Server component to start required. Use 'server start ?' for help.")
		return
	}

	dnsData := data.GetInstance()
	dnsServerSettings := dnsData.Settings

	startCommands := map[string]func(){
		"dns":  func() { restartDNSServer(dnsServerSettings.DNSPort) },
		"mdns": func() { startMDNSServer(dnsServerSettings.MDNSPort) },
		"api":  func() { startGinAPI(dnsServerSettings.RESTPort) },
		"dhcp": func() { /* startDHCP() */ },
	}

	if cmd, ok := startCommands[args[0]]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component to start: %s. Use 'server start ?' for help.\n", args[0])
	}
}

func handleServerStop(args []string) {
	if len(args) == 0 {
		fmt.Println("Server component to stop required. Use 'server stop ?' for help.")
		return
	}

	stopCommands := map[string]func(){
		"dns":  func() { stopDNSServer() },
		"mdns": func() { /* stopMDNSServer() */ },
		"api":  func() { /* stopGinAPI() */ },
		"dhcp": func() { /* stopDHCP() */ },
	}

	if cmd, ok := stopCommands[args[0]]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component to stop: %s. Use 'server stop ?' for help.\n", args[0])
	}
}

func handleServerStatus(args []string) {
	if len(args) == 0 {
		fmt.Println("Server component required. Use 'server status ?' for help.")
		return
	}

	statusCommands := map[string]func(){
		"dns":  func() { fmt.Println("DNS Server Status: ", getServerStatus()) },
		"mdns": func() { /* getMDNSStatus() */ },
		"api":  func() { /* getAPIStatus() */ },
		"dhcp": func() { /* getDHCPStatus() */ },
	}

	if cmd, ok := statusCommands[args[0]]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown status component: %s. Use 'server status ?' for help.\n", args[0])
	}
}

func handleServerConfigure(args []string) {
	// Placeholder for server configuration
	fmt.Println("Server configuration not implemented yet")
}

func setupAutocomplete(rl *readline.Instance, context string) {
	updatePrompt(rl, context)

	completer := readline.NewPrefixCompleter(
		readline.PcItem("record", readline.PcItemDynamic(func(_ string) []string { return getRecordItems() })),
		readline.PcItem("cache", readline.PcItemDynamic(func(_ string) []string { return getCacheItems() })),
		readline.PcItem("dns", readline.PcItemDynamic(func(_ string) []string { return getDNSItems() })),
		readline.PcItem("server", readline.PcItemDynamic(func(_ string) []string { return getServerItems() })),
		readline.PcItem("stats"),
		readline.PcItem("help"),
		readline.PcItem("h"),
		readline.PcItem("?"),
		readline.PcItem("exit"),
		readline.PcItem("quit"),
		readline.PcItem("q"),
	)

	rl.Config.AutoComplete = completer
}

func getRecordItems() []string {
	return []string{"add", "remove", "update", "list", "clear", "load", "save", "?"}
}

func getCacheItems() []string {
	return []string{"list", "remove", "clear", "load", "save", "?"}
}

func getDNSItems() []string {
	return []string{"add", "remove", "update", "list", "clear", "load", "save", "?"}
}

func getServerItems() []string {
	return []string{"start", "stop", "status", "configure", "load", "save", "?"}
}

func updatePrompt(rl *readline.Instance, currentContext string) {
	prompt := "> "
	if currentContext != "" {
		prompt = fmt.Sprintf("(%s) > ", currentContext)
	}
	rl.SetPrompt(prompt)
	rl.Refresh()
}

//help

func showHelp(context string) {
	commands := loadCommands()
	caser := cases.Title(language.English)

	if context == "" {
		fmt.Println("Available commands:")
		helpPrinter(commands, false, false)
		commonHelp(false)
	} else if cmd, exists := commands[context]; exists {
		fmt.Printf("%s commands:\n", caser.String(context))
		helpPrinter(map[string]cmdHelp{context: cmd}, true, true)
		commonHelp(true)
	} else {
		fmt.Printf("Unknown context: %s. Available commands:\n", context)
		helpPrinter(commands, false, false)
		commonHelp(false)
	}
}

// helpPrinter prints help for commands, optionally including subcommands.
func helpPrinter(commands map[string]cmdHelp, includeSubCommands bool, isSubCmd bool) {
	var lines []string

	for _, cmd := range commands {
		if !isSubCmd {
			lines = append(lines, fmt.Sprintf("%-15s %s", cmd.Name, cmd.Description))
		}

		if includeSubCommands && len(cmd.SubCommands) > 0 {
			for _, subCmd := range cmd.SubCommands {
				lines = append(lines, fmt.Sprintf("  %-15s %s", subCmd.Name, subCmd.Description))
			}
		}
	}

	sort.Strings(lines)

	for _, line := range lines {
		fmt.Println(line)
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
				"load":   {"load", "- Load DNS records from a file", nil},
				"save":   {"save", "- Save DNS records to a file", nil},
			},
		},
		"cache": {
			Name:        "cache",
			Description: "- Cache Management",
			SubCommands: map[string]cmdHelp{
				"remove": {"remove", "- Remove an entry", nil},
				"list":   {"list", "- List all cache entries", nil},
				"clear":  {"clear", "- Clear the cache", nil},
				"load":   {"load", "- Load Cache records from a file", nil},
				"save":   {"save", "- Save Cache records to a file", nil},
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
				"load":   {"load", "- Load DNS servers from a file", nil},
				"save":   {"save", "- Save DNS servers to a file", nil},
			},
		},
		"server": {
			Name:        "server",
			Description: "- Server Management",
			SubCommands: map[string]cmdHelp{
				"start":     {"start", "- Start the server", nil},
				"stop":      {"stop", "- Stop the server", nil},
				"status":    {"status", "- Show server status", nil},
				"configure": {"configure", "- Set/List configuration", nil},
				"save":      {"save", "- Save the current settings", nil},
				"load":      {"load", "- Load the settings from the files", nil},
			},
		},
	}
}
