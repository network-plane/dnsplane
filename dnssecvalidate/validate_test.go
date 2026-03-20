// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package dnssecvalidate

import (
	"testing"

	"dnsplane/config"

	"github.com/miekg/dns"
)

func TestApplyToUpstreamAnswer_Off(t *testing.T) {
	out, fail := ApplyToUpstreamAnswer(nil, &dns.Msg{}, dns.Question{Name: "a.", Qtype: dns.TypeA}, config.Config{})
	if out != OutcomeOff || fail {
		t.Fatalf("got %v %v", out, fail)
	}
}
