package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenFero/openfero/pkg/alertstore/memory"
	"github.com/OpenFero/openfero/pkg/handlers"
)

func TestGetAlertsHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/alerts", nil)
	if err != nil {
		t.Fatal(err)
	}
	responserecorder := httptest.NewRecorder()

	// Create a server with a memory alert store
	store := memory.NewMemoryStore(10)
	server := &handlers.Server{
		AlertStore: store,
	}

	server.AlertsGetHandler(responserecorder, req)

	if status := responserecorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestSingleAlertPostAlertsHandler(t *testing.T) {
	// Test implementation will be added later
}

func MultipleAlertPostAlertsHandler(t *testing.T) {
	// Test implementation will be added later
}

func MalformedJSONPostAlertsHandler(t *testing.T) {
	// Test implementation will be added later
}
