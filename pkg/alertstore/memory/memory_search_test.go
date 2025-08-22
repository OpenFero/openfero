package memory

import (
	"testing"

	"github.com/OpenFero/openfero/pkg/alertstore"
)

func TestSearchFunctionality(t *testing.T) {
	store := NewMemoryStore(10)

	// Create test alerts with different statuses
	alerts := []struct {
		labels map[string]string
		status string
	}{
		{map[string]string{"alertname": "TestAlert1"}, "firing"},
		{map[string]string{"alertname": "TestAlert2"}, "resolved"},
		{map[string]string{"alertname": "AnotherAlert"}, "FIRING"}, // uppercase status
		{map[string]string{"alertname": "YetAnotherAlert"}, "RESOLVED"}, // uppercase status
	}

	// Save all alerts
	for _, alert := range alerts {
		storeAlert := alertstore.Alert{Labels: alert.labels}
		err := store.SaveAlert(storeAlert, alert.status)
		if err != nil {
			t.Fatalf("Failed to save alert: %v", err)
		}
	}

	tests := []struct {
		name        string
		query       string
		expectCount int
	}{
		{
			name:        "Search for 'firing' (lowercase) should find both firing alerts",
			query:       "firing",
			expectCount: 2, // Should find both "firing" and "FIRING"
		},
		{
			name:        "Search for 'FIRING' (uppercase) should find both firing alerts",
			query:       "FIRING", 
			expectCount: 2, // Should find both "firing" and "FIRING"
		},
		{
			name:        "Search for 'resolved' (lowercase) should find both resolved alerts",
			query:       "resolved",
			expectCount: 2, // Should find both "resolved" and "RESOLVED"
		},
		{
			name:        "Search for 'RESOLVED' (uppercase) should find both resolved alerts", 
			query:       "RESOLVED",
			expectCount: 2, // Should find both "resolved" and "RESOLVED"
		},
		{
			name:        "Search for 'TestAlert' should find alerts with that name",
			query:       "TestAlert",
			expectCount: 2, // Should find TestAlert1 and TestAlert2
		},
		{
			name:        "Search for 'testalert' (lowercase) should find alerts with that name",
			query:       "testalert",
			expectCount: 2, // Should find TestAlert1 and TestAlert2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := store.GetAlerts(tt.query, 100)
			if err != nil {
				t.Fatalf("GetAlerts failed: %v", err)
			}

			if len(results) != tt.expectCount {
				t.Errorf("Expected %d results for query '%s', got %d", 
					tt.expectCount, tt.query, len(results))
				
				// Debug: Print what we got
				t.Logf("Results for query '%s':", tt.query)
				for i, result := range results {
					t.Logf("  %d: alertname=%s, status=%s", 
						i, result.Alert.Labels["alertname"], result.Status)
				}
			}
		})
	}
}
