// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
//
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

func TestAddRecordRejectsEmptyRequired(t *testing.T) {
	for _, r := range []dnsrecords.DNSRecord{
		{Name: "", Type: "A", Value: "127.0.0.1"},
		{Name: "x.com", Type: "", Value: "127.0.0.1"},
		{Name: "x.com", Type: "A", Value: ""},
	} {
		if _, _, err := dnsrecords.AddRecord(r, nil, false); err == nil {
			t.Errorf("AddRecord with empty required field should error, record %+v", r)
		}
	}
}

func TestAddRecordRejectsInvalidIPForA(t *testing.T) {
	record := dnsrecords.DNSRecord{Name: "example.com", Type: "A", Value: "not-an-ip"}
	if _, _, err := dnsrecords.AddRecord(record, nil, false); err == nil {
		t.Fatal("expected error for invalid A record value")
	}
}

func TestNormalizeRecordNameKey(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"EXAMPLE.COM.", "example.com"},
		{"  foo.bar  ", "foo.bar"},
		{"FOO.BAR.", "foo.bar"},
	}
	for _, tt := range tests {
		got := dnsrecords.NormalizeRecordNameKey(tt.in)
		if got != tt.want {
			t.Errorf("NormalizeRecordNameKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCanonicalizeRecordNameForStorage(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"example.com.", "example.com"},
		{"EXAMPLE.COM..", "EXAMPLE.COM"},
		{"  a.b  ", "a.b"},
	}
	for _, tt := range tests {
		got := dnsrecords.CanonicalizeRecordNameForStorage(tt.in)
		if got != tt.want {
			t.Errorf("CanonicalizeRecordNameForStorage(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeRecordType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"a", "A"},
		{"  AAAA  ", "AAAA"},
		{"cname", "CNAME"},
	}
	for _, tt := range tests {
		got := dnsrecords.NormalizeRecordType(tt.in)
		if got != tt.want {
			t.Errorf("NormalizeRecordType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeRecordValueKey(t *testing.T) {
	// A/AAAA: lowercased
	if got := dnsrecords.NormalizeRecordValueKey("A", " 127.0.0.1 "); got != "127.0.0.1" {
		t.Errorf("NormalizeRecordValueKey(A, \" 127.0.0.1 \") = %q", got)
	}
	// CNAME/NS/PTR: name-normalized
	if got := dnsrecords.NormalizeRecordValueKey("CNAME", "TARGET.EXAMPLE.COM."); got != "target.example.com" {
		t.Errorf("NormalizeRecordValueKey(CNAME, ...) = %q", got)
	}
}

func TestFindAllRecords(t *testing.T) {
	records := []dnsrecords.DNSRecord{
		{Name: "example.com", Type: "A", Value: "127.0.0.1", TTL: 3600},
		{Name: "example.com", Type: "A", Value: "127.0.0.2", TTL: 3600},
		{Name: "other.com", Type: "A", Value: "10.0.0.1", TTL: 3600},
	}
	got := dnsrecords.FindAllRecords(records, "example.com", "A", false)
	if len(got) != 2 {
		t.Fatalf("FindAllRecords(example.com, A) len = %d, want 2", len(got))
	}
	got = dnsrecords.FindAllRecords(records, "missing.com", "A", false)
	if len(got) != 0 {
		t.Errorf("FindAllRecords(missing.com, A) len = %d, want 0", len(got))
	}
}

func TestList(t *testing.T) {
	records := []dnsrecords.DNSRecord{
		{Name: "a.com", Type: "A", Value: "1.1.1.1", TTL: 3600},
	}
	result, err := dnsrecords.List(records, nil)
	if err != nil {
		t.Fatalf("List(nil) err = %v", err)
	}
	if len(result.Records) != 1 {
		t.Errorf("List(nil) records = %d, want 1", len(result.Records))
	}
	result, err = dnsrecords.List(records, []string{"details"})
	if err != nil {
		t.Fatalf("List(details) err = %v", err)
	}
	if !result.Detailed {
		t.Error("List(details) Detailed = false, want true")
	}
}

// FuzzAddRecord exercises AddRecord with fuzzed name, type, and value.
func FuzzAddRecord(f *testing.F) {
	f.Add("example.com.", "A", "127.0.0.1")
	f.Fuzz(func(t *testing.T, name, recordType, value string) {
		record := dnsrecords.DNSRecord{Name: name, Type: recordType, Value: value}
		_, _, _ = dnsrecords.AddRecord(record, nil, false)
	})
}

// FuzzNormalizeRecordNameKey exercises name normalization with arbitrary strings.
func FuzzNormalizeRecordNameKey(f *testing.F) {
	f.Add("example.com.")
	f.Add("EXAMPLE.COM")
	f.Add("")
	f.Fuzz(func(t *testing.T, name string) {
		_ = dnsrecords.NormalizeRecordNameKey(name)
		_ = dnsrecords.CanonicalizeRecordNameForStorage(name)
	})
}

// FuzzNormalizeRecordTypeAndValue exercises type/value normalization with arbitrary strings.
func FuzzNormalizeRecordTypeAndValue(f *testing.F) {
	f.Add("A", "127.0.0.1")
	f.Add("AAAA", "::1")
	f.Add("CNAME", "foo.example.com")
	f.Fuzz(func(t *testing.T, recordType, value string) {
		_ = dnsrecords.NormalizeRecordType(recordType)
		_ = dnsrecords.NormalizeRecordValueKey(recordType, value)
	})
}

// FuzzFindAllRecords exercises record lookup with fuzzed query name and type.
func FuzzFindAllRecords(f *testing.F) {
	records := []dnsrecords.DNSRecord{
		{Name: "example.com", Type: "A", Value: "127.0.0.1", TTL: 3600},
		{Name: "example.com", Type: "AAAA", Value: "::1", TTL: 3600},
		{Name: "foo.example.com", Type: "CNAME", Value: "example.com", TTL: 3600},
	}
	f.Add("example.com", "A")
	f.Add("example.com.", "AAAA")
	f.Add("1.0.0.127.in-addr.arpa", "PTR")
	f.Fuzz(func(t *testing.T, lookupRecord, recordType string) {
		_ = dnsrecords.FindAllRecords(records, lookupRecord, recordType, true)
		_ = dnsrecords.FindRecord(records, lookupRecord, recordType, true)
	})
}
