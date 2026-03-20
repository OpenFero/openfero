package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenFero/openfero/pkg/alertstore/memory"
	"github.com/OpenFero/openfero/pkg/handlers"
	"github.com/OpenFero/openfero/pkg/services"
	"k8s.io/client-go/kubernetes/fake"
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
	store := memory.NewMemoryStore(10)
	if err := store.Initialize(); err != nil {
		t.Fatal(err)
	}

	fakeClient := fake.NewSimpleClientset()
	opService := services.NewOperariusService(fakeClient)

	server := &handlers.Server{
		AlertStore:       store,
		OperariusService: opService,
	}

	payload := `{
		"version": "4",
		"groupKey": "test-group",
		"status": "firing",
		"receiver": "openfero",
		"commonLabels": {"alertname": "TestAlert"},
		"alerts": [
			{
				"status": "firing",
				"labels": {"alertname": "TestAlert", "severity": "warning"},
				"annotations": {"summary": "Test alert"}
			}
		]
	}`

	req := httptest.NewRequest("POST", "/alerts", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.AlertsPostHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	// Verify alert was stored
	alerts, err := store.GetAlerts("", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert in store, got %d", len(alerts))
	}
	if alerts[0].Alert.Labels["alertname"] != "TestAlert" {
		t.Errorf("Expected alertname 'TestAlert', got '%s'", alerts[0].Alert.Labels["alertname"])
	}
}

func TestMultipleAlertPostAlertsHandler(t *testing.T) {
	store := memory.NewMemoryStore(10)
	if err := store.Initialize(); err != nil {
		t.Fatal(err)
	}

	fakeClient := fake.NewSimpleClientset()
	opService := services.NewOperariusService(fakeClient)

	server := &handlers.Server{
		AlertStore:       store,
		OperariusService: opService,
	}

	payload := `{
		"version": "4",
		"groupKey": "test-group",
		"status": "firing",
		"receiver": "openfero",
		"commonLabels": {"alertname": "TestAlert"},
		"alerts": [
			{
				"status": "firing",
				"labels": {"alertname": "TestAlert", "namespace": "ns-a"},
				"annotations": {"summary": "Alert A"}
			},
			{
				"status": "firing",
				"labels": {"alertname": "TestAlert", "namespace": "ns-b"},
				"annotations": {"summary": "Alert B"}
			},
			{
				"status": "firing",
				"labels": {"alertname": "TestAlert", "namespace": "ns-c"},
				"annotations": {"summary": "Alert C"}
			}
		]
	}`

	req := httptest.NewRequest("POST", "/alerts", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.AlertsPostHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	// Verify all alerts were stored
	alerts, err := store.GetAlerts("", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 3 {
		t.Errorf("Expected 3 alerts in store, got %d", len(alerts))
	}
}

func TestMalformedJSONPostAlertsHandler(t *testing.T) {
	store := memory.NewMemoryStore(10)
	if err := store.Initialize(); err != nil {
		t.Fatal(err)
	}

	server := &handlers.Server{
		AlertStore: store,
	}

	payload := `{this is not valid json`

	req := httptest.NewRequest("POST", "/alerts", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.AlertsPostHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	// Verify no alerts were stored
	alerts, err := store.GetAlerts("", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts in store, got %d", len(alerts))
	}
}
