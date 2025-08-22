package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenFero/openfero/pkg/alertstore/memory"
	"github.com/OpenFero/openfero/pkg/handlers"
)

func TestAuthMiddleware_None(t *testing.T) {
	authConfig := handlers.AuthConfig{
		Method: handlers.AuthMethodNone,
	}

	middleware := handlers.AuthMiddleware(authConfig)
	handler := middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest("POST", "/alerts", strings.NewReader("{}"))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAuthMiddleware_BasicAuth_Success(t *testing.T) {
	authConfig := handlers.AuthConfig{
		Method:    handlers.AuthMethodBasic,
		BasicUser: "testuser",
		BasicPass: "testpass",
	}

	middleware := handlers.AuthMiddleware(authConfig)
	handler := middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest("POST", "/alerts", strings.NewReader("{}"))
	req.SetBasicAuth("testuser", "testpass")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAuthMiddleware_BasicAuth_Failure(t *testing.T) {
	authConfig := handlers.AuthConfig{
		Method:    handlers.AuthMethodBasic,
		BasicUser: "testuser",
		BasicPass: "testpass",
	}

	middleware := handlers.AuthMiddleware(authConfig)
	handler := middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Test with wrong credentials
	req := httptest.NewRequest("POST", "/alerts", strings.NewReader("{}"))
	req.SetBasicAuth("wronguser", "wrongpass")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	// Check WWW-Authenticate header
	authHeader := rec.Header().Get("WWW-Authenticate")
	expected := "Basic realm=\"OpenFero\""
	if authHeader != expected {
		t.Errorf("Expected WWW-Authenticate header %q, got %q", expected, authHeader)
	}
}

func TestAuthMiddleware_BasicAuth_NoCredentials(t *testing.T) {
	authConfig := handlers.AuthConfig{
		Method:    handlers.AuthMethodBasic,
		BasicUser: "testuser",
		BasicPass: "testpass",
	}

	middleware := handlers.AuthMiddleware(authConfig)
	handler := middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Test without credentials
	req := httptest.NewRequest("POST", "/alerts", strings.NewReader("{}"))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAuthMiddleware_BearerToken_Success(t *testing.T) {
	authConfig := handlers.AuthConfig{
		Method:      handlers.AuthMethodBearer,
		BearerToken: "secret-token-123",
	}

	middleware := handlers.AuthMiddleware(authConfig)
	handler := middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest("POST", "/alerts", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer secret-token-123")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAuthMiddleware_BearerToken_Failure(t *testing.T) {
	authConfig := handlers.AuthConfig{
		Method:      handlers.AuthMethodBearer,
		BearerToken: "secret-token-123",
	}

	middleware := handlers.AuthMiddleware(authConfig)
	handler := middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Test with wrong token
	req := httptest.NewRequest("POST", "/alerts", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	// Check WWW-Authenticate header
	authHeader := rec.Header().Get("WWW-Authenticate")
	expected := "Bearer realm=\"OpenFero\""
	if authHeader != expected {
		t.Errorf("Expected WWW-Authenticate header %q, got %q", expected, authHeader)
	}
}

func TestAuthMiddleware_BearerToken_NoToken(t *testing.T) {
	authConfig := handlers.AuthConfig{
		Method:      handlers.AuthMethodBearer,
		BearerToken: "secret-token-123",
	}

	middleware := handlers.AuthMiddleware(authConfig)
	handler := middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Test without Authorization header
	req := httptest.NewRequest("POST", "/alerts", strings.NewReader("{}"))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAuthMiddleware_InvalidMethod(t *testing.T) {
	authConfig := handlers.AuthConfig{
		Method: handlers.AuthMethod("invalid"),
	}

	middleware := handlers.AuthMiddleware(authConfig)
	handler := middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest("POST", "/alerts", strings.NewReader("{}"))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestValidateAuthConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    handlers.AuthConfig
		expectErr bool
	}{
		{
			name: "none auth valid",
			config: handlers.AuthConfig{
				Method: handlers.AuthMethodNone,
			},
			expectErr: false,
		},
		{
			name: "basic auth valid",
			config: handlers.AuthConfig{
				Method:    handlers.AuthMethodBasic,
				BasicUser: "user",
				BasicPass: "pass",
			},
			expectErr: false,
		},
		{
			name: "basic auth missing user",
			config: handlers.AuthConfig{
				Method:    handlers.AuthMethodBasic,
				BasicPass: "pass",
			},
			expectErr: true,
		},
		{
			name: "basic auth missing password",
			config: handlers.AuthConfig{
				Method:    handlers.AuthMethodBasic,
				BasicUser: "user",
			},
			expectErr: true,
		},
		{
			name: "bearer auth valid",
			config: handlers.AuthConfig{
				Method:      handlers.AuthMethodBearer,
				BearerToken: "token",
			},
			expectErr: false,
		},
		{
			name: "bearer auth missing token",
			config: handlers.AuthConfig{
				Method: handlers.AuthMethodBearer,
			},
			expectErr: true,
		},
		{
			name: "oauth2 valid",
			config: handlers.AuthConfig{
				Method:         handlers.AuthMethodOAuth2,
				OAuth2Issuer:   "https://issuer.example.com",
				OAuth2Audience: "openfero",
			},
			expectErr: false,
		},
		{
			name: "oauth2 missing issuer",
			config: handlers.AuthConfig{
				Method:         handlers.AuthMethodOAuth2,
				OAuth2Audience: "openfero",
			},
			expectErr: true,
		},
		{
			name: "oauth2 missing audience",
			config: handlers.AuthConfig{
				Method:       handlers.AuthMethodOAuth2,
				OAuth2Issuer: "https://issuer.example.com",
			},
			expectErr: true,
		},
		{
			name: "invalid method",
			config: handlers.AuthConfig{
				Method: handlers.AuthMethod("invalid"),
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAuthConfig(tt.config)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestIntegration_AlertsPostHandler_WithAuth(t *testing.T) {
	// Create a test alert store
	store := memory.NewMemoryStore(10)

	// Create server with Basic Auth and minimal KubeClient
	server := &handlers.Server{
		AlertStore: store,
		KubeClient: nil, // We don't need real Kubernetes for auth testing
		AuthConfig: handlers.AuthConfig{
			Method:    handlers.AuthMethodBasic,
			BasicUser: "testuser",
			BasicPass: "testpass",
		},
	}

	// Create a simplified handler for auth testing that doesn't require Kubernetes
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		// Just decode the JSON to verify it's valid
		dec := json.NewDecoder(r.Body)
		defer r.Body.Close()

		var message interface{}
		if err := dec.Decode(&message); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// Return success without creating jobs
		w.WriteHeader(http.StatusOK)
	}

	// Create authenticated middleware
	authMiddleware := handlers.AuthMiddleware(server.AuthConfig)
	authenticatedHandler := authMiddleware(testHandler)

	// Test valid alert payload
	alertPayload := `{
		"version": "4",
		"groupKey": "test-group",
		"status": "firing",
		"receiver": "openfero",
		"alerts": [
			{
				"status": "firing",
				"labels": {
					"alertname": "TestAlert",
					"severity": "warning"
				},
				"annotations": {
					"summary": "Test alert summary"
				}
			}
		]
	}`

	// Test without authentication - should fail
	req := httptest.NewRequest("POST", "/alerts", strings.NewReader(alertPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	authenticatedHandler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d without auth, got %d", http.StatusUnauthorized, rec.Code)
	}

	// Test with valid authentication - should succeed
	req = httptest.NewRequest("POST", "/alerts", strings.NewReader(alertPayload))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("testuser", "testpass")
	rec = httptest.NewRecorder()

	authenticatedHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d with valid auth, got %d", http.StatusOK, rec.Code)
	}
}
