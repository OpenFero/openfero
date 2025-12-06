package handlers

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/models"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UIHandler handles GET requests to /
func UIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(ContentTypeHeader, "text/html")

	log.Debug("Processing UI request",
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.String("remoteAddr", r.RemoteAddr))

	// Parse templates
	tmpl, err := template.ParseFiles(
		"web/templates/alertStore.html.templ",
		"web/templates/navbar.html.templ",
	)
	if err != nil {
		log.Error("Failed to parse templates", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	query := r.URL.Query().Get("q")
	log.Debug("Fetching alerts with query filter", zap.String("query", query))
	alerts := GetAlerts(query)

	data := struct {
		Title      string
		ShowSearch bool
		Alerts     []models.AlertStoreEntry
		Version    string
		Commit     string
		BuildDate  string
	}{
		Title:      "Alerts",
		ShowSearch: true,
		Alerts:     alerts,
		Version:    buildInformation.Version,
		Commit:     buildInformation.Commit,
		BuildDate:  buildInformation.BuildDate,
	}

	// Execute templates
	if err = tmpl.Execute(w, data); err != nil {
		log.Error("Failed to execute templates", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}

	log.Debug("UI request completed successfully",
		zap.String("path", r.URL.Path),
		zap.Int("alertCount", len(alerts)))
}

// GetAlerts fetches alerts from the alert store
func GetAlerts(query string) []models.AlertStoreEntry {
	log.Debug("Fetching alerts from alert store", zap.String("query", query))

	// URL-encode the query parameter to handle special characters
	encodedQuery := url.QueryEscape(query)
	resp, err := http.Get("http://localhost:8080/alertStore?q=" + encodedQuery)
	if err != nil {
		log.Error("Failed to get alerts from alert store", zap.Error(err))
		return nil
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Error("Failed to close response body", zap.Error(closeErr))
		}
	}()

	var alerts []models.AlertStoreEntry
	err = json.NewDecoder(resp.Body).Decode(&alerts)
	if err != nil {
		log.Error("Failed to decode alerts response", zap.Error(err))
		return nil
	}

	log.Debug("Successfully retrieved alerts", zap.Int("count", len(alerts)))
	return alerts
}

// JobsUIHandler handles GET requests to /jobs
func (s *Server) JobsUIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(ContentTypeHeader, "text/html")

	log.Debug("Processing jobs UI request",
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.String("remoteAddr", r.RemoteAddr))

	var jobInfos []models.JobInfo

	if s.OperariusService != nil {
		// Fetch Operarii
		operarii, err := s.OperariusService.GetOperariiForNamespace(r.Context(), "")
		if err != nil {
			log.Error("Failed to get Operarii", zap.Error(err))
		} else {
			log.Debug("Retrieved Operarii", zap.Int("count", len(operarii)))
			for _, op := range operarii {
				image := "unknown"
				if len(op.Spec.JobTemplate.Spec.Template.Spec.Containers) > 0 {
					image = op.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image
				}

				jobInfos = append(jobInfos, models.JobInfo{
					ConfigMapName: op.Name,
					JobName:       op.Spec.AlertSelector.AlertName,
					Image:         image,
				})
			}
		}
	}

	// Parse and execute template
	tmpl, err := template.ParseFiles(
		"web/templates/jobs.html.templ",
		"web/templates/navbar.html.templ",
	)
	if err != nil {
		log.Error("Failed to parse job templates", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	data := struct {
		Title      string
		ShowSearch bool
		Jobs       []models.JobInfo
		Version    string
		Commit     string
		BuildDate  string
	}{
		Title:      "Jobs",
		ShowSearch: false,
		Jobs:       jobInfos,
		Version:    buildInformation.Version,
		Commit:     buildInformation.Commit,
		BuildDate:  buildInformation.BuildDate,
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Error("Failed to execute job templates", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}

	log.Debug("Jobs UI request completed successfully",
		zap.String("path", r.URL.Path),
		zap.Int("jobCount", len(jobInfos)))
}

// JobsAPIHandler handles GET requests to /api/jobs - returns JSON
// @Summary Get all configured jobs
// @Description Returns a list of all job definitions from ConfigMaps
// @Tags jobs
// @Produce json
// @Success 200 {array} models.JobInfo
// @Router /api/jobs [get]
func (s *Server) JobsAPIHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("Processing jobs API request",
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.String("remoteAddr", r.RemoteAddr))

	var jobInfos []models.JobInfo

	if s.OperariusService != nil {
		// Fetch Operarii
		operarii, err := s.OperariusService.GetOperariiForNamespace(r.Context(), "")
		if err != nil {
			log.Error("Failed to get Operarii", zap.Error(err))
		} else {
			log.Debug("Retrieved Operarii", zap.Int("count", len(operarii)))
			for _, op := range operarii {
				image := "unknown"
				if len(op.Spec.JobTemplate.Spec.Template.Spec.Containers) > 0 {
					image = op.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image
				}

				var lastExecutionTime *time.Time
				if op.Status.LastExecutionTime != nil {
					t := op.Status.LastExecutionTime.Time
					lastExecutionTime = &t
				}

				jobInfos = append(jobInfos, models.JobInfo{
					ConfigMapName:       op.Name,
					JobName:             op.Spec.AlertSelector.AlertName, // Use AlertName as JobName for display
					Namespace:           op.Namespace,
					Image:               image,
					ExecutionCount:      op.Status.ExecutionCount,
					LastExecutionTime:   lastExecutionTime,
					LastExecutedJobName: op.Status.LastExecutedJobName,
					LastExecutionStatus: op.Status.LastExecutionStatus,
				})
			}
		}
	}

	// Enrich with live status for active jobs
	ctx := r.Context()
	for i := range jobInfos {
		// If we have a last executed job name, check its current status
		if jobInfos[i].LastExecutedJobName != "" && jobInfos[i].Namespace != "" {
			job, err := s.KubeClient.Clientset.BatchV1().Jobs(jobInfos[i].Namespace).Get(ctx, jobInfos[i].LastExecutedJobName, metav1.GetOptions{})
			if err == nil {
				// Determine status
				status := "Pending"
				if job.Status.Succeeded > 0 {
					status = "Successful"
				} else if job.Status.Failed > 0 {
					status = "Failed"
				} else if job.Status.Active > 0 {
					status = "Running"
				}
				// Update status if it's different (e.g. Running vs Pending stored in CRD)
				// Note: We don't persist "Running" to CRD to avoid churn, but we show it in UI
				jobInfos[i].LastExecutionStatus = status
			}
		}
	}

	w.Header().Set(ContentTypeHeader, ApplicationJSONVal)
	json.NewEncoder(w).Encode(jobInfos)
}

// AssetsHandler serves static assets
func AssetsHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("Serving asset", zap.String("path", r.URL.Path))

	// set content type based on file extension
	contentType := ""
	extension := filepath.Ext(r.URL.Path)
	switch extension {
	case ".css", ".min.css":
		contentType = "text/css"
	case ".js", ".min.js":
		contentType = "application/javascript"
	case ".woff", ".woff2":
		contentType = "font/woff2"
	case ".ttf":
		contentType = "font/ttf"
	case ".eot":
		contentType = "application/vnd.ms-fontobject"
	case ".svg":
		contentType = "image/svg+xml"
	}

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	log.Debug("Asset content type determined",
		zap.String("path", r.URL.Path),
		zap.String("extension", extension),
		zap.String("contentType", contentType))

	// sanitize the URL path to prevent path traversal
	path, err := VerifyPath(r.URL.Path)
	if err != nil {
		log.Warn("Invalid asset path specified",
			zap.String("path", r.URL.Path),
			zap.Error(err))
		http.Error(w, "Invalid path specified", http.StatusBadRequest)
		return
	}

	log.Debug("Serving filesystem asset",
		zap.String("requestPath", r.URL.Path),
		zap.String("filesystemPath", path))

	// serve assets from the web/assets directory
	http.ServeFile(w, r, path)
}

// VerifyPath verifies and evaluates the given path to ensure it is safe
func VerifyPath(path string) (string, error) {
	errmsg := "unsafe or invalid path specified"
	wd, err := os.Getwd()
	if err != nil {
		log.Error("Failed to get working directory", zap.Error(err))
		return "", errors.New(errmsg)
	}
	trustedRoot := filepath.Join(wd, "web")
	log.Debug("Path verification",
		zap.String("path", path),
		zap.String("trustedRoot", trustedRoot))

	// Clean the path to remove any .. or . elements
	cleanPath := filepath.Clean(path)
	// Join the trusted root and the cleaned path
	absPath, err := filepath.Abs(filepath.Join(trustedRoot, cleanPath))
	if err != nil {
		log.Error("Failed to get absolute path",
			zap.String("path", path),
			zap.Error(err))
		return "", errors.New(errmsg)
	}

	if !strings.HasPrefix(absPath, trustedRoot) {
		log.Warn("Path traversal attempt detected",
			zap.String("requestedPath", path),
			zap.String("resolvedPath", absPath),
			zap.String("trustedRoot", trustedRoot))
		return "", errors.New(errmsg)
	}

	return absPath, nil
}

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
