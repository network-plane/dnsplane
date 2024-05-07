package main

import (
	"dnsresolver/data"
	"dnsresolver/dnsrecords"
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
			fmt.Printf("Unknown %s subcommand: %s\n", context, args[argPos])
		}
	}
}

func handleRecord(args []string, currentContext string) {
	commands := map[string]func([]string){
		"add":    func(args []string) { dnsrecords.Add(args, gDNSRecords) },
		"remove": func(args []string) { dnsrecords.Remove(args, gDNSRecords) },
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
		"clear": func(args []string) {
			// clearCache()
		},
		"list": func(args []string) {
			// listCache()
		},
	}
	handleCommand(args, "cache", commands)
}

func handleDNS(args []string, currentContext string) {
	commands := map[string]func([]string){
		"add":    func(args []string) { /* addDNSServer(args) */ },
		"remove": func(args []string) { /* removeDNSServer(args) */ },
		"update": func(args []string) { /* updateDNSServer(args) */ },
		"list":   func(args []string) { /* listDNSServers() */ },
		"clear":  func(args []string) { /* clearDNSServers() */ },
	}
	handleCommand(args, "dns", commands)
}

func handleServer(args []string, currentContext string) {
	commands := map[string]func([]string){
		"start":    func(args []string) { /* startServer() */ },
		"stop":     func(args []string) { /* stopServer() */ },
		"fallback": func(args []string) { /* setFallbackServer(args) */ },
		"timeout":  func(args []string) { /* setTimeout(args) */ },
		"port":     func(args []string) { /* setPort(args) */ },
	}
	handleCommand(args, "server", commands)
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
				readline.PcItem("clear"),
				readline.PcItem("list"),
				readline.PcItem("?"),
			),
			readline.PcItem("dns",
				readline.PcItem("add"),
				readline.PcItem("remove"),
				readline.PcItem("update"),
				readline.PcItem("list"),
				readline.PcItem("clear"),
				readline.PcItem("test"),
				readline.PcItem("load"),
				readline.PcItem("save"),
				readline.PcItem("?"),
			),
			readline.PcItem("server",
				readline.PcItem("fallback"),
				readline.PcItem("timeout"),
				readline.PcItem("port"),
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
			readline.PcItem("clear"),
			readline.PcItem("list"),
			readline.PcItem("?"),
		)
	case "dns":
		rl.Config.AutoComplete = readline.NewPrefixCompleter(
			readline.PcItem("add"),
			readline.PcItem("remove"),
			readline.PcItem("update"),
			readline.PcItem("list"),
			readline.PcItem("clear"),
			readline.PcItem("test"),
			readline.PcItem("load"),
			readline.PcItem("save"),
			readline.PcItem("?"),
		)
	case "server":
		rl.Config.AutoComplete = readline.NewPrefixCompleter(
			readline.PcItem("fallback"),
			readline.PcItem("timeout"),
			readline.PcItem("port"),
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
