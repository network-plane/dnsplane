// Package dnsrecords contains the functions to add, list, remove, clear and update DNS records.
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package dnsrecords

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"dnsplane/cliutil"
	"dnsplane/converters"
	"dnsplane/ipvalidator"

	"github.com/miekg/dns"
)

// DNSRecord holds the data for a DNS record
type DNSRecord struct {
	ID          string    `json:"id,omitempty"`
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

// NewRecordID returns a new RFC 4122-style UUID (v4) as a lowercase hex string.
func NewRecordID() string {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		// Unlikely; still produce a distinct value.
		now := time.Now().UnixNano()
		for i := 0; i < 8; i++ {
			b[i] = byte(now >> (8 * i))
		}
		_, _ = rand.Read(b[8:])
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b[:])
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32]
}

func findDNSRecordIndexByID(dnsRecords []DNSRecord, id string) int {
	id = strings.TrimSpace(id)
	if id == "" {
		return -1
	}
	for i, record := range dnsRecords {
		if strings.TrimSpace(record.ID) == id {
			return i
		}
	}
	return -1
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

// canonicalizeRecordNameForStorage trims space and strips trailing dots so stored names are consistent (no FQDN trailing dot).
func canonicalizeRecordNameForStorage(name string) string {
	name = strings.TrimSpace(name)
	// TrimRight (not a per-dot loop): a long run of '.' is O(n) once, avoiding pathological CPU on hostile input.
	return strings.TrimRight(name, ".")
}

// CanonicalizeRecordNameForStorage is the exported form for use when loading records from file.
func CanonicalizeRecordNameForStorage(name string) string {
	return canonicalizeRecordNameForStorage(name)
}

func normalizeRecordNameKey(name string) string {
	name = canonicalizeRecordNameForStorage(name)
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

// FilterRecords returns records matching optional name substring and optional exact DNS type.
// nameSubstr: if non-empty, the normalized record owner name must contain the normalized substring.
// typ: if non-empty, must be a valid DNS type (case-insensitive); the record's type must match.
// Both conditions are combined with AND when both are set.
func FilterRecords(records []DNSRecord, nameSubstr, typ string) ([]DNSRecord, error) {
	typ = strings.TrimSpace(typ)
	if typ != "" {
		nt := normalizeRecordType(typ)
		if _, ok := dns.StringToType[nt]; !ok {
			return nil, fmt.Errorf("%w: invalid DNS type %q", ErrInvalidArgs, typ)
		}
		typ = nt
	}
	needle := strings.TrimSpace(nameSubstr)
	var nameKeyNeedle string
	if needle != "" {
		nameKeyNeedle = normalizeRecordNameKey(needle)
	}
	out := make([]DNSRecord, 0)
	for _, r := range records {
		if typ != "" && normalizeRecordType(r.Type) != typ {
			continue
		}
		if nameKeyNeedle != "" && !strings.Contains(normalizeRecordNameKey(r.Name), nameKeyNeedle) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// ApplyLegacyListFilter applies the TUI single-string filter: if the whole string is a valid DNS type,
// filter by that type; otherwise substring match on the normalized owner name.
func ApplyLegacyListFilter(records []DNSRecord, filter string) []DNSRecord {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return records
	}
	if t := normalizeRecordType(filter); t != "" {
		if _, ok := dns.StringToType[t]; ok {
			out, _ := FilterRecords(records, "", t)
			return out
		}
	}
	out, _ := FilterRecords(records, filter, "")
	return out
}

// Add inserts a DNS record or updates an existing one when allowed. It returns the
// updated slice alongside informational messages.
func Add(fullCommand []string, dnsRecords []DNSRecord, allowUpdate bool) ([]DNSRecord, []Message, error) {
	if checkHelpCommand(fullCommand) {
		return dnsRecords, usageAdd(), ErrHelpRequested
	}

	dnsRecord, err := parseDNSRecordArgs(fullCommand)
	if err != nil {
		msgs := append([]Message{{Level: LevelError, Text: err.Error()}}, usageAdd()...)
		return dnsRecords, msgs, ErrInvalidArgs
	}
	return addRecordInternal(dnsRecord, dnsRecords, allowUpdate)
}

// AddRecord appends a DNSRecord to the collection, performing duplicate checks.
func AddRecord(record DNSRecord, dnsRecords []DNSRecord, allowUpdate bool) ([]DNSRecord, []Message, error) {
	record.Name = canonicalizeRecordNameForStorage(record.Name)
	record.Value = strings.TrimSpace(record.Value)
	record.Type = normalizeRecordType(record.Type)

	if record.Name == "" || record.Type == "" || record.Value == "" {
		msg := Message{Level: LevelError, Text: "name, type, and value are required"}
		return dnsRecords, []Message{msg}, ErrInvalidArgs
	}

	if _, ok := dns.StringToType[record.Type]; !ok {
		msg := Message{Level: LevelError, Text: fmt.Sprintf("invalid DNS record type: %s", record.Type)}
		return dnsRecords, []Message{msg}, ErrInvalidArgs
	}

	if record.TTL == 0 {
		record.TTL = 3600
	}

	if err := validateRecordValue(record.Type, record.Value); err != nil {
		msg := Message{Level: LevelError, Text: err.Error()}
		return dnsRecords, []Message{msg}, ErrInvalidArgs
	}

	record.AddedOn = time.Now()
	return addRecordInternal(record, dnsRecords, allowUpdate)
}

func addRecordInternal(dnsRecord DNSRecord, dnsRecords []DNSRecord, allowUpdate bool) ([]DNSRecord, []Message, error) {
	dnsRecord.Name = canonicalizeRecordNameForStorage(dnsRecord.Name)
	messages := make([]Message, 0)
	dnsRecord.Type = normalizeRecordType(dnsRecord.Type)
	if dnsRecord.AddedOn.IsZero() {
		dnsRecord.AddedOn = time.Now()
	}

	existingIndex := findDNSRecordIndex(dnsRecords, dnsRecord.Name, dnsRecord.Type, dnsRecord.Value)
	if existingIndex != -1 {
		oldRecord := dnsRecords[existingIndex]
		if allowUpdate {
			if strings.TrimSpace(dnsRecord.ID) == "" {
				dnsRecord.ID = oldRecord.ID
			}
			if strings.TrimSpace(dnsRecord.ID) == "" {
				dnsRecord.ID = NewRecordID()
			}
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

	if tid := strings.TrimSpace(dnsRecord.ID); tid != "" {
		if findDNSRecordIndexByID(dnsRecords, tid) != -1 {
			msg := Message{Level: LevelError, Text: fmt.Sprintf("a record with id %q already exists", tid)}
			return dnsRecords, []Message{msg}, ErrInvalidArgs
		}
	} else {
		dnsRecord.ID = NewRecordID()
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
		result.Records = ApplyLegacyListFilter(result.Records, result.Filter)
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

	if len(fullCommand) == 0 {
		msgs := append([]Message{{Level: LevelError, Text: "remove requires at least a record name."}}, usageRemove()...)
		return dnsRecords, msgs, ErrInvalidArgs
	}

	name := strings.TrimSpace(fullCommand[0])
	recordType := ""
	value := ""
	existingIndex := -1

	switch len(fullCommand) {
	case 1:
		if name == "" {
			msgs := append([]Message{{Level: LevelError, Text: "record name cannot be empty."}}, usageRemove()...)
			return dnsRecords, msgs, ErrInvalidArgs
		}
		targetName := normalizeRecordNameKey(name)
		for i, record := range dnsRecords {
			if normalizeRecordNameKey(record.Name) == targetName {
				if existingIndex != -1 {
					msgs := append([]Message{{Level: LevelWarn, Text: fmt.Sprintf("Multiple records match %s. Please include type and value.", name)}}, usageRemove()...)
					return dnsRecords, msgs, ErrInvalidArgs
				}
				existingIndex = i
				recordType = record.Type
				value = record.Value
			}
		}
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
	default:
		msgs := append([]Message{{Level: LevelError, Text: "invalid record format. Please use: remove <Name> [Type] <Value>"}}, usageRemove()...)
		return dnsRecords, msgs, ErrInvalidArgs
	}

	if name == "" || recordType == "" || value == "" {
		msgs := append([]Message{{Level: LevelError, Text: "invalid record format. Please use: remove <Name> [Type] <Value>"}}, usageRemove()...)
		return dnsRecords, msgs, ErrInvalidArgs
	}

	normType := normalizeRecordType(recordType)
	targetName := normalizeRecordNameKey(name)
	targetValue := normalizeRecordValueKey(normType, value)

	if existingIndex == -1 {
		for i, record := range dnsRecords {
			if normalizeRecordNameKey(record.Name) == targetName &&
				normalizeRecordType(record.Type) == normType &&
				normalizeRecordValueKey(record.Type, record.Value) == targetValue {
				existingIndex = i
				break
			}
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

// UpdateRecordByID replaces the record with the given id. The id field in `in` is ignored; body must include name, type, value (ttl optional).
func UpdateRecordByID(id string, in DNSRecord, dnsRecords []DNSRecord) ([]DNSRecord, []Message, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return dnsRecords, []Message{{Level: LevelError, Text: "id is required"}}, ErrInvalidArgs
	}
	idx := findDNSRecordIndexByID(dnsRecords, id)
	if idx == -1 {
		return dnsRecords, []Message{{Level: LevelError, Text: "no record with that id"}}, ErrInvalidArgs
	}

	in.Name = canonicalizeRecordNameForStorage(in.Name)
	in.Value = strings.TrimSpace(in.Value)
	in.Type = normalizeRecordType(in.Type)
	if in.Name == "" || in.Type == "" || in.Value == "" {
		return dnsRecords, []Message{{Level: LevelError, Text: "name, type, and value are required"}}, ErrInvalidArgs
	}
	if _, ok := dns.StringToType[in.Type]; !ok {
		return dnsRecords, []Message{{Level: LevelError, Text: fmt.Sprintf("invalid DNS record type: %s", in.Type)}}, ErrInvalidArgs
	}
	if in.TTL == 0 {
		in.TTL = 3600
	}
	if err := validateRecordValue(in.Type, in.Value); err != nil {
		return dnsRecords, []Message{{Level: LevelError, Text: err.Error()}}, ErrInvalidArgs
	}

	other := findDNSRecordIndex(dnsRecords, in.Name, in.Type, in.Value)
	if other != -1 && other != idx {
		msg := Message{Level: LevelWarn, Text: "A record already exists with the same name, type, and value."}
		return dnsRecords, []Message{msg}, ErrInvalidArgs
	}

	old := dnsRecords[idx]
	in.ID = id
	in.AddedOn = old.AddedOn
	if in.MACAddress == "" {
		in.MACAddress = old.MACAddress
	}
	in.CacheRecord = old.CacheRecord
	in.LastQuery = old.LastQuery
	in.UpdatedOn = time.Now()
	dnsRecords[idx] = in
	messages := []Message{{Level: LevelInfo, Text: fmt.Sprintf("Updated record id=%s", id)}}
	return dnsRecords, messages, nil
}

// RemoveRecordByID removes the record with the given id.
func RemoveRecordByID(id string, dnsRecords []DNSRecord) ([]DNSRecord, []Message, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return dnsRecords, []Message{{Level: LevelError, Text: "id is required"}}, ErrInvalidArgs
	}
	idx := findDNSRecordIndexByID(dnsRecords, id)
	if idx == -1 {
		return dnsRecords, []Message{{Level: LevelWarn, Text: "no record with that id"}}, ErrInvalidArgs
	}
	removedRecord := dnsRecords[idx]
	removedRec := converters.ConvertValuesToStrings(
		converters.GetFieldValuesByNamesArray(removedRecord,
			[]string{"Name", "Type", "Value", "TTL"}))
	dnsRecords = append(dnsRecords[:idx], dnsRecords[idx+1:]...)
	return dnsRecords, []Message{{Level: LevelInfo, Text: fmt.Sprintf("Removed: %v", removedRec)}}, nil
}

// Helper function to check if the help command is invoked.
func checkHelpCommand(fullCommand []string) bool {
	return cliutil.IsHelpRequest(fullCommand)
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
	switch ipversion {
	case 4:
		return "A"
	case 6:
		return "AAAA"
	default:
		return ""
	}
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

	if err := validateRecordValue(recordType, value); err != nil {
		return DNSRecord{}, err
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

func validateRecordValue(recordType, value string) error {
	switch recordType {
	case "A", "AAAA":
		if !ipvalidator.IsValidIP(value) {
			return fmt.Errorf("invalid IP address: %s", value)
		}
	case "CNAME", "NS", "PTR", "TXT", "SRV", "SOA", "MX", "NAPTR", "CAA", "TLSA", "DS", "DNSKEY", "RRSIG", "NSEC", "NSEC3", "NSEC3PARAM":
		if _, ok := dns.IsDomainName(value); !ok {
			return fmt.Errorf("invalid domain name: %s", value)
		}
	}
	return nil
}

// FindRecord searches for a DNS record in the list of DNS records.
// Returns the first matching record (for backward compatibility).
func FindRecord(dnsRecords []DNSRecord, lookupRecord, recordType string, autoBuildPTRFromA bool) *dns.RR {
	all := FindAllRecords(dnsRecords, lookupRecord, recordType, autoBuildPTRFromA)
	if len(all) == 0 {
		return nil
	}
	return &all[0]
}

// FindAllRecords searches for all DNS records matching the given name and type.
// This is the correct behavior for DNS servers - multiple A/AAAA records for the same domain are legal.
func FindAllRecords(dnsRecords []DNSRecord, lookupRecord, recordType string, autoBuildPTRFromA bool) []dns.RR {
	var results []dns.RR
	normalizedLookup := normalizeRecordNameKey(lookupRecord)
	normalizedType := normalizeRecordType(recordType)

	// For PTR queries, convert reverse DNS format to IP address
	var lookupIP string
	if recordType == "PTR" {
		lookupIP = converters.ConvertReverseDNSToIP(lookupRecord)
	}

	for _, record := range dnsRecords {
		if recordType == "PTR" {
			// Handle auto-build PTR from A records
			if autoBuildPTRFromA && (record.Type == "A" || record.Type == "AAAA") {
				recordIP := normalizeRecordValueKey(record.Type, record.Value)
				if lookupIP != "" && recordIP == strings.ToLower(lookupIP) {
					// Build PTR record from A/AAAA record
					ptrDomain := strings.TrimRight(record.Name, ".")
					if !strings.HasSuffix(ptrDomain, ".") {
						ptrDomain += "."
					}
					recordString := fmt.Sprintf("%s %d IN PTR %s", lookupRecord, record.TTL, ptrDomain)
					rr, err := dns.NewRR(recordString)
					if err == nil {
						results = append(results, rr)
					}
				}
			}

			// Handle explicit PTR records
			if record.Type == "PTR" && lookupIP != "" {
				recordIP := normalizeRecordNameKey(record.Name)
				if recordIP == strings.ToLower(lookupIP) {
					// Found PTR record matching the IP
					ptrDomain := strings.TrimRight(record.Value, ".")
					if !strings.HasSuffix(ptrDomain, ".") {
						ptrDomain += "."
					}
					recordString := fmt.Sprintf("%s %d IN PTR %s", lookupRecord, record.TTL, ptrDomain)
					rr, err := dns.NewRR(recordString)
					if err == nil {
						results = append(results, rr)
					}
				}
			}
			continue
		}

		// Match by name and type (normalized) for non-PTR queries
		normalizedRecordName := normalizeRecordNameKey(record.Name)
		normalizedRecordType := normalizeRecordType(record.Type)
		if normalizedRecordName == normalizedLookup && normalizedRecordType == normalizedType {
			rrString := fmt.Sprintf("%s %d IN %s %s", record.Name, record.TTL, record.Type, record.Value)
			rr, err := dns.NewRR(rrString)
			if err == nil {
				results = append(results, rr)
			}
		}
	}
	return results
}
