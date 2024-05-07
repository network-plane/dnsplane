package main

import (
	"dnsresolver/cache"
	"dnsresolver/data"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsserver"
	"fmt"

	"github.com/chzyer/readline"
)

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
			if cmd, found := commands[args[argPos+1]]; found {
				cmd(args[argPos+1:])
			} else {
				fmt.Printf("Unknown %s subcommand: %s\n", context, args[argPos+1])
			}
		}
	}
}

func handleRecord(args []string, currentContext string) {
	commands := map[string]func([]string){
		"add":    func(args []string) { dnsrecords.Add(args, gDNSRecords) },
		"remove": func(args []string) { gDNSRecords = dnsrecords.Remove(args, gDNSRecords) },
		"update": func(args []string) { dnsrecords.Update(args, gDNSRecords) },
		"list":   func(args []string) { dnsrecords.List(gDNSRecords) },
		"clear":  func(args []string) { gDNSRecords = []dnsrecords.DNSRecord{} },
		"load":   func(args []string) { data.LoadDNSRecords() },
		"save":   func(args []string) { data.SaveDNSRecords(gDNSRecords) },
	}
	handleCommand(args, "record", commands)
}

func handleCache(args []string, currentContext string) {
	commands := map[string]func([]string){
		"list":   func(args []string) { cache.List(cacheRecords) },
		"remove": func(args []string) { cacheRecords = cache.Remove(args, cacheRecords) },
		"clear":  func(args []string) { cacheRecords = []cache.Record{} },
		"load":   func(args []string) { data.LoadCacheRecords() },
		"save":   func(args []string) { data.SaveCacheRecords(cacheRecords) },
	}
	handleCommand(args, "cache", commands)
}

func handleDNS(args []string, currentContext string) {
	commands := map[string]func([]string){
		"add":    func(args []string) { dnsserver.Add(args, dnsServers) },
		"remove": func(args []string) { dnsServers = dnsserver.Remove(args, dnsServers) },
		"update": func(args []string) { dnsServers = dnsserver.Update(args, dnsServers) },
		"list":   func(args []string) { dnsserver.List(dnsServers) },
		"clear":  func(args []string) { dnsServers = []dnsserver.DNSServer{} },
		"load":   func(args []string) { data.LoadDNSServers() },
		"save":   func(args []string) { data.SaveDNSServers(dnsServers) },
	}
	handleCommand(args, "dns", commands)
}

func handleServer(args []string, currentContext string) {
	commands := map[string]func([]string){
		"start":     func(args []string) { /* restartDNSServer(dnsServerSettings.DNSPort) */ },
		"stop":      func(args []string) { /* stopDNSServer() */ },
		"status":    func(args []string) { fmt.Println("Server Status: ", getServerStatus()) },
		"configure": func(args []string) { /* config(args) */ },
		"load":      func(args []string) { dnsServerSettings = loadSettings() },
		"save":      func(args []string) { saveSettings(dnsServerSettings) },
	}
	handleCommand(args, "server", commands)
}

func handleServerStart(args []string, currentContext string) {
	args = args[1:]
	commands := map[string]func([]string){
		"dns":  func(args []string) { restartDNSServer(dnsServerSettings.DNSPort) },
		"mdns": func(args []string) { startMDNSServer(dnsServerSettings.MDNSPort) },
		"api":  func(args []string) { startGinAPI(dnsServerSettings.RESTPort) },
		"dhcp": func(args []string) { /* startDHCP() */ },
	}
	handleCommand(args, "start", commands)
}

func handleServerStop(args []string, currentContext string) {
	args = args[1:]
	commands := map[string]func([]string){
		"dns":  func(args []string) { stopDNSServer() },
		"mdns": func(args []string) { /* stopMDNSServer() */ },
		"api":  func(args []string) { /* stopGinAPI() */ },
		"dhcp": func(args []string) { /* startDHCP() */ },
	}
	handleCommand(args, "start", commands)
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
					readline.PcItem("dns"),
					readline.PcItem("mdns"),
					readline.PcItem("api"),
					readline.PcItem("dhcp"),
				),
				readline.PcItem("status"),
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
			readline.PcItem("server",
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
				readline.PcItem("status"),
				readline.PcItem("configure"),
				readline.PcItem("load"),
				readline.PcItem("save"),
				readline.PcItem("?"),
			),
			readline.PcItem("status"),
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
