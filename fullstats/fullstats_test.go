package fullstats

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew_Disabled(t *testing.T) {
	tracker, err := New("", false)
	if err != nil {
		t.Fatalf("New(disabled): %v", err)
	}
	if tracker != nil {
		t.Error("New(disabled) should return nil tracker")
	}
}

func TestNew_Enabled(t *testing.T) {
	dir := t.TempDir()
	tracker, err := New(dir, true)
	if err != nil {
		t.Fatalf("New(enabled): %v", err)
	}
	if tracker == nil {
		t.Fatal("New(enabled) returned nil tracker")
	}
	defer tracker.Close()

	dbPath := filepath.Join(dir, dbFileName)
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("database file not created: %v", err)
	}
}
