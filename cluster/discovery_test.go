// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package cluster

import (
	"net"
	"testing"
)

func TestParseSRVQuery(t *testing.T) {
	svc, proto, zone, ok := ParseSRVQuery("_dnsplane._tcp.example.com.")
	if !ok || svc != "dnsplane" || proto != "tcp" || zone != "example.com" {
		t.Fatalf("got %q %q %q ok=%v", svc, proto, zone, ok)
	}
	_, _, _, ok = ParseSRVQuery("example.com")
	if ok {
		t.Fatal("expected not ok")
	}
}

func TestLookupSRVTargets_fake(t *testing.T) {
	fake := func(service, proto, name string) (string, []*net.SRV, error) {
		if service != "dnsplane" || proto != "tcp" || name != "example.com" {
			t.Fatalf("unexpected %s %s %s", service, proto, name)
		}
		return "", []*net.SRV{
			{Target: "a.example.com.", Port: 7946, Priority: 10, Weight: 5},
			{Target: "b.example.com.", Port: 7946, Priority: 10, Weight: 1},
		}, nil
	}
	addrs, err := LookupSRVTargets(fake, "_dnsplane._tcp.example.com.")
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 2 || addrs[0] != "a.example.com:7946" {
		t.Fatalf("got %v", addrs)
	}
}

func TestMergePeerAddrs(t *testing.T) {
	got := MergePeerAddrs([]string{"10.0.0.1:7946", " 10.0.0.2:7946 "}, []string{"10.0.0.1:7946", "10.0.0.3:7946"})
	want := []string{"10.0.0.1:7946", "10.0.0.2:7946", "10.0.0.3:7946"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d: got %q want %q", i, got[i], want[i])
		}
	}
}
