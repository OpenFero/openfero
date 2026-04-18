package handlers

import (
"net/http"
"net/http/httptest"
"os"
"testing"

"github.com/OpenFero/openfero/pkg/logging"
)

func TestMain(m *testing.M) {
	if err := logging.SetLevel("warn"); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestHealthzGetHandler(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.HealthzGetHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body %q, got %q", "ok", rec.Body.String())
	}
}

func TestReadinessGetHandler(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
	rec := httptest.NewRecorder()

	server.ReadinessGetHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body %q, got %q", "ok", rec.Body.String())
	}
}

func TestStartupzGetHandler_NotReady(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	rec := httptest.NewRecorder()

	server.StartupzGetHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
	if rec.Body.String() != "starting" {
		t.Errorf("expected body %q, got %q", "starting", rec.Body.String())
	}
}

func TestStartupzGetHandler_Ready(t *testing.T) {
	server := &Server{}
	server.StartupComplete.Store(true)

	req := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	rec := httptest.NewRecorder()

	server.StartupzGetHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body %q, got %q", "ok", rec.Body.String())
	}
}
