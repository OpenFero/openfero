package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/models"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
					OperariusName:       op.Name,
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
