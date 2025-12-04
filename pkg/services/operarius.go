package services

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	operariusv1alpha1 "github.com/OpenFero/openfero/api/v1alpha1"
	k8sclient "github.com/OpenFero/openfero/pkg/kubernetes"
	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/models"
	"go.uber.org/zap"
)

// OperariusService handles Operarius CRD-based job creation
type OperariusService struct {
	kubeClient       kubernetes.Interface
	operariusClient  *k8sclient.OperariusClient
}

// NewOperariusService creates a new OperariusService
func NewOperariusService(kubeClient kubernetes.Interface) *OperariusService {
	return &OperariusService{
		kubeClient: kubeClient,
	}
}

// NewOperariusServiceWithClient creates a new OperariusService with an Operarius client
func NewOperariusServiceWithClient(kubeClient kubernetes.Interface, operariusClient *k8sclient.OperariusClient) *OperariusService {
	return &OperariusService{
		kubeClient:      kubeClient,
		operariusClient: operariusClient,
	}
}

// FindMatchingOperarius finds the best matching Operarius for an alert from a webhook
func (s *OperariusService) FindMatchingOperarius(hookMessage models.HookMessage, operarii []operariusv1alpha1.Operarius) (*operariusv1alpha1.Operarius, error) {
	var matchingOperarii []operariusv1alpha1.Operarius

	for _, operarius := range operarii {
		if s.matchesHookMessage(operarius, hookMessage) {
			matchingOperarii = append(matchingOperarii, operarius)
		}
	}

	if len(matchingOperarii) == 0 {
		// Try to get alertname from first alert or common labels
		alertName := "unknown"
		if len(hookMessage.Alerts) > 0 {
			if name, exists := hookMessage.Alerts[0].Labels["alertname"]; exists {
				alertName = name
			}
		} else if name, exists := hookMessage.CommonLabels["alertname"]; exists {
			alertName = name
		}
		return nil, fmt.Errorf("no matching Operarius found for alert %s with status %s", alertName, hookMessage.Status)
	}

	// Return the highest priority Operarius
	bestMatch := &matchingOperarii[0]
	for i := 1; i < len(matchingOperarii); i++ {
		if matchingOperarii[i].Spec.Priority > bestMatch.Spec.Priority {
			bestMatch = &matchingOperarii[i]
		}
	}

	return bestMatch, nil
}

// matchesHookMessage checks if an Operarius matches the given hook message
func (s *OperariusService) matchesHookMessage(operarius operariusv1alpha1.Operarius, hookMessage models.HookMessage) bool {
	selector := operarius.Spec.AlertSelector

	// Check if enabled
	if operarius.Spec.Enabled != nil && !*operarius.Spec.Enabled {
		return false
	}

	// Check status
	if selector.Status != hookMessage.Status {
		return false
	}

	// Check alert name - it can be in individual alerts or common labels
	alertName := ""
	if len(hookMessage.Alerts) > 0 {
		if name, exists := hookMessage.Alerts[0].Labels["alertname"]; exists {
			alertName = name
		}
	}
	if alertName == "" {
		if name, exists := hookMessage.CommonLabels["alertname"]; exists {
			alertName = name
		}
	}

	if selector.AlertName != alertName {
		return false
	}

	// Check additional labels against common labels and first alert labels
	labelsToCheck := make(map[string]string)
	// Start with common labels
	for k, v := range hookMessage.CommonLabels {
		labelsToCheck[k] = v
	}
	// Override with first alert's labels if available
	if len(hookMessage.Alerts) > 0 {
		for k, v := range hookMessage.Alerts[0].Labels {
			labelsToCheck[k] = v
		}
	}

	for key, value := range selector.Labels {
		alertValue, exists := labelsToCheck[key]
		if !exists || alertValue != value {
			return false
		}
	}

	return true
}

// CreateJobFromOperarius creates a Kubernetes Job from an Operarius CRD
func (s *OperariusService) CreateJobFromOperarius(ctx context.Context, operarius *operariusv1alpha1.Operarius, hookMessage models.HookMessage) (*batchv1.Job, error) {
	// Deep copy the job template to avoid modifying the original
	jobTemplate := operarius.Spec.JobTemplate.DeepCopy()

	// Get alert name and group key from hook message
	alertName := "unknown"
	if len(hookMessage.Alerts) > 0 {
		if name, exists := hookMessage.Alerts[0].Labels["alertname"]; exists {
			alertName = name
		}
	} else if name, exists := hookMessage.CommonLabels["alertname"]; exists {
		alertName = name
	}

	// Create the job with proper metadata
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", operarius.Name),
			Namespace:    operarius.Namespace,
			Labels:       make(map[string]string),
			Annotations:  make(map[string]string),
		},
		Spec: jobTemplate.Spec,
	}

	// Copy labels and annotations from template ObjectMeta
	for k, v := range jobTemplate.Labels {
		job.Labels[k] = v
	}
	for k, v := range jobTemplate.Annotations {
		job.Annotations[k] = v
	}

	// Add OpenFero-specific labels
	job.Labels["openfero.io/operarius"] = operarius.Name
	job.Labels["openfero.io/alert"] = alertName
	job.Labels["openfero.io/group-key"] = hookMessage.GroupKey
	job.Labels["openfero.io/managed-by"] = "openfero"
	job.Labels["openfero.io/status"] = hookMessage.Status

	// Apply template variables to the job
	if err := s.applyTemplateVariables(job, hookMessage); err != nil {
		return nil, fmt.Errorf("failed to apply template variables: %w", err)
	}

	// Create the job in Kubernetes
	createdJob, err := s.kubeClient.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return createdJob, nil
}

// applyTemplateVariables applies Go template variables to the job
func (s *OperariusService) applyTemplateVariables(job *batchv1.Job, hookMessage models.HookMessage) error {
	// Template data structure - provide both individual alert and hook message data
	templateData := struct {
		Alert       models.Alert
		HookMessage models.HookMessage
		// For backward compatibility, expose common fields at top level
		Labels      map[string]string
		Annotations map[string]string
		GroupKey    string
		Status      string
	}{
		HookMessage: hookMessage,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		GroupKey:    hookMessage.GroupKey,
		Status:      hookMessage.Status,
	}

	// Use first alert if available, otherwise create a synthetic one from common data
	if len(hookMessage.Alerts) > 0 {
		templateData.Alert = hookMessage.Alerts[0]
		templateData.Labels = hookMessage.Alerts[0].Labels
		templateData.Annotations = hookMessage.Alerts[0].Annotations
	} else {
		// Create synthetic alert from common data
		templateData.Alert = models.Alert{
			Labels:      hookMessage.CommonLabels,
			Annotations: hookMessage.CommonAnnotations,
		}
		templateData.Labels = hookMessage.CommonLabels
		templateData.Annotations = hookMessage.CommonAnnotations
	}

	// Apply templates to container environment variables
	for i := range job.Spec.Template.Spec.Containers {
		container := &job.Spec.Template.Spec.Containers[i]

		// Process environment variables
		for j := range container.Env {
			envVar := &container.Env[j]
			if envVar.Value != "" {
				processedValue, err := s.processTemplate(envVar.Value, templateData)
				if err != nil {
					return fmt.Errorf("failed to process template for env var %s: %w", envVar.Name, err)
				}
				envVar.Value = processedValue
			}
		}

		// Process command
		for j := range container.Command {
			processedCmd, err := s.processTemplate(container.Command[j], templateData)
			if err != nil {
				return fmt.Errorf("failed to process template for command: %w", err)
			}
			container.Command[j] = processedCmd
		}

		// Process args
		for j := range container.Args {
			processedArg, err := s.processTemplate(container.Args[j], templateData)
			if err != nil {
				return fmt.Errorf("failed to process template for arg: %w", err)
			}
			container.Args[j] = processedArg
		}
	}

	return nil
}

// processTemplate processes a Go template string with the given data
func (s *OperariusService) processTemplate(templateStr string, data interface{}) (string, error) {
	// Skip processing if no template variables found
	if !strings.Contains(templateStr, "{{") {
		return templateStr, nil
	}

	tmpl, err := template.New("operarius").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// CheckDeduplication checks if a job should be created based on deduplication settings
func (s *OperariusService) CheckDeduplication(ctx context.Context, operarius *operariusv1alpha1.Operarius, hookMessage models.HookMessage) (bool, error) {
	if operarius.Spec.Deduplication == nil || !operarius.Spec.Deduplication.Enabled {
		return true, nil // No deduplication, always create
	}

	// Create label selector for finding existing jobs
	labelSelector := labels.Set{
		"openfero.io/operarius": operarius.Name,
		"openfero.io/group-key": hookMessage.GroupKey,
	}.AsSelector()

	// Look for recent jobs
	jobs, err := s.kubeClient.BatchV1().Jobs(operarius.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return false, fmt.Errorf("failed to list jobs: %w", err)
	}

	// Check if any job was created within the TTL window
	ttl := operarius.Spec.Deduplication.TTL
	if ttl <= 0 {
		ttl = 300 // Default 5 minutes
	}

	for _, job := range jobs.Items {
		// Check if job is recent enough to trigger deduplication
		if job.CreationTimestamp.Time.Add(time.Duration(ttl) * time.Second).After(time.Now()) {
			return false, nil // Don't create, still within deduplication window
		}
	}

	return true, nil // OK to create
}

// GetOperariiForNamespace retrieves all Operarii in a namespace
func (s *OperariusService) GetOperariiForNamespace(ctx context.Context, namespace string) ([]operariusv1alpha1.Operarius, error) {
	// Check if we have an Operarius client configured
	if s.operariusClient == nil {
		log.Debug("No Operarius client configured, returning empty list")
		return []operariusv1alpha1.Operarius{}, nil
	}

	// Try to get from cache first
	operarii, err := s.operariusClient.List()
	if err != nil {
		log.Warn("Failed to list Operarii from cache, trying API",
			zap.Error(err))
		// Fallback to direct API call
		operarii, err = s.operariusClient.ListFromAPI(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list Operarii: %w", err)
		}
	}

	log.Debug("Retrieved Operarii",
		zap.String("namespace", namespace),
		zap.Int("count", len(operarii)))

	return operarii, nil
}

// UpdateOperariusStatus updates the status of an Operarius after job creation
func (s *OperariusService) UpdateOperariusStatus(ctx context.Context, operarius *operariusv1alpha1.Operarius, jobName string) error {
	if s.operariusClient == nil {
		log.Debug("No Operarius client configured, skipping status update")
		return nil
	}

	// Update status fields
	now := metav1.Now()
	operarius.Status.ExecutionCount++
	operarius.Status.LastExecutionTime = &now
	operarius.Status.LastExecutedJobName = jobName

	// Update condition to Ready
	readyCondition := operariusv1alpha1.OperariusCondition{
		Type:               operariusv1alpha1.OperariusConditionReady,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "JobCreated",
		Message:            fmt.Sprintf("Successfully created job %s", jobName),
	}
	operarius.Status.Conditions = updateConditions(operarius.Status.Conditions, readyCondition)

	// Update via API
	if err := s.operariusClient.UpdateStatus(ctx, operarius); err != nil {
		return fmt.Errorf("failed to update Operarius status: %w", err)
	}

	log.Info("Updated Operarius status",
		zap.String("operarius", operarius.Name),
		zap.String("jobName", jobName),
		zap.Int32("executionCount", operarius.Status.ExecutionCount))

	return nil
}

// updateConditions updates or adds a condition to the conditions slice
func updateConditions(conditions []operariusv1alpha1.OperariusCondition, newCondition operariusv1alpha1.OperariusCondition) []operariusv1alpha1.OperariusCondition {
	for i, c := range conditions {
		if c.Type == newCondition.Type {
			conditions[i] = newCondition
			return conditions
		}
	}
	return append(conditions, newCondition)
}
