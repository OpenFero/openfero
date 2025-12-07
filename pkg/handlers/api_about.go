package handlers

import (
	"encoding/json"
	"net/http"

	log "github.com/OpenFero/openfero/pkg/logging"
	"go.uber.org/zap"
)

// BuildInfo contains information about the build
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
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

// AboutAPIHandler handles GET requests to /api/about - returns JSON
// @Summary Get build information
// @Description Returns version, commit hash, and build date
// @Tags about
// @Produce json
// @Success 200 {object} BuildInfo
// @Router /api/about [get]
func AboutAPIHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("Processing about API request",
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method))

	w.Header().Set(ContentTypeHeader, ApplicationJSONVal)
	if err := json.NewEncoder(w).Encode(buildInformation); err != nil {
		log.Error("Failed to encode build info", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	log.Debug("About API request completed successfully")
}
