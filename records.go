package main

import (
	"fmt"
	"strconv"

	"github.com/miekg/dns"
)

func addDNSRecord(fullCommand []string) {

	if len(fullCommand) > 1 && fullCommand[1] == "?" {
		fmt.Println("Enter the DNS record in the format: Name Type Value TTL")
		fmt.Println("Example: example.com A 127.0.0.1 3600")
		return
	}

	//Add A Record to dnsRecord
	if len(fullCommand) < 5 {
		println("Invalid DNS record format. Please enter the DNS record in the format: Name Type Value TTL")
		return
	}

	//check if type (fullCommand[2]) is valid for DNS type
	if _, ok := dns.StringToType[fullCommand[3]]; !ok {
		fmt.Println("Invalid DNS record type. Please enter a valid DNS record type.")
		return
	}

	ttl64, err := strconv.ParseUint(fullCommand[5], 10, 32)
	if err != nil {
		fmt.Println("Invalid TTL value. Please enter a valid TTL value.")
		return
	}
	ttl := uint32(ttl64)

	dnsRecord := DNSRecord{
		Name:  fullCommand[1],
		Type:  fullCommand[2],
		Value: fullCommand[3],
		TTL:   ttl,
	}

	dnsRecords = append(dnsRecords, dnsRecord)
	fmt.Println("Added:", dnsRecord)
}

func listRecords() {
	if len(dnsRecords) == 0 {
		fmt.Println("No records found.")
		return
	}

	// Find maximum lengths of Name and Value fields
	maxNameLength := 0
	maxValueLength := 0
	for _, record := range dnsRecords {
		if len(record.Name) > maxNameLength {
			maxNameLength = len(record.Name)
		}
		if len(record.Value) > maxValueLength {
			maxValueLength = len(record.Value)
		}
	}

	// Define format string with variable widths for Name and Value
	formatString := fmt.Sprintf("%%-%ds %%-7s %%-%ds %%-5s\n", maxNameLength+4, maxValueLength+4)

	fmt.Printf(formatString, "Name", "Type", "Value", "TTL")

	for _, record := range dnsRecords {
		fmt.Printf(formatString, record.Name, record.Type, record.Value, strconv.Itoa(int(record.TTL)))

		if !record.AddedOn.IsZero() {
			fmt.Println("Added On:", record.AddedOn)
		}
		if !record.UpdatedOn.IsZero() {
			fmt.Println("Updated On:", record.UpdatedOn)
		}
		if !record.LastQuery.IsZero() {
			fmt.Println("Last Query:", record.LastQuery)
		}
		if record.MACAddress != "" {
			fmt.Println("MAC Address:", record.MACAddress)
		}
		fmt.Println()
	}
}

func removeDNSRecord(fullCommand []string) {
	if len(fullCommand) > 1 && fullCommand[1] == "?" {
		fmt.Println("Enter the DNS record in the format: Name Type Value TTL")
		fmt.Println("Example: example.com A")
		return
	}

	if len(fullCommand) < 2 {
		fmt.Println("Please specify at least the record name.")
		return
	}

	name := fullCommand[1]

	matchingRecords := []DNSRecord{}
	for _, record := range dnsRecords {
		if record.Name == name {
			matchingRecords = append(matchingRecords, record)
		}
	}

	if len(matchingRecords) == 0 {
		fmt.Println("No records found with the name:", name)
		return
	}

	if len(fullCommand) == 2 {
		if len(matchingRecords) == 1 {
			removeAndPrint(matchingRecords[0])
			return
		}
		fmt.Println("Multiple records found with the name:", name)
		for i, record := range matchingRecords {
			fmt.Printf("%d. %v\n", i+1, record)
		}
		return
	}

	if len(fullCommand) < 4 {
		fmt.Println("Please specify at least the record name and type.")
		return
	}

	recordType := fullCommand[2]
	if len(fullCommand) == 4 {
		matchingTypeRecords := []DNSRecord{}
		for _, record := range matchingRecords {
			if record.Type == recordType {
				matchingTypeRecords = append(matchingTypeRecords, record)
			}
		}
		if len(matchingTypeRecords) == 1 {
			removeAndPrint(matchingTypeRecords[0])
		} else {
			fmt.Println("Multiple records found with the same name and type:")
			for i, record := range matchingTypeRecords {
				fmt.Printf("%d. %v\n", i+1, record)
			}
		}
		return
	}

	if len(fullCommand) == 5 {
		value := fullCommand[3]
		ttl64, err := strconv.ParseUint(fullCommand[4], 10, 32)
		if err != nil {
			fmt.Println("Invalid TTL value.")
			return
		}
		ttl := uint32(ttl64)

		for i, record := range dnsRecords {
			if record.Name == name && record.Type == recordType && record.Value == value && record.TTL == ttl {
				dnsRecords = append(dnsRecords[:i], dnsRecords[i+1:]...)
				fmt.Println("Removed:", record)
				return
			}
		}
		fmt.Println("No record found with the specified details.")
	}
}

func removeAndPrint(record DNSRecord) {
	for i, r := range dnsRecords {
		if r == record {
			dnsRecords = append(dnsRecords[:i], dnsRecords[i+1:]...)
			fmt.Println("Removed: ", record)
			return
		}
	}
}

func clearDNSRecords() {
	dnsRecords = []DNSRecord{}
	fmt.Println("All records cleared.")
}

func updateDNSRecord(fullCommand []string) {
	if len(fullCommand) > 1 && fullCommand[1] == "?" {
		fmt.Println("Enter the DNS record in the format: Name Type [NewValue] [NewTTL]")
		fmt.Println("Example: example.com A 192.168.0.1 7200")
		return
	}

	if len(fullCommand) < 3 {
		println("Invalid DNS record format. Please enter the DNS record in the format: Name Type [NewValue] [NewTTL]")
		return
	}

	// Validate the record type
	if _, ok := dns.StringToType[fullCommand[2]]; !ok {
		fmt.Println("Invalid DNS record type. Please enter a valid DNS record type.")
		return
	}

	name, recordType := fullCommand[1], fullCommand[2]
	var newValue *string
	var newTTL *uint32

	// Optional fields: NewValue and NewTTL
	if len(fullCommand) > 3 {
		newValue = &fullCommand[3]
	}

	if len(fullCommand) > 4 {
		ttl64, err := strconv.ParseUint(fullCommand[4], 10, 32)
		if err != nil {
			fmt.Println("Invalid TTL value. Please enter a valid TTL value.")
			return
		}
		ttl := uint32(ttl64)
		newTTL = &ttl
	}

	var found bool
	for i, record := range dnsRecords {
		if record.Name == name && record.Type == recordType {
			oldRecord := dnsRecords[i]
			// Update fields conditionally
			if newValue != nil {
				dnsRecords[i].Value = *newValue
			}
			if newTTL != nil {
				dnsRecords[i].TTL = *newTTL
			}

			fmt.Println("Updated:")
			fmt.Println("Old Record:", oldRecord)
			fmt.Println("New Record:", dnsRecords[i])
			found = true
			break
		}
	}

	if !found {
		fmt.Println("No matching DNS record found to update.")
	}
}
