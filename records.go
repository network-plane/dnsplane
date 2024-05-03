package main

import (
	"fmt"
	"strconv"

	"github.com/miekg/dns"
)

func addRecord(fullCommand []string) {

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

// func listRecords() {
// 	if len(dnsRecords) == 0 {
// 		fmt.Println("No records found.")
// 		return
// 	}
// 	for _, record := range dnsRecords {
// 		fmt.Println(record)
// 	}
// }

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

func removeRecord(fullCommand []string) {
	if len(fullCommand) < 2 {
		fmt.Println("Please provide the name of the record to remove.")
		return
	}

	nameToRemove := fullCommand[1]
	typeToRemove := ""
	valueToRemove := ""
	ttlToRemove := ""

	if len(fullCommand) > 2 {
		typeToRemove = fullCommand[2]
	}
	if len(fullCommand) > 3 {
		valueToRemove = fullCommand[3]
	}
	if len(fullCommand) > 4 {
		ttlToRemove = fullCommand[4]
	}

	removed := false

	for i := len(dnsRecords) - 1; i >= 0; i-- {
		record := dnsRecords[i]

		if record.Name == nameToRemove && (typeToRemove == "" || record.Type == typeToRemove) &&
			(valueToRemove == "" || record.Value == valueToRemove) &&
			(ttlToRemove == "" || strconv.FormatUint(uint64(record.TTL), 10) == ttlToRemove) {

			if !removed {
				fmt.Println("Removed Records:")
				removed = true
			}
			fmt.Println(record)
			// Remove the record from dnsRecords
			dnsRecords = append(dnsRecords[:i], dnsRecords[i+1:]...)
		}
	}

	if !removed {
		fmt.Println("No matching records found for removal.")
	}
}
