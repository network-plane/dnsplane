package commandhandler

import (
	"dnsresolver/cliutil"
	"dnsresolver/data"
	"dnsresolver/dnsrecordcache"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsservers"
	"dnsresolver/tui"
	"fmt"
	"strings"
	"time"
)

// Function variables for server control
var (
	stopDNSServerFunc    func()
	restartDNSServerFunc func(string)
	getServerStatusFunc  func() bool
	startGinAPIFunc      func(string)
)

type simpleCommand struct {
	name string
	help string
	exec func([]string)
}

func (c simpleCommand) Name() string       { return c.name }
func (c simpleCommand) Help() string       { return c.help }
func (c simpleCommand) Exec(args []string) { c.exec(args) }

// RegisterCommands registers all DNS related contexts and commands with the TUI package.
func RegisterCommands() {
	tui.RegisterCommand("", simpleCommand{"stats", "- Display server statistics", handleStats})

	tui.RegisterContext("record", "- Record Management")
	tui.RegisterCommand("record", simpleCommand{"add", "- Add a new DNS record", recordAdd})
	tui.RegisterCommand("record", simpleCommand{"remove", "- Remove a DNS record", recordRemove})
	tui.RegisterCommand("record", simpleCommand{"update", "- Update a DNS record", recordUpdate})
	tui.RegisterCommand("record", simpleCommand{"list", "- List all DNS records", recordList})
	tui.RegisterCommand("record", simpleCommand{"clear", "- Clear all DNS records", recordClear})
	tui.RegisterCommand("record", simpleCommand{"load", "- Load DNS records from a file", recordLoad})
	tui.RegisterCommand("record", simpleCommand{"save", "- Save DNS records to a file", recordSave})

	tui.RegisterContext("cache", "- Cache Management")
	tui.RegisterCommand("cache", simpleCommand{"list", "- List all cache entries", cacheList})
	tui.RegisterCommand("cache", simpleCommand{"remove", "- Remove a cache entry", cacheRemove})
	tui.RegisterCommand("cache", simpleCommand{"clear", "- Clear the cache", cacheClear})
	tui.RegisterCommand("cache", simpleCommand{"load", "- Load cache records from a file", cacheLoad})
	tui.RegisterCommand("cache", simpleCommand{"save", "- Save cache records to a file", cacheSave})

	tui.RegisterContext("dns", "- DNS Server Management")
	tui.RegisterCommand("dns", simpleCommand{"add", "- Add a new DNS server", dnsAdd})
	tui.RegisterCommand("dns", simpleCommand{"remove", "- Remove a DNS server", dnsRemove})
	tui.RegisterCommand("dns", simpleCommand{"update", "- Update a DNS server", dnsUpdate})
	tui.RegisterCommand("dns", simpleCommand{"list", "- List all DNS servers", dnsList})
	tui.RegisterCommand("dns", simpleCommand{"clear", "- Clear all DNS servers", dnsClear})
	tui.RegisterCommand("dns", simpleCommand{"load", "- Load DNS servers from a file", dnsLoad})
	tui.RegisterCommand("dns", simpleCommand{"save", "- Save DNS servers to a file", dnsSave})

	tui.RegisterContext("server", "- Server Management")
	tui.RegisterCommand("server", simpleCommand{"start", "- Start server components", handleServerStart})
	tui.RegisterCommand("server", simpleCommand{"stop", "- Stop server components", handleServerStop})
	tui.RegisterCommand("server", simpleCommand{"status", "- Show server component status", handleServerStatus})
	tui.RegisterCommand("server", simpleCommand{"configure", "- Set or list server configuration", handleServerConfigure})
	tui.RegisterCommand("server", simpleCommand{"load", "- Load server settings from a file", handleServerLoad})
	tui.RegisterCommand("server", simpleCommand{"save", "- Save server settings to a file", handleServerSave})
}

// Record commands
func recordAdd(args []string) {
	dnsData := data.GetInstance()
	dnsData.Initialize()
	records := dnsData.DNSRecords
	records = dnsrecords.Add(args, records, false)
	dnsData.UpdateRecords(records)
}

func recordRemove(args []string) {
	dnsData := data.GetInstance()
	records := dnsData.DNSRecords
	records = dnsrecords.Remove(args, records)
	dnsData.UpdateRecords(records)
}

func recordUpdate(args []string) {
	dnsData := data.GetInstance()
	records := dnsData.DNSRecords
	records = dnsrecords.Add(args, records, true)
	dnsData.UpdateRecords(records)
}

func recordList(args []string) {
	if cliutil.IsHelpRequest(args) {
		printRecordListUsage()
		return
	}
	dnsData := data.GetInstance()
	records := dnsData.DNSRecords
	if len(args) > 0 && args[0] != "d" && args[0] != "details" {
		fmt.Println("Filtering records by:", args[0])
	}
	dnsrecords.List(records, args)
}

func recordClear(args []string) {
	if cliutil.IsHelpRequest(args) {
		printRecordClearUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("record clear does not accept arguments.")
		printRecordClearUsage()
		return
	}
	dnsData := data.GetInstance()
	dnsData.UpdateRecordsInMemory([]dnsrecords.DNSRecord{})
	fmt.Println("All DNS records have been cleared.")
}

func recordLoad(args []string) {
	if cliutil.IsHelpRequest(args) {
		printRecordLoadUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("record load does not accept arguments.")
		printRecordLoadUsage()
		return
	}
	dnsData := data.GetInstance()
	records := data.LoadDNSRecords()
	dnsData.UpdateRecords(records)
	fmt.Println("DNS records loaded.")
}

func recordSave(args []string) {
	if cliutil.IsHelpRequest(args) {
		printRecordSaveUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("record save does not accept arguments.")
		printRecordSaveUsage()
		return
	}
	dnsData := data.GetInstance()
	records := dnsData.DNSRecords
	if err := data.SaveDNSRecords(records); err != nil {
		fmt.Println("Error saving DNS records:", err)
		return
	}
	fmt.Println("DNS records saved.")
}

// Cache commands
func cacheList(args []string) {
	if cliutil.IsHelpRequest(args) {
		printCacheListUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("cache list does not accept arguments.")
		printCacheListUsage()
		return
	}
	dnsData := data.GetInstance()
	cache := dnsData.CacheRecords
	dnsrecordcache.List(cache)
}

func cacheRemove(args []string) {
	dnsData := data.GetInstance()
	cache := dnsData.GetCacheRecords()
	cache = dnsrecordcache.Remove(args, cache)
	dnsData.UpdateCacheRecordsInMemory(cache)
}

func cacheClear(args []string) {
	if cliutil.IsHelpRequest(args) {
		printCacheClearUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("cache clear does not accept arguments.")
		printCacheClearUsage()
		return
	}
	dnsData := data.GetInstance()
	dnsData.UpdateCacheRecordsInMemory([]dnsrecordcache.CacheRecord{})
	fmt.Println("Cache cleared.")
}

func cacheLoad(args []string) {
	if cliutil.IsHelpRequest(args) {
		printCacheLoadUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("cache load does not accept arguments.")
		printCacheLoadUsage()
		return
	}
	dnsData := data.GetInstance()
	cache := data.LoadCacheRecords()
	dnsData.UpdateCacheRecordsInMemory(cache)
	fmt.Println("Cache records loaded.")
}

func cacheSave(args []string) {
	if cliutil.IsHelpRequest(args) {
		printCacheSaveUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("cache save does not accept arguments.")
		printCacheSaveUsage()
		return
	}
	dnsData := data.GetInstance()
	cache := dnsData.CacheRecords
	if err := data.SaveCacheRecords(cache); err != nil {
		fmt.Println("Error saving cache records:", err)
		return
	}
	fmt.Println("Cache records saved.")
}

// DNS server list commands
func dnsAdd(args []string) {
	dnsData := data.GetInstance()
	servers := dnsData.DNSServers
	servers = dnsservers.Add(args, servers)
	dnsData.UpdateServers(servers)
}

func dnsRemove(args []string) {
	dnsData := data.GetInstance()
	servers := dnsData.DNSServers
	servers = dnsservers.Remove(args, servers)
	dnsData.UpdateServers(servers)
}

func dnsUpdate(args []string) {
	dnsData := data.GetInstance()
	servers := dnsData.DNSServers
	servers = dnsservers.Update(args, servers)
	dnsData.UpdateServers(servers)
}

func dnsList(args []string) {
	if cliutil.IsHelpRequest(args) {
		printDNSListUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("dns list does not accept arguments.")
		printDNSListUsage()
		return
	}
	dnsData := data.GetInstance()
	servers := dnsData.DNSServers
	dnsservers.List(servers)
}

func dnsClear(args []string) {
	if cliutil.IsHelpRequest(args) {
		printDNSClearUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("dns clear does not accept arguments.")
		printDNSClearUsage()
		return
	}
	dnsData := data.GetInstance()
	dnsData.UpdateServers([]dnsservers.DNSServer{})
	fmt.Println("All DNS servers have been cleared.")
}

func dnsLoad(args []string) {
	if cliutil.IsHelpRequest(args) {
		printDNSLoadUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("dns load does not accept arguments.")
		printDNSLoadUsage()
		return
	}
	dnsData := data.GetInstance()
	servers := data.LoadDNSServers()
	dnsData.UpdateServers(servers)
	fmt.Println("DNS servers loaded.")
}

func dnsSave(args []string) {
	if cliutil.IsHelpRequest(args) {
		printDNSSaveUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("dns save does not accept arguments.")
		printDNSSaveUsage()
		return
	}
	dnsData := data.GetInstance()
	servers := dnsData.DNSServers
	if err := data.SaveDNSServers(servers); err != nil {
		fmt.Println("Error saving DNS servers:", err)
		return
	}
	fmt.Println("DNS servers saved.")
}

// Server commands rely on function variables.
func handleServerLoad(args []string) {
	if cliutil.IsHelpRequest(args) {
		printServerLoadUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("server load does not accept arguments.")
		printServerLoadUsage()
		return
	}
	dnsData := data.GetInstance()
	settings := data.LoadSettings()
	dnsData.UpdateSettings(settings)
	fmt.Println("Server settings loaded.")
}

func handleServerSave(args []string) {
	if cliutil.IsHelpRequest(args) {
		printServerSaveUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("server save does not accept arguments.")
		printServerSaveUsage()
		return
	}
	dnsData := data.GetInstance()
	data.SaveSettings(dnsData.Settings)
	fmt.Println("Server settings saved.")
}

func handleServerStart(args []string) {
	if cliutil.IsHelpRequest(args) {
		printServerStartUsage()
		return
	}
	if len(args) == 0 {
		fmt.Println("Server component to start required.")
		printServerStartUsage()
		return
	}
	dnsData := data.GetInstance()
	settings := dnsData.Settings
	startCommands := map[string]func(){
		"dns": func() {
			if restartDNSServerFunc != nil {
				restartDNSServerFunc(settings.DNSPort)
			}
			fmt.Println("DNS server started.")
		},
		"api": func() {
			if startGinAPIFunc != nil {
				startGinAPIFunc(settings.RESTPort)
			}
			fmt.Println("API server started.")
		},
	}
	component := strings.ToLower(args[0])
	if cmd, ok := startCommands[component]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component to start: %s\n", args[0])
		printServerStartUsage()
	}
}

func handleServerStop(args []string) {
	if cliutil.IsHelpRequest(args) {
		printServerStopUsage()
		return
	}
	if len(args) == 0 {
		fmt.Println("Server component to stop required.")
		printServerStopUsage()
		return
	}
	stopCommands := map[string]func(){
		"dns": func() {
			if stopDNSServerFunc != nil {
				stopDNSServerFunc()
			}
			fmt.Println("DNS server stopped.")
		},
		"api": func() {
			fmt.Println("API server stop not implemented yet.")
		},
	}
	component := strings.ToLower(args[0])
	if cmd, ok := stopCommands[component]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component to stop: %s\n", args[0])
		printServerStopUsage()
	}
}

func handleServerStatus(args []string) {
	if cliutil.IsHelpRequest(args) {
		printServerStatusUsage()
		return
	}
	if len(args) == 0 {
		fmt.Println("Server component required.")
		printServerStatusUsage()
		return
	}
	statusCommands := map[string]func(){
		"dns": func() {
			status := "stopped"
			if getServerStatusFunc != nil && getServerStatusFunc() {
				status = "running"
			}
			fmt.Printf("DNS Server is %s.\n", status)
		},
		"api": func() {
			fmt.Println("API server status not implemented yet.")
		},
	}
	component := strings.ToLower(args[0])
	if cmd, ok := statusCommands[component]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component: %s\n", args[0])
		printServerStatusUsage()
	}
}

func handleServerConfigure(args []string) {
	dnsData := data.GetInstance()
	settings := dnsData.Settings
	if cliutil.IsHelpRequest(args) {
		printServerConfigureUsage()
		return
	}
	if len(args) == 0 {
		fmt.Println("Current Server Configuration:")
		fmt.Printf("DNS Port: %s\n", settings.DNSPort)
		fmt.Printf("API Port: %s\n", settings.RESTPort)
		fmt.Printf("Fallback Server IP: %s\n", settings.FallbackServerIP)
		fmt.Printf("Fallback Server Port: %s\n", settings.FallbackServerPort)
		return
	}
	if len(args) < 2 {
		fmt.Println("server configure requires both setting and value.")
		printServerConfigureUsage()
		return
	}
	if len(args) > 2 {
		fmt.Println("server configure accepts exactly two arguments.")
		printServerConfigureUsage()
		return
	}
	setting := strings.ToLower(args[0])
	value := args[1]
	switch setting {
	case "dns_port":
		settings.DNSPort = value
		fmt.Printf("DNS Port set to %s\n", value)
	case "api_port":
		settings.RESTPort = value
		fmt.Printf("API Port set to %s\n", value)
	case "fallback_ip":
		settings.FallbackServerIP = value
		fmt.Printf("Fallback Server IP set to %s\n", value)
	case "fallback_port":
		settings.FallbackServerPort = value
		fmt.Printf("Fallback Server Port set to %s\n", value)
	default:
		fmt.Printf("Unknown setting: %s\n", setting)
		printServerConfigureUsage()
		return
	}
	dnsData.UpdateSettings(settings)
	fmt.Println("Server configuration updated.")
}

// Stats command
func handleStats(args []string) {
	if cliutil.IsHelpRequest(args) {
		printStatsUsage()
		return
	}
	if len(args) > 0 {
		fmt.Println("stats does not accept arguments.")
		printStatsUsage()
		return
	}
	dnsData := data.GetInstance()
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

// Helper for formatting uptime
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

func printRecordClearUsage() {
	fmt.Println("Usage: record clear")
	fmt.Println("Description: Remove all DNS records from memory.")
	printHelpAliasesHint()
}

func printRecordLoadUsage() {
	fmt.Println("Usage: record load")
	fmt.Println("Description: Load DNS records from the default storage file.")
	printHelpAliasesHint()
}

func printRecordSaveUsage() {
	fmt.Println("Usage: record save")
	fmt.Println("Description: Save current DNS records to the default storage file.")
	printHelpAliasesHint()
}

func printRecordListUsage() {
	fmt.Println("Usage: record list [details|d] [filter]")
	fmt.Println("Description: List DNS records. Use 'details' to include timestamps, or provide a filter by name/type.")
	printHelpAliasesHint()
}

func printCacheListUsage() {
	fmt.Println("Usage: cache list")
	fmt.Println("Description: List all cache entries in memory.")
	printHelpAliasesHint()
}

func printCacheClearUsage() {
	fmt.Println("Usage: cache clear")
	fmt.Println("Description: Remove every cached DNS entry.")
	printHelpAliasesHint()
}

func printCacheLoadUsage() {
	fmt.Println("Usage: cache load")
	fmt.Println("Description: Load cache records from the default storage file.")
	printHelpAliasesHint()
}

func printCacheSaveUsage() {
	fmt.Println("Usage: cache save")
	fmt.Println("Description: Save cache records to the default storage file.")
	printHelpAliasesHint()
}

func printDNSListUsage() {
	fmt.Println("Usage: dns list")
	fmt.Println("Description: Show all configured upstream DNS servers.")
	printHelpAliasesHint()
}

func printDNSClearUsage() {
	fmt.Println("Usage: dns clear")
	fmt.Println("Description: Remove all configured upstream DNS servers.")
	printHelpAliasesHint()
}

func printDNSLoadUsage() {
	fmt.Println("Usage: dns load")
	fmt.Println("Description: Load DNS server definitions from the default storage file.")
	printHelpAliasesHint()
}

func printDNSSaveUsage() {
	fmt.Println("Usage: dns save")
	fmt.Println("Description: Save DNS server definitions to the default storage file.")
	printHelpAliasesHint()
}

func printServerStartUsage() {
	fmt.Println("Usage: server start <dns|api>")
	fmt.Println("Description: Start the specified server component.")
	printHelpAliasesHint()
}

func printServerStopUsage() {
	fmt.Println("Usage: server stop <dns|api>")
	fmt.Println("Description: Stop the specified server component.")
	printHelpAliasesHint()
}

func printServerStatusUsage() {
	fmt.Println("Usage: server status <dns|api>")
	fmt.Println("Description: Show the status of the specified server component.")
	printHelpAliasesHint()
}

func printServerConfigureUsage() {
	fmt.Println("Usage: server configure <dns_port|api_port|fallback_ip|fallback_port> <value>")
	fmt.Println("Description: Update a server configuration setting. Run without arguments to view current settings.")
	printHelpAliasesHint()
}

func printServerLoadUsage() {
	fmt.Println("Usage: server load")
	fmt.Println("Description: Load server settings from the default storage file.")
	printHelpAliasesHint()
}

func printServerSaveUsage() {
	fmt.Println("Usage: server save")
	fmt.Println("Description: Save current server settings to the default storage file.")
	printHelpAliasesHint()
}

func printStatsUsage() {
	fmt.Println("Usage: stats")
	fmt.Println("Description: Display runtime statistics for the resolver.")
	printHelpAliasesHint()
}

func printHelpAliasesHint() {
	fmt.Println("Hint: append '?', 'help', or 'h' after the command to view this usage.")
}
