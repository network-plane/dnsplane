// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package data

import (
	"os"
	"path/filepath"
	"testing"

	"dnsplane/config"
)

func TestZoneFilesFromNamedConf(t *testing.T) {
	dir := t.TempDir()
	nc := filepath.Join(dir, "named.conf")
	content := `
zone "example.com" {
    type master;
    file "zones/example.com.db";
};
`
	if err := os.WriteFile(nc, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	zonesDir := filepath.Join(dir, "zones")
	if err := os.MkdirAll(zonesDir, 0o750); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(zonesDir, "example.com.db")
	zoneBody := `$ORIGIN example.com.
@ 3600 IN SOA ns1.example.com. hostmaster.example.com. 1 7200 900 1209600 3600
@ IN NS ns1.example.com.
www IN A 192.0.2.1
`
	if err := os.WriteFile(db, []byte(zoneBody), 0o600); err != nil {
		t.Fatal(err)
	}

	paths, err := zoneFilesFromNamedConf(nc, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != db {
		t.Fatalf("paths = %v, want [%s]", paths, db)
	}

	rs := &config.RecordsSourceConfig{Type: config.RecordsSourceBindDir, Location: dir, NamedConf: nc}
	recs, err := loadRecordsBindDir(rs)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) < 3 {
		t.Fatalf("expected at least 3 RRs, got %d", len(recs))
	}
}

func TestLoadRecordsBindDirGlobMerge(t *testing.T) {
	dir := t.TempDir()
	z1 := filepath.Join(dir, "a.db")
	z2 := filepath.Join(dir, "b.db")
	if err := os.WriteFile(z1, []byte(`$ORIGIN x.example.
@ IN SOA ns1.x.example. h.x.example. 1 7200 900 1209600 3600
dup 100 IN A 192.0.2.1
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(z2, []byte(`$ORIGIN x.example.
@ IN SOA ns1.x.example. h.x.example. 2 7200 900 1209600 3600
dup 200 IN A 192.0.2.1
`), 0o600); err != nil {
		t.Fatal(err)
	}
	rs := &config.RecordsSourceConfig{Type: config.RecordsSourceBindDir, Location: dir, IncludePattern: "*.db"}
	recs, err := loadRecordsBindDir(rs)
	if err != nil {
		t.Fatal(err)
	}
	var dupTTL uint32
	for _, r := range recs {
		if r.Name == "dup.x.example" && r.Type == "A" {
			dupTTL = r.TTL
			break
		}
	}
	if dupTTL != 200 {
		t.Fatalf("last file should win for same name+type+value, want TTL 200, got %d", dupTTL)
	}
}
