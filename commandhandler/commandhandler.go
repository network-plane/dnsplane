package commandhandler

import (
	"dnsresolver/data"
	"dnsresolver/dnsrecordcache"
	"dnsresolver/dnsrecords"
	"dnsresolver/dnsservers"
	"dnsresolver/tui"
	"fmt"
	"time"
)

// Function variables for server control
var (
	stopDNSServerFunc    func()
	restartDNSServerFunc func(string)
	getServerStatusFunc  func() bool
	startMDNSServerFunc  func(string)
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
	dnsData := data.GetInstance()
	records := dnsData.DNSRecords
	if len(args) > 0 && args[0] != "d" && args[0] != "details" {
		if args[0] == "?" || args[0] == "h" || args[0] == "help" {
			fmt.Println("Usage: record list [details] [filter]")
			fmt.Println("  details: Show detailed information")
			fmt.Println("  filter: Filter records by name or type")
			return
		}
		fmt.Println("Filtering records by:", args[0])
	}
	dnsrecords.List(records, args)
}

func recordClear(args []string) {
	dnsData := data.GetInstance()
	dnsData.UpdateRecordsInMemory([]dnsrecords.DNSRecord{})
	fmt.Println("All DNS records have been cleared.")
}

func recordLoad(args []string) {
	dnsData := data.GetInstance()
	records := data.LoadDNSRecords()
	dnsData.UpdateRecords(records)
	fmt.Println("DNS records loaded.")
}

func recordSave(args []string) {
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
	dnsData := data.GetInstance()
	dnsData.UpdateCacheRecordsInMemory([]dnsrecordcache.CacheRecord{})
	fmt.Println("Cache cleared.")
}

func cacheLoad(args []string) {
	dnsData := data.GetInstance()
	cache := data.LoadCacheRecords()
	dnsData.UpdateCacheRecordsInMemory(cache)
	fmt.Println("Cache records loaded.")
}

func cacheSave(args []string) {
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
	dnsData := data.GetInstance()
	servers := dnsData.DNSServers
	dnsservers.List(servers)
}

func dnsClear(args []string) {
	dnsData := data.GetInstance()
	dnsData.UpdateServers([]dnsservers.DNSServer{})
	fmt.Println("All DNS servers have been cleared.")
}

func dnsLoad(args []string) {
	dnsData := data.GetInstance()
	servers := data.LoadDNSServers()
	dnsData.UpdateServers(servers)
	fmt.Println("DNS servers loaded.")
}

func dnsSave(args []string) {
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
	dnsData := data.GetInstance()
	settings := data.LoadSettings()
	dnsData.UpdateSettings(settings)
	fmt.Println("Server settings loaded.")
}

func handleServerSave(args []string) {
	dnsData := data.GetInstance()
	data.SaveSettings(dnsData.Settings)
	fmt.Println("Server settings saved.")
}

func handleServerStart(args []string) {
	if len(args) == 0 {
		fmt.Println("Server component to start required. Use 'server start ?' for help.")
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
		"mdns": func() {
			if startMDNSServerFunc != nil {
				startMDNSServerFunc(settings.MDNSPort)
			}
			fmt.Println("mDNS server started.")
		},
		"api": func() {
			if startGinAPIFunc != nil {
				startGinAPIFunc(settings.RESTPort)
			}
			fmt.Println("API server started.")
		},
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
		"dns": func() {
			if stopDNSServerFunc != nil {
				stopDNSServerFunc()
			}
			fmt.Println("DNS server stopped.")
		},
		"mdns": func() {
			fmt.Println("mDNS server stop not implemented yet.")
		},
		"api": func() {
			fmt.Println("API server stop not implemented yet.")
		},
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
		"dns": func() {
			status := "stopped"
			if getServerStatusFunc != nil && getServerStatusFunc() {
				status = "running"
			}
			fmt.Printf("DNS Server is %s.\n", status)
		},
		"mdns": func() {
			fmt.Println("mDNS server status not implemented yet.")
		},
		"api": func() {
			fmt.Println("API server status not implemented yet.")
		},
	}
	if cmd, ok := statusCommands[args[0]]; ok {
		cmd()
	} else {
		fmt.Printf("Unknown component: %s. Use 'server status ?' for help.\n", args[0])
	}
}

func handleServerConfigure(args []string) {
	dnsData := data.GetInstance()
	settings := dnsData.Settings
	if len(args) == 0 {
		fmt.Println("Current Server Configuration:")
		fmt.Printf("DNS Port: %s\n", settings.DNSPort)
		fmt.Printf("mDNS Port: %s\n", settings.MDNSPort)
		fmt.Printf("API Port: %s\n", settings.RESTPort)
		fmt.Printf("Fallback Server IP: %s\n", settings.FallbackServerIP)
		fmt.Printf("Fallback Server Port: %s\n", settings.FallbackServerPort)
		return
	}
	if len(args) < 2 {
		fmt.Println("Usage: server configure [setting] [value]")
		return
	}
	setting := args[0]
	value := args[1]
	switch setting {
	case "dns_port":
		settings.DNSPort = value
		fmt.Printf("DNS Port set to %s\n", value)
	case "mdns_port":
		settings.MDNSPort = value
		fmt.Printf("mDNS Port set to %s\n", value)
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
		return
	}
	dnsData.UpdateSettings(settings)
	fmt.Println("Server configuration updated.")
}

// Stats command
func handleStats(args []string) {
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
