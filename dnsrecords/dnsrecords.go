// Package dnsrecords contains the functions to add, list, remove, clear and update DNS records.
package dnsrecords

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dnsresolver/cliutil"
	"dnsresolver/converters"
	"dnsresolver/ipvalidator"

	"github.com/miekg/dns"
)

// DNSRecord holds the data for a DNS record
type DNSRecord struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Value       string    `json:"value"`
	TTL         uint32    `json:"ttl"`
	AddedOn     time.Time `json:"added_on,omitempty"`
	UpdatedOn   time.Time `json:"updated_on,omitempty"`
	MACAddress  string    `json:"mac,omitempty"`
	CacheRecord bool      `json:"cache_record,omitempty"`
	LastQuery   time.Time `json:"last_query,omitempty"`
}

var (
	// ErrHelpRequested indicates the caller asked for usage information.
	ErrHelpRequested = errors.New("help requested")
	// ErrInvalidArgs indicates user-provided arguments were invalid.
	ErrInvalidArgs = errors.New("invalid arguments")
)

// Level represents a message severity level returned from operations.
type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Message conveys informational output from record operations.
type Message struct {
	Level Level
	Text  string
}

// ListResult captures the outcome of listing DNS records.
type ListResult struct {
	Records  []DNSRecord
	Detailed bool
	Filter   string
	Messages []Message
}

// findDNSRecordIndex returns the index of the DNSRecord in dnsRecords
// that matches the given name, type, and value. If no match is found, it returns -1.
func findDNSRecordIndex(dnsRecords []DNSRecord, name, recordType, value string) int {
	targetName := normalizeRecordNameKey(name)
	targetType := normalizeRecordType(recordType)
	targetValue := normalizeRecordValueKey(targetType, value)

	for i, record := range dnsRecords {
		if normalizeRecordNameKey(record.Name) == targetName &&
			normalizeRecordType(record.Type) == targetType &&
			normalizeRecordValueKey(record.Type, record.Value) == targetValue {
			return i
		}
	}
	return -1
}

func normalizeRecordType(recordType string) string {
	return strings.ToUpper(strings.TrimSpace(recordType))
}

func normalizeRecordNameKey(name string) string {
	name = strings.TrimSpace(name)
	for strings.HasSuffix(name, ".") {
		name = strings.TrimSuffix(name, ".")
	}
	return strings.ToLower(name)
}

func normalizeRecordValueKey(recordType, value string) string {
	recordType = normalizeRecordType(recordType)
	value = strings.TrimSpace(value)
	if recordType == "" {
		return value
	}
	switch recordType {
	case "CNAME", "NS", "PTR":
		return normalizeRecordNameKey(value)
	case "A", "AAAA":
		return strings.ToLower(value)
	default:
		return value
	}
}

// Exported helpers for re-use in other packages.
func NormalizeRecordNameKey(name string) string {
	return normalizeRecordNameKey(name)
}

func NormalizeRecordType(recordType string) string {
	return normalizeRecordType(recordType)
}

func NormalizeRecordValueKey(recordType, value string) string {
	return normalizeRecordValueKey(recordType, value)
}

// Add inserts a DNS record or updates an existing one when allowed. It returns the
// updated slice alongside informational messages.
func Add(fullCommand []string, dnsRecords []DNSRecord, allowUpdate bool) ([]DNSRecord, []Message, error) {
	messages := make([]Message, 0)
	if checkHelpCommand(fullCommand) {
		return dnsRecords, usageAdd(), ErrHelpRequested
	}

	dnsRecord, err := parseDNSRecordArgs(fullCommand)
	if err != nil {
		msgs := append([]Message{{Level: LevelError, Text: err.Error()}}, usageAdd()...)
		return dnsRecords, msgs, ErrInvalidArgs
	}
	dnsRecord.Type = normalizeRecordType(dnsRecord.Type)
	dnsRecord.AddedOn = time.Now()

	existingIndex := findDNSRecordIndex(dnsRecords, dnsRecord.Name, dnsRecord.Type, dnsRecord.Value)
	if existingIndex != -1 {
		oldRecord := dnsRecords[existingIndex]
		if allowUpdate {
			dnsRecord.UpdatedOn = time.Now()
			dnsRecords[existingIndex] = dnsRecord
			updatedRecToPrint := converters.ConvertValuesToStrings(
				converters.GetFieldValuesByNamesArray(dnsRecord,
					[]string{"Name", "Type", "Value", "TTL"}))
			oldRecToPrint := converters.ConvertValuesToStrings(
				converters.GetFieldValuesByNamesArray(oldRecord,
					[]string{"Name", "Type", "Value", "TTL"}))
			messages = append(messages,
				Message{Level: LevelInfo, Text: "Existing record found. Updating..."},
				Message{Level: LevelInfo, Text: fmt.Sprintf("Previous: %v", oldRecToPrint)},
				Message{Level: LevelInfo, Text: fmt.Sprintf("Current : %v", updatedRecToPrint)},
			)
			return dnsRecords, messages, nil
		}
		attemptedRec := converters.ConvertValuesToStrings(
			converters.GetFieldValuesByNamesArray(dnsRecord,
				[]string{"Name", "Type", "Value", "TTL"}))
		existingRec := converters.ConvertValuesToStrings(
			converters.GetFieldValuesByNamesArray(oldRecord,
				[]string{"Name", "Type", "Value", "TTL"}))
		messages = append(messages,
			Message{Level: LevelWarn, Text: "A record already exists."},
			Message{Level: LevelWarn, Text: fmt.Sprintf("Attempted: %v", attemptedRec)},
			Message{Level: LevelWarn, Text: fmt.Sprintf("Current  : %v", existingRec)},
		)
		return dnsRecords, messages, nil
	}

	dnsRecords = append(dnsRecords, dnsRecord)
	addedRec := converters.ConvertValuesToStrings(
		converters.GetFieldValuesByNamesArray(dnsRecord,
			[]string{"Name", "Type", "Value", "TTL"}))
	messages = append(messages, Message{Level: LevelInfo, Text: fmt.Sprintf("Added: %v", addedRec)})
	return dnsRecords, messages, nil
}

// List prepares a view of DNS records along with parsing options from args.
func List(dnsRecords []DNSRecord, args []string) (ListResult, error) {
	result := ListResult{Records: dnsRecords}
	if checkHelpCommand(args) {
		result.Messages = usageList()
		return result, ErrHelpRequested
	}

	if len(args) > 0 {
		if args[0] == "details" || args[0] == "d" {
			result.Detailed = true
			if len(args) > 1 {
				result.Filter = args[1]
			}
		} else {
			result.Filter = args[0]
		}
	}

	if result.Filter != "" {
		result.Messages = append(result.Messages, Message{Level: LevelInfo, Text: fmt.Sprintf("Filtering records by: %s", result.Filter)})
	}

	if len(result.Records) == 0 {
		result.Messages = append(result.Messages, Message{Level: LevelInfo, Text: "No records found."})
	}

	return result, nil
}

// Remove deletes a DNS record from the list when found.
func Remove(fullCommand []string, dnsRecords []DNSRecord) ([]DNSRecord, []Message, error) {
	messages := make([]Message, 0)
	if checkHelpCommand(fullCommand) {
		return dnsRecords, usageRemove(), ErrHelpRequested
	}

	if len(fullCommand) < 2 || len(fullCommand) > 3 {
		msgs := append([]Message{{Level: LevelError, Text: "invalid record format. Please use: remove <Name> [Type] <Value>"}}, usageRemove()...)
		return dnsRecords, msgs, ErrInvalidArgs
	}

	name := strings.TrimSpace(fullCommand[0])
	recordType := ""
	value := ""

	switch len(fullCommand) {
	case 2:
		var detectedType string
		name, value, detectedType = validateIPAndDomain(fullCommand[0], fullCommand[1])
		if name == "" || detectedType == "" {
			msgs := append([]Message{{Level: LevelError, Text: "invalid record format. Please use: remove <Name> [Type] <Value>"}}, usageRemove()...)
			return dnsRecords, msgs, ErrInvalidArgs
		}
		recordType = detectedType
	case 3:
		recordType = fullCommand[1]
		value = fullCommand[2]
	}

	if name == "" || recordType == "" || value == "" {
		msgs := append([]Message{{Level: LevelError, Text: "invalid record format. Please use: remove <Name> [Type] <Value>"}}, usageRemove()...)
		return dnsRecords, msgs, ErrInvalidArgs
	}

	normType := normalizeRecordType(recordType)
	targetName := normalizeRecordNameKey(name)
	targetValue := normalizeRecordValueKey(normType, value)

	existingIndex := -1
	for i, record := range dnsRecords {
		if normalizeRecordNameKey(record.Name) == targetName &&
			normalizeRecordType(record.Type) == normType &&
			normalizeRecordValueKey(record.Type, record.Value) == targetValue {
			existingIndex = i
			break
		}
	}

	if existingIndex == -1 {
		msg := Message{Level: LevelWarn, Text: fmt.Sprintf("No record found for [%s %s %s].", name, recordType, value)}
		msgs := append([]Message{msg}, usageRemove()...)
		return dnsRecords, msgs, ErrInvalidArgs
	}

	removedRecord := dnsRecords[existingIndex]
	removedRec := converters.ConvertValuesToStrings(
		converters.GetFieldValuesByNamesArray(removedRecord,
			[]string{"Name", "Type", "Value", "TTL"}))

	dnsRecords = append(dnsRecords[:existingIndex], dnsRecords[existingIndex+1:]...)
	messages = append(messages, Message{Level: LevelInfo, Text: fmt.Sprintf("Removed: %v", removedRec)})
	return dnsRecords, messages, nil
}

// Helper function to check if the help command is invoked.
func checkHelpCommand(fullCommand []string) bool {
	return len(fullCommand) == 0 || cliutil.IsHelpRequest(fullCommand)
}

func usageAdd() []Message {
	msgs := []Message{
		{Level: LevelInfo, Text: "Usage  : add <Name> [Type] <Value> [TTL]"},
		{Level: LevelInfo, Text: "Examples:"},
		{Level: LevelInfo, Text: "  add example.com 127.0.0.1"},
		{Level: LevelInfo, Text: "  add example.com A 127.0.0.1"},
		{Level: LevelInfo, Text: "  add example.com A 127.0.0.1 3600"},
	}
	return append(msgs, helpHint())
}

func usageRemove() []Message {
	msgs := []Message{
		{Level: LevelInfo, Text: "Usage  : remove <Name> [Type] <Value>"},
		{Level: LevelInfo, Text: "Examples:"},
		{Level: LevelInfo, Text: "  remove example.com 127.0.0.1"},
		{Level: LevelInfo, Text: "  remove example.com A 127.0.0.1"},
	}
	return append(msgs, helpHint())
}

func usageList() []Message {
	msgs := []Message{
		{Level: LevelInfo, Text: "Usage  : record list [details|d] [filter]"},
		{Level: LevelInfo, Text: "Description: List DNS records. Use 'details' to include timestamps, or provide a filter by name/type."},
	}
	return append(msgs, helpHint())
}

func helpHint() Message {
	return Message{Level: LevelInfo, Text: "Hint: append '?', 'help', or 'h' after the command to view this usage."}
}

// ipvToRecordType returns the DNS record type for the given IP version.
func ipvToRecordType(ipversion int) string {
	if ipversion == 4 {
		return "A"
	} else if ipversion == 6 {
		return "AAAA"
	}
	return ""
}

// validateIPAndDomain attempts to validate the arguments as IP and domain.
func validateIPAndDomain(arg1, arg2 string) (string, string, string) {
	// First attempt: arg1 as IP, arg2 as domain
	if ipvalidator.IsValidIP(arg1) {
		_, isDomain := dns.IsDomainName(arg2)
		if isDomain {
			return strings.TrimSpace(arg2), strings.TrimSpace(arg1), ipvToRecordType(ipvalidator.GetIPVersion(arg1)) // Return domain, IP, version
		}
	}

	// Second attempt: arg1 as domain, arg2 as IP
	if ipvalidator.IsValidIP(arg2) {
		_, isDomain := dns.IsDomainName(arg1)
		if isDomain {
			return strings.TrimSpace(arg1), strings.TrimSpace(arg2), ipvToRecordType(ipvalidator.GetIPVersion(arg2)) // Return domain, IP, version
		}
	}

	// If neither combination works, return empty strings
	return "", "", ""
}

// Helper function to parse DNS record arguments and return a DNSRecord struct.
func parseDNSRecordArgs(args []string) (DNSRecord, error) {
	if len(args) < 2 {
		return DNSRecord{}, fmt.Errorf("invalid DNS record format. Please enter the DNS record in the format: <Name> [Type] <Value> [TTL]")
	}

	var (
		name       string
		recordType string
		value      string
		ttlStr     string
	)

	switch {
	case len(args) == 2:
		name, value, recordType = validateIPAndDomain(args[0], args[1])
		if name == "" || recordType == "" || value == "" {
			return DNSRecord{}, fmt.Errorf("invalid DNS record format. Please enter the DNS record in the format: <Name> [Type] <Value> [TTL]")
		}
		ttlStr = "3600"
	case len(args) >= 3:
		name = args[0]
		recordType = strings.ToUpper(args[1])
		value = args[2]
		if len(args) >= 4 {
			ttlStr = args[3]
		} else {
			ttlStr = "3600"
		}
		if len(args) > 4 {
			return DNSRecord{}, fmt.Errorf("invalid DNS record format. Please enter the DNS record in the format: <Name> [Type] <Value> [TTL]")
		}
	}

	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	recordType = normalizeRecordType(recordType)

	// Validate DNS record type against known types
	if _, ok := dns.StringToType[recordType]; !ok {
		return DNSRecord{}, fmt.Errorf("invalid DNS record type: %s", recordType)
	}

	// Use a switch to handle record-specific validations
	switch recordType {
	case "A", "AAAA":
		// For A/AAAA, ensure value is a valid IP
		if !ipvalidator.IsValidIP(value) {
			return DNSRecord{}, fmt.Errorf("invalid IP address: %s", value)
		}
	case "CNAME", "NS", "PTR", "TXT", "SRV", "SOA", "MX", "NAPTR", "CAA", "TLSA", "DS", "DNSKEY", "RRSIG", "NSEC", "NSEC3", "NSEC3PARAM":
		// For these types, ensure value is a valid domain name
		if _, ok := dns.IsDomainName(value); !ok {
			return DNSRecord{}, fmt.Errorf("invalid domain name: %s", value)
		}
	default:
		// For all other types, we don't need to validate the value for now
	}

	// Finally, parse the TTL
	ttl64, err := strconv.ParseUint(ttlStr, 10, 32)
	if err != nil {
		return DNSRecord{}, fmt.Errorf("invalid TTL value: %s", ttlStr)
	}
	ttl := uint32(ttl64)

	dnsRecord := DNSRecord{
		Name:  name,
		Type:  recordType,
		Value: value,
		TTL:   ttl,
	}

	return dnsRecord, nil
}

// FindRecord searches for a DNS record in the list of DNS records.
func FindRecord(dnsRecords []DNSRecord, lookupRecord, recordType string, autoBuildPTRFromA bool) *dns.RR {
	for _, record := range dnsRecords {
		if record.Type == "PTR" || (recordType == "PTR" && autoBuildPTRFromA) {
			if record.Value == lookupRecord {
				recordString := fmt.Sprintf("%s %d IN PTR %s.", converters.ConvertIPToReverseDNS(lookupRecord), record.TTL, strings.TrimRight(record.Name, "."))
				fmt.Println("recordstring", recordString)

				rr := recordString
				dnsRecord, err := dns.NewRR(rr)
				if err != nil {
					fmt.Println("Error creating PTR record", err)
					return nil // Error handling if the PTR record can't be created
				}
				// fmt.Println(dnsRecord.String())
				return &dnsRecord
			}
		}

		if record.Name == lookupRecord && record.Type == recordType {
			rr := fmt.Sprintf("%s %d IN %s %s", record.Name, record.TTL, record.Type, record.Value)
			dnsRecord, err := dns.NewRR(rr)
			if err != nil {
				return nil
			}
			return &dnsRecord
		}
	}
	return nil
}
