// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
//
package cliutil

import (
	"testing"
)

func TestIsHelpToken(t *testing.T) {
	helpTokens := []string{"?", "help", "h", "  ?  ", "HELP", "H"}
	for _, token := range helpTokens {
		if !IsHelpToken(token) {
			t.Errorf("IsHelpToken(%q) = false, want true", token)
		}
	}
	nonHelpTokens := []string{"", "x", "helpme", "add", "hx"}
	for _, token := range nonHelpTokens {
		if IsHelpToken(token) {
			t.Errorf("IsHelpToken(%q) = true, want false", token)
		}
	}
}

func TestIsHelpRequest(t *testing.T) {
	if !IsHelpRequest([]string{"?"}) {
		t.Error("IsHelpRequest([\"?\"]) = false, want true")
	}
	if !IsHelpRequest([]string{"help"}) {
		t.Error("IsHelpRequest([\"help\"]) = false, want true")
	}
	if IsHelpRequest(nil) {
		t.Error("IsHelpRequest(nil) = true, want false")
	}
	if IsHelpRequest([]string{}) {
		t.Error("IsHelpRequest([]) = true, want false")
	}
	if IsHelpRequest([]string{"add", "example.com"}) {
		t.Error("IsHelpRequest([\"add\", \"example.com\"]) = true, want false")
	}
}

func TestContainsHelpToken(t *testing.T) {
	if !ContainsHelpToken([]string{"add", "?"}) {
		t.Error("ContainsHelpToken([\"add\", \"?\"]) = false, want true")
	}
	if !ContainsHelpToken([]string{"help"}) {
		t.Error("ContainsHelpToken([\"help\"]) = false, want true")
	}
	if ContainsHelpToken([]string{"add", "example.com"}) {
		t.Error("ContainsHelpToken([\"add\", \"example.com\"]) = true, want false")
	}
	if ContainsHelpToken(nil) {
		t.Error("ContainsHelpToken(nil) = true, want false")
	}
}
