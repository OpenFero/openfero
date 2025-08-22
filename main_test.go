package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/OpenFero/openfero/pkg/alertstore"
	"github.com/OpenFero/openfero/pkg/alertstore/memory"
	"github.com/OpenFero/openfero/pkg/handlers"
	"github.com/OpenFero/openfero/pkg/models"
	"github.com/OpenFero/openfero/pkg/utils"
)

func TestMain(m *testing.M) {
	// Initialize logger before running tests
	if err := initLogger("debug"); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No newlines or carriage returns",
			input:    "HelloWorld",
			expected: "HelloWorld",
		},
		{
			name:     "Newline characters",
			input:    "Hello\nWorld",
			expected: "HelloWorld",
		},
		{
			name:     "Carriage return characters",
			input:    "Hello\rWorld",
			expected: "HelloWorld",
		},
		{
			name:     "Newline and carriage return characters",
			input:    "Hello\n\rWorld",
			expected: "HelloWorld",
		},
		{
			name:     "Multiple newline and carriage return characters",
			input:    "\nHello\r\nWorld\r",
			expected: "HelloWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.SanitizeInput(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeInput(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStringWithCharset(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		charset string
	}{
		{
			name:    "Generate string of length 10 with alphanumeric charset",
			length:  10,
			charset: "abcdefghijklmnopqrstuvwxyz0123456789",
		},
		{
			name:    "Generate string of length 5 with numeric charset",
			length:  5,
			charset: "0123456789",
		},
		{
			name:    "Generate string of length 8 with alphabetic charset",
			length:  8,
			charset: "abcdefghijklmnopqrstuvwxyz",
		},
		{
			name:    "Generate string of length 0",
			length:  0,
			charset: "abcdefghijklmnopqrstuvwxyz0123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.StringWithCharset(tt.length, tt.charset)
			if len(result) != tt.length {
				t.Errorf("stringWithCharset(%d, %q) = %q; want length %d", tt.length, tt.charset, result, tt.length)
			}

			for _, char := range result {
				if !strings.ContainsRune(tt.charset, char) && tt.charset != "" {
					t.Errorf("stringWithCharset(%d, %q) = %q; contains invalid character %q", tt.length, tt.charset, result, char)
				}
			}
		})
	}
}

// Benchmark the stringWithCharset function
func BenchmarkStringWithCharset(b *testing.B) {
	for i := 0; i < b.N; i++ {
		utils.StringWithCharset(10, "abcdefghijklmnopqrstuvwxyz0123456789")
	}
}

func TestSaveAlert(t *testing.T) {
	tests := []struct {
		name           string
		initialAlerts  []alertstore.AlertEntry
		newAlert       models.Alert
		newStatus      string
		expectedLabels map[string][]string // Expected label values by key
		alertStoreSize int
	}{
		{
			name: "Add alert to non-full store",
			initialAlerts: []alertstore.AlertEntry{
				{
					Alert:  alertstore.Alert{Labels: map[string]string{"alertname": "alert1"}},
					Status: "firing",
				},
			},
			newAlert:  models.Alert{Labels: map[string]string{"alertname": "alert2"}},
			newStatus: "firing",
			expectedLabels: map[string][]string{
				"alertname": {"alert1", "alert2"}, // We expect both alert1 and alert2 to be present
			},
			alertStoreSize: 10,
		},
		{
			name:          "Add alert to empty store",
			initialAlerts: []alertstore.AlertEntry{},
			newAlert:      models.Alert{Labels: map[string]string{"alertname": "alert1"}},
			newStatus:     "firing",
			expectedLabels: map[string][]string{
				"alertname": {"alert1"}, // We expect only alert1 to be present
			},
			alertStoreSize: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize a memory store for testing
			store := memory.NewMemoryStore(tt.alertStoreSize)

			// Pre-populate the store with initial alerts
			for _, entry := range tt.initialAlerts {
				_ = store.SaveAlert(entry.Alert, entry.Status)
			}

			// Convert the Alert to alertstore.Alert and save it
			storeAlert := tt.newAlert.ToAlertStoreAlert()
			err := store.SaveAlert(storeAlert, tt.newStatus)
			if err != nil {
				t.Fatalf("Failed to save alert: %v", err)
			}

			// Get all alerts from the store
			alerts, err := store.GetAlerts("", 0)
			if err != nil {
				t.Fatalf("Failed to get alerts: %v", err)
			}

			// Verify the expected number of alerts
			expectedCount := len(tt.initialAlerts) + 1 // Initial alerts + newly added alert
			if len(alerts) != expectedCount {
				t.Errorf("SaveAlert() store length = %d, want %d", len(alerts), expectedCount)
			}

			// Check that all expected labels are present
			for key, expectedValues := range tt.expectedLabels {
				// Collect actual values for this label
				actualValues := make([]string, 0)
				for _, alert := range alerts {
					if val, ok := alert.Alert.Labels[key]; ok {
						actualValues = append(actualValues, val)
					}
				}

				// Check that we have all expected values
				if len(actualValues) != len(expectedValues) {
					t.Errorf("SaveAlert() found %d values for label %s, want %d",
						len(actualValues), key, len(expectedValues))
				}

				// Check that each expected value is present (ignoring order)
				for _, expected := range expectedValues {
					found := false
					for _, actual := range actualValues {
						if expected == actual {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("SaveAlert() missing expected label value %s=%s", key, expected)
					}
				}
			}

			// Verify all alerts have the correct status
			for _, alert := range alerts {
				// Initial alerts should have their original status, new alert should have tt.newStatus
				var isNewAlert bool
				if val, ok := alert.Alert.Labels["alertname"]; ok {
					isNewAlert = val == tt.newAlert.Labels["alertname"]
				}

				if isNewAlert && alert.Status != tt.newStatus {
					t.Errorf("New alert has incorrect status: got %s, want %s",
						alert.Status, tt.newStatus)
				}
			}
		})
	}
}

func BenchmarkSaveAlert(b *testing.B) {
	store := memory.NewMemoryStore(10)

	// Initial alerts
	initialAlert := alertstore.Alert{Labels: map[string]string{"alertname": "alert1"}}
	_ = store.SaveAlert(initialAlert, "firing")

	// New alert to add repeatedly
	newAlert := models.Alert{Labels: map[string]string{"alertname": "alert3"}}
	storeAlert := newAlert.ToAlertStoreAlert()

	for i := 0; i < b.N; i++ {
		_ = store.SaveAlert(storeAlert, "firing")
	}
}

// Commented test functions left as is since they're already commented out
// func TestAlertStoreGetHandler(t *testing.T) {...}

func BenchmarkAlertStoreGetHandler(b *testing.B) {
	req, err := http.NewRequest("GET", "/alertStore", nil)
	if err != nil {
		b.Fatal(err)
	}

	// Create store and server
	store := memory.NewMemoryStore(10)
	server := &handlers.Server{AlertStore: store}

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		server.AlertStoreGetHandler(rr, req)
	}
}

func BenchmarkAlertStoreGetHandlerWithQuery(b *testing.B) {
	req, err := http.NewRequest("GET", "/alertStore?q=testAlert", nil)
	if err != nil {
		b.Fatal(err)
	}

	// Create store and server
	store := memory.NewMemoryStore(10)
	server := &handlers.Server{AlertStore: store}

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		server.AlertStoreGetHandler(rr, req)
	}
}
