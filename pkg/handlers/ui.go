package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/models"
	"github.com/ghodss/yaml"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

	// Get all ConfigMaps from store
	configMaps := s.KubeClient.ConfigMapStore.List()
	log.Debug("Retrieved ConfigMaps from store", zap.Int("count", len(configMaps)))

	var jobInfos []models.JobInfo
	for _, obj := range configMaps {
		configMap := obj.(*corev1.ConfigMap)

		// Process each job definition in ConfigMap
		for name, jobDef := range configMap.Data {
			log.Debug("Processing job definition",
				zap.String("configMap", configMap.Name),
				zap.String("jobName", name))

			// Parse YAML job definition
			yamlJobDefinition := []byte(jobDef)
			jsonBytes, err := yaml.YAMLToJSON(yamlJobDefinition)
			if err != nil {
				log.Error("Failed to convert YAML job definition to JSON",
					zap.String("configMap", configMap.Name),
					zap.String("jobName", name),
					zap.Error(err))
				continue
			}

			jobObject := &batchv1.Job{}
			if err := json.Unmarshal(jsonBytes, jobObject); err != nil {
				log.Error("Failed to unmarshal job definition",
					zap.String("configMap", configMap.Name),
					zap.String("jobName", name),
					zap.Error(err))
				continue
			}

			// Extract container image
			if len(jobObject.Spec.Template.Spec.Containers) > 0 {
				image := jobObject.Spec.Template.Spec.Containers[0].Image
				jobInfos = append(jobInfos, models.JobInfo{
					ConfigMapName: configMap.Name,
					JobName:       name,
					Image:         image,
				})
				log.Debug("Added job info",
					zap.String("configMap", configMap.Name),
					zap.String("jobName", name),
					zap.String("image", image))
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
	w.Header().Set(ContentTypeHeader, ApplicationJSONVal)
	w.WriteHeader(http.StatusOK)
	log.Debug("Health check successful")
}

// ReadinessGetHandler handles readiness probe requests
func (s *Server) ReadinessGetHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("Readiness check requested", zap.String("path", r.URL.Path))

	_, err := s.KubeClient.Clientset.CoreV1().ConfigMaps(s.KubeClient.ConfigmapNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error("Readiness check failed - unable to list ConfigMaps",
			zap.String("namespace", s.KubeClient.ConfigmapNamespace),
			zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	w.Header().Set(ContentTypeHeader, ApplicationJSONVal)
	w.WriteHeader(http.StatusOK)
	log.Debug("Readiness check successful",
		zap.String("namespace", s.KubeClient.ConfigmapNamespace))
}
