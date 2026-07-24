package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"text/template"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	operariusv1alpha1 "github.com/OpenFero/openfero/api/v1alpha1"
	k8sclient "github.com/OpenFero/openfero/pkg/kubernetes"
	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/models"
	"github.com/OpenFero/openfero/pkg/utils"
)

// ErrJobDeduplicated is returned by CreateJobFromOperarius when a Job for the
// same Operarius and alert group already exists within the current
// deduplication window. CheckDeduplication's list-based check is advisory and
// racy under concurrent requests; this error signals the atomic backstop
// (a deterministic Job name plus the API server's uniqueness guarantee) that
// actually prevents duplicate Jobs from being created.
var ErrJobDeduplicated = errors.New("job already exists for this deduplication window")

// OperariusClientInterface defines the interface for Operarius client operations
// This allows for mocking in tests
type OperariusClientInterface interface {
	List() ([]operariusv1alpha1.Operarius, error)
	ListFromAPI(ctx context.Context) ([]operariusv1alpha1.Operarius, error)
	Get(name string) (*operariusv1alpha1.Operarius, error)
	UpdateStatus(ctx context.Context, operarius *operariusv1alpha1.Operarius) error
	GetNamespace() string
}

// OperariusBroadcaster is a function that broadcasts Operarius updates
type OperariusBroadcaster func(operarius operariusv1alpha1.Operarius)

// OperariusService handles Operarius CRD-based job creation
type OperariusService struct {
	kubeClient      kubernetes.Interface
	operariusClient OperariusClientInterface
	broadcaster     OperariusBroadcaster
}

// NewOperariusService creates a new OperariusService
func NewOperariusService(kubeClient kubernetes.Interface) *OperariusService {
	return &OperariusService{
		kubeClient: kubeClient,
	}
}

// NewOperariusServiceWithClient creates a new OperariusService with an Operarius client
func NewOperariusServiceWithClient(kubeClient kubernetes.Interface, operariusClient OperariusClientInterface) *OperariusService {
	return &OperariusService{
		kubeClient:      kubeClient,
		operariusClient: operariusClient,
	}
}

// NewOperariusServiceWithK8sClient creates a new OperariusService with a concrete k8s Operarius client
// This is a convenience function for production use
func NewOperariusServiceWithK8sClient(kubeClient kubernetes.Interface, operariusClient *k8sclient.OperariusClient) *OperariusService {
	return &OperariusService{
		kubeClient:      kubeClient,
		operariusClient: operariusClient,
	}
}

// SetBroadcaster sets the broadcaster function
func (s *OperariusService) SetBroadcaster(broadcaster OperariusBroadcaster) {
	s.broadcaster = broadcaster
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
	maps.Copy(labelsToCheck, hookMessage.CommonLabels)
	// Override with first alert's labels if available
	if len(hookMessage.Alerts) > 0 {
		maps.Copy(labelsToCheck, hookMessage.Alerts[0].Labels)
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
			Namespace:   operarius.Namespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: jobTemplate.Spec,
	}

	// When time-based deduplication is enabled, use a deterministic name
	// instead of GenerateName. Two requests racing past CheckDeduplication's
	// list-based check will compute the same name; the API server allows only
	// one of the resulting Create calls to succeed, so the race is closed
	// atomically instead of relying on the advisory pre-check alone.
	if dedup := operarius.Spec.Deduplication; dedup != nil && dedup.Enabled && dedup.TTL > 0 {
		job.Name = dedupJobName(operarius.Name, hookMessage.GroupKey, dedup.TTL)
	} else {
		job.GenerateName = fmt.Sprintf("%s-", operarius.Name)
	}

	// Copy labels and annotations from template ObjectMeta
	maps.Copy(job.Labels, jobTemplate.Labels)
	maps.Copy(job.Annotations, jobTemplate.Annotations)

	// Add OpenFero-specific labels
	job.Labels["openfero.io/operarius"] = operarius.Name
	job.Labels["openfero.io/alert"] = alertName
	job.Labels["openfero.io/group-key"] = utils.HashGroupKey(hookMessage.GroupKey)
	job.Labels["openfero.io/managed-by"] = "openfero"
	job.Labels["openfero.io/status"] = hookMessage.Status

	// Add alert labels as environment variables (OPENFERO_* prefix)
	// Use first alert if available, otherwise use common labels
	var alertLabels map[string]string
	if len(hookMessage.Alerts) > 0 {
		alertLabels = hookMessage.Alerts[0].Labels
	} else {
		alertLabels = hookMessage.CommonLabels
	}
	for i := range job.Spec.Template.Spec.Containers {
		for labelKey, labelValue := range alertLabels {
			envName := "OPENFERO_" + strings.ToUpper(strings.ReplaceAll(labelKey, "-", "_"))
			job.Spec.Template.Spec.Containers[i].Env = append(job.Spec.Template.Spec.Containers[i].Env, corev1.EnvVar{
				Name:  envName,
				Value: labelValue,
			})
		}
	}

	// Apply template variables to the job
	if err := s.applyTemplateVariables(job, hookMessage); err != nil {
		return nil, fmt.Errorf("failed to apply template variables: %w", err)
	}

	// Create the job in Kubernetes
	createdJob, err := s.kubeClient.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		if job.Name != "" && k8serrors.IsAlreadyExists(err) {
			return nil, ErrJobDeduplicated
		}
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return createdJob, nil
}

// dedupJobName derives a deterministic Job name for a given Operarius and
// alert group within the current deduplication window ("bucket" = TTL-sized,
// epoch-aligned interval). Aligning the window to fixed epoch boundaries
// (rather than a sliding window anchored to the last Job's creation time)
// trades a small amount of precision at window boundaries for atomicity: the
// name can be computed independently by concurrent callers and Kubernetes'
// name-uniqueness check becomes the actual deduplication guard.
func dedupJobName(operariusName, groupKey string, ttlSeconds int32) string {
	window := time.Now().Unix() / int64(ttlSeconds)
	name := strings.ToLower(fmt.Sprintf("%s-%s-%d", operariusName, utils.HashGroupKey(groupKey), window))
	if len(name) > 63 {
		name = name[:63]
	}
	return strings.TrimRight(name, "-")
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
func (s *OperariusService) processTemplate(templateStr string, data any) (string, error) {
	// Skip processing if no template variables found
	if !strings.Contains(templateStr, "{{") {
		return templateStr, nil
	}

	tmpl, err := template.New("operarius").
		Option("missingkey=error").
		Funcs(template.FuncMap{}).
		Parse(templateStr)
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
		"openfero.io/group-key": utils.HashGroupKey(hookMessage.GroupKey),
	}.AsSelector()

	// Look for existing jobs
	jobs, err := s.kubeClient.BatchV1().Jobs(operarius.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return false, fmt.Errorf("failed to list jobs: %w", err)
	}

	// Time-based check: block if a job was created within the TTL window.
	// TTL <= 0 means time-based deduplication is disabled.
	if operarius.Spec.Deduplication.TTL > 0 {
		for _, job := range jobs.Items {
			if job.CreationTimestamp.Time.Add(time.Duration(operarius.Spec.Deduplication.TTL) * time.Second).After(time.Now()) {
				return false, nil // Don't create, still within deduplication window
			}
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

	// Prefer the informer cache: it is kept current via a Kubernetes watch
	// (updates typically land well under a second) and reading from it avoids
	// hitting the API server on every single incoming alert. Falling back to
	// a live API call on every request would defeat the purpose of running
	// the informer and add unnecessary API server load and latency, most
	// painfully during exactly the alert bursts this service exists to react to.
	operarii, err := s.operariusClient.List()
	if err != nil {
		// Fall back to a direct API read if the cache isn't available yet,
		// e.g. the informer hasn't finished its initial sync.
		log.Warn("Failed to list Operarii from cache, falling back to API",
			"error", err)
		operarii, err = s.operariusClient.ListFromAPI(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list Operarii: %w", err)
		}
	}

	log.Debug("Retrieved Operarii",
		"namespace", namespace,
		"count", len(operarii))

	return operarii, nil
}

// UpdateOperariusDedupStatus updates only the LastExecutionStatus field of an Operarius
// when a job creation is skipped due to deduplication. It does not increment ExecutionCount
// or change LastExecutionTime / LastExecutedJobName.
func (s *OperariusService) UpdateOperariusDedupStatus(ctx context.Context, operarius *operariusv1alpha1.Operarius) error {
	if s.operariusClient == nil {
		log.Debug("No Operarius client configured, skipping dedup status update")
		return nil
	}

	operarius.Status.LastExecutionStatus = "Skipped: Deduplication"

	if err := s.operariusClient.UpdateStatus(ctx, operarius); err != nil {
		return fmt.Errorf("failed to update Operarius dedup status: %w", err)
	}

	log.Debug("Updated Operarius dedup status",
		"operarius", operarius.Name)

	return nil
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
	operarius.Status.LastExecutionStatus = "Pending"

	// Update via API
	if err := s.operariusClient.UpdateStatus(ctx, operarius); err != nil {
		return fmt.Errorf("failed to update Operarius status: %w", err)
	}

	log.Info("Updated Operarius status",
		"operarius", operarius.Name,
		"jobName", jobName,
		"executionCount", operarius.Status.ExecutionCount)

	if s.broadcaster != nil {
		s.broadcaster(*operarius)
	}

	return nil
}

// UpdateOperariusStatusFromJob updates the Operarius status based on the job status
func (s *OperariusService) UpdateOperariusStatusFromJob(ctx context.Context, operarius *operariusv1alpha1.Operarius, job *batchv1.Job) error {
	if s.operariusClient == nil {
		return nil
	}

	// Determine status
	var newStatus string
	if job.Status.Succeeded > 0 {
		newStatus = "Successful"
	} else if job.Status.Failed > 0 {
		newStatus = "Failed"
	} else if job.Status.Active > 0 {
		newStatus = "Running"
	} else {
		newStatus = "Pending"
	}

	// If status is Running, we broadcast but don't persist to avoid churn
	if newStatus == "Running" || newStatus == "Pending" {
		if s.broadcaster != nil {
			// Create a copy to avoid modifying the original if we were to persist later (though we don't here)
			opCopy := operarius.DeepCopy()
			opCopy.Status.LastExecutionStatus = newStatus
			s.broadcaster(*opCopy)
		}
		return nil
	}

	// For terminal states, we persist
	// Skip if status hasn't changed
	if operarius.Status.LastExecutionStatus == newStatus {
		return nil
	}

	operarius.Status.LastExecutionStatus = newStatus

	// Update via API
	if err := s.operariusClient.UpdateStatus(ctx, operarius); err != nil {
		return fmt.Errorf("failed to update Operarius status: %w", err)
	}

	log.Info("Updated Operarius status from job",
		"operarius", operarius.Name,
		"job", job.Name,
		"status", newStatus)

	if s.broadcaster != nil {
		s.broadcaster(*operarius)
	}

	return nil
}

// GetOperarius retrieves a specific Operarius by name
func (s *OperariusService) GetOperarius(ctx context.Context, name, namespace string) (*operariusv1alpha1.Operarius, error) {
	if s.operariusClient == nil {
		return nil, fmt.Errorf("no Operarius client configured")
	}

	// Ensure namespace matches
	if namespace != s.operariusClient.GetNamespace() {
		return nil, fmt.Errorf("namespace mismatch: expected %s, got %s", s.operariusClient.GetNamespace(), namespace)
	}

	return s.operariusClient.Get(name)
}

// ToJobInfo converts an Operarius to a JobInfo model
func (s *OperariusService) ToJobInfo(op operariusv1alpha1.Operarius) models.JobInfo {
	image := "unknown"
	if len(op.Spec.JobTemplate.Spec.Template.Spec.Containers) > 0 {
		image = op.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image
	}

	var lastExecutionTime *time.Time
	if op.Status.LastExecutionTime != nil {
		t := op.Status.LastExecutionTime.Time
		lastExecutionTime = &t
	}

	return models.JobInfo{
		OperariusName:       op.Name,
		JobName:             op.Spec.AlertSelector.AlertName,
		Namespace:           op.Namespace,
		Image:               image,
		ExecutionCount:      op.Status.ExecutionCount,
		LastExecutionTime:   lastExecutionTime,
		LastExecutedJobName: op.Status.LastExecutedJobName,
		LastExecutionStatus: op.Status.LastExecutionStatus,
	}
}
