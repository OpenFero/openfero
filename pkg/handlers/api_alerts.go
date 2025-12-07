package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/OpenFero/openfero/pkg/alertstore"
	"github.com/OpenFero/openfero/pkg/kubernetes"
	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/metadata"
	"github.com/OpenFero/openfero/pkg/models"
	"github.com/OpenFero/openfero/pkg/services"
	"github.com/OpenFero/openfero/pkg/utils"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ContentTypeHeader  = "Content-Type"
	ApplicationJSONVal = "application/json"
)

// Server holds dependencies for handlers
type Server struct {
	KubeClient       *kubernetes.Client
	AlertStore       alertstore.Store
	AuthConfig       AuthConfig
	OperariusService *services.OperariusService // Service for Operarius CRDs
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
		zap.Int("alertCount", alertcount),
		zap.String("groupKey", message.GroupKey))

	if !services.CheckAlertStatus(status) {
		log.Warn("Status of alert was neither firing nor resolved, stop creating a response job.")
		return
	}

	if s.OperariusService != nil {
		s.handleOperariusBasedJobs(r.Context(), message)
	} else {
		log.Error("OperariusService is not initialized")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleOperariusBasedJobs handles job creation using Operarius CRDs
func (s *Server) handleOperariusBasedJobs(ctx context.Context, hookMessage models.HookMessage) {
	log.Debug("Processing webhook with Operarius CRDs",
		zap.String("status", hookMessage.Status),
		zap.String("groupKey", hookMessage.GroupKey),
		zap.Int("alertCount", len(hookMessage.Alerts)))

	var jobInfo *alertstore.JobInfo

	// Get all available Operarii (in a real implementation, you'd use a controller-runtime client)
	// For now, this is a placeholder - you'll need to implement GetOperariiForNamespace
	operarii, err := s.OperariusService.GetOperariiForNamespace(ctx, "openfero")
	if err != nil {
		log.Error("Failed to get Operarii", zap.Error(err))
		// Continue to store alert even if we can't get Operarii
	} else {
		// Find matching Operarius
		operarius, err := s.OperariusService.FindMatchingOperarius(hookMessage, operarii)
		if err != nil {
			log.Info("No matching Operarius found - alert will be stored without remediation", zap.Error(err))
		} else {
			log.Info("Found matching Operarius",
				zap.String("operarius", operarius.Name),
				zap.String("namespace", operarius.Namespace),
				zap.Int32("priority", operarius.Spec.Priority))

			// Check deduplication
			shouldCreate, err := s.OperariusService.CheckDeduplication(ctx, operarius, hookMessage)
			if err != nil {
				log.Error("Failed to check deduplication", zap.Error(err))
			} else if !shouldCreate {
				log.Info("Skipping job creation due to deduplication",
					zap.String("operarius", operarius.Name),
					zap.String("groupKey", hookMessage.GroupKey))

				var lastExecutionTime *time.Time
				if operarius.Status.LastExecutionTime != nil {
					t := operarius.Status.LastExecutionTime.Time
					lastExecutionTime = &t
				}

				jobInfo = &alertstore.JobInfo{
					JobName:             "N/A (Deduplicated)",
					Namespace:           operarius.Namespace,
					OperariusName:       operarius.Name,
					Image:               "N/A",
					ExecutionCount:      operarius.Status.ExecutionCount,
					LastExecutionTime:   lastExecutionTime,
					LastExecutedJobName: operarius.Status.LastExecutedJobName,
					LastExecutionStatus: "Skipped: Deduplication",
				}
			} else {
				// Create the job
				job, err := s.OperariusService.CreateJobFromOperarius(ctx, operarius, hookMessage)
				if err != nil {
					log.Error("Failed to create job from Operarius",
						zap.Error(err),
						zap.String("operarius", operarius.Name))
					metadata.JobsFailedTotal.Inc()
				} else {
					// Increment successful job creation metric
					metadata.JobsCreatedTotal.Inc()

					log.Info("Successfully created remediation job",
						zap.String("jobName", job.Name),
						zap.String("operarius", operarius.Name),
						zap.String("namespace", job.Namespace),
						zap.String("groupKey", hookMessage.GroupKey))

					// Update Operarius status with execution info
					if err := s.OperariusService.UpdateOperariusStatus(ctx, operarius, job.Name); err != nil {
						log.Warn("Failed to update Operarius status",
							zap.Error(err),
							zap.String("operarius", operarius.Name))
						// Don't return - job was created successfully, status update is best-effort
					}

					var lastExecutionTime *time.Time
					if operarius.Status.LastExecutionTime != nil {
						t := operarius.Status.LastExecutionTime.Time
						lastExecutionTime = &t
					}

					jobInfo = &alertstore.JobInfo{
						JobName:             job.Name,
						Namespace:           job.Namespace,
						OperariusName:       operarius.Name, // Operarius name for tracking
						Image:               getFirstContainerImage(job),
						ExecutionCount:      operarius.Status.ExecutionCount,
						LastExecutionTime:   lastExecutionTime,
						LastExecutedJobName: operarius.Status.LastExecutedJobName,
						LastExecutionStatus: operarius.Status.LastExecutionStatus,
					}
				}
			}
		}
	}

	// Store alert in alert store for tracking and broadcast to SSE clients
	for _, alert := range hookMessage.Alerts {
		if jobInfo != nil {
			// Use the service function which handles both storage and SSE broadcast
			services.SaveAlertWithJobInfo(s.AlertStore, alert, hookMessage.Status, jobInfo)
		} else {
			// Save without job info
			services.SaveAlert(s.AlertStore, alert, hookMessage.Status)
		}
	}
}

// getFirstContainerImage extracts the image from the first container in the job
func getFirstContainerImage(job *batchv1.Job) string {
	if len(job.Spec.Template.Spec.Containers) > 0 {
		return job.Spec.Template.Spec.Containers[0].Image
	}
	return "unknown"
}

// AlertStoreGetHandler handles GET requests to /alertStore
func (s *Server) AlertStoreGetHandler(w http.ResponseWriter, r *http.Request) {
	// Get search query parameter
	query := r.URL.Query().Get("q")
	limit := 100 // Default limit

	alerts, err := s.AlertStore.GetAlerts(query, limit)
	if err != nil {
		log.Error("Error retrieving alerts", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Enrich alerts with live job status
	ctx := r.Context()
	for i := range alerts {
		if alerts[i].JobInfo != nil && alerts[i].JobInfo.JobName != "" && alerts[i].JobInfo.Namespace != "" {
			// Skip lookup if job was skipped or has invalid name (e.g. "N/A (Deduplicated)")
			if strings.HasPrefix(alerts[i].JobInfo.LastExecutionStatus, "Skipped") || strings.Contains(alerts[i].JobInfo.JobName, " ") {
				continue
			}

			job, err := s.KubeClient.Clientset.BatchV1().Jobs(alerts[i].JobInfo.Namespace).Get(ctx, alerts[i].JobInfo.JobName, metav1.GetOptions{})
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
				alerts[i].JobInfo.LastExecutionStatus = status
			} else {
				// Job might be deleted or error
				log.Debug("Failed to get job for alert",
					zap.String("job", alerts[i].JobInfo.JobName),
					zap.Error(err))
			}
		}
	}

	w.Header().Set(ContentTypeHeader, ApplicationJSONVal)
	err = json.NewEncoder(w).Encode(alerts)
	if err != nil {
		log.Error("Error encoding alerts", zap.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
	}
}
