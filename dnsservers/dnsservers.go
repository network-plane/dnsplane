// Package dnsservers provides the data structure and functions for managing DNS servers.
package dnsservers

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"dnsplane/cliutil"
)

// DNSServer holds the data for a DNS server
type DNSServer struct {
	Address       string    `json:"address"`
	Port          string    `json:"port"`
	Active        bool      `json:"active"`
	LocalResolver bool      `json:"local_resolver"`
	AdBlocker     bool      `json:"adblocker"`
	LastUsed      time.Time `json:"last_used,omitempty"`
	LastSuccess   time.Time `json:"last_success,omitempty"`
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

// Helper function to parse and apply command arguments to a DNSServer.

func applyArgsToDNSServer(server *DNSServer, args []string) error {
	if len(args) >= 1 {
		server.Address = args[0]
		if net.ParseIP(server.Address) == nil {
			return fmt.Errorf("invalid IP address: %s", server.Address)
		}
	} else {
		return fmt.Errorf("address is required")
	}

	if len(args) >= 2 {
		if _, err := strconv.Atoi(args[1]); err != nil {
			return fmt.Errorf("invalid port: %s", args[1])
		}
		server.Port = args[1]
	}

	boolFields := []struct {
		name     string
		index    int
		assignTo *bool
	}{
		{"Active", 2, &server.Active},
		{"LocalResolver", 3, &server.LocalResolver},
		{"AdBlocker", 4, &server.AdBlocker},
	}

	for _, field := range boolFields {
		if len(args) > field.index {
			value, err := strconv.ParseBool(args[field.index])
			if err != nil {
				return fmt.Errorf("invalid value for %s: %s", field.name, args[field.index])
			}
			*field.assignTo = value
		}
	}

	maxArgs := 2 + len(boolFields)
	if len(args) > maxArgs {
		return fmt.Errorf("too many arguments provided; expected at most %d parameters", maxArgs)
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

// Helper function to handle the help command.
func usageAdd() []Message {
	msgs := []Message{
		{Level: LevelInfo, Text: "Usage  : add <Address> [Port] [Active] [LocalResolver] [AdBlocker]"},
		{Level: LevelInfo, Text: "Example: add 1.1.1.1 53 true false false"},
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
		{Level: LevelInfo, Text: "Usage  : update <Address> [Port] [Active] [LocalResolver] [AdBlocker]"},
		{Level: LevelInfo, Text: "Example: update 1.1.1.1 53 false true true"},
	}
	return append(msgs, helpHint())
}

func helpHint() Message {
	return Message{Level: LevelInfo, Text: "Hint: append '?', 'help', or 'h' after the command to view this usage."}
}
