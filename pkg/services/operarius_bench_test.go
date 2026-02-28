package services

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"text/template"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	operariusv1alpha1 "github.com/OpenFero/openfero/api/v1alpha1"
	"github.com/OpenFero/openfero/pkg/models"
)

// --- Benchmark fixtures ---

func benchmarkOperarii(count int) []operariusv1alpha1.Operarius {
	enabled := true
	operarii := make([]operariusv1alpha1.Operarius, count)
	for i := 0; i < count; i++ {
		labels := map[string]string{
			"severity": "warning",
		}
		if i%3 == 0 {
			labels["team"] = "platform"
		}
		if i%5 == 0 {
			labels["env"] = "production"
		}
		operarii[i] = operariusv1alpha1.Operarius{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("operarius-%d", i),
				Namespace: "openfero",
			},
			Spec: operariusv1alpha1.OperariusSpec{
				AlertSelector: operariusv1alpha1.AlertSelector{
					AlertName: fmt.Sprintf("Alert%d", i),
					Status:    "firing",
					Labels:    labels,
				},
				Priority: int32(i * 10),
				Enabled:  &enabled,
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyNever,
								Containers: []corev1.Container{
									{
										Name:  "worker",
										Image: fmt.Sprintf("worker-image-%d:latest", i),
										Command: []string{
											"/bin/sh",
											"-c",
											"echo remediation",
										},
										Env: []corev1.EnvVar{
											{Name: "NAMESPACE", Value: "{{ .Labels.namespace }}"},
											{Name: "ALERT_NAME", Value: "{{ .Labels.alertname }}"},
											{Name: "STATUS", Value: "{{ .Status }}"},
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}
	return operarii
}

func benchmarkHookMessage(alertName string) models.HookMessage {
	return models.HookMessage{
		Version:  "4",
		GroupKey: fmt.Sprintf("{}:{alertname=%q}", alertName),
		Status:   "firing",
		Receiver: "openfero",
		GroupLabels: map[string]string{
			"alertname": alertName,
		},
		CommonLabels: map[string]string{
			"alertname": alertName,
			"severity":  "warning",
			"namespace": "default",
			"service":   "my-service",
			"team":      "platform",
			"env":       "production",
		},
		CommonAnnotations: map[string]string{
			"summary":     fmt.Sprintf("Alert %s is firing", alertName),
			"description": "This is a test alert for benchmarking purposes",
			"runbook_url": "https://runbooks.example.com/alert",
		},
		Alerts: []models.Alert{
			{
				Labels: map[string]string{
					"alertname": alertName,
					"severity":  "warning",
					"namespace": "default",
					"pod":       "my-pod-abc123",
					"container": "main",
					"service":   "my-service",
					"team":      "platform",
					"env":       "production",
					"instance":  "10.0.0.1:9090",
					"job":       "kubelet",
				},
				Annotations: map[string]string{
					"summary":     fmt.Sprintf("Alert %s is firing", alertName),
					"description": "This is a test alert for benchmarking purposes with detailed description",
					"runbook_url": "https://runbooks.example.com/alert",
				},
				StartsAt: "2026-02-17T10:00:00.000Z",
			},
		},
	}
}

func benchmarkOperariusForAlert(alertName string) *operariusv1alpha1.Operarius {
	enabled := true
	return &operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("openfero-%s-firing", alertName),
			Namespace: "openfero",
		},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: alertName,
				Status:    "firing",
				Labels: map[string]string{
					"severity": "warning",
				},
			},
			Priority: 100,
			Enabled:  &enabled,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "remediation",
					},
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:  "worker",
									Image: "remediation-worker:latest",
									Command: []string{
										"/bin/sh",
										"-c",
										"echo running remediation for {{ .Labels.namespace }}",
									},
									Args: []string{
										"--alert={{ .Labels.alertname }}",
										"--namespace={{ .Labels.namespace }}",
										"--pod={{ .Labels.pod }}",
									},
									Env: []corev1.EnvVar{
										{Name: "ALERT_NAMESPACE", Value: "{{ .Labels.namespace }}"},
										{Name: "ALERT_NAME", Value: "{{ .Labels.alertname }}"},
										{Name: "ALERT_STATUS", Value: "{{ .Status }}"},
										{Name: "ALERT_POD", Value: "{{ .Labels.pod }}"},
										{Name: "GROUP_KEY", Value: "{{ .GroupKey }}"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// --- matchesHookMessage benchmarks ---

func BenchmarkMatchesHookMessage_SingleOperarius(b *testing.B) {
	service := NewOperariusService(fake.NewSimpleClientset())
	operarii := benchmarkOperarii(1)
	hookMsg := benchmarkHookMessage(operarii[0].Spec.AlertSelector.AlertName)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		service.matchesHookMessage(operarii[0], hookMsg)
	}
}

func BenchmarkMatchesHookMessage_WithLabels(b *testing.B) {
	service := NewOperariusService(fake.NewSimpleClientset())
	enabled := true
	operarius := operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "openfero"},
		Spec: operariusv1alpha1.OperariusSpec{
			AlertSelector: operariusv1alpha1.AlertSelector{
				AlertName: "KubeQuotaAlmostFull",
				Status:    "firing",
				Labels: map[string]string{
					"severity":  "warning",
					"team":      "platform",
					"env":       "production",
					"namespace": "default",
				},
			},
			Enabled: &enabled,
		},
	}
	hookMsg := benchmarkHookMessage("KubeQuotaAlmostFull")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		service.matchesHookMessage(operarius, hookMsg)
	}
}

func BenchmarkMatchesHookMessage_NoMatch(b *testing.B) {
	service := NewOperariusService(fake.NewSimpleClientset())
	operarii := benchmarkOperarii(1)
	hookMsg := benchmarkHookMessage("CompletelyDifferentAlert")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		service.matchesHookMessage(operarii[0], hookMsg)
	}
}

// --- FindMatchingOperarius benchmarks ---

func BenchmarkFindMatchingOperarius_10(b *testing.B) {
	benchmarkFindMatching(b, 10)
}

func BenchmarkFindMatchingOperarius_50(b *testing.B) {
	benchmarkFindMatching(b, 50)
}

func BenchmarkFindMatchingOperarius_100(b *testing.B) {
	benchmarkFindMatching(b, 100)
}

func BenchmarkFindMatchingOperarius_500(b *testing.B) {
	benchmarkFindMatching(b, 500)
}

func benchmarkFindMatching(b *testing.B, count int) {
	b.Helper()
	service := NewOperariusService(fake.NewSimpleClientset())
	operarii := benchmarkOperarii(count)

	// Match the last operarius to force full scan
	lastAlertName := operarii[count-1].Spec.AlertSelector.AlertName
	hookMsg := benchmarkHookMessage(lastAlertName)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = service.FindMatchingOperarius(hookMsg, operarii)
	}
}

func BenchmarkFindMatchingOperarius_NoMatch_100(b *testing.B) {
	service := NewOperariusService(fake.NewSimpleClientset())
	operarii := benchmarkOperarii(100)
	hookMsg := benchmarkHookMessage("NonExistentAlert")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = service.FindMatchingOperarius(hookMsg, operarii)
	}
}

// --- processTemplate benchmarks ---

func BenchmarkProcessTemplate_Simple(b *testing.B) {
	service := NewOperariusService(fake.NewSimpleClientset())
	data := struct {
		Labels map[string]string
		Status string
	}{
		Labels: map[string]string{"alertname": "TestAlert", "namespace": "default"},
		Status: "firing",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = service.processTemplate("{{ .Labels.alertname }}", data)
	}
}

func BenchmarkProcessTemplate_Complex(b *testing.B) {
	service := NewOperariusService(fake.NewSimpleClientset())
	hookMsg := benchmarkHookMessage("KubeQuotaAlmostFull")
	data := struct {
		Alert       models.Alert
		HookMessage models.HookMessage
		Labels      map[string]string
		Annotations map[string]string
		GroupKey    string
		Status      string
	}{
		Alert:       hookMsg.Alerts[0],
		HookMessage: hookMsg,
		Labels:      hookMsg.Alerts[0].Labels,
		Annotations: hookMsg.Alerts[0].Annotations,
		GroupKey:    hookMsg.GroupKey,
		Status:      hookMsg.Status,
	}
	templateStr := "Alert {{ .Labels.alertname }} in ns {{ .Labels.namespace }} pod {{ .Labels.pod }} status {{ .Status }} group {{ .GroupKey }}"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = service.processTemplate(templateStr, data)
	}
}

func BenchmarkProcessTemplate_NoTemplate(b *testing.B) {
	service := NewOperariusService(fake.NewSimpleClientset())

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = service.processTemplate("plain-string-no-template", nil)
	}
}

func BenchmarkProcessTemplate_MultipleVars(b *testing.B) {
	service := NewOperariusService(fake.NewSimpleClientset())
	hookMsg := benchmarkHookMessage("KubeQuotaAlmostFull")
	data := struct {
		Alert       models.Alert
		HookMessage models.HookMessage
		Labels      map[string]string
		Annotations map[string]string
		GroupKey    string
		Status      string
	}{
		Alert:       hookMsg.Alerts[0],
		HookMessage: hookMsg,
		Labels:      hookMsg.Alerts[0].Labels,
		Annotations: hookMsg.Alerts[0].Annotations,
		GroupKey:    hookMsg.GroupKey,
		Status:      hookMsg.Status,
	}
	templateStr := "ns={{ .Labels.namespace }} alert={{ .Labels.alertname }} pod={{ .Labels.pod }} svc={{ .Labels.service }} env={{ .Labels.env }} severity={{ .Labels.severity }} instance={{ .Labels.instance }} status={{ .Status }}"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = service.processTemplate(templateStr, data)
	}
}

// BenchmarkProcessTemplate_RawGoTemplate benchmarks raw Go template parsing and execution
// as a baseline without the OpenFero wrapper overhead
func BenchmarkProcessTemplate_RawGoTemplate(b *testing.B) {
	data := map[string]string{
		"alertname": "KubeQuotaAlmostFull",
		"namespace": "default",
		"pod":       "my-pod-abc123",
	}
	templateStr := "alert={{ .alertname }} ns={{ .namespace }} pod={{ .pod }}"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tmpl, _ := template.New("bench").Parse(templateStr)
		var buf bytes.Buffer
		_ = tmpl.Execute(&buf, data)
	}
}

// --- CreateJobFromOperarius benchmarks ---

func BenchmarkCreateJobFromOperarius_Simple(b *testing.B) {
	fakeClient := fake.NewSimpleClientset()
	service := NewOperariusService(fakeClient)
	operarius := benchmarkOperariusForAlert("KubeQuotaAlmostFull")
	hookMsg := benchmarkHookMessage("KubeQuotaAlmostFull")
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = service.CreateJobFromOperarius(ctx, operarius, hookMsg)
	}
}

func BenchmarkCreateJobFromOperarius_ManyLabels(b *testing.B) {
	fakeClient := fake.NewSimpleClientset()
	service := NewOperariusService(fakeClient)
	operarius := benchmarkOperariusForAlert("KubeQuotaAlmostFull")

	// Create a hook message with many labels to stress env var injection
	hookMsg := benchmarkHookMessage("KubeQuotaAlmostFull")
	for i := 0; i < 20; i++ {
		hookMsg.Alerts[0].Labels[fmt.Sprintf("custom_label_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = service.CreateJobFromOperarius(ctx, operarius, hookMsg)
	}
}

// --- CheckDeduplication benchmarks ---

func BenchmarkCheckDeduplication_Disabled(b *testing.B) {
	fakeClient := fake.NewSimpleClientset()
	service := NewOperariusService(fakeClient)
	operarius := benchmarkOperariusForAlert("TestAlert")
	// Deduplication is nil by default
	hookMsg := benchmarkHookMessage("TestAlert")
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = service.CheckDeduplication(ctx, operarius, hookMsg)
	}
}

func BenchmarkCheckDeduplication_Enabled_NoExistingJobs(b *testing.B) {
	fakeClient := fake.NewSimpleClientset()
	service := NewOperariusService(fakeClient)
	operarius := benchmarkOperariusForAlert("TestAlert")
	operarius.Spec.Deduplication = &operariusv1alpha1.DeduplicationConfig{
		Enabled: true,
		TTL:     300,
	}
	hookMsg := benchmarkHookMessage("TestAlert")
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = service.CheckDeduplication(ctx, operarius, hookMsg)
	}
}

func BenchmarkCheckDeduplication_Enabled_WithExistingJobs(b *testing.B) {
	// Pre-populate with existing jobs
	fakeClient := fake.NewSimpleClientset()
	service := NewOperariusService(fakeClient)
	operarius := benchmarkOperariusForAlert("TestAlert")
	operarius.Spec.Deduplication = &operariusv1alpha1.DeduplicationConfig{
		Enabled: true,
		TTL:     300,
	}
	hookMsg := benchmarkHookMessage("TestAlert")
	ctx := context.Background()

	// Create some existing jobs that match
	for i := 0; i < 5; i++ {
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("existing-job-%d", i),
				Namespace: "openfero",
				Labels: map[string]string{
					"openfero.io/operarius": operarius.Name,
					"openfero.io/group-key": "test-group-key",
				},
				CreationTimestamp: metav1.Now(),
			},
		}
		_, _ = fakeClient.BatchV1().Jobs("openfero").Create(ctx, job, metav1.CreateOptions{})
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = service.CheckDeduplication(ctx, operarius, hookMsg)
	}
}

// --- ToJobInfo benchmarks ---

func BenchmarkToJobInfo(b *testing.B) {
	service := NewOperariusService(fake.NewSimpleClientset())
	operarius := *benchmarkOperariusForAlert("KubeQuotaAlmostFull")
	now := metav1.Now()
	operarius.Status = operariusv1alpha1.OperariusStatus{
		LastExecutionTime:   &now,
		ExecutionCount:      42,
		LastExecutedJobName: "openfero-KubeQuotaAlmostFull-firing-abc123",
		LastExecutionStatus: "Successful",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = service.ToJobInfo(operarius)
	}
}

// --- Composite benchmarks (realistic workflows) ---

func BenchmarkFullAlertProcessingPipeline(b *testing.B) {
	fakeClient := fake.NewSimpleClientset()
	service := NewOperariusService(fakeClient)
	operarii := benchmarkOperarii(50)

	// Add our target operarius
	target := *benchmarkOperariusForAlert("KubeQuotaAlmostFull")
	operarii = append(operarii, target)

	hookMsg := benchmarkHookMessage("KubeQuotaAlmostFull")
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Step 1: Find matching operarius
		matched, err := service.FindMatchingOperarius(hookMsg, operarii)
		if err != nil {
			b.Fatal(err)
		}

		// Step 2: Check deduplication
		shouldCreate, err := service.CheckDeduplication(ctx, matched, hookMsg)
		if err != nil {
			b.Fatal(err)
		}

		// Step 3: Create job (if deduplication allows)
		if shouldCreate {
			_, _ = service.CreateJobFromOperarius(ctx, matched, hookMsg)
		}
	}
}
