package main

import (
	"encoding/json"
	"log"
	"os"
)

func getDNSServers() []string {
	var servers Servers
	data, err := os.ReadFile("servers.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(data, &servers)
	if err != nil {
		log.Fatal(err)
	}
	return servers.Servers
}

func getDNSRecords() []DNSRecord {
	var records Records
	data, err := os.ReadFile("records.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(data, &records)
	if err != nil {
		log.Fatal(err)
	}
	return records.Records
}

func getSettings() DNSServerSettings {
	var settings DNSServerSettings
	data, err := os.ReadFile("dnsresolver.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(data, &settings)
	if err != nil {
		log.Fatal(err)
	}
	return settings
}
