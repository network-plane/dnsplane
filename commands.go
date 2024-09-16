package main

import (
	"dnsresolver/data"
	"dnsresolver/dnsrecordcache"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsserver"
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
		handleRecord(args)
	case "cache":
		handleCache(args)
	case "dns":
		handleDNS(args)
	case "server":
		handleServer(args)
	case "start":
		handleServerStart(args)
	case "stop":
		handleServerStop(args)
	case "status":
		handleServerStatus(args)
	case "configure":
		handleServerConfigure(args)
	default:
		fmt.Println("Unknown subcommand:", args[0])
	}
}

// Handlers for the commands
func handleStats() {
	showStats()
}

func handleCommand(args []string, context string, commands map[string]func([]string)) {
	var argPos int
	if context == "" {
		argPos = 1
		if len(args) < 2 {
			fmt.Printf("%s subcommand required. Use '%s ?' for help.\n", context, context)
			return
		}
	} else {
		argPos = 0
	}

	if checkHelp(args[argPos], context) {
		if cmd, found := commands[args[argPos]]; found {
			cmd(args)
		} else {
			// fmt.Println(commands)
			if len(args) > argPos+1 {
				if cmd, found := commands[args[argPos+1]]; found {
					cmd(args[argPos+1:])
				} else {
					fmt.Printf("Unknown %s subcommand: %s\n", context, args[argPos+1])
				}
			}
		}
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
	commands := map[string]func([]string){
		"start":     func(args []string) {},
		"stop":      func(args []string) {},
		"status":    func(args []string) {},
		"configure": func(args []string) { /* config(args) */ },
		"load":      func(args []string) { dnsServerSettings = loadSettings() },
		"save":      func(args []string) { saveSettings(dnsServerSettings) },
	}
	handleCommand(args, "server", commands)
}

func handleServerStart(args []string) {
	args = args[1:]
	commands := map[string]func([]string){
		"dns":  func(args []string) { restartDNSServer(dnsServerSettings.DNSPort) },
		"mdns": func(args []string) { startMDNSServer(dnsServerSettings.MDNSPort) },
		"api":  func(args []string) { startGinAPI(dnsServerSettings.RESTPort) },
		"dhcp": func(args []string) { /* startDHCP() */ },
	}
	handleCommand(args, "start", commands)
}

func handleServerStop(args []string) {
	args = args[1:]
	commands := map[string]func([]string){
		"dns":  func(args []string) { stopDNSServer() },
		"mdns": func(args []string) { /* stopMDNSServer() */ },
		"api":  func(args []string) { /* stopGinAPI() */ },
		"dhcp": func(args []string) { /* startDHCP() */ },
	}
	handleCommand(args, "stop", commands)
}

func handleServerStatus(args []string) {
	args = args[1:]
	commands := map[string]func([]string){
		"dns":  func(args []string) { fmt.Println("DNS Server Status: ", getServerStatus()) },
		"mdns": func(args []string) { /* stopMDNSServer() */ },
		"api":  func(args []string) { /* stopGinAPI() */ },
		"dhcp": func(args []string) { /* startDHCP() */ },
	}
	handleCommand(args, "status", commands)
}

func handleServerConfigure(args []string) {
	args = args[1:]
	commands := map[string]func([]string){}
	handleCommand(args, "configure", commands)
}

func setupAutocomplete(rl *readline.Instance, context string) {
	updatePrompt(rl, context)

	switch context {
	case "":
		rl.Config.AutoComplete = readline.NewPrefixCompleter(
			readline.PcItem("stats"),
			readline.PcItem("record",
				readline.PcItem("add"),
				readline.PcItem("remove"),
				readline.PcItem("update"),
				readline.PcItem("list"),
				readline.PcItem("clear"),
				readline.PcItem("load"),
				readline.PcItem("save"),
				readline.PcItem("?"),
			),
			readline.PcItem("cache",
				readline.PcItem("list"),
				readline.PcItem("remove"),
				readline.PcItem("clear"),
				readline.PcItem("load"),
				readline.PcItem("save"),
				readline.PcItem("?"),
			),
			readline.PcItem("dns",
				readline.PcItem("add"),
				readline.PcItem("remove"),
				readline.PcItem("update"),
				readline.PcItem("list"),
				readline.PcItem("clear"),
				readline.PcItem("load"),
				readline.PcItem("save"),
				readline.PcItem("?"),
			),
			readline.PcItem("server",
				readline.PcItem("start",
					readline.PcItem("dns"),
					readline.PcItem("mdns"),
					readline.PcItem("api"),
					readline.PcItem("dhcp"),
				),
				readline.PcItem("stop",
					readline.PcItem("all"),
					readline.PcItem("dns"),
					readline.PcItem("mdns"),
					readline.PcItem("api"),
					readline.PcItem("dhcp"),
				),
				readline.PcItem("status",
					readline.PcItem("all"),
					readline.PcItem("dns"),
					readline.PcItem("mdns"),
					readline.PcItem("api"),
					readline.PcItem("dhcp"),
				),
				readline.PcItem("configure"),
				readline.PcItem("load"),
				readline.PcItem("save"),
				readline.PcItem("?"),
			),
			readline.PcItem("exit"),
			readline.PcItem("quit"),
			readline.PcItem("q"),
			readline.PcItem("help"),
			readline.PcItem("h"),
			readline.PcItem("?"),
		)
	case "record":
		rl.Config.AutoComplete = readline.NewPrefixCompleter(
			readline.PcItem("add"),
			readline.PcItem("remove"),
			readline.PcItem("update"),
			readline.PcItem("list"),
			readline.PcItem("clear"),
			readline.PcItem("load"),
			readline.PcItem("save"),
			readline.PcItem("?"),
		)
	case "cache":
		rl.Config.AutoComplete = readline.NewPrefixCompleter(
			readline.PcItem("list"),
			readline.PcItem("remove"),
			readline.PcItem("clear"),
			readline.PcItem("load"),
			readline.PcItem("save"),
			readline.PcItem("?"),
		)
	case "dns":
		rl.Config.AutoComplete = readline.NewPrefixCompleter(
			readline.PcItem("add"),
			readline.PcItem("remove"),
			readline.PcItem("update"),
			readline.PcItem("list"),
			readline.PcItem("clear"),
			readline.PcItem("load"),
			readline.PcItem("save"),
			readline.PcItem("?"),
		)
	case "server":
		rl.Config.AutoComplete = readline.NewPrefixCompleter(
			readline.PcItem("start",
				readline.PcItem("dns"),
				readline.PcItem("mdns"),
				readline.PcItem("api"),
				readline.PcItem("dhcp"),
			),
			readline.PcItem("stop",
				readline.PcItem("dns"),
				readline.PcItem("mdns"),
				readline.PcItem("api"),
				readline.PcItem("dhcp"),
			),
			readline.PcItem("status",
				readline.PcItem("all"),
				readline.PcItem("dns"),
				readline.PcItem("mdns"),
				readline.PcItem("api"),
				readline.PcItem("dhcp"),
			),
			readline.PcItem("configure"),
			readline.PcItem("load"),
			readline.PcItem("save"),
			readline.PcItem("?"),
		)
	}
}

func updatePrompt(rl *readline.Instance, currentContext string) {
	if currentContext == "" {
		rl.SetPrompt("> ")
	} else {
		rl.SetPrompt(fmt.Sprintf("(%s) > ", currentContext))
	}
	rl.Refresh()
}
