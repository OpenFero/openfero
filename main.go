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
	authToken := flag.String("authToken", "", "Authentication token for API requests. If not set, authentication is disabled.")

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
	defer store.Close()

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

	// Initialize HTTP server
	server := &handlers.Server{
		KubeClient: kubeClient,
		AlertStore: store,
		AuthToken:  *authToken,
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
	http.HandleFunc("POST /alerts", server.AlertsPostHandler)
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
