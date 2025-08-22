package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenFero/openfero/pkg/alertstore"
	"github.com/OpenFero/openfero/pkg/alertstore/memory"
	"github.com/OpenFero/openfero/pkg/handlers"
)

func TestAlertStoreSearchEndpoint(t *testing.T) {
	// Create store and populate with test data
	store := memory.NewMemoryStore(10)

	testAlerts := []struct {
		labels map[string]string
		status string
	}{
		{map[string]string{"alertname": "TestAlert1"}, "firing"},
		{map[string]string{"alertname": "TestAlert2"}, "resolved"},
		{map[string]string{"alertname": "AnotherAlert"}, "FIRING"},      // uppercase status
		{map[string]string{"alertname": "YetAnotherAlert"}, "RESOLVED"}, // uppercase status
	}

	// Save all alerts
	for _, alert := range testAlerts {
		storeAlert := alertstore.Alert{Labels: alert.labels}
		err := store.SaveAlert(storeAlert, alert.status)
		if err != nil {
			t.Fatalf("Failed to save alert: %v", err)
		}
	}

	server := &handlers.Server{AlertStore: store}

	tests := []struct {
		name        string
		query       string
		expectCount int
	}{
		{
			name:        "Search for 'firing' should find both firing alerts",
			query:       "firing",
			expectCount: 2,
		},
		{
			name:        "Search for 'FIRING' should find both firing alerts",
			query:       "FIRING",
			expectCount: 2,
		},
		{
			name:        "Search for 'resolved' should find both resolved alerts",
			query:       "resolved",
			expectCount: 2,
		},
		{
			name:        "Search for 'RESOLVED' should find both resolved alerts",
			query:       "RESOLVED",
			expectCount: 2,
		},
		{
			name:        "Search for 'TestAlert' should find matching alerts",
			query:       "TestAlert",
			expectCount: 2,
		},
		{
			name:        "Empty query should return all alerts",
			query:       "",
			expectCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			url := "/alertStore"
			if tt.query != "" {
				url += "?q=" + tt.query
			}

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call the handler
			server.AlertStoreGetHandler(rr, req)

			// Check status code
			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, http.StatusOK)
			}

			// Parse response
			var alerts []alertstore.AlertEntry
			err = json.NewDecoder(rr.Body).Decode(&alerts)
			if err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Check count
			if len(alerts) != tt.expectCount {
				t.Errorf("Expected %d results for query '%s', got %d",
					tt.expectCount, tt.query, len(alerts))

				// Debug output
				t.Logf("Results for query '%s':", tt.query)
				for i, alert := range alerts {
					t.Logf("  %d: alertname=%s, status=%s",
						i, alert.Alert.Labels["alertname"], alert.Status)
				}
			}
		})
	}
}
