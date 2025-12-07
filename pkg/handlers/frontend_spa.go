package handlers

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	log "github.com/OpenFero/openfero/pkg/logging"
	"go.uber.org/zap"
)

// FrontendFS holds the embedded Vue.js frontend files
// This will be set by main.go after embedding
var FrontendFS embed.FS

// frontendHandler serves the Vue.js SPA from embedded files
type frontendHandler struct {
	staticHandler http.Handler
	indexHTML     []byte
}

// NewFrontendHandler creates a new handler for serving the Vue.js SPA
func NewFrontendHandler(frontendFS embed.FS) http.Handler {
	// Get the dist subdirectory from the embedded filesystem
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Error("Failed to get frontend dist subdirectory", zap.Error(err))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Frontend not available", http.StatusInternalServerError)
		})
	}

	// Read index.html for SPA fallback
	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		log.Error("Failed to read index.html", zap.Error(err))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Frontend not available", http.StatusInternalServerError)
		})
	}

	return &frontendHandler{
		staticHandler: http.FileServer(http.FS(distFS)),
		indexHTML:     indexHTML,
	}
}

// ServeHTTP handles frontend requests
func (h *frontendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean the path
	urlPath := path.Clean(r.URL.Path)
	if urlPath == "" {
		urlPath = "/"
	}

	log.Debug("Frontend request",
		zap.String("path", urlPath),
		zap.String("method", r.Method))

	// Check if this is an API route (should not be handled here)
	if isAPIRoute(urlPath) {
		http.NotFound(w, r)
		return
	}

	// Check if the request is for a static asset (has file extension)
	if hasFileExtension(urlPath) {
		h.staticHandler.ServeHTTP(w, r)
		return
	}

	// For all other routes, serve index.html (SPA routing)
	w.Header().Set(ContentTypeHeader, "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if _, err := w.Write(h.indexHTML); err != nil {
		log.Error("Failed to write index.html", zap.Error(err))
	}
}

// isAPIRoute checks if the path is an API route
func isAPIRoute(urlPath string) bool {
	apiPrefixes := []string{
		"/alertStore",
		"/alerts",
		"/healthz",
		"/readiness",
		"/metrics",
		"/swagger",
	}

	for _, prefix := range apiPrefixes {
		if strings.HasPrefix(urlPath, prefix) {
			return true
		}
	}
	return false
}

// hasFileExtension checks if the path has a file extension
func hasFileExtension(urlPath string) bool {
	// Common static file extensions
	extensions := []string{
		".js", ".css", ".html", ".json", ".map",
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico",
		".woff", ".woff2", ".ttf", ".eot",
		".txt", ".xml",
	}

	for _, ext := range extensions {
		if strings.HasSuffix(urlPath, ext) {
			return true
		}
	}
	return false
}

// LegacyUIRedirectHandler redirects old HTMX routes to the new SPA
func LegacyUIRedirectHandler(w http.ResponseWriter, r *http.Request) {
	// Map old routes to new Vue.js routes
	newPath := r.URL.Path
	switch r.URL.Path {
	case "/":
		newPath = "/"
	case "/jobs":
		newPath = "/jobs"
	case "/about":
		// About is now a modal in the navbar, redirect to home
		newPath = "/"
	}

	http.Redirect(w, r, newPath, http.StatusTemporaryRedirect)
}
