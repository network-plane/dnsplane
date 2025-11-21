package dnsrecords_test

import (
	"testing"

	"dnsplane/dnsrecords"
)

func TestAddRecordAddsRecord(t *testing.T) {
	record := dnsrecords.DNSRecord{
		Name:  "example.com.",
		Type:  "A",
		Value: "127.0.0.1",
	}

	updated, _, err := dnsrecords.AddRecord(record, nil, false)
	if err != nil {
		t.Fatalf("AddRecord returned error: %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("expected 1 record, got %d", len(updated))
	}
	if updated[0].TTL != 3600 {
		t.Fatalf("expected TTL default to 3600, got %d", updated[0].TTL)
	}
}

func TestAddRecordRejectsInvalidType(t *testing.T) {
	record := dnsrecords.DNSRecord{
		Name:  "example.com.",
		Type:  "INVALID",
		Value: "127.0.0.1",
	}

	if _, _, err := dnsrecords.AddRecord(record, nil, false); err == nil {
		t.Fatal("expected error for invalid type, got nil")
	}
}
