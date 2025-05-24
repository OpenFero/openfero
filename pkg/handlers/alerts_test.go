package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenFero/openfero/pkg/alertstore/memory"
	"github.com/OpenFero/openfero/pkg/models"
	"github.com/OpenFero/openfero/pkg/logging"
	"go.uber.org/zap"
	"os"
)

func TestMain(m *testing.M) {
	// Setup logger for tests
	cfg := zap.NewDevelopmentConfig() // You can use NewNopConfig() for no output
	err := logging.SetConfig(cfg)
	if err != nil {
		// Can't use log.Fatal here as logger might not be working
		println("Failed to set up logger for tests:", err.Error())
		os.Exit(1)
	}
	// Run tests
	exitVal := m.Run()
	os.Exit(exitVal)
}

func TestAlertsPostHandler_Authentication(t *testing.T) {
	// Minimal valid body for requests that should pass auth
	validBody := models.HookMessage{
		Status: "firing",
		Alerts: []models.Alert{},
	}
	validBodyBytes, _ := json.Marshal(validBody)

	// Empty body for requests that should fail auth (body content doesn't matter then)
	emptyBodyBytes := []byte("{}")

	tests := []struct {
		name                string
		authToken           string // Server's configured auth token
		requestHeader       string // Value for the "Authorization" header in the request
		requestBody         []byte
		expectStatusCode    int
		expectAuthFailure   bool
	}{
		{
			name:                "Auth Disabled - No Token Set on Server",
			authToken:           "", // Auth disabled
			requestHeader:       "", // No header from client
			requestBody:         validBodyBytes,
			expectStatusCode:    http.StatusOK, // Or whatever status if body processing is successful
			expectAuthFailure:   false,
		},
		{
			name:                "Auth Disabled - Client Sends Token Anyway",
			authToken:           "", // Auth disabled
			requestHeader:       "Bearer randomclienttoken",
			requestBody:         validBodyBytes,
			expectStatusCode:    http.StatusOK, // Auth is off, so token is ignored
			expectAuthFailure:   false,
		},
		{
			name:                "Auth Enabled - Valid Token",
			authToken:           "secret-token",
			requestHeader:       "Bearer secret-token",
			requestBody:         validBodyBytes,
			expectStatusCode:    http.StatusOK,
			expectAuthFailure:   false,
		},
		{
			name:                "Auth Enabled - Invalid Token",
			authToken:           "secret-token",
			requestHeader:       "Bearer wrong-token",
			requestBody:         emptyBodyBytes,
			expectStatusCode:    http.StatusUnauthorized,
			expectAuthFailure:   true,
		},
		{
			name:                "Auth Enabled - No Authorization Header",
			authToken:           "secret-token",
			requestHeader:       "", // No header sent
			requestBody:         emptyBodyBytes,
			expectStatusCode:    http.StatusUnauthorized,
			expectAuthFailure:   true,
		},
		{
			name:                "Auth Enabled - Malformed Header (No Bearer prefix)",
			authToken:           "secret-token",
			requestHeader:       "Basic somecredentials",
			requestBody:         emptyBodyBytes,
			expectStatusCode:    http.StatusUnauthorized,
			expectAuthFailure:   true,
		},
		{
			name:                "Auth Enabled - Malformed Header (Bearer but no token value)",
			authToken:           "secret-token",
			requestHeader:       "Bearer ", // Note the trailing space
			requestBody:         emptyBodyBytes,
			expectStatusCode:    http.StatusUnauthorized,
			expectAuthFailure:   true,
		},
		{
			name:                "Auth Enabled - Malformed Header (Bearer but only spaces as token)",
			authToken:           "secret-token",
			requestHeader:       "Bearer   ", // Note the trailing spaces
			requestBody:         emptyBodyBytes,
			expectStatusCode:    http.StatusUnauthorized,
			expectAuthFailure:   true,
		},
		{
			name:                "Auth Enabled - Valid Token with extra spaces around Bearer",
			authToken:           "secret-token",
			requestHeader:       "   Bearer   secret-token   ",
			requestBody:         validBodyBytes,
			// The current implementation of AlertsPostHandler uses `strings.SplitN(authHeader, " ", 2)`
			// which would make `parts[0]` be "   Bearer" and `parts[1]` be "  secret-token   "
			// This would fail the `parts[0] != "Bearer"` check.
			// If strict parsing of "Bearer" is intended, this should fail.
			// If flexibility is desired, the handler would need `strings.TrimSpace` on parts[0].
			// Given the current handler code, this will be treated as a malformed header.
			expectStatusCode:    http.StatusUnauthorized, // Based on current handler code
			expectAuthFailure:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBodyReader := bytes.NewReader(tt.requestBody)
			req, err := http.NewRequest("POST", "/alerts", reqBodyReader)
			if err != nil {
				t.Fatalf("could not create request: %v", err)
			}

			if tt.requestHeader != "" {
				// Special case: if header is exactly "Bearer ", SplitN might behave unexpectedly for some parts[1] access
				// For "Bearer  ", parts would be ["Bearer", " "].
				// For "Bearer ", parts would be ["Bearer", ""].
				// This is fine, the handler should reject these.
				req.Header.Set("Authorization", tt.requestHeader)
			}
			
			// If the body is not valid JSON for models.HookMessage, Decode will fail.
			// For successful auth cases, we use validBodyBytes.
			// For auth failure cases, the body decode won't be reached if auth fails first.
			// If auth is disabled, or passes, and the body is invalid, it will result in http.StatusBadRequest.
			// We need to ensure our expected status codes reflect this.

			rr := httptest.NewRecorder()
			server := &Server{
				AuthToken:  tt.authToken,
				AlertStore: memory.NewMemoryStore(10), // KubeClient can be nil for these tests
			}

			handler := http.HandlerFunc(server.AlertsPostHandler)
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectStatusCode {
				t.Errorf("expected status code %d, got %d. Response body: %s", tt.expectStatusCode, rr.Code, rr.Body.String())
			}

			// Further check for specific error messages if needed for 401 cases
			if tt.expectAuthFailure && rr.Code == http.StatusUnauthorized {
				responseBody := rr.Body.String()
				if strings.Contains(responseBody, "invalid request body") {
					t.Errorf("expected auth error, but got 'invalid request body', likely auth passed and body parsing failed or was unexpected")
				}
				// We can check for specific error messages here if we want to be more precise
				// e.g., "Authorization header missing", "Invalid token", "Invalid Authorization header format"
			}
			
			// If auth is NOT expected to fail, but we get a 400, it might be our validBody.
			// The handler returns http.StatusOK for successful POSTs after processing.
			// Let's adjust expectations for successful cases.
			// The handler logic is:
			// 1. Auth Check (potential 401)
			// 2. Decode body (potential 400 for bad JSON)
			// 3. Process (logging, then returns implicitly, meaning http.StatusOK is set by default if no error written by then)
			// So, if auth passes, and body is valid, it should be StatusOK.
			// If auth passes, and body is invalid (e.g. using emptyBodyBytes for a success case), it would be 400.
			// My test cases use validBodyBytes for success cases, so StatusOK is correct.
		})
	}
}

// Placeholder for any existing tests if the file were to be appended to.
// Since it's a new file, this is not strictly needed but good for structure.
func TestExistingFunctionality(t *testing.T) {
    // t.Skip("Skipping placeholder test for existing functionality")
}

// It's also good practice to test the AlertsGetHandler if it's in the same file,
// but the task is specific to AlertsPostHandler authentication.
func TestAlertsGetHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/alerts", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server := &Server{} // No auth token needed for GET
	handler := http.HandlerFunc(server.AlertsGetHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is "OK" as per handler logic
	expected := "\"OK\"\n" // JSON encoder adds a newline
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %q want %q",
			rr.Body.String(), expected)
	}
}
