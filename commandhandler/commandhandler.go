package commandhandler

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"dnsresolver/data"
	"dnsresolver/dnsrecordcache"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsservers"

	"github.com/chzyer/readline"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// cmdHelp represents a command and its subcommands for help display
type cmdHelp struct {
	Name        string
	Description string
	SubCommands map[string]cmdHelp
}

// Excluded commands from history
var excludedCommands = map[string]bool{
	"q": true, "quit": true, "exit": true, "h": true, "help": true, "ls": true, "l": true, "/": true,
}

// Function variables for server control
var (
	stopDNSServerFunc    func()
	restartDNSServerFunc func(string)
	getServerStatusFunc  func() bool
	startMDNSServerFunc  func(string)
	startGinAPIFunc      func(string)
)

// HandleCommandLoop manages the interactive command loop
func HandleCommandLoop(rl *readline.Instance, startDNS func(string), stopDNS func(), restartDNS func(string), getStatus func() bool, startMDNS func(string), startAPI func(string)) {
	// Assign function variables
	stopDNSServerFunc = stopDNS
	restartDNSServerFunc = restartDNS
	getServerStatusFunc = getStatus
	startMDNSServerFunc = startMDNS
	startGinAPIFunc = startAPI

	var currentContext string
	setupAutocomplete(rl, currentContext)

	for {
		updatePrompt(rl, currentContext)
		commandLine, err := rl.Readline()
		if err != nil {
			break
		}

		commandLine = strings.TrimSpace(commandLine)
		if commandLine == "" {
			continue
		}

		args := strings.Fields(commandLine)
		if len(args) == 0 {
			continue
		}

		if isExitCommand(args[0]) {
			fmt.Println("Shutting down.")
			os.Exit(0)
		}

		if !excludedCommands[args[0]] {
			if err := rl.SaveHistory(commandLine); err != nil {
				fmt.Println("Error saving history:", err)
			}
		}

		if currentContext == "" {
			handleGlobalCommands(args, rl, &currentContext)
		} else {
			handleContextCommands(args, rl, &currentContext)
		}
	}
}

// isExitCommand checks if the command is an exit command
func isExitCommand(cmd string) bool {
	return cmd == "q" || cmd == "quit" || cmd == "exit"
}

// handleGlobalCommands processes commands at the global context
func handleGlobalCommands(args []string, rl *readline.Instance, currentContext *string) {
	switch args[0] {
	case "stats":
		handleStats()
	case "record", "cache", "dns", "server":
		handleContextCommand(args[0], args[1:], rl, currentContext)
	case "help", "h", "?", "ls", "l":
		showHelp("")
	case "exit", "quit", "q":
		fmt.Println("Shutting down.")
		os.Exit(0)
	default:
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", args[0])
	}
}

// handleContextCommands processes commands within a specific context
func handleContextCommands(args []string, rl *readline.Instance, currentContext *string) {
	if args[0] == "/" {
		*currentContext = "" // Exit context
		setupAutocomplete(rl, *currentContext)
		return
	}

	if args[0] == "help" || args[0] == "?" || args[0] == "h" {
		showHelp(*currentContext)
		return
	}

	// Handle subcommand within context
	handleContextCommand(*currentContext, args, rl, currentContext)
}

// handleContextCommand handles commands within a context or executes a subcommand directly
func handleContextCommand(context string, args []string, rl *readline.Instance, currentContext *string) {
	if len(args) == 0 {
		// Enter context
		*currentContext = context
		setupAutocomplete(rl, *currentContext)
		return
	}

	handlers := map[string]func([]string){
		"record": handleRecord,
		"cache":  handleCache,
		"dns":    handleDNS,
		"server": handleServer,
	}

	if handler, ok := handlers[context]; ok {
		handler(args)
	} else {
		fmt.Printf("Unknown context: %s. Type 'help' for available commands.\n", context)
	}
}

// handleRecord processes 'record' subcommands
func handleRecord(args []string) {
	dnsData := data.GetInstance()
	dnsData.Initialize()

	gDNSRecords := dnsData.DNSRecords

	commands := map[string]func([]string){
		"add": func(args []string) {
			gDNSRecords = dnsrecords.Add(args, gDNSRecords)
			dnsData.UpdateRecords(gDNSRecords)
		},
		"remove": func(args []string) {
			gDNSRecords = dnsrecords.Remove(args, gDNSRecords)
			dnsData.UpdateRecords(gDNSRecords)
		},
		"update": func(args []string) {
			gDNSRecords = dnsrecords.Update(args, gDNSRecords)
			dnsData.UpdateRecords(gDNSRecords)
		},
		"list": func(args []string) {
			dnsrecords.List(gDNSRecords)
		},
		"clear": func(args []string) {
			gDNSRecords = []dnsrecords.DNSRecord{}
			dnsData.UpdateRecords(gDNSRecords)
			fmt.Println("All DNS records have been cleared.")
		},
		"load": func(args []string) {
			gDNSRecords = data.LoadDNSRecords()
			dnsData.UpdateRecords(gDNSRecords)
			fmt.Println("DNS records loaded.")
		},
		"save": func(args []string) {
			data.SaveDNSRecords(gDNSRecords)
			fmt.Println("DNS records saved.")
		},
	}

	executeSubCommand("record", args, commands)
}

// handleCache processes 'cache' subcommands
func handleCache(args []string) {
	dnsData := data.GetInstance()
	cacheRecordsData := dnsData.CacheRecords

	commands := map[string]func([]string){
		"list": func(args []string) {
			dnsrecordcache.List(cacheRecordsData)
		},
		"remove": func(args []string) {
			cacheRecordsData = dnsrecordcache.Remove(args, cacheRecordsData)
			dnsData.UpdateCacheRecords(cacheRecordsData)
		},
		"clear": func(args []string) {
			cacheRecordsData = []dnsrecordcache.CacheRecord{}
			dnsData.UpdateCacheRecords(cacheRecordsData)
			fmt.Println("Cache cleared.")
		},
		"load": func(args []string) {
			cacheRecordsData = data.LoadCacheRecords()
			dnsData.UpdateCacheRecords(cacheRecordsData)
			fmt.Println("Cache records loaded.")
		},
		"save": func(args []string) {
			data.SaveCacheRecords(cacheRecordsData)
			fmt.Println("Cache records saved.")
		},
	}

	executeSubCommand("cache", args, commands)
}

// handleDNS processes 'dns' subcommands
func handleDNS(args []string) {
	dnsData := data.GetInstance()
	dnsServers := dnsData.DNSServers

	commands := map[string]func([]string){
		"add": func(args []string) {
			dnsServers = dnsservers.Add(args, dnsServers)
			dnsData.UpdateServers(dnsServers)
		},
		"remove": func(args []string) {
			dnsServers = dnsservers.Remove(args, dnsServers)
			dnsData.UpdateServers(dnsServers)
		},
		"update": func(args []string) {
			dnsServers = dnsservers.Update(args, dnsServers)
			dnsData.UpdateServers(dnsServers)
		},
		"list": func(args []string) {
			dnsservers.List(dnsServers)
		},
		"clear": func(args []string) {
			dnsServers = []dnsservers.DNSServer{}
			dnsData.UpdateServers(dnsServers)
			fmt.Println("All DNS servers have been cleared.")
		},
		"load": func(args []string) {
			dnsServers = data.LoadDNSServers()
			dnsData.UpdateServers(dnsServers)
			fmt.Println("DNS servers loaded.")
		},
		"save": func(args []string) {
			data.SaveDNSServers(dnsServers)
			fmt.Println("DNS servers saved.")
		},
	}

	executeSubCommand("dns", args, commands)
}

// handleServer manages 'server' subcommands, utilizing the passed-in functions
func handleServer(args []string) {
	commands := map[string]func([]string){
		"start":     handleServerStart,
		"stop":      handleServerStop,
		"status":    handleServerStatus,
		"configure": handleServerConfigure,
		"load":      handleServerLoad,
		"save":      handleServerSave,
	}

	executeSubCommand("server", args, commands)
}

// executeSubCommand executes the appropriate subcommand handler
func executeSubCommand(context string, args []string, commands map[string]func([]string)) {
	if len(args) == 0 {
		fmt.Printf("Subcommand required. Use '%s ?' for help.\n", context)
		return
	}

	subCmd := args[0]

	if !checkHelp(subCmd, context) {
		return
	}

	if handler, ok := commands[subCmd]; ok {
		handler(args[1:])
	} else {
		fmt.Printf("Unknown '%s' subcommand: %s. Use '%s ?' for help.\n", context, subCmd, context)
	}
}

// handleServerLoad handles 'server load' command
func handleServerLoad(args []string) {
	dnsData := data.GetInstance()
	dnsServerSettings := data.LoadSettings()
	dnsData.UpdateSettings(dnsServerSettings)
	fmt.Println("Server settings loaded.")
}

// handleServerSave handles 'server save' command
func handleServerSave(args []string) {
	dnsData := data.GetInstance()
	data.SaveSettings(dnsData.Settings)
	fmt.Println("Server settings saved.")
}

// handleServerStart handles 'server start' command
func handleServerStart(args []string) {
	if len(args) == 0 {
		fmt.Println("Server component to start required. Use 'server start ?' for help.")
		return
	}

	dnsData := data.GetInstance()
	dnsServerSettings := dnsData.Settings

	startCommands := map[string]func(){
		"dns": func() {
			restartDNSServerFunc(dnsServerSettings.DNSPort)
			fmt.Println("DNS server started.")
		},
		"mdns": func() {
			startMDNSServerFunc(dnsServerSettings.MDNSPort)
			fmt.Println("mDNS server started.")
		},
		"api": func() {
			startGinAPIFunc(dnsServerSettings.RESTPort)
			fmt.Println("API server started.")
		},
	}

	component := args[0]

	if cmd, ok := startCommands[component]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component to start: %s. Use 'server start ?' for help.\n", component)
	}
}

// handleServerStop handles 'server stop' command
func handleServerStop(args []string) {
	if len(args) == 0 {
		fmt.Println("Server component to stop required. Use 'server stop ?' for help.")
		return
	}

	stopCommands := map[string]func(){
		"dns": func() {
			stopDNSServerFunc()
			fmt.Println("DNS server stopped.")
		},
		"mdns": func() {
			// Implement mDNS stop logic if needed
			fmt.Println("mDNS server stop not implemented yet.")
		},
		"api": func() {
			// Implement API stop logic if needed
			fmt.Println("API server stop not implemented yet.")
		},
	}

	component := args[0]

	if cmd, ok := stopCommands[component]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component to stop: %s. Use 'server stop ?' for help.\n", component)
	}
}

// handleServerStatus handles 'server status' command
func handleServerStatus(args []string) {
	if len(args) == 0 {
		fmt.Println("Server component required. Use 'server status ?' for help.")
		return
	}

	statusCommands := map[string]func(){
		"dns": func() {
			status := "stopped"
			if getServerStatusFunc() {
				status = "running"
			}
			fmt.Printf("DNS Server is %s.\n", status)
		},
		"mdns": func() {
			// Implement mDNS status logic if needed
			fmt.Println("mDNS server status not implemented yet.")
		},
		"api": func() {
			// Implement API status logic if needed
			fmt.Println("API server status not implemented yet.")
		},
	}

	component := args[0]

	if cmd, ok := statusCommands[component]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component: %s. Use 'server status ?' for help.\n", component)
	}
}

// handleServerConfigure handles 'server configure' command
func handleServerConfigure(args []string) {
	dnsData := data.GetInstance()
	dnsServerSettings := dnsData.Settings

	if len(args) == 0 {
		// List current configuration
		fmt.Println("Current Server Configuration:")
		fmt.Printf("DNS Port: %s\n", dnsServerSettings.DNSPort)
		fmt.Printf("mDNS Port: %s\n", dnsServerSettings.MDNSPort)
		fmt.Printf("API Port: %s\n", dnsServerSettings.RESTPort)
		fmt.Printf("Fallback Server IP: %s\n", dnsServerSettings.FallbackServerIP)
		fmt.Printf("Fallback Server Port: %s\n", dnsServerSettings.FallbackServerPort)
	} else {
		// Set configuration parameters
		if len(args) < 2 {
			fmt.Println("Usage: server configure [setting] [value]")
			return
		}
		setting := args[0]
		value := args[1]

		switch setting {
		case "dns_port":
			dnsServerSettings.DNSPort = value
			fmt.Printf("DNS Port set to %s\n", value)
		case "mdns_port":
			dnsServerSettings.MDNSPort = value
			fmt.Printf("mDNS Port set to %s\n", value)
		case "api_port":
			dnsServerSettings.RESTPort = value
			fmt.Printf("API Port set to %s\n", value)
		case "fallback_ip":
			dnsServerSettings.FallbackServerIP = value
			fmt.Printf("Fallback Server IP set to %s\n", value)
		case "fallback_port":
			dnsServerSettings.FallbackServerPort = value
			fmt.Printf("Fallback Server Port set to %s\n", value)
		default:
			fmt.Printf("Unknown setting: %s\n", setting)
			return
		}

		dnsData.UpdateSettings(dnsServerSettings)
		fmt.Println("Server configuration updated.")
	}
}

// setupAutocomplete sets up command autocompletion
func setupAutocomplete(rl *readline.Instance, context string) {
	updatePrompt(rl, context)

	var completer *readline.PrefixCompleter

	if context == "" {
		completer = readline.NewPrefixCompleter(
			readline.PcItem("record"),
			readline.PcItem("cache"),
			readline.PcItem("dns"),
			readline.PcItem("server"),
			readline.PcItem("stats"),
			readline.PcItem("help"),
			readline.PcItem("h"),
			readline.PcItem("?"),
			readline.PcItem("exit"),
			readline.PcItem("quit"),
			readline.PcItem("q"),
		)
	} else {
		subCommands := getSubCommands(context)
		completer = readline.NewPrefixCompleter(
			readline.PcItemDynamic(func(string) []string { return subCommands }),
		)
	}

	rl.Config.AutoComplete = completer
}

// getSubCommands returns the subcommands for the given context
func getSubCommands(context string) []string {
	commands := loadCommands()
	if cmd, exists := commands[context]; exists {
		subCmds := []string{}
		for subCmd := range cmd.SubCommands {
			subCmds = append(subCmds, subCmd)
		}
		// Add common commands
		subCmds = append(subCmds, "/", "help", "h", "?")
		sort.Strings(subCmds)
		return subCmds
	}
	return []string{}
}

// updatePrompt updates the readline prompt based on the current context
func updatePrompt(rl *readline.Instance, currentContext string) {
	prompt := "> "
	if currentContext != "" {
		prompt = fmt.Sprintf("(%s) > ", currentContext)
	}
	rl.SetPrompt(prompt)
	rl.Refresh()
}

// showHelp displays help information based on the context
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

// checkHelp determines if the argument is for help.
func checkHelp(arg, context string) bool {
	helpCommands := []string{"?", "help", "h"}

	for _, cmd := range helpCommands {
		if arg == cmd {
			showHelp(context)
			return false
		}
	}

	return true
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
				"remove": {"remove", "- Remove a cache entry", nil},
				"list":   {"list", "- List all cache entries", nil},
				"clear":  {"clear", "- Clear the cache", nil},
				"load":   {"load", "- Load cache records from a file", nil},
				"save":   {"save", "- Save cache records to a file", nil},
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
				"start":     {"start", "- Start server components", nil},
				"stop":      {"stop", "- Stop server components", nil},
				"status":    {"status", "- Show server component status", nil},
				"configure": {"configure", "- Set or list server configuration", nil},
				"load":      {"load", "- Load server settings from a file", nil},
				"save":      {"save", "- Save server settings to a file", nil},
			},
		},
	}
}

// handleStats displays server statistics
func handleStats() {
	dnsData := data.GetInstance()

	fmt.Println("Stats:")
	fmt.Println("Server start time:", dnsData.Stats.ServerStartTime)
	fmt.Println("Server Up Time:", serverUpTimeFormat(dnsData.Stats.ServerStartTime))
	fmt.Println()
	fmt.Println("Total Records:", len(dnsData.DNSRecords))
	fmt.Println("Total DNS Servers:", len(dnsData.DNSServers))
	fmt.Println("Total Cache Records:", len(dnsData.CacheRecords))
	fmt.Println()
	fmt.Println("Total queries received:", dnsData.Stats.TotalQueries)
	fmt.Println("Total queries answered:", dnsData.Stats.TotalQueriesAnswered)
	fmt.Println("Total cache hits:", dnsData.Stats.TotalCacheHits)
	fmt.Println("Total queries forwarded:", dnsData.Stats.TotalQueriesForwarded)
}

// Helper function for formatting uptime
func serverUpTimeFormat(startTime time.Time) string {
	duration := time.Since(startTime)

	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	duration -= minutes * time.Minute
	seconds := duration / time.Second

	if days > 0 {
		return fmt.Sprintf("%d days, %d hours, %d minutes, %d seconds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours, %d minutes, %d seconds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d minutes, %d seconds", minutes, seconds)
	}
	return fmt.Sprintf("%d seconds", seconds)
}
