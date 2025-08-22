package main

import (
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	_ "github.com/OpenFero/openfero/pkg/docs"
	"github.com/OpenFero/openfero/pkg/handlers"
	"github.com/OpenFero/openfero/pkg/kubernetes"
	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/metadata"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OpenFero/openfero/pkg/alertstore"
	"github.com/OpenFero/openfero/pkg/alertstore/memberlist"
	"github.com/OpenFero/openfero/pkg/alertstore/memory"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// initLogger initializes the logger with the given log level
func initLogger(logLevel string) error {
	var cfg zap.Config
	switch strings.ToLower(logLevel) {
	case "debug":
		cfg = zap.NewDevelopmentConfig()
	case "info":
		cfg = zap.NewProductionConfig()
	default:
		return fmt.Errorf("invalid log level specified: %s", logLevel)
	}

	return log.SetConfig(cfg)
}

// validateAuthConfig validates the authentication configuration
func validateAuthConfig(config handlers.AuthConfig) error {
	switch config.Method {
	case handlers.AuthMethodNone:
		// No validation needed for "none"
		return nil
	case handlers.AuthMethodBasic:
		if config.BasicUser == "" || config.BasicPass == "" {
			return fmt.Errorf("basic authentication requires both username and password")
		}
		return nil
	case handlers.AuthMethodBearer:
		if config.BearerToken == "" {
			return fmt.Errorf("bearer authentication requires a token")
		}
		return nil
	case handlers.AuthMethodOAuth2:
		if config.OAuth2Issuer == "" || config.OAuth2Audience == "" {
			return fmt.Errorf("OAuth2 authentication requires both issuer and audience")
		}
		return nil
	default:
		return fmt.Errorf("unsupported authentication method: %s", config.Method)
	}
}

// @title OpenFero API
// @version 1.0
// @description OpenFero is intended as an event-triggered job scheduler for code agnostic recovery jobs.

// @contact.name GitHub Issues
// @contact.url https://github.com/OpenFero/openfero/issues

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /
func main() {
	// Parse command line arguments
	addr := flag.String("addr", ":8080", "address to listen for webhook")
	logLevel := flag.String("logLevel", "info", "log level")
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	configmapNamespace := flag.String("configmapNamespace", "", "Kubernetes namespace where jobs are defined")
	jobDestinationNamespace := flag.String("jobDestinationNamespace", "", "Kubernetes namespace where jobs will be created")
	readTimeout := flag.Int("readTimeout", 5, "read timeout in seconds")
	writeTimeout := flag.Int("writeTimeout", 10, "write timeout in seconds")
	alertStoreSize := flag.Int("alertStoreSize", 10, "size of the alert store")
	alertStoreType := flag.String("alertStoreType", "memory", "type of alert store (memory, memberlist)")
	alertStoreClusterName := flag.String("alertStoreClusterName", "openfero", "Cluster name for memberlist alert store")
	labelSelector := flag.String("labelSelector", "app=openfero", "label selector for OpenFero ConfigMaps in the format key=value")

	// Authentication flags
	authMethod := flag.String("authMethod", "none", "authentication method for webhook endpoint (none, basic, bearer, oauth2)")
	authBasicUser := flag.String("authBasicUser", "", "username for basic authentication")
	authBasicPass := flag.String("authBasicPass", "", "password for basic authentication")
	authBearerToken := flag.String("authBearerToken", "", "bearer token for token-based authentication")
	authOAuth2Issuer := flag.String("authOAuth2Issuer", "", "OAuth2 token issuer URL")
	authOAuth2Audience := flag.String("authOAuth2Audience", "", "OAuth2 token audience")

	flag.Parse()

	// Configure logger first
	if err := initLogger(*logLevel); err != nil {
		log.Fatal("Could not set log configuration")
	}

	log.Info("Starting OpenFero", zap.String("version", version), zap.String("commit", commit), zap.String("date", date))

	// Initialize the appropriate alert store based on configuration
	var store alertstore.Store
	switch *alertStoreType {
	case "memberlist":
		store = memberlist.NewMemberlistStore(*alertStoreClusterName, *alertStoreSize)
	default:
		store = memory.NewMemoryStore(*alertStoreSize)
	}

	// Initialize the alert store
	if err := store.Initialize(); err != nil {
		log.Fatal("Failed to initialize alert store", zap.String("error", err.Error()))
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Error("Failed to close alert store", zap.Error(err))
		}
	}()

	// Use the in-cluster config to create a kubernetes client
	clientset := kubernetes.InitKubeClient(kubeconfig)

	// Get current namespace if not specified
	currentNamespace, err := kubernetes.GetCurrentNamespace()
	if err != nil {
		log.Fatal("Current kubernetes namespace could not be found", zap.String("error", err.Error()))
	}

	if *configmapNamespace == "" {
		configmapNamespace = &currentNamespace
	}

	if *jobDestinationNamespace == "" {
		jobDestinationNamespace = &currentNamespace
	}

	// Parse label selector
	parsedLabelSelector, err := metav1.ParseToLabelSelector(*labelSelector)
	if err != nil {
		log.Fatal("Could not parse label selector", zap.String("error", err.Error()))
	}

	log.Debug("Using label selector: " + metav1.FormatLabelSelector(parsedLabelSelector))

	// Create informer factories
	configMapInformer := kubernetes.InitConfigMapInformer(clientset, *configmapNamespace, parsedLabelSelector)
	jobInformer := kubernetes.InitJobInformer(clientset, *jobDestinationNamespace, parsedLabelSelector)

	// Initialize Kubernetes client
	kubeClient := &kubernetes.Client{
		Clientset:               *clientset,
		JobDestinationNamespace: *jobDestinationNamespace,
		ConfigmapNamespace:      *configmapNamespace,
		ConfigMapStore:          configMapInformer,
		JobStore:                jobInformer,
		LabelSelector:           parsedLabelSelector,
	}

	// Validate and create authentication configuration
	authConfig := handlers.AuthConfig{
		Method:         handlers.AuthMethod(*authMethod),
		BasicUser:      *authBasicUser,
		BasicPass:      *authBasicPass,
		BearerToken:    *authBearerToken,
		OAuth2Issuer:   *authOAuth2Issuer,
		OAuth2Audience: *authOAuth2Audience,
	}

	// Validate authentication configuration
	if err := validateAuthConfig(authConfig); err != nil {
		log.Fatal("Invalid authentication configuration", zap.Error(err))
	}

	// Log authentication configuration (without sensitive data)
	if authConfig.Method != handlers.AuthMethodNone {
		log.Info("Authentication enabled for webhook endpoint",
			zap.String("method", string(authConfig.Method)))
	} else {
		log.Info("No authentication configured for webhook endpoint")
	}

	// Initialize HTTP server
	server := &handlers.Server{
		KubeClient: kubeClient,
		AlertStore: store,
		AuthConfig: authConfig,
	}

	// Pass build information to handlers
	handlers.SetBuildInfo(version, commit, date)

	// Register metrics and set prometheus handler
	metadata.AddMetricsToPrometheusRegistry()
	http.HandleFunc("GET "+metadata.MetricsPath, func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	})

	// Register HTTP routes
	log.Info("Starting webhook receiver")
	http.HandleFunc("GET /healthz", server.HealthzGetHandler)
	http.HandleFunc("GET /readiness", server.ReadinessGetHandler)
	http.HandleFunc("GET /alertStore", server.AlertStoreGetHandler)
	http.HandleFunc("GET /alerts", server.AlertsGetHandler)

	// Apply authentication middleware to the webhook endpoint
	authMiddleware := handlers.AuthMiddleware(authConfig)
	http.HandleFunc("POST /alerts", authMiddleware(server.AlertsPostHandler))

	// Other routes remain unprotected
	http.HandleFunc("GET /", handlers.UIHandler)
	http.HandleFunc("GET /jobs", server.JobsUIHandler)
	http.HandleFunc("GET /about", handlers.AboutHandler)
	http.HandleFunc("GET /assets/", handlers.AssetsHandler)
	http.Handle("GET /swagger/", httpSwagger.Handler(
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("swagger-ui"),
	))

	// Create and start HTTP server
	srv := &http.Server{
		Addr:         *addr,
		ReadTimeout:  time.Duration(*readTimeout) * time.Second,
		WriteTimeout: time.Duration(*writeTimeout) * time.Second,
	}

	log.Info("Starting server on " + *addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal("error starting server: ", zap.String("error", err.Error()))
	}
}
