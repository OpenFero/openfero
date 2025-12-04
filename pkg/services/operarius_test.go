package services

import (
	"context"
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
	"github.com/OpenFero/openfero/pkg/models"
)

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
	assert.Equal(t, "test-group", job.Labels["openfero.io/group-key"])
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
				"openfero.io/group-key": "test-group",
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
				"openfero.io/group-key": "test-group",
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
