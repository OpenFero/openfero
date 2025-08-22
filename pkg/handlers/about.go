package handlers

import (
	"html/template"
	"net/http"

	log "github.com/OpenFero/openfero/pkg/logging"
	"go.uber.org/zap"
)

// BuildInfo contains information about the build
type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// Global variable to store build information
var buildInformation BuildInfo

// SetBuildInfo sets the build information
func SetBuildInfo(version, commit, date string) {
	buildInformation = BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildDate: date,
	}
}

// AboutHandler handles GET requests to /about
func AboutHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(ContentTypeHeader, "text/html")

	log.Debug("Processing about page request",
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.String("remoteAddr", r.RemoteAddr))

	// Parse templates
	tmpl, err := template.ParseFiles(
		"web/templates/about.html.templ",
		"web/templates/navbar.html.templ",
	)
	if err != nil {
		log.Error("Failed to parse templates", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	data := struct {
		Title      string
		ShowSearch bool
		Version    string
		Commit     string
		BuildDate  string
	}{
		Title:      "About",
		ShowSearch: false,
		Version:    buildInformation.Version,
		Commit:     buildInformation.Commit,
		BuildDate:  buildInformation.BuildDate,
	}

	// Execute templates
	if err = tmpl.Execute(w, data); err != nil {
		log.Error("Failed to execute templates", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}

	log.Debug("About page request completed successfully",
		zap.String("path", r.URL.Path))
}
