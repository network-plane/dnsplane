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

	fmt.Println("Adding DNS record:", dnsRecord)
}

func listRecords() {
	if len(dnsRecords) == 0 {
		fmt.Println("No records found.")
		return
	}
	for _, record := range dnsRecords {
		fmt.Println(record)
	}
}

func removeRecord(fullCommand []string) {
	if len(fullCommand) < 2 {
		fmt.Println("Please specify the record to remove.")
		return
	}

	// Remove A Record from dnsRecord
	fmt.Println("Removing DNS record:", fullCommand[1])
}
