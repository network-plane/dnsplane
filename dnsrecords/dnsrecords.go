// Package dnsrecords contains the functions to add, list, remove, clear and update DNS records.
package dnsrecords

import (
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

// Add a new DNS record to the list of DNS records or update an existing one.
func Add(fullCommand []string, dnsRecords []DNSRecord, allowUpdate bool) []DNSRecord {
	if checkHelpCommand(fullCommand) {
		printAddUsage()
		return dnsRecords
	}

	// 1) Parse arguments to get a DNSRecord struct
	dnsRecord, err := parseDNSRecordArgs(fullCommand)
	if err != nil {
		fmt.Println("Error:", err)
		printAddUsage()
		return dnsRecords
	}
	dnsRecord.Type = normalizeRecordType(dnsRecord.Type)
	dnsRecord.AddedOn = time.Now()

	// 2) Use helper to find if a record with the same Name and Value already exists
	existingIndex := findDNSRecordIndex(dnsRecords, dnsRecord.Name, dnsRecord.Type, dnsRecord.Value)

	// 3) If found in the slice, either update it (if allowed) or inform user it already exists
	if existingIndex != -1 {
		oldRecord := dnsRecords[existingIndex]

		// If updates are allowed, overwrite
		if allowUpdate {
			dnsRecords[existingIndex] = dnsRecord

			updatedRecToPrint := converters.ConvertValuesToStrings(
				converters.GetFieldValuesByNamesArray(dnsRecord,
					[]string{"Name", "Type", "Value", "TTL"}))
			oldRecToPrint := converters.ConvertValuesToStrings(
				converters.GetFieldValuesByNamesArray(oldRecord,
					[]string{"Name", "Type", "Value", "TTL"}))

			fmt.Println("Existing record found. Updating...")
			fmt.Println("Previous:", oldRecToPrint)
			fmt.Println("Current :", updatedRecToPrint)

		} else {
			// If updates are NOT allowed, just inform the user
			attemptedRecToPrint := converters.ConvertValuesToStrings(
				converters.GetFieldValuesByNamesArray(dnsRecord,
					[]string{"Name", "Type", "Value", "TTL"}))
			oldRecToPrint := converters.ConvertValuesToStrings(
				converters.GetFieldValuesByNamesArray(oldRecord,
					[]string{"Name", "Type", "Value", "TTL"}))

			fmt.Println("A record already exists.")
			fmt.Println("Attempted:", attemptedRecToPrint)
			fmt.Println("Current  :", oldRecToPrint)
		}
		return dnsRecords
	}

	// 4) If not found in the slice, append the new record
	dnsRecords = append(dnsRecords, dnsRecord)
	addedRecToPrint := converters.ConvertValuesToStrings(
		converters.GetFieldValuesByNamesArray(dnsRecord,
			[]string{"Name", "Type", "Value", "TTL"}))

	fmt.Println("Added:", addedRecToPrint)
	return dnsRecords
}

// List all the DNS records in the list of DNS records.
func List(dnsRecords []DNSRecord, args []string) {
	if len(dnsRecords) == 0 {
		fmt.Println("No records found.")
		return
	}

	// Find maximum lengths of Name and Value fields
	maxNameLength := 4  // Length of "Name"
	maxValueLength := 5 // Length of "Value"
	for _, record := range dnsRecords {
		if len(record.Name) > maxNameLength {
			maxNameLength = len(record.Name)
		}
		if len(record.Value) > maxValueLength {
			maxValueLength = len(record.Value)
		}
	}

	// Helper function to check if the user wants to see details
	isDetails := func(args []string) bool {
		return len(args) > 0 && (args[0] == "details" || args[0] == "d")
	}

	// Define format string with variable widths for Name and Value
	formatString := fmt.Sprintf("%%-%ds %%-7s %%-%ds %%-5s\n", maxNameLength+2, maxValueLength+2)

	fmt.Printf(formatString, "Name", "Type", "Value", "TTL")

	for _, record := range dnsRecords {
		valToPrint := converters.ConvertValuesToStrings(converters.GetFieldValuesByNamesArray(record, []string{"Name", "Type", "Value", "TTL"}))
		fmt.Printf(formatString, valToPrint[0], valToPrint[1], valToPrint[2], valToPrint[3])

		if !record.AddedOn.IsZero() && isDetails(args) {
			fmt.Println("Added On:", record.AddedOn)
		}

		if !record.UpdatedOn.IsZero() && isDetails(args) {
			fmt.Println("Updated On:", record.UpdatedOn)
		}

		if !record.LastQuery.IsZero() && isDetails(args) {
			fmt.Println("Last Query:", record.LastQuery)
		}

		if record.MACAddress != "" && isDetails(args) {
			fmt.Println("MAC Address:", record.MACAddress)
		}

		if record.CacheRecord && isDetails(args) {
			fmt.Println("Cache Record: true")
		}

		fmt.Println()
	}
}

// Remove deletes a DNS record from the list of DNS records if found.
func Remove(fullCommand []string, dnsRecords []DNSRecord) []DNSRecord {
	if checkHelpCommand(fullCommand) {
		printRemoveUsage()
		return dnsRecords
	}

	var (
		inputName  = strings.TrimSpace(fullCommand[0])
		name       string
		recordType string
		value      string
	)

	switch len(fullCommand) {
	case 2:
		// remove <Name> <IP>
		var detectedType string
		name, value, detectedType = validateIPAndDomain(fullCommand[0], fullCommand[1])
		if name == "" || detectedType == "" {
			fmt.Println("Invalid record format. Please use: remove <Name> [Type] <Value>")
			printRemoveUsage()
			return dnsRecords
		}
		recordType = detectedType
	case 3:
		name = fullCommand[0]
		recordType = fullCommand[1]
		value = fullCommand[2]
	default:
		fmt.Println("Invalid usage. Please see help:")
		printRemoveUsage()
		return dnsRecords
	}

	if name == "" || recordType == "" || value == "" {
		fmt.Println("Invalid record format. Please use: remove <Name> [Type] <Value>")
		printRemoveUsage()
		return dnsRecords
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
		fmt.Printf("No record found for [%s %s %s].\n", inputName, recordType, value)
		printRemoveUsage()
		return dnsRecords
	}

	removedRecord := dnsRecords[existingIndex]
	removedRecToPrint := converters.ConvertValuesToStrings(
		converters.GetFieldValuesByNamesArray(removedRecord,
			[]string{"Name", "Type", "Value", "TTL"}))

	// Remove it from the slice
	dnsRecords = append(dnsRecords[:existingIndex], dnsRecords[existingIndex+1:]...)

	// 5) Print removal details
	fmt.Println("Removed:", removedRecToPrint)
	return dnsRecords
}

// Helper function to check if the help command is invoked.
func checkHelpCommand(fullCommand []string) bool {
	return len(fullCommand) == 0 || cliutil.IsHelpRequest(fullCommand)
}

func printAddUsage() {
	fmt.Println("Usage  : add <Name> [Type] <Value> [TTL]")
	fmt.Println("Examples:")
	fmt.Println("  add example.com 127.0.0.1")
	fmt.Println("  add example.com A 127.0.0.1")
	fmt.Println("  add example.com A 127.0.0.1 3600")
	printHelpAliasesHint()
}

func printRemoveUsage() {
	fmt.Println("Usage  : remove <Name> [Type] <Value>")
	fmt.Println("Examples:")
	fmt.Println("  remove example.com 127.0.0.1")
	fmt.Println("  remove example.com A 127.0.0.1")
	printHelpAliasesHint()
}

func printHelpAliasesHint() {
	fmt.Println("Hint: append '?', 'help', or 'h' after the command to view this usage.")
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
