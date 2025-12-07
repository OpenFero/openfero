package handlers

import (
	"net/http"

	log "github.com/OpenFero/openfero/pkg/logging"
	"go.uber.org/zap"
)

// HealthzGetHandler handles health status requests
func (s *Server) HealthzGetHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("Health check requested", zap.String("path", r.URL.Path))
	w.Header().Set(ContentTypeHeader, "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
	log.Debug("Health check successful")
}

// ReadinessGetHandler handles readiness probe requests
func (s *Server) ReadinessGetHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("Readiness check requested", zap.String("path", r.URL.Path))

	_, err := s.KubeClient.Clientset.Discovery().ServerVersion()
	if err != nil {
		log.Error("Readiness check failed - unable to contact API server",
			zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	w.Header().Set(ContentTypeHeader, "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
	log.Debug("Readiness check successful")
}
