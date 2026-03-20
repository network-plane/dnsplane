// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package cluster

import (
	"encoding/json"
	"testing"
)

func TestAdminConfigApplyMessageJSON(t *testing.T) {
	tok := "adm"
	replica := true
	m := AdminConfigApplyMessage{
		Type:        TypeAdminConfigApply,
		AdminToken:  tok,
		ReplicaOnly: &replica,
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var out AdminConfigApplyMessage
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Type != TypeAdminConfigApply || out.AdminToken != tok {
		t.Fatalf("round-trip: %+v", out)
	}
	if out.ReplicaOnly == nil || !*out.ReplicaOnly {
		t.Fatal("replica_only")
	}
}
