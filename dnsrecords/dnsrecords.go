// Package dnsrecords contains the functions to add, list, remove, clear and update DNS records.
package dnsrecords

import (
	"fmt"
	"strconv"
	"time"

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

// Add a new DNS record to the list of DNS records.
func Add(fullCommand []string, dnsRecords []DNSRecord) []DNSRecord {
	if checkHelpCommand(fullCommand) {
		fmt.Println("Usage  : add <Name> [Type] <Value> [TTL]")
		fmt.Println("Example: example.com 127.0.0.1")
		fmt.Println("         example.com A 127.0.0.1")
		fmt.Println("         example.com A 127.0.0.1 3600")

		return dnsRecords
	}

	dnsRecord, err := parseDNSRecordArgs(fullCommand)
	if err != nil {
		fmt.Println("Error:", err)
		return dnsRecords
	}

	dnsRecord.AddedOn = time.Now()

	dnsRecords = append(dnsRecords, dnsRecord)
	addedRecToPrint := converters.ConvertValuesToStrings(converters.GetFieldValuesByNamesArray(dnsRecord, []string{"Name", "Type", "Value", "TTL"}))
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

	// Define format string with variable widths for Name and Value
	formatString := fmt.Sprintf("%%-%ds %%-7s %%-%ds %%-5s\n", maxNameLength+2, maxValueLength+2)

	fmt.Printf(formatString, "Name", "Type", "Value", "TTL")

	for _, record := range dnsRecords {
		fmt.Println()
		valToPrint := converters.ConvertValuesToStrings(converters.GetFieldValuesByNamesArray(record, []string{"Name", "Type", "Value", "TTL"}))
		fmt.Printf(formatString, valToPrint[0], valToPrint[1], valToPrint[2], valToPrint[3])

		if !record.AddedOn.IsZero() && len(args) > 0 && (args[0] == "details" || args[0] == "d") {
			fmt.Println("Added On:", record.AddedOn)
		}
		if !record.UpdatedOn.IsZero() && len(args) > 0 && (args[0] == "details" || args[0] == "d") {
			fmt.Println("Updated On:", record.UpdatedOn)
		}
		if !record.LastQuery.IsZero() && len(args) > 0 && (args[0] == "details" || args[0] == "d") {
			fmt.Println("Last Query:", record.LastQuery)
		}
		if record.MACAddress != "" && len(args) > 0 && (args[0] == "details" || args[0] == "d") {
			fmt.Println("MAC Address:", record.MACAddress)
		}
		if record.CacheRecord && len(args) > 0 && (args[0] == "details" || args[0] == "d") {
			fmt.Println("Cache Record: true")
		}
	}
}

// Remove a DNS record from the list of DNS records.
func Remove(fullCommand []string, dnsRecords []DNSRecord) []DNSRecord {
	if checkHelpCommand(fullCommand) {
		fmt.Println("Usage: remove <Name> [Type] [Value] [TTL]")
		fmt.Println("Example: example.com A 127.0.0.1 3600")
		return dnsRecords
	}

	if len(fullCommand) < 1 {
		fmt.Println("Error: Please specify at least the record name.")
		return dnsRecords
	}

	name := fullCommand[0]
	recordType := ""
	value := ""
	var ttl uint32

	if len(fullCommand) > 1 {
		recordType = fullCommand[1]
	}
	if len(fullCommand) > 2 {
		value = fullCommand[2]
	}
	if len(fullCommand) > 3 {
		ttl64, err := strconv.ParseUint(fullCommand[3], 10, 32)
		if err != nil {
			fmt.Println("Error: Invalid TTL value.")
			return dnsRecords
		}
		ttl = uint32(ttl64)
	}

	indexes := findDNSRecordIndexes(dnsRecords, name, recordType, value, ttl)
	if len(indexes) == 0 {
		fmt.Println("Error: No records found with the specified details.")
		return dnsRecords
	}

	if len(indexes) == 1 {
		index := indexes[0]
		removedRecord := dnsRecords[index]
		dnsRecords = append(dnsRecords[:index], dnsRecords[index+1:]...)
		removedRecToPrint := converters.ConvertValuesToStrings(converters.GetFieldValuesByNamesArray(removedRecord, []string{"Name", "Type", "Value", "TTL"}))
		fmt.Println("Removed:", removedRecToPrint)
		return dnsRecords
	}

	fmt.Println("Multiple records found with the specified details:")
	for _, idx := range indexes {
		record := dnsRecords[idx]
		recordToPrint := converters.ConvertValuesToStrings(converters.GetFieldValuesByNamesArray(record, []string{"Name", "Type", "Value", "TTL"}))
		fmt.Println(recordToPrint)
	}
	fmt.Println("Please specify more details to remove a specific record.")

	return dnsRecords
}

// Update a DNS record in the list of DNS records.
func Update(fullCommand []string, dnsRecords []DNSRecord) []DNSRecord {
	if checkHelpCommand(fullCommand) {
		fmt.Println("Usage: update <Name> <Type> [NewValue] [NewTTL]")
		fmt.Println("Example: update example.com A 192.168.0.1 7200")
		return dnsRecords
	}

	if len(fullCommand) < 2 {
		fmt.Println("Error: Please specify at least the record name and type.")
		return dnsRecords
	}

	name := fullCommand[0]
	recordType := fullCommand[1]
	var newValue *string
	var newTTL *uint32

	if _, ok := dns.StringToType[recordType]; !ok {
		fmt.Println("Error: Invalid DNS record type:", recordType)
		return dnsRecords
	}

	if len(fullCommand) > 2 {
		newValue = &fullCommand[2]
	}

	if len(fullCommand) > 3 {
		ttl64, err := strconv.ParseUint(fullCommand[3], 10, 32)
		if err != nil {
			fmt.Println("Error: Invalid TTL value:", fullCommand[3])
			return dnsRecords
		}
		ttl := uint32(ttl64)
		newTTL = &ttl
	}

	// Find matching records
	indexes := findDNSRecordIndexes(dnsRecords, name, recordType, "", 0)
	if len(indexes) == 0 {
		fmt.Println("Error: No matching DNS record found to update.")
		return dnsRecords
	}

	if len(indexes) > 1 {
		fmt.Println("Multiple records found with the specified name and type. Please specify value and TTL to identify the record uniquely.")
		for _, idx := range indexes {
			record := dnsRecords[idx]
			recordToPrint := converters.ConvertValuesToStrings(converters.GetFieldValuesByNamesArray(record, []string{"Name", "Type", "Value", "TTL"}))
			fmt.Println(recordToPrint)
		}
		return dnsRecords
	}

	index := indexes[0]
	oldRecord := dnsRecords[index]

	if newValue != nil {
		dnsRecords[index].Value = *newValue
	}
	if newTTL != nil {
		dnsRecords[index].TTL = *newTTL
	}
	dnsRecords[index].UpdatedOn = time.Now()

	fmt.Println("Updated:")
	oldRecToPrint := converters.ConvertValuesToStrings(converters.GetFieldValuesByNamesArray(oldRecord, []string{"Name", "Type", "Value", "TTL"}))
	fmt.Println("Old Record:", oldRecToPrint)
	updatedRecToPrint := converters.ConvertValuesToStrings(converters.GetFieldValuesByNamesArray(dnsRecords[index], []string{"Name", "Type", "Value", "TTL"}))
	fmt.Println("New Record:", updatedRecToPrint)

	return dnsRecords
}

// Helper function to check if the help command is invoked.
func checkHelpCommand(fullCommand []string) bool {
	return len(fullCommand) <= 0 || fullCommand[0] == "?"
}

func ipvToRecordType(ipversion int) string {
	if ipversion == 4 {
		return "A"
	} else if ipversion == 6 {
		return "AAAA"
	}
	return ""
}

func validateIPAndDomain(arg1, arg2 string) (string, string, string) {
	// First attempt: arg1 as IP, arg2 as domain
	if ipvalidator.IsValidIP(arg1) {
		_, isDomain := dns.IsDomainName(arg2)
		if isDomain {
			return arg2, arg1, ipvToRecordType(ipvalidator.GetIPVersion(arg1)) // Return domain, IP, version
		}
	}

	// Second attempt: arg1 as domain, arg2 as IP
	if ipvalidator.IsValidIP(arg2) {
		_, isDomain := dns.IsDomainName(arg1)
		if isDomain {
			return arg1, arg2, ipvToRecordType(ipvalidator.GetIPVersion(arg2)) // Return domain, IP, version
		}
	}

	// If neither combination works, return empty strings
	return "", "", ""
}

// Parses arguments and returns a DNSRecord.
func parseDNSRecordArgs(args []string) (DNSRecord, error) {
	var name, recordType, value, ttlStr string

	if len(args) >= 2 {
		name, value, recordType = validateIPAndDomain(args[0], args[1])
		if name == "" {
			return DNSRecord{}, fmt.Errorf("invalid DNS record format. Please enter the DNS record in the format: <Name> [Type] <Value> [TTL]")
		} else {
			ttlStr = "3600"
		}
	}

	if len(args) < 3 && name == "" {
		return DNSRecord{}, fmt.Errorf("invalid DNS record format. Please enter the DNS record in the format: <Name> [Type] <Value> [TTL]")
	}

	// If name is still empty, parse the arguments normally
	if name == "" {
		name = args[0]
		recordType = args[1]
		value = args[2]
		if len(args) < 4 {
			ttlStr = args[3]
		} else {
			ttlStr = "3600"
		}
	}

	// Validate DNS record type
	if _, ok := dns.StringToType[recordType]; !ok {
		return DNSRecord{}, fmt.Errorf("invalid DNS record type: %s", recordType)
	}

	//check if the value is a valid IP address in case of A or AAAA record
	if recordType == "A" || recordType == "AAAA" {
		if !ipvalidator.IsValidIP(value) {
			return DNSRecord{}, fmt.Errorf("invalid IP address: %s", value)
		}
	}

	//check if the value is a valid domain name in case of these records
	if recordType == "CNAME" || recordType == "NS" || recordType == "PTR" || recordType == "TXT" || recordType == "SRV" || recordType == "SOA" || recordType == "MX" || recordType == "NAPTR" || recordType == "CAA" || recordType == "TLSA" || recordType == "DS" || recordType == "DNSKEY" || recordType == "RRSIG" || recordType == "NSEC" || recordType == "NSEC3" || recordType == "NSEC3PARAM" {
		if _, ok := dns.IsDomainName(value); !ok {
			return DNSRecord{}, fmt.Errorf("invalid domain name: %s", value)
		}
	}

	// Parse TTL
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

// Helper function to find indexes of matching DNSRecords.
func findDNSRecordIndexes(dnsRecords []DNSRecord, name, recordType, value string, ttl uint32) []int {
	var indexes []int
	for i, record := range dnsRecords {
		if record.Name == name {
			if recordType == "" || record.Type == recordType {
				if value == "" || record.Value == value {
					if ttl == 0 || record.TTL == ttl {
						indexes = append(indexes, i)
					}
				}
			}
		}
	}
	return indexes
}
