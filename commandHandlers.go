package main

import (
	"fmt"

	"github.com/bettercap/readline"
)

func handleStats() {
	showStats()
}

func handleRecord(args []string, currentContext string) {
	var argPos int
	if currentContext == "" {
		argPos = 1
		if len(args) < 2 {
			fmt.Println("record subcommand required. Use 'record ?' for help.")
			return
		}
	} else {
		argPos = 0
	}

	switch args[argPos] {
	case "add":
		addRecord(args)
	case "remove":
		// removeRecord(args)
	case "update":
		// updateRecord(args)
	case "list":
		listRecords()
	case "clear":
		// clearRecords()
	case "test":
		// testRecord(args)
	case "load":
		// loadRecords(args)
	case "save":
		// saveRecords(args)
	case "?":
		recordHelp()
	default:
		fmt.Println("Unknown record subcommand:", args[argPos])
	}
}

func handleCache(args []string, currentContext string) {
	var argPos int
	if currentContext == "" {
		argPos = 1
		if len(args) < 2 {
			fmt.Println("cache subcommand required. Use 'cache ?' for help.")
			return
		}
	} else {
		argPos = 0
	}

	switch args[argPos] {
	case "clear":
		// clearCache()
	case "list":
		// listCache()
	case "?":
		cacheHelp()
	default:
		fmt.Println("Unknown cache subcommand:", args[argPos])
	}
}

func handleDNS(args []string, currentContext string) {
	var argPos int
	if currentContext == "" {
		argPos = 1
		if len(args) < 2 {
			fmt.Println("dns subcommand required. Use 'dns ?' for help.")
			return
		}
	} else {
		argPos = 0
	}

	switch args[argPos] {
	case "add":
		// addDNSServer(args)
	case "remove":
		// removeDNSServer(args)
	case "update":
		// updateDNSServer(args)
	case "list":
		// listDNSServers()
	case "clear":
		// clearDNSServers()
	case "?":
		dnsHelp()
	default:
		fmt.Println("Unknown DNS subcommand:", args[argPos])
	}
}

func handleServer(args []string, currentContext string) {
	var argPos int
	if currentContext == "" {
		argPos = 1
		if len(args) < 2 {
			fmt.Println("server subcommand required. Use 'server ?' for help.")
			return
		}
	} else {
		argPos = 0
	}

	switch args[argPos] {
	case "fallback":
		// setFallbackServer(args)
	case "timeout":
		// setTimeout(args)
	case "port":
		// setPort(args)
	case "?":
		serverHelp()
	default:
		fmt.Println("Unknown server subcommand:", args[argPos])
	}
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
				readline.PcItem("test"),
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
			readline.PcItem("test"),
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
