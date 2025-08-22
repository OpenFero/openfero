package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/OpenFero/openfero/pkg/alertstore"
	"github.com/OpenFero/openfero/pkg/kubernetes"
	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/models"
	"github.com/OpenFero/openfero/pkg/services"
	"github.com/OpenFero/openfero/pkg/utils"
	"go.uber.org/zap"
)

const (
	ContentTypeHeader  = "Content-Type"
	ApplicationJSONVal = "application/json"
)

// Server holds dependencies for handlers
type Server struct {
	KubeClient *kubernetes.Client
	AlertStore alertstore.Store
	AuthConfig AuthConfig
}

// AlertsGetHandler handles GET requests to /alerts
func (s *Server) AlertsGetHandler(w http.ResponseWriter, r *http.Request) {
	// Alertmanager expects a 200 OK response, otherwise send_resolved will never work
	enc := json.NewEncoder(w)
	w.Header().Set(ContentTypeHeader, ApplicationJSONVal)
	w.WriteHeader(http.StatusOK)

	if err := enc.Encode("OK"); err != nil {
		log.Error("error encoding messages: ", zap.String("error", err.Error()))
		http.Error(w, "", http.StatusInternalServerError)
	}
}

// AlertsPostHandler handles POST requests to /alerts
func (s *Server) AlertsPostHandler(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Error("Failed to close request body", zap.Error(err))
		}
	}()

	message := models.HookMessage{}
	if err := dec.Decode(&message); err != nil {
		log.Error("error decoding message: ", zap.String("error", err.Error()))
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	status := utils.SanitizeInput(message.Status)
	alertcount := len(message.Alerts)

	// Use zap's fields for structured logging instead of string concatenation
	log.Debug("Webhook received",
		zap.String("status", status),
		zap.Int("alertCount", alertcount))

	if !services.CheckAlertStatus(status) {
		log.Warn("Status of alert was neither firing nor resolved, stop creating a response job.")
		return
	}

	// Use zap's fields for structured logging
	log.Debug("Creating response jobs",
		zap.Int("jobCount", alertcount),
		zap.String("groupKey", message.GroupKey))

	for _, alert := range message.Alerts {
		go services.CreateResponseJob(s.KubeClient, s.AlertStore, alert, status, message.GroupKey)
	}
}

// AlertStoreGetHandler handles GET requests to /alertStore
func (s *Server) AlertStoreGetHandler(w http.ResponseWriter, r *http.Request) {
	// Get search query parameter
	query := r.URL.Query().Get("q")
	limit := 100 // Default limit

	alerts, err := s.AlertStore.GetAlerts(query, limit)
	if err != nil {
		log.Error("Error retrieving alerts", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	w.Header().Set(ContentTypeHeader, ApplicationJSONVal)
	err = json.NewEncoder(w).Encode(alerts)
	if err != nil {
		log.Error("Error encoding alerts", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}
}
