package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"

	log "github.com/OpenFero/openfero/pkg/logging"
	"go.uber.org/zap"
)

// AuthMethod represents the type of authentication method
type AuthMethod string

const (
	AuthMethodNone   AuthMethod = "none"
	AuthMethodBasic  AuthMethod = "basic"
	AuthMethodBearer AuthMethod = "bearer"
	AuthMethodOAuth2 AuthMethod = "oauth2"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Method         AuthMethod
	BasicUser      string
	BasicPass      string
	BearerToken    string
	OAuth2Issuer   string
	OAuth2Audience string
}

// AuthMiddleware creates a middleware function that handles authentication
func AuthMiddleware(config AuthConfig) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication if method is "none"
			if config.Method == AuthMethodNone {
				next(w, r)
				return
			}

			// Authenticate based on configured method
			authenticated := false
			var authMethod string

			switch config.Method {
			case AuthMethodBasic:
				authenticated, authMethod = authenticateBasic(r, config.BasicUser, config.BasicPass)
			case AuthMethodBearer:
				authenticated, authMethod = authenticateBearer(r, config.BearerToken)
			case AuthMethodOAuth2:
				authenticated, authMethod = authenticateOAuth2(r, config.OAuth2Issuer, config.OAuth2Audience)
			default:
				log.Warn("Unknown authentication method", zap.String("method", string(config.Method)))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if !authenticated {
				log.Warn("Authentication failed",
					zap.String("method", authMethod),
					zap.String("remoteAddr", r.RemoteAddr),
					zap.String("userAgent", r.UserAgent()))

				// Set appropriate WWW-Authenticate header
				switch config.Method {
				case AuthMethodBasic:
					w.Header().Set("WWW-Authenticate", "Basic realm=\"OpenFero\"")
				case AuthMethodBearer:
					w.Header().Set("WWW-Authenticate", "Bearer realm=\"OpenFero\"")
				case AuthMethodOAuth2:
					w.Header().Set("WWW-Authenticate", "Bearer realm=\"OpenFero\"")
				}

				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			log.Debug("Authentication successful",
				zap.String("method", authMethod),
				zap.String("remoteAddr", r.RemoteAddr))

			next(w, r)
		}
	}
}

// authenticateBasic performs HTTP Basic Authentication
func authenticateBasic(r *http.Request, expectedUser, expectedPass string) (bool, string) {
	if expectedUser == "" || expectedPass == "" {
		log.Error("Basic auth credentials not configured")
		return false, "basic"
	}

	user, pass, ok := r.BasicAuth()
	if !ok {
		return false, "basic"
	}

	// Use constant-time comparison to prevent timing attacks
	userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(expectedUser)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(expectedPass)) == 1

	return userMatch && passMatch, "basic"
}

// authenticateBearer performs Bearer Token Authentication
func authenticateBearer(r *http.Request, expectedToken string) (bool, string) {
	if expectedToken == "" {
		log.Error("Bearer token not configured")
		return false, "bearer"
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false, "bearer"
	}

	// Check if header starts with "Bearer "
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return false, "bearer"
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)

	// Use constant-time comparison to prevent timing attacks
	tokenMatch := subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) == 1

	return tokenMatch, "bearer"
}

// authenticateOAuth2 performs OAuth2 token validation
func authenticateOAuth2(r *http.Request, issuer, audience string) (bool, string) {
	// TODO: Implement OAuth2 JWT token validation
	// This would involve:
	// 1. Extracting JWT token from Authorization header
	// 2. Validating JWT signature against issuer's public key
	// 3. Verifying token claims (issuer, audience, expiration, etc.)
	// 4. For now, return false to indicate not implemented

	log.Warn("OAuth2 authentication is not yet implemented")
	return false, "oauth2"
}
