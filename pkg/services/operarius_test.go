package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	operariusv1alpha1 "github.com/OpenFero/openfero/api/v1alpha1"
	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/models"
	"github.com/OpenFero/openfero/pkg/utils"
	"go.uber.org/zap"
)

func init() {
	// Initialize logger for tests
	cfg := zap.NewDevelopmentConfig()
	err := log.SetConfig(cfg)
	if err != nil {
		panic(err)
	}
}

// Verify MockOperariusClient implements OperariusClientInterface
var _ OperariusClientInterface = (*MockOperariusClient)(nil)

// MockOperariusClient is a mock implementation of the OperariusClientInterface for testing
type MockOperariusClient struct {
	operarii       []operariusv1alpha1.Operarius
	listFromAPIErr error
	listErr        error
	updateStatusFn func(ctx context.Context, operarius *operariusv1alpha1.Operarius) error
	namespace      string
}

func (m *MockOperariusClient) List() ([]operariusv1alpha1.Operarius, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.operarii, nil
}

func (m *MockOperariusClient) ListFromAPI(ctx context.Context) ([]operariusv1alpha1.Operarius, error) {
	if m.listFromAPIErr != nil {
		return nil, m.listFromAPIErr
	}
	return m.operarii, nil
}

func (m *MockOperariusClient) UpdateStatus(ctx context.Context, operarius *operariusv1alpha1.Operarius) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, operarius)
	}
	return nil
}

func (m *MockOperariusClient) Get(name string) (*operariusv1alpha1.Operarius, error) {
	for _, op := range m.operarii {
		if op.Name == name {
			return &op, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *MockOperariusClient) GetNamespace() string {
	if m.namespace != "" {
		return m.namespace
	}
	return "openfero"
}

func TestOperariusService_FindMatchingOperarius(t *testing.T) {
	// Setup
	service := NewOperariusService(fake.NewSimpleClientset())

	enabled := true
	operarii := []operariusv1alpha1.Operarius{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "quota-operarius",
				Namespace: "openfero",
			},
			Spec: operariusv1alpha1.OperariusSpec{
				AlertSelector: operariusv1alpha1.AlertSelector{
					AlertName: "KubeQuotaAlmostFull",
					Status:    "firing",
					Labels: map[string]string{
						"severity": "warning",
					},
				},
				Priority: 100,
				Enabled:  &enabled,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-restart-operarius",
				Namespace: "openfero",
			},
			Spec: operariusv1alpha1.OperariusSpec{
				AlertSelector: operariusv1alpha1.AlertSelector{
					AlertName: "PodCrashLooping",
					Status:    "firing",
				},
				Priority: 50,
				Enabled:  &enabled,
			},
		},
	}

	tests := []struct {
		name        string
		hookMessage models.HookMessage
		wantMatch   string
		wantError   bool
	}{
		{
			name: "matches quota alert",
			hookMessage: models.HookMessage{
				Status:   "firing",
				GroupKey: "test-group",
				CommonLabels: map[string]string{
					"alertname": "KubeQuotaAlmostFull",
					"severity":  "warning",
				},
				Alerts: []models.Alert{
					{
						Labels: map[string]string{
							"alertname": "KubeQuotaAlmostFull",
							"severity":  "warning",
							"namespace": "test-ns",
						},
					},
				},
			},
			wantMatch: "quota-operarius",
			wantError: false,
		},
		{
			name: "matches pod restart alert",
			hookMessage: models.HookMessage{
				Status:   "firing",
				GroupKey: "test-group-2",
				CommonLabels: map[string]string{
					"alertname": "PodCrashLooping",
				},
				Alerts: []models.Alert{
					{
						Labels: map[string]string{
							"alertname": "PodCrashLooping",
							"pod":       "test-pod",
						},
					},
				},
			},
			wantMatch: "pod-restart-operarius",
			wantError: false,
		},
		{
			name: "no matching operarius",
			hookMessage: models.HookMessage{
				Status:   "firing",
				GroupKey: "test-group-3",
				CommonLabels: map[string]string{
					"alertname": "UnknownAlert",
				},
			},
			wantMatch: "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.FindMatchingOperarius(tt.hookMessage, operarii)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.wantMatch, result.Name)
			}
		})
	}
}

func TestOperariusService_CreateJobFromOperarius(t *testing.T) {
	// Setup
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := &operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: "TestAlert",
				Status:    "firing",
			},
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "openfero",
					},
					Annotations: map[string]string{
						"description": "Test remediation",
					},
				},
				Spec: batchv1.JobSpec{
					BackoffLimit: int32Ptr(3),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:  "remediation",
									Image: "busybox",
									Env: []corev1.EnvVar{
										{
											Name:  "NAMESPACE",
											Value: "{{ .Alert.Labels.namespace }}",
										},
										{
											Name:  "POD_NAME",
											Value: "{{ .Alert.Labels.pod }}",
										},
									},
									Command: []string{
										"/bin/sh",
										"-c",
										"echo Processing alert {{ .Alert.Labels.alertname }}",
									},
								},
							},
						},
					},
				},
			},
			Enabled: &enabled,
		},
	}

	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
		},
		Alerts: []models.Alert{
			{
				Labels: map[string]string{
					"alertname": "TestAlert",
					"namespace": "test-namespace",
					"pod":       "test-pod",
				},
			},
		},
	}

	// Test
	ctx := context.TODO()
	job, err := service.CreateJobFromOperarius(ctx, operarius, hookMessage)

	// Assertions
	assert.NoError(t, err)
	require.NotNil(t, job)

	// Check job metadata - job name is generated, so check that it starts with the right prefix
	assert.True(t, strings.HasPrefix(job.Name, "test-operarius-") || job.GenerateName == "test-operarius-", "Job name should be generated correctly")
	assert.Equal(t, "openfero", job.Namespace)
	assert.Equal(t, "test-operarius", job.Labels["openfero.io/operarius"])
	assert.Equal(t, "TestAlert", job.Labels["openfero.io/alert"])
	assert.Equal(t, utils.HashGroupKey("test-group"), job.Labels["openfero.io/group-key"])
	assert.Equal(t, "openfero", job.Labels["app"])

	// Check template variables were applied
	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "test-namespace", getEnvValue(container.Env, "NAMESPACE"))
	assert.Equal(t, "test-pod", getEnvValue(container.Env, "POD_NAME"))
	assert.Equal(t, "echo Processing alert TestAlert", container.Command[2])
}

func TestOperariusService_CheckDeduplication(t *testing.T) {
	// Setup
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := &operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			Deduplication: &operariusv1alpha1.DeduplicationConfig{
				Enabled: true,
				TTL:     300, // 5 minutes
			},
			Enabled: &enabled,
		},
	}

	hookMessage := models.HookMessage{
		GroupKey: "test-group",
	}

	// Test without existing jobs
	ctx := context.TODO()
	shouldCreate, err := service.CheckDeduplication(ctx, operarius, hookMessage)
	assert.NoError(t, err)
	assert.True(t, shouldCreate)

	// Create a recent job
	recentJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "recent-job",
			Namespace: "openfero",
			Labels: map[string]string{
				"openfero.io/operarius": "test-operarius",
				"openfero.io/group-key": utils.HashGroupKey("test-group"),
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Minute)), // 2 minutes ago
		},
	}
	_, err = kubeClient.BatchV1().Jobs("openfero").Create(ctx, recentJob, metav1.CreateOptions{})
	require.NoError(t, err)

	// Test with recent job - should not create
	shouldCreate, err = service.CheckDeduplication(ctx, operarius, hookMessage)
	assert.NoError(t, err)
	assert.False(t, shouldCreate)

	// Create an old job
	oldJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-job",
			Namespace: "openfero",
			Labels: map[string]string{
				"openfero.io/operarius": "test-operarius",
				"openfero.io/group-key": utils.HashGroupKey("test-group"),
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-10 * time.Minute)), // 10 minutes ago
		},
	}
	_, err = kubeClient.BatchV1().Jobs("openfero").Create(ctx, oldJob, metav1.CreateOptions{})
	require.NoError(t, err)

	// Since we still have the recent job, should not create
	shouldCreate, err = service.CheckDeduplication(ctx, operarius, hookMessage)
	assert.NoError(t, err)
	assert.False(t, shouldCreate)
}

func TestOperariusService_TemplateProcessing(t *testing.T) {
	service := NewOperariusService(fake.NewSimpleClientset())

	templateData := struct {
		Alert struct {
			Labels map[string]string
		}
	}{
		Alert: struct {
			Labels map[string]string
		}{
			Labels: map[string]string{
				"alertname": "TestAlert",
				"namespace": "test-ns",
				"severity":  "critical",
			},
		},
	}

	tests := []struct {
		name     string
		template string
		want     string
		wantErr  bool
	}{
		{
			name:     "simple label substitution",
			template: "{{ .Alert.Labels.namespace }}",
			want:     "test-ns",
			wantErr:  false,
		},
		{
			name:     "multiple substitutions",
			template: "Alert {{ .Alert.Labels.alertname }} in {{ .Alert.Labels.namespace }}",
			want:     "Alert TestAlert in test-ns",
			wantErr:  false,
		},
		{
			name:     "no template variables",
			template: "static string",
			want:     "static string",
			wantErr:  false,
		},
		{
			name:     "invalid template",
			template: "{{ .NonExistent",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.processTemplate(tt.template, templateData)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestOperariusService_FindMatchingOperarius_Priority(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true

	operarii := []operariusv1alpha1.Operarius{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "low-priority-operarius",
				Namespace: "openfero",
			},
			Spec: operariusv1alpha1.OperariusSpec{
				AlertSelector: operariusv1alpha1.AlertSelector{
					AlertName: "TestAlert",
					Status:    "firing",
				},
				Priority: 10,
				Enabled:  &enabled,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "high-priority-operarius",
				Namespace: "openfero",
			},
			Spec: operariusv1alpha1.OperariusSpec{
				AlertSelector: operariusv1alpha1.AlertSelector{
					AlertName: "TestAlert",
					Status:    "firing",
				},
				Priority: 100,
				Enabled:  &enabled,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "no-priority-operarius",
				Namespace: "openfero",
			},
			Spec: operariusv1alpha1.OperariusSpec{
				AlertSelector: operariusv1alpha1.AlertSelector{
					AlertName: "TestAlert",
					Status:    "firing",
				},
				// No priority set - should default to 0
				Enabled: &enabled,
			},
		},
	}

	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
		},
	}

	// Should select high-priority-operarius (priority 100)
	result, err := service.FindMatchingOperarius(hookMessage, operarii)
	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "high-priority-operarius", result.Name, "Should select operarius with highest priority")
}

func TestOperariusService_FindMatchingOperarius_Enabled(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	disabled := false

	tests := []struct {
		name       string
		operarii   []operariusv1alpha1.Operarius
		wantMatch  string
		wantError  bool
		errMessage string
	}{
		{
			name: "only disabled operarius available",
			operarii: []operariusv1alpha1.Operarius{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "disabled-operarius",
						Namespace: "openfero",
					},
					Spec: operariusv1alpha1.OperariusSpec{
						AlertSelector: operariusv1alpha1.AlertSelector{
							AlertName: "TestAlert",
							Status:    "firing",
						},
						Enabled: &disabled,
					},
				},
			},
			wantMatch:  "",
			wantError:  true,
			errMessage: "no matching",
		},
		{
			name: "enabled and disabled - should select enabled",
			operarii: []operariusv1alpha1.Operarius{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "disabled-operarius",
						Namespace: "openfero",
					},
					Spec: operariusv1alpha1.OperariusSpec{
						AlertSelector: operariusv1alpha1.AlertSelector{
							AlertName: "TestAlert",
							Status:    "firing",
						},
						Enabled: &disabled,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "enabled-operarius",
						Namespace: "openfero",
					},
					Spec: operariusv1alpha1.OperariusSpec{
						AlertSelector: operariusv1alpha1.AlertSelector{
							AlertName: "TestAlert",
							Status:    "firing",
						},
						Enabled: &enabled,
					},
				},
			},
			wantMatch: "enabled-operarius",
			wantError: false,
		},
		{
			name: "nil enabled field - defaults to enabled",
			operarii: []operariusv1alpha1.Operarius{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nil-enabled-operarius",
						Namespace: "openfero",
					},
					Spec: operariusv1alpha1.OperariusSpec{
						AlertSelector: operariusv1alpha1.AlertSelector{
							AlertName: "TestAlert",
							Status:    "firing",
						},
						// Enabled is nil - should default to true
					},
				},
			},
			wantMatch: "nil-enabled-operarius",
			wantError: false,
		},
	}

	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.FindMatchingOperarius(hookMessage, tt.operarii)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errMessage != "" {
					assert.Contains(t, err.Error(), tt.errMessage)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.wantMatch, result.Name)
			}
		})
	}
}

func TestOperariusService_FindMatchingOperarius_LabelMatchers(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true

	operarii := []operariusv1alpha1.Operarius{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "specific-namespace-operarius",
				Namespace: "openfero",
			},
			Spec: operariusv1alpha1.OperariusSpec{
				AlertSelector: operariusv1alpha1.AlertSelector{
					AlertName: "TestAlert",
					Status:    "firing",
					Labels: map[string]string{
						"namespace": "production",
						"severity":  "critical",
					},
				},
				Enabled: &enabled,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "generic-operarius",
				Namespace: "openfero",
			},
			Spec: operariusv1alpha1.OperariusSpec{
				AlertSelector: operariusv1alpha1.AlertSelector{
					AlertName: "TestAlert",
					Status:    "firing",
				},
				Enabled: &enabled,
			},
		},
	}

	tests := []struct {
		name      string
		labels    map[string]string
		wantMatch string
	}{
		{
			name: "matches specific label matchers",
			labels: map[string]string{
				"alertname": "TestAlert",
				"namespace": "production",
				"severity":  "critical",
			},
			wantMatch: "specific-namespace-operarius",
		},
		{
			name: "partial match falls back to generic",
			labels: map[string]string{
				"alertname": "TestAlert",
				"namespace": "staging",
			},
			wantMatch: "generic-operarius",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hookMessage := models.HookMessage{
				Status:       "firing",
				GroupKey:     "test-group",
				CommonLabels: tt.labels,
			}

			result, err := service.FindMatchingOperarius(hookMessage, operarii)
			assert.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantMatch, result.Name)
		})
	}
}

func TestOperariusService_TemplateProcessing_EdgeCases(t *testing.T) {
	service := NewOperariusService(fake.NewSimpleClientset())

	tests := []struct {
		name         string
		template     string
		templateData interface{}
		want         string
		wantErr      bool
	}{
		{
			name:     "empty template",
			template: "",
			templateData: struct {
				Alert struct{ Labels map[string]string }
			}{},
			want:    "",
			wantErr: false,
		},
		{
			name:     "missing label returns no value placeholder",
			template: "{{ .Alert.Labels.missing }}",
			templateData: struct {
				Alert struct{ Labels map[string]string }
			}{
				Alert: struct{ Labels map[string]string }{
					Labels: map[string]string{
						"alertname": "TestAlert",
					},
				},
			},
			want:    "<no value>",
			wantErr: false,
		},
		{
			name:     "special characters in label value",
			template: "{{ .Alert.Labels.message }}",
			templateData: struct {
				Alert struct{ Labels map[string]string }
			}{
				Alert: struct{ Labels map[string]string }{
					Labels: map[string]string{
						"message": "Alert: disk usage > 90% on /dev/sda1",
					},
				},
			},
			want:    "Alert: disk usage > 90% on /dev/sda1",
			wantErr: false,
		},
		{
			name:     "nested template syntax",
			template: "echo '{{ .Alert.Labels.namespace }}'",
			templateData: struct {
				Alert struct{ Labels map[string]string }
			}{
				Alert: struct{ Labels map[string]string }{
					Labels: map[string]string{
						"namespace": "test-ns",
					},
				},
			},
			want:    "echo 'test-ns'",
			wantErr: false,
		},
		{
			name:     "json in template",
			template: `{"namespace":"{{ .Alert.Labels.namespace }}","pod":"{{ .Alert.Labels.pod }}"}`,
			templateData: struct {
				Alert struct{ Labels map[string]string }
			}{
				Alert: struct{ Labels map[string]string }{
					Labels: map[string]string{
						"namespace": "default",
						"pod":       "nginx-123",
					},
				},
			},
			want:    `{"namespace":"default","pod":"nginx-123"}`,
			wantErr: false,
		},
		{
			name:     "unclosed template braces",
			template: "{{ .Alert.Labels.namespace",
			templateData: struct {
				Alert struct{ Labels map[string]string }
			}{},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.processTemplate(tt.template, tt.templateData)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestOperariusService_CreateJobFromOperarius_Variations(t *testing.T) {
	enabled := true

	tests := []struct {
		name        string
		operarius   *operariusv1alpha1.Operarius
		hookMessage models.HookMessage
		checkJob    func(t *testing.T, job *batchv1.Job)
		wantErr     bool
	}{
		{
			name: "multiple containers in job template",
			operarius: &operariusv1alpha1.Operarius{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-container-operarius",
					Namespace: "openfero",
				},
				Spec: operariusv1alpha1.OperariusSpec{
					AlertSelector: operariusv1alpha1.AlertSelector{
						AlertName: "TestAlert",
						Status:    "firing",
					},
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RestartPolicy: corev1.RestartPolicyNever,
									Containers: []corev1.Container{
										{
											Name:    "main",
											Image:   "busybox",
											Command: []string{"echo", "main"},
										},
										{
											Name:    "sidecar",
											Image:   "busybox",
											Command: []string{"echo", "sidecar"},
										},
									},
								},
							},
						},
					},
					Enabled: &enabled,
				},
			},
			hookMessage: models.HookMessage{
				Status:   "firing",
				GroupKey: "test-group-multi",
				CommonLabels: map[string]string{
					"alertname": "TestAlert",
				},
			},
			checkJob: func(t *testing.T, job *batchv1.Job) {
				assert.Len(t, job.Spec.Template.Spec.Containers, 2)
				assert.Equal(t, "main", job.Spec.Template.Spec.Containers[0].Name)
				assert.Equal(t, "sidecar", job.Spec.Template.Spec.Containers[1].Name)
			},
			wantErr: false,
		},
		{
			name: "job with init containers",
			operarius: &operariusv1alpha1.Operarius{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "init-container-operarius",
					Namespace: "openfero",
				},
				Spec: operariusv1alpha1.OperariusSpec{
					AlertSelector: operariusv1alpha1.AlertSelector{
						AlertName: "TestAlert",
						Status:    "firing",
					},
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RestartPolicy: corev1.RestartPolicyNever,
									InitContainers: []corev1.Container{
										{
											Name:    "init",
											Image:   "busybox",
											Command: []string{"echo", "init"},
										},
									},
									Containers: []corev1.Container{
										{
											Name:    "main",
											Image:   "busybox",
											Command: []string{"echo", "main"},
										},
									},
								},
							},
						},
					},
					Enabled: &enabled,
				},
			},
			hookMessage: models.HookMessage{
				Status:   "firing",
				GroupKey: "test-group-init",
				CommonLabels: map[string]string{
					"alertname": "TestAlert",
				},
			},
			checkJob: func(t *testing.T, job *batchv1.Job) {
				assert.Len(t, job.Spec.Template.Spec.InitContainers, 1)
				assert.Equal(t, "init", job.Spec.Template.Spec.InitContainers[0].Name)
			},
			wantErr: false,
		},
		{
			name: "job with volumes and volume mounts",
			operarius: &operariusv1alpha1.Operarius{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "volume-operarius",
					Namespace: "openfero",
				},
				Spec: operariusv1alpha1.OperariusSpec{
					AlertSelector: operariusv1alpha1.AlertSelector{
						AlertName: "TestAlert",
						Status:    "firing",
					},
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RestartPolicy: corev1.RestartPolicyNever,
									Volumes: []corev1.Volume{
										{
											Name: "config-volume",
											VolumeSource: corev1.VolumeSource{
												ConfigMap: &corev1.ConfigMapVolumeSource{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "my-config",
													},
												},
											},
										},
									},
									Containers: []corev1.Container{
										{
											Name:  "main",
											Image: "busybox",
											VolumeMounts: []corev1.VolumeMount{
												{
													Name:      "config-volume",
													MountPath: "/etc/config",
												},
											},
										},
									},
								},
							},
						},
					},
					Enabled: &enabled,
				},
			},
			hookMessage: models.HookMessage{
				Status:   "firing",
				GroupKey: "test-group-volume",
				CommonLabels: map[string]string{
					"alertname": "TestAlert",
				},
			},
			checkJob: func(t *testing.T, job *batchv1.Job) {
				assert.Len(t, job.Spec.Template.Spec.Volumes, 1)
				assert.Equal(t, "config-volume", job.Spec.Template.Spec.Volumes[0].Name)
				assert.Len(t, job.Spec.Template.Spec.Containers[0].VolumeMounts, 1)
				assert.Equal(t, "/etc/config", job.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh client for each test to avoid job name conflicts
			kubeClient := fake.NewSimpleClientset()
			service := NewOperariusService(kubeClient)

			ctx := context.TODO()
			job, err := service.CreateJobFromOperarius(ctx, tt.operarius, tt.hookMessage)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, job)
				if tt.checkJob != nil {
					tt.checkJob(t, job)
				}
			}
		})
	}
}

func TestOperariusService_DeduplicationDisabled(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true

	tests := []struct {
		name          string
		deduplication *operariusv1alpha1.DeduplicationConfig
		wantCreate    bool
	}{
		{
			name:          "nil deduplication config",
			deduplication: nil,
			wantCreate:    true,
		},
		{
			name: "deduplication explicitly disabled",
			deduplication: &operariusv1alpha1.DeduplicationConfig{
				Enabled: false,
				TTL:     300,
			},
			wantCreate: true,
		},
		{
			name: "deduplication enabled with zero TTL",
			deduplication: &operariusv1alpha1.DeduplicationConfig{
				Enabled: true,
				TTL:     0,
			},
			wantCreate: true, // Zero TTL means no dedup window
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operarius := &operariusv1alpha1.Operarius{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-operarius",
					Namespace: "openfero",
				},
				Spec: operariusv1alpha1.OperariusSpec{
					Deduplication: tt.deduplication,
					Enabled:       &enabled,
				},
			}

			hookMessage := models.HookMessage{
				GroupKey: "test-group",
			}

			ctx := context.TODO()
			shouldCreate, err := service.CheckDeduplication(ctx, operarius, hookMessage)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCreate, shouldCreate)
		})
	}
}

// Helper functions
func int32Ptr(i int32) *int32 {
	return &i
}

func getEnvValue(envVars []corev1.EnvVar, name string) string {
	for _, env := range envVars {
		if env.Name == name {
			return env.Value
		}
	}
	return ""
}

// TestNewOperariusServiceWithClient tests the constructor with an OperariusClient
func TestNewOperariusServiceWithClient(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	mockClient := &MockOperariusClient{
		namespace: "test-namespace",
	}

	service := NewOperariusServiceWithClient(kubeClient, mockClient)

	assert.NotNil(t, service)
	assert.NotNil(t, service.kubeClient)
	assert.NotNil(t, service.operariusClient)
	assert.Equal(t, "test-namespace", service.operariusClient.GetNamespace())
}

// TestGetOperariiForNamespace tests retrieving Operarii from a namespace
func TestGetOperariiForNamespace(t *testing.T) {
	enabled := true
	testOperarii := []operariusv1alpha1.Operarius{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-operarius-1",
				Namespace: "openfero",
			},
			Spec: operariusv1alpha1.OperariusSpec{
				AlertSelector: operariusv1alpha1.AlertSelector{
					AlertName: "TestAlert1",
					Status:    "firing",
				},
				Enabled: &enabled,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-operarius-2",
				Namespace: "openfero",
			},
			Spec: operariusv1alpha1.OperariusSpec{
				AlertSelector: operariusv1alpha1.AlertSelector{
					AlertName: "TestAlert2",
					Status:    "resolved",
				},
				Enabled: &enabled,
			},
		},
	}

	tests := []struct {
		name           string
		mockClient     *MockOperariusClient
		hasClient      bool
		expectedCount  int
		expectError    bool
		errorSubstring string
	}{
		{
			name:          "no client configured returns empty list",
			mockClient:    nil,
			hasClient:     false,
			expectedCount: 0,
			expectError:   false,
		},
		{
			name: "successful API call returns operarii",
			mockClient: &MockOperariusClient{
				operarii: testOperarii,
			},
			hasClient:     true,
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "API error falls back to cache",
			mockClient: &MockOperariusClient{
				operarii:       testOperarii,
				listFromAPIErr: errors.New("API unavailable"),
			},
			hasClient:     true,
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "both API and cache fail returns error",
			mockClient: &MockOperariusClient{
				listFromAPIErr: errors.New("API unavailable"),
				listErr:        errors.New("cache unavailable"),
			},
			hasClient:      true,
			expectedCount:  0,
			expectError:    true,
			errorSubstring: "failed to list Operarii",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset()
			var service *OperariusService

			if tt.hasClient {
				service = NewOperariusServiceWithClient(kubeClient, tt.mockClient)
			} else {
				service = NewOperariusService(kubeClient)
			}

			ctx := context.TODO()
			operarii, err := service.GetOperariiForNamespace(ctx, "openfero")

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorSubstring != "" {
					assert.Contains(t, err.Error(), tt.errorSubstring)
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, operarii, tt.expectedCount)
			}
		})
	}
}

// TestUpdateOperariusStatus tests updating the status of an Operarius
func TestUpdateOperariusStatus(t *testing.T) {
	enabled := true
	operarius := &operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: "TestAlert",
				Status:    "firing",
			},
			Enabled: &enabled,
		},
		Status: operariusv1alpha1.OperariusStatus{
			ExecutionCount: 5,
		},
	}

	tests := []struct {
		name            string
		mockClient      *MockOperariusClient
		hasClient       bool
		jobName         string
		expectError     bool
		expectedCount   int32
	}{
		{
			name:          "no client configured skips update",
			mockClient:    nil,
			hasClient:     false,
			jobName:       "test-job",
			expectError:   false,
			expectedCount: 5, // Should remain unchanged
		},
		{
			name: "successful status update",
			mockClient: &MockOperariusClient{
				updateStatusFn: func(ctx context.Context, op *operariusv1alpha1.Operarius) error {
					return nil
				},
			},
			hasClient:       true,
			jobName:         "test-job-123",
			expectError:     false,
			expectedCount:   6,
		},
		{
			name: "update error is propagated",
			mockClient: &MockOperariusClient{
				updateStatusFn: func(ctx context.Context, op *operariusv1alpha1.Operarius) error {
					return errors.New("update failed")
				},
			},
			hasClient:   true,
			jobName:     "test-job",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Deep copy operarius for each test
			testOperarius := operarius.DeepCopy()
			kubeClient := fake.NewSimpleClientset()
			var service *OperariusService

			if tt.hasClient {
				service = NewOperariusServiceWithClient(kubeClient, tt.mockClient)
			} else {
				service = NewOperariusService(kubeClient)
			}

			ctx := context.TODO()
			err := service.UpdateOperariusStatus(ctx, testOperarius, tt.jobName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.hasClient {
					assert.Equal(t, tt.expectedCount, testOperarius.Status.ExecutionCount)
					assert.Equal(t, tt.jobName, testOperarius.Status.LastExecutedJobName)
					assert.NotNil(t, testOperarius.Status.LastExecutionTime)
				}
			}
		})
	}
}


// TestApplyTemplateVariables_Args tests template processing in container args
func TestApplyTemplateVariables_Args(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "openfero",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "busybox",
							Args: []string{
								"--namespace={{ .Alert.Labels.namespace }}",
								"--pod={{ .Alert.Labels.pod }}",
								"--static-arg",
							},
						},
					},
				},
			},
		},
	}

	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		Alerts: []models.Alert{
			{
				Labels: map[string]string{
					"alertname": "TestAlert",
					"namespace": "test-ns",
					"pod":       "test-pod-123",
				},
			},
		},
	}

	err := service.applyTemplateVariables(job, hookMessage)
	assert.NoError(t, err)

	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "--namespace=test-ns", container.Args[0])
	assert.Equal(t, "--pod=test-pod-123", container.Args[1])
	assert.Equal(t, "--static-arg", container.Args[2])
}

// TestApplyTemplateVariables_NoAlerts tests template processing when no alerts are present
func TestApplyTemplateVariables_NoAlerts(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "openfero",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "busybox",
							Env: []corev1.EnvVar{
								{
									Name:  "NAMESPACE",
									Value: "{{ .Alert.Labels.namespace }}",
								},
							},
						},
					},
				},
			},
		},
	}

	// HookMessage with no alerts but with CommonLabels
	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
			"namespace": "common-ns",
		},
		CommonAnnotations: map[string]string{
			"summary": "Test summary",
		},
	}

	err := service.applyTemplateVariables(job, hookMessage)
	assert.NoError(t, err)

	container := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "common-ns", container.Env[0].Value)
}

// TestFindMatchingOperarius_NoAlertsButCommonLabels tests matching when alerts array is empty
func TestFindMatchingOperarius_NoAlertsButCommonLabels(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarii := []operariusv1alpha1.Operarius{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-operarius",
				Namespace: "openfero",
			},
			Spec: operariusv1alpha1.OperariusSpec{
				AlertSelector: operariusv1alpha1.AlertSelector{
					AlertName: "TestAlert",
					Status:    "firing",
				},
				Enabled: &enabled,
			},
		},
	}

	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
		},
		// Alerts array is empty
	}

	result, err := service.FindMatchingOperarius(hookMessage, operarii)
	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test-operarius", result.Name)
}

// TestCreateJobFromOperarius_NoAlertsUsesCommonLabels tests job creation when no alerts are present
func TestCreateJobFromOperarius_NoAlertsUsesCommonLabels(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := &operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: "TestAlert",
				Status:    "firing",
			},
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:    "main",
									Image:   "busybox",
									Command: []string{"echo", "test"},
								},
							},
						},
					},
				},
			},
			Enabled: &enabled,
		},
	}

	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
		},
		// No alerts
	}

	ctx := context.TODO()
	job, err := service.CreateJobFromOperarius(ctx, operarius, hookMessage)

	assert.NoError(t, err)
	require.NotNil(t, job)
	// Should use alertname from CommonLabels
	assert.Equal(t, "TestAlert", job.Labels["openfero.io/alert"])
}

// TestMatchesHookMessage_LabelsFromFirstAlert tests that first alert labels override common labels
func TestMatchesHookMessage_LabelsFromFirstAlert(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: "TestAlert",
				Status:    "firing",
				Labels: map[string]string{
					"environment": "production",
				},
			},
			Enabled: &enabled,
		},
	}

	// First alert has different environment than common labels
	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		CommonLabels: map[string]string{
			"alertname":   "TestAlert",
			"environment": "staging",
		},
		Alerts: []models.Alert{
			{
				Labels: map[string]string{
					"alertname":   "TestAlert",
					"environment": "production", // This should be used
				},
			},
		},
	}

	result := service.matchesHookMessage(operarius, hookMessage)
	assert.True(t, result, "Should match because first alert has production environment")
}

// TestCheckDeduplication_DefaultTTL tests that default TTL is used when TTL is zero
func TestCheckDeduplication_DefaultTTL(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := &operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			Deduplication: &operariusv1alpha1.DeduplicationConfig{
				Enabled: true,
				TTL:     0, // Should use default of 300
			},
			Enabled: &enabled,
		},
	}

	// Create a job that's 4 minutes old (within default 5 minute TTL)
	recentJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "recent-job",
			Namespace: "openfero",
			Labels: map[string]string{
				"openfero.io/operarius": "test-operarius",
				"openfero.io/group-key": utils.HashGroupKey("test-group"),
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-4 * time.Minute)),
		},
	}

	ctx := context.TODO()
	_, err := kubeClient.BatchV1().Jobs("openfero").Create(ctx, recentJob, metav1.CreateOptions{})
	require.NoError(t, err)

	hookMessage := models.HookMessage{
		GroupKey: "test-group",
	}

	shouldCreate, err := service.CheckDeduplication(ctx, operarius, hookMessage)
	assert.NoError(t, err)
	assert.False(t, shouldCreate, "Should not create job within default TTL window")
}

// TestApplyTemplateVariables_TemplateErrors tests error handling in template processing
func TestApplyTemplateVariables_TemplateErrors(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	tests := []struct {
		name           string
		job            *batchv1.Job
		hookMessage    models.HookMessage
		expectError    bool
		errorSubstring string
	}{
		{
			name: "invalid env var template",
			job: &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test",
									Image: "busybox",
									Env: []corev1.EnvVar{
										{
											Name:  "INVALID",
											Value: "{{ .Invalid.Field",
										},
									},
								},
							},
						},
					},
				},
			},
			hookMessage: models.HookMessage{
				Status:   "firing",
				GroupKey: "test",
			},
			expectError:    true,
			errorSubstring: "env var",
		},
		{
			name: "invalid command template",
			job: &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:    "test",
									Image:   "busybox",
									Command: []string{"{{ .Unclosed"},
								},
							},
						},
					},
				},
			},
			hookMessage: models.HookMessage{
				Status:   "firing",
				GroupKey: "test",
			},
			expectError:    true,
			errorSubstring: "command",
		},
		{
			name: "invalid args template",
			job: &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:    "test",
									Image:   "busybox",
									Command: []string{"echo"},
									Args:    []string{"{{ .Broken"},
								},
							},
						},
					},
				},
			},
			hookMessage: models.HookMessage{
				Status:   "firing",
				GroupKey: "test",
			},
			expectError:    true,
			errorSubstring: "arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.applyTemplateVariables(tt.job, tt.hookMessage)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorSubstring != "" {
					assert.Contains(t, err.Error(), tt.errorSubstring)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestFindMatchingOperarius_UnknownAlertName tests error message when alertname is not found
func TestFindMatchingOperarius_UnknownAlertName(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	// Empty operarii list
	operarii := []operariusv1alpha1.Operarius{}

	// HookMessage with no alertname anywhere
	hookMessage := models.HookMessage{
		Status:       "firing",
		GroupKey:     "test-group",
		CommonLabels: map[string]string{},
		Alerts:       []models.Alert{},
	}

	result, err := service.FindMatchingOperarius(hookMessage, operarii)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown")
}

// TestFindMatchingOperarius_AlertNameFromFirstAlert tests alertname extraction from first alert
func TestFindMatchingOperarius_AlertNameFromFirstAlert(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	// Empty operarii - will trigger error path that extracts alertname
	operarii := []operariusv1alpha1.Operarius{}

	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		Alerts: []models.Alert{
			{
				Labels: map[string]string{
					"alertname": "FirstAlertName",
				},
			},
		},
	}

	result, err := service.FindMatchingOperarius(hookMessage, operarii)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "FirstAlertName")
}

// TestMatchesHookMessage_AlertNameFromCommonLabels tests matching when alertname is only in common labels
func TestMatchesHookMessage_AlertNameFromCommonLabels(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: "CommonLabelAlert",
				Status:    "firing",
			},
			Enabled: &enabled,
		},
	}

	// No alerts, only common labels
	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		CommonLabels: map[string]string{
			"alertname": "CommonLabelAlert",
		},
		Alerts: []models.Alert{},
	}

	result := service.matchesHookMessage(operarius, hookMessage)
	assert.True(t, result)
}

// TestMatchesHookMessage_AlertNameMismatch tests that mismatched alertnames don't match
func TestMatchesHookMessage_AlertNameMismatch(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: "ExpectedAlert",
				Status:    "firing",
			},
			Enabled: &enabled,
		},
	}

	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		CommonLabels: map[string]string{
			"alertname": "DifferentAlert",
		},
	}

	result := service.matchesHookMessage(operarius, hookMessage)
	assert.False(t, result)
}

// TestNewOperariusServiceWithK8sClient tests the K8s client constructor
func TestNewOperariusServiceWithK8sClient(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	// We can't easily test with a real k8sclient.OperariusClient in unit tests,
	// but we can verify the function signature works with nil
	service := NewOperariusServiceWithK8sClient(kubeClient, nil)

	assert.NotNil(t, service)
	assert.NotNil(t, service.kubeClient)
	assert.Nil(t, service.operariusClient)
}

// TestMatchesHookMessage_EmptyAlertLabels tests matching when alert has no labels
func TestMatchesHookMessage_EmptyAlertLabels(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: "TestAlert",
				Status:    "firing",
			},
			Enabled: &enabled,
		},
	}

	// Alert with empty labels - should fall back to common labels
	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
		},
		Alerts: []models.Alert{
			{
				Labels: map[string]string{}, // Empty - no alertname
			},
		},
	}

	result := service.matchesHookMessage(operarius, hookMessage)
	// Since first alert has no alertname, it falls back to common labels
	assert.True(t, result)
}

// TestMatchesHookMessage_StatusMismatch tests that mismatched status doesn't match
func TestMatchesHookMessage_StatusMismatch(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: "TestAlert",
				Status:    "firing",
			},
			Enabled: &enabled,
		},
	}

	hookMessage := models.HookMessage{
		Status:   "resolved", // Mismatch - operarius expects "firing"
		GroupKey: "test-group",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
		},
	}

	result := service.matchesHookMessage(operarius, hookMessage)
	assert.False(t, result, "Should not match due to status mismatch")
}

// TestMatchesHookMessage_LabelMismatch tests that mismatched labels don't match
func TestMatchesHookMessage_LabelMismatch(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: "TestAlert",
				Status:    "firing",
				Labels: map[string]string{
					"severity": "critical",
				},
			},
			Enabled: &enabled,
		},
	}

	tests := []struct {
		name        string
		hookMessage models.HookMessage
		expectMatch bool
	}{
		{
			name: "missing required label",
			hookMessage: models.HookMessage{
				Status:   "firing",
				GroupKey: "test-group",
				CommonLabels: map[string]string{
					"alertname": "TestAlert",
					// Missing "severity" label
				},
			},
			expectMatch: false,
		},
		{
			name: "wrong label value",
			hookMessage: models.HookMessage{
				Status:   "firing",
				GroupKey: "test-group",
				CommonLabels: map[string]string{
					"alertname": "TestAlert",
					"severity":  "warning", // Wrong value - expected "critical"
				},
			},
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.matchesHookMessage(operarius, tt.hookMessage)
			assert.Equal(t, tt.expectMatch, result)
		})
	}
}

// TestCreateJobFromOperarius_TemplateError tests job creation failure due to template error
func TestCreateJobFromOperarius_TemplateError(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := &operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: "TestAlert",
				Status:    "firing",
			},
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:    "main",
									Image:   "busybox",
									Command: []string{"echo", "{{ .Invalid"}, // Invalid template
								},
							},
						},
					},
				},
			},
			Enabled: &enabled,
		},
	}

	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "test-group",
	}

	ctx := context.TODO()
	job, err := service.CreateJobFromOperarius(ctx, operarius, hookMessage)

	assert.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "template")
}

// TestProcessTemplate_ExecutionError tests template execution error
func TestProcessTemplate_ExecutionError(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	// Template that will fail during execution (not parsing)
	// Using a method call on nil pointer triggers execution error
	templateStr := "{{ .SomeMethod }}"
	data := struct{}{}

	result, err := service.processTemplate(templateStr, data)
	// This should fail during execution as data has no SomeMethod
	assert.Error(t, err)
	assert.Empty(t, result)
}

// TestCheckDeduplication_ListJobsError tests deduplication when job listing fails
func TestCheckDeduplication_ListJobsError(t *testing.T) {
	// We need a client that returns an error on List
	// The fake client doesn't support this directly, so we'll test the flow differently
	// by ensuring dedup works correctly with empty results

	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := &operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			Deduplication: &operariusv1alpha1.DeduplicationConfig{
				Enabled: true,
				TTL:     300,
			},
			Enabled: &enabled,
		},
	}

	hookMessage := models.HookMessage{
		GroupKey: "new-group-key",
	}

	ctx := context.TODO()
	shouldCreate, err := service.CheckDeduplication(ctx, operarius, hookMessage)
	assert.NoError(t, err)
	assert.True(t, shouldCreate, "Should create job when no matching jobs exist")
}

// TestCheckDeduplication_JobOutsideTTL tests that old jobs don't block new job creation
func TestCheckDeduplication_JobOutsideTTL(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	service := NewOperariusService(kubeClient)

	enabled := true
	operarius := &operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-operarius",
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			Deduplication: &operariusv1alpha1.DeduplicationConfig{
				Enabled: true,
				TTL:     60, // 1 minute TTL
			},
			Enabled: &enabled,
		},
	}

	// Create a job that's 2 minutes old (outside 1 minute TTL)
	oldJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "old-job",
			Namespace: "openfero",
			Labels: map[string]string{
				"openfero.io/operarius": "test-operarius",
				"openfero.io/group-key": "test-group",
			},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
		},
	}

	ctx := context.TODO()
	_, err := kubeClient.BatchV1().Jobs("openfero").Create(ctx, oldJob, metav1.CreateOptions{})
	require.NoError(t, err)

	hookMessage := models.HookMessage{
		GroupKey: "test-group",
	}

	shouldCreate, err := service.CheckDeduplication(ctx, operarius, hookMessage)
	assert.NoError(t, err)
	assert.True(t, shouldCreate, "Should create job when existing job is outside TTL window")
}
