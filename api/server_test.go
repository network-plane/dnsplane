package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	healthHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("healthHandler status = %d, want 200", rec.Code)
	}
}

func TestReadyHandler_WithoutState(t *testing.T) {
	// With apiState nil, ready should report not ready
	apiServerMu.Lock()
	old := apiState
	apiState = nil
	apiServerMu.Unlock()
	defer func() {
		apiServerMu.Lock()
		apiState = old
		apiServerMu.Unlock()
	}()

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	readyHandler(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("readyHandler with nil state status = %d, want 503", rec.Code)
	}
}
