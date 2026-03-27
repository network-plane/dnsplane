// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package zones_test

import (
	"path/filepath"
	"strings"
	"testing"

	"dnsplane/dnsrecords"
	"dnsplane/zones"
)

func TestParseFileMinimal(t *testing.T) {
	path := filepath.Join("testdata", "minimal.zone")
	res, err := zones.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) < 8 {
		t.Fatalf("expected at least 8 records, got %d", len(res.Records))
	}
	var sawSOA, sawMX, sawTXT bool
	for _, r := range res.Records {
		switch r.Type {
		case "SOA":
			sawSOA = true
			if r.Name != "example.com" {
				t.Errorf("SOA name %q", r.Name)
			}
		case "MX":
			sawMX = true
			if r.Value != "10 mail.example.com." {
				t.Errorf("MX value %q", r.Value)
			}
		case "TXT":
			sawTXT = true
		}
	}
	if !sawSOA || !sawMX || !sawTXT {
		t.Fatalf("missing types: soa=%v mx=%v txt=%v", sawSOA, sawMX, sawTXT)
	}
	// Round-trip through resolver-style RR build
	rrs := dnsrecords.FindAllRecords(res.Records, "www.example.com.", "A", false)
	if len(rrs) != 1 {
		t.Fatalf("FindAllRecords www A: %d", len(rrs))
	}
}

func TestParseFileRelativeOrigin(t *testing.T) {
	path := filepath.Join("testdata", "relative.zone")
	res, err := zones.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rrs := dnsrecords.FindAllRecords(res.Records, "app.corp.example.net.", "A", false)
	if len(rrs) != 1 {
		t.Fatalf("app A: %d", len(rrs))
	}
}

func TestParseReaderSkipsSRV(t *testing.T) {
	z := `$ORIGIN example.com.
_sip._tcp 60 IN SRV 10 5 5060 sip.example.com.
www IN A 1.2.3.4
`
	res, err := zones.ParseReader(strings.NewReader(z), "inline")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 1 || res.Records[0].Type != "A" {
		t.Fatalf("got %+v warnings=%v", res.Records, res.Warnings)
	}
}
