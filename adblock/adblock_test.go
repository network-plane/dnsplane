package adblock

import (
	"testing"
)

func TestNewBlockList(t *testing.T) {
	bl := NewBlockList()
	if bl == nil {
		t.Fatal("NewBlockList returned nil")
	}
	if bl.Count() != 0 {
		t.Errorf("new block list count = %d, want 0", bl.Count())
	}
}

func TestBlockList_AddDomain_IsBlocked(t *testing.T) {
	bl := NewBlockList()
	bl.AddDomain("ads.example.com")
	if bl.Count() != 1 {
		t.Errorf("Count() = %d, want 1", bl.Count())
	}
	if !bl.IsBlocked("ads.example.com") {
		t.Error("IsBlocked(ads.example.com) = false, want true")
	}
	if !bl.IsBlocked("ADS.EXAMPLE.COM.") {
		t.Error("IsBlocked(normalized) should match")
	}
	// Subdomain of blocked domain should be blocked
	if !bl.IsBlocked("tracker.ads.example.com") {
		t.Error("IsBlocked(subdomain) = false, want true")
	}
	if bl.IsBlocked("example.com") {
		t.Error("IsBlocked(example.com) = true, want false (only subdomain blocked)")
	}
	if bl.IsBlocked("other.com") {
		t.Error("IsBlocked(other.com) = true, want false")
	}
}

func TestBlockList_AddDomains_GetAll(t *testing.T) {
	bl := NewBlockList()
	bl.AddDomains([]string{"a.com", "b.com", "a.com"})
	if bl.Count() != 2 {
		t.Errorf("Count() = %d, want 2 (deduped)", bl.Count())
	}
	all := bl.GetAll()
	if len(all) != 2 {
		t.Errorf("GetAll() len = %d, want 2", len(all))
	}
}

func TestBlockList_RemoveDomain_Clear(t *testing.T) {
	bl := NewBlockList()
	bl.AddDomain("evil.com")
	bl.RemoveDomain("evil.com")
	if bl.Count() != 0 {
		t.Errorf("after RemoveDomain Count = %d", bl.Count())
	}
	if bl.IsBlocked("evil.com") {
		t.Error("IsBlocked after RemoveDomain should be false")
	}
	bl.AddDomain("a.com")
	bl.AddDomain("b.com")
	bl.Clear()
	if bl.Count() != 0 {
		t.Errorf("after Clear Count = %d", bl.Count())
	}
}

func TestBlockList_NilSafe(t *testing.T) {
	var bl *BlockList
	if bl.IsBlocked("x.com") {
		t.Error("nil IsBlocked should return false")
	}
	bl.AddDomain("x.com")    // no-op
	bl.RemoveDomain("x.com") // no-op
	bl.Clear()               // no-op
	if bl.Count() != 0 {
		t.Error("nil Count should return 0")
	}
	if bl.GetAll() != nil {
		t.Error("nil GetAll should return nil")
	}
}
