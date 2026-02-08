// Package dnsservers provides the data structure and functions for managing DNS servers.
package dnsservers

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"dnsplane/cliutil"
)

// DNSServer holds the data for a DNS server.
// DomainWhitelist is optional: if non-empty, this server is only used for query names
// that match one of the entries (exact or suffix, e.g. internal.example.com matches
// api.internal.example.com). Empty or nil means the server is "global" and receives
// all queries not claimed by a whitelisted server.
type DNSServer struct {
	Address          string    `json:"address"`
	Port             string    `json:"port"`
	Active           bool      `json:"active"`
	LocalResolver    bool      `json:"local_resolver"`
	AdBlocker        bool      `json:"adblocker"`
	DomainWhitelist  []string  `json:"domain_whitelist,omitempty"`
	LastUsed         time.Time `json:"last_used,omitempty"`
	LastSuccess      time.Time `json:"last_success,omitempty"`
}

var (
	ErrHelpRequested = errors.New("help requested")
	ErrInvalidArgs   = errors.New("invalid arguments")
)

type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Message struct {
	Level Level
	Text  string
}

type ListResult struct {
	Servers  []DNSServer
	Messages []Message
}

// GetDNSArray returns an array of DNS servers in the format "Address:Port".
func GetDNSArray(dnsServerData []DNSServer, activeOnly bool) []string {
	var dnsArray []string
	for _, dnsServer := range dnsServerData {
		if activeOnly && !dnsServer.Active {
			continue
		}
		dnsArray = append(dnsArray, dnsServer.Address+":"+dnsServer.Port)
	}
	return dnsArray
}

// normalizeQueryNameForWhitelist returns the query name normalized for whitelist matching (lowercase, trailing dot stripped).
func normalizeQueryNameForWhitelist(queryName string) string {
	s := strings.TrimSpace(queryName)
	for strings.HasSuffix(s, ".") {
		s = strings.TrimSuffix(s, ".")
	}
	return strings.ToLower(s)
}

// ServerMatchesQuery reports whether the query name matches the server's domain whitelist.
// Query name is normalized (lowercase, no trailing dot). Match is exact or suffix:
// if whitelist contains "internal.example.com", then "internal.example.com" and "api.internal.example.com" match.
// If the server has no whitelist (nil or empty), this returns false (server is global, not domain-specific).
func ServerMatchesQuery(server DNSServer, queryName string) bool {
	if len(server.DomainWhitelist) == 0 {
		return false
	}
	q := normalizeQueryNameForWhitelist(queryName)
	if q == "" {
		return false
	}
	for _, entry := range server.DomainWhitelist {
		e := strings.ToLower(strings.TrimSpace(entry))
		for strings.HasSuffix(e, ".") {
			e = strings.TrimSuffix(e, ".")
		}
		if e == "" {
			continue
		}
		if q == e {
			return true
		}
		if strings.HasSuffix(q, "."+e) {
			return true
		}
	}
	return false
}

// GetServersForQuery returns the list of "Address:Port" servers to use for the given query name.
// If any server has a non-empty whitelist and matches the query, only active matching servers are returned (whitelist-only; no fallback to global).
// If the query matches a whitelist but no matching server is active, returns nil (strict: do not use global servers).
// Otherwise returns global servers only (servers with no whitelist or empty whitelist).
// activeOnly filters to Active servers in both cases.
func GetServersForQuery(dnsServerData []DNSServer, queryName string, activeOnly bool) []string {
	var whitelisted []string
	for _, s := range dnsServerData {
		if activeOnly && !s.Active {
			continue
		}
		if ServerMatchesQuery(s, queryName) {
			whitelisted = append(whitelisted, s.Address+":"+s.Port)
		}
	}
	if len(whitelisted) > 0 {
		return whitelisted
	}
	// Query matched a whitelist but no active server? (e.g. only matching server is inactive) → return none
	for _, s := range dnsServerData {
		if ServerMatchesQuery(s, queryName) {
			return nil
		}
	}
	// Global: servers with no whitelist
	var global []string
	for _, s := range dnsServerData {
		if activeOnly && !s.Active {
			continue
		}
		if len(s.DomainWhitelist) == 0 {
			global = append(global, s.Address+":"+s.Port)
		}
	}
	return global
}

// Add adds a DNS server to the list, returning the updated slice and messages.
func Add(fullCommand []string, dnsServers []DNSServer) ([]DNSServer, []Message, error) {
	messages := make([]Message, 0)
	if cliutil.IsHelpRequest(fullCommand) {
		return dnsServers, usageAdd(), ErrHelpRequested
	}

	server := DNSServer{
		Port:          "53",
		Active:        true,
		LocalResolver: true,
		AdBlocker:     false,
	}

	if err := applyArgsToDNSServer(&server, fullCommand); err != nil {
		msgs := append([]Message{{Level: LevelError, Text: err.Error()}}, usageAdd()...)
		return dnsServers, msgs, ErrInvalidArgs
	}

	dnsServers = append(dnsServers, server)
	messages = append(messages, Message{Level: LevelInfo, Text: fmt.Sprintf("Added DNS server: %s:%s", server.Address, server.Port)})
	return dnsServers, messages, nil
}

// Remove removes a DNS server from the list of DNS servers.

func Remove(fullCommand []string, dnsServerData []DNSServer) ([]DNSServer, []Message, error) {
	messages := make([]Message, 0)
	if cliutil.IsHelpRequest(fullCommand) {
		return dnsServerData, usageRemove(), ErrHelpRequested
	}

	if len(fullCommand) != 1 {
		msgs := append([]Message{{Level: LevelError, Text: "address is required"}}, usageRemove()...)
		return dnsServerData, msgs, ErrInvalidArgs
	}

	address := fullCommand[0]
	index := findDNSServerIndex(dnsServerData, address)
	if index == -1 {
		msgs := append([]Message{{Level: LevelWarn, Text: fmt.Sprintf("No DNS server found with the address: %s", address)}}, usageRemove()...)
		return dnsServerData, msgs, ErrInvalidArgs
	}

	dnsServerData = append(dnsServerData[:index], dnsServerData[index+1:]...)
	messages = append(messages, Message{Level: LevelInfo, Text: fmt.Sprintf("Removed DNS server: %s", address)})
	return dnsServerData, messages, nil
}

// Update modifies a DNS server's record in the list of DNS servers.

func Update(fullCommand []string, dnsServerData []DNSServer) ([]DNSServer, []Message, error) {
	messages := make([]Message, 0)
	if cliutil.IsHelpRequest(fullCommand) {
		return dnsServerData, usageUpdate(), ErrHelpRequested
	}

	if len(fullCommand) < 1 {
		msgs := append([]Message{{Level: LevelError, Text: "address is required."}}, usageUpdate()...)
		return dnsServerData, msgs, ErrInvalidArgs
	}

	address := fullCommand[0]
	index := findDNSServerIndex(dnsServerData, address)
	if index == -1 {
		msgs := append([]Message{{Level: LevelWarn, Text: fmt.Sprintf("DNS server not found: %s", address)}}, usageUpdate()...)
		return dnsServerData, msgs, ErrInvalidArgs
	}

	server := dnsServerData[index]
	if err := applyArgsToDNSServer(&server, fullCommand); err != nil {
		msgs := append([]Message{{Level: LevelError, Text: err.Error()}}, usageUpdate()...)
		return dnsServerData, msgs, ErrInvalidArgs
	}

	dnsServerData[index] = server
	messages = append(messages, Message{Level: LevelInfo, Text: fmt.Sprintf("Updated DNS server: %s", address)})
	return dnsServerData, messages, nil
}

// List lists all the DNS servers in the list of DNS servers.
func List(dnsServerData []DNSServer) ListResult {
	result := ListResult{Servers: dnsServerData}
	if len(dnsServerData) == 0 {
		result.Messages = append(result.Messages, Message{Level: LevelInfo, Text: "No DNS servers found."})
	}
	return result
}

// applyArgsToDNSServer parses args into a DNSServer. Required ordered args: address; optional second arg is port if it does not contain ':'.
// Remaining args are named parameters (order-independent): active:true|false, localresolver:true|false, adblocker:true|false, whitelist:suffix1,suffix2
func applyArgsToDNSServer(server *DNSServer, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("address is required")
	}
	server.Address = args[0]
	if net.ParseIP(server.Address) == nil {
		return fmt.Errorf("invalid IP address: %s", server.Address)
	}

	rest := args[1:]
	// Optional positional port: only if next arg exists and is not a named param (no ':')
	if len(rest) >= 1 && !strings.Contains(rest[0], ":") {
		port := rest[0]
		if _, err := strconv.Atoi(port); err != nil {
			return fmt.Errorf("invalid port: %s", port)
		}
		server.Port = port
		rest = rest[1:]
	}

	for _, arg := range rest {
		idx := strings.Index(arg, ":")
		if idx <= 0 {
			return fmt.Errorf("unexpected argument %q; use named parameters (e.g. active:true whitelist:example.com)", arg)
		}
		key := strings.ToLower(strings.TrimSpace(arg[:idx]))
		val := strings.TrimSpace(arg[idx+1:])
		switch key {
		case "active":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf("invalid active value: %s", val)
			}
			server.Active = b
		case "localresolver":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf("invalid localresolver value: %s", val)
			}
			server.LocalResolver = b
		case "adblocker":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf("invalid adblocker value: %s", val)
			}
			server.AdBlocker = b
		case "whitelist":
			if val == "" {
				server.DomainWhitelist = nil
			} else {
				parts := strings.Split(val, ",")
				server.DomainWhitelist = make([]string, 0, len(parts))
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						server.DomainWhitelist = append(server.DomainWhitelist, p)
					}
				}
			}
		default:
			return fmt.Errorf("unknown parameter %q; allowed: active, localresolver, adblocker, whitelist", key)
		}
	}
	return nil
}

// Helper function to find the index of a DNSServer by address.
func findDNSServerIndex(dnsServers []DNSServer, address string) int {
	for i, server := range dnsServers {
		if server.Address == address {
			return i
		}
	}
	return -1
}

// UsageAdd returns the usage messages for the add command (for TUI help).
func UsageAdd() []Message {
	return usageAdd()
}

// UsageUpdate returns the usage messages for the update command (for TUI help).
func UsageUpdate() []Message {
	return usageUpdate()
}

// UsageRemove returns the usage messages for the remove command (for TUI help).
func UsageRemove() []Message {
	return usageRemove()
}

// Helper function to handle the help command.
func usageAdd() []Message {
	msgs := []Message{
		{Level: LevelInfo, Text: "Usage  : add <Address> [Port] [active:true|false] [localresolver:true|false] [adblocker:true|false] [whitelist:suffix1,suffix2,...]"},
		{Level: LevelInfo, Text: "Example: add 1.1.1.1 53"},
		{Level: LevelInfo, Text: "Example: add 192.168.5.5 53 active:true localresolver:true adblocker:false whitelist:example.com,example.org"},
	}
	return append(msgs, helpHint())
}

func usageRemove() []Message {
	msgs := []Message{
		{Level: LevelInfo, Text: "Usage  : remove <Address>"},
		{Level: LevelInfo, Text: "Example: remove 127.0.0.1"},
	}
	return append(msgs, helpHint())
}

func usageUpdate() []Message {
	msgs := []Message{
		{Level: LevelInfo, Text: "Usage  : update <Address> [Port] [active:true|false] [localresolver:true|false] [adblocker:true|false] [whitelist:suffix1,suffix2,...]"},
		{Level: LevelInfo, Text: "Example: update 1.1.1.1 53 active:false"},
		{Level: LevelInfo, Text: "Example: update 192.168.5.5 adblocker:true whitelist:example.com,example.org"},
	}
	return append(msgs, helpHint())
}

func helpHint() Message {
	return Message{Level: LevelInfo, Text: "Hint: append '?', 'help', or 'h' after the command to view this usage."}
}
