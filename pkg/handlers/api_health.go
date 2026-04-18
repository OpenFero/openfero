package handlers

import (
	"net/http"

	log "github.com/OpenFero/openfero/pkg/logging"
)

// HealthzGetHandler handles health status requests
// @Summary Liveness probe
// @Description Returns 200 OK if the process is alive
// @Tags health
// @Produce plain
// @Success 200 {string} string "ok"
// @Router /healthz [get]
func (s *Server) HealthzGetHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("Health check requested", "path", r.URL.Path)
	w.Header().Set(ContentTypeHeader, "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// ReadinessGetHandler handles readiness probe requests
// @Summary Readiness probe
// @Description Returns 200 OK if the application is ready to receive traffic
// @Tags health
// @Produce plain
// @Success 200 {string} string "ok"
// @Router /readiness [get]
func (s *Server) ReadinessGetHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("Readiness check requested", "path", r.URL.Path)
	w.Header().Set(ContentTypeHeader, "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// StartupzGetHandler handles startup probe requests
// @Summary Startup probe
// @Description Returns 200 OK once informer caches have synced and the application is initialized
// @Tags health
// @Produce plain
// @Success 200 {string} string "ok"
// @Failure 503 {string} string "starting"
// @Router /startupz [get]
func (s *Server) StartupzGetHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("Startup check requested", "path", r.URL.Path)
	w.Header().Set(ContentTypeHeader, "text/plain")
	if !s.StartupComplete.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("starting"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
