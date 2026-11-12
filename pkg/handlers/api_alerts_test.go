package handlers

import (
	"context"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	operariusv1alpha1 "github.com/OpenFero/openfero/api/v1alpha1"
	"github.com/OpenFero/openfero/pkg/alertstore/memory"
	"github.com/OpenFero/openfero/pkg/metadata"
	"github.com/OpenFero/openfero/pkg/models"
	"github.com/OpenFero/openfero/pkg/services"
)

// stubOperariusClient is a minimal services.OperariusClientInterface backed
// by an in-memory list, sufficient for exercising handleOperariusBasedJobs
// without a real Kubernetes API server.
type stubOperariusClient struct {
	mu        sync.Mutex
	operarii  []operariusv1alpha1.Operarius
	namespace string
}

func (s *stubOperariusClient) List() ([]operariusv1alpha1.Operarius, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]operariusv1alpha1.Operarius(nil), s.operarii...), nil
}

func (s *stubOperariusClient) ListFromAPI(ctx context.Context) ([]operariusv1alpha1.Operarius, error) {
	return s.List()
}

func (s *stubOperariusClient) Get(name string) (*operariusv1alpha1.Operarius, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.operarii {
		if s.operarii[i].Name == name {
			op := s.operarii[i]
			return &op, nil
		}
	}
	return nil, nil
}

func (s *stubOperariusClient) UpdateStatus(ctx context.Context, operarius *operariusv1alpha1.Operarius) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.operarii {
		if s.operarii[i].Name == operarius.Name {
			s.operarii[i].Status = operarius.Status
		}
	}
	return nil
}

func (s *stubOperariusClient) GetNamespace() string {
	return s.namespace
}

func dedupTestOperarius() operariusv1alpha1.Operarius {
	enabled := true
	return operariusv1alpha1.Operarius{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dedup-operarius",
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
									Name:    "remediation",
									Image:   "busybox",
									Command: []string{"echo", "ok"},
								},
							},
						},
					},
				},
			},
			Enabled: &enabled,
			Deduplication: &operariusv1alpha1.DeduplicationConfig{
				Enabled: true,
				TTL:     1_000_000_000, // effectively "forever" for the duration of the test
			},
		},
	}
}

// TestHandleOperariusBasedJobs_ConcurrentDeduplication is a regression test
// ensuring that concurrent webhook deliveries for the same alert group only
// ever result in one remediation Job, and that the "losing" requests are
// recorded as deduplicated rather than as job creation failures.
func TestHandleOperariusBasedJobs_ConcurrentDeduplication(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	operariusClient := &stubOperariusClient{
		namespace: "openfero",
		operarii:  []operariusv1alpha1.Operarius{dedupTestOperarius()},
	}

	server := &Server{
		KubeClient:       nil,
		AlertStore:       memory.NewMemoryStore(100),
		OperariusService: services.NewOperariusServiceWithClient(kubeClient, operariusClient),
	}
	require.NoError(t, server.AlertStore.Initialize())

	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "concurrent-group",
		Alerts: []models.Alert{
			{Labels: map[string]string{"alertname": "TestAlert"}},
		},
	}

	failedBefore := testutil.ToFloat64(metadata.JobsFailedTotal)

	const concurrency = 15
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			server.handleOperariusBasedJobs(context.Background(), hookMessage)
		}()
	}
	wg.Wait()

	// Exactly one Job should exist despite the concurrent deliveries.
	jobs, err := kubeClient.BatchV1().Jobs("openfero").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, jobs.Items, 1, "concurrent alerts for the same group must only create one Job")

	// None of the concurrent requests should have been counted as a failure.
	failedAfter := testutil.ToFloat64(metadata.JobsFailedTotal)
	assert.Equal(t, failedBefore, failedAfter, "deduplicated job creation must not increment JobsFailedTotal")

	// Every alert delivery should still be recorded in the alert store.
	entries, err := server.AlertStore.GetAlerts("", 0)
	require.NoError(t, err)
	assert.Len(t, entries, concurrency)

	successCount, skippedCount := 0, 0
	for _, entry := range entries {
		require.NotNil(t, entry.JobInfo)
		if entry.JobInfo.LastExecutionStatus == "Skipped: Deduplication" {
			skippedCount++
		} else {
			successCount++
		}
	}
	assert.Equal(t, 1, successCount, "exactly one request should have created the real job")
	assert.Equal(t, concurrency-1, skippedCount, "all other requests should be marked as deduplicated")
}

// TestHandleOperariusBasedJobs_DryRun verifies that an Operarius with DryRun=true
// does not create a Kubernetes Job, does not increment ExecutionCount, and records
// the skipped status in both the Operarius status and the alert store.
func TestHandleOperariusBasedJobs_DryRun(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	op := dedupTestOperarius()
	op.Spec.Deduplication = nil
	op.Spec.DryRun = true

	operariusClient := &stubOperariusClient{
		namespace: "openfero",
		operarii:  []operariusv1alpha1.Operarius{op},
	}

	server := &Server{
		KubeClient:       nil,
		AlertStore:       memory.NewMemoryStore(100),
		OperariusService: services.NewOperariusServiceWithClient(kubeClient, operariusClient),
	}
	require.NoError(t, server.AlertStore.Initialize())

	hookMessage := models.HookMessage{
		Status:   "firing",
		GroupKey: "dryrun-group",
		Alerts: []models.Alert{
			{Labels: map[string]string{"alertname": "TestAlert"}},
		},
	}

	failedBefore := testutil.ToFloat64(metadata.JobsFailedTotal)
	dryRunBefore := testutil.ToFloat64(metadata.AlertsMatchedTotal.WithLabelValues("dry_run"))
	server.handleOperariusBasedJobs(context.Background(), hookMessage)

	jobs, err := kubeClient.BatchV1().Jobs("openfero").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, jobs.Items, "dry-run Operarius must not create a Job")

	assert.Equal(t, failedBefore, testutil.ToFloat64(metadata.JobsFailedTotal), "dry-run must not increment JobsFailedTotal")
	assert.Equal(t, dryRunBefore+1, testutil.ToFloat64(metadata.AlertsMatchedTotal.WithLabelValues("dry_run")), "dry-run must increment dry_run outcome counter")

	updatedOp, err := operariusClient.Get(op.Name)
	require.NoError(t, err)
	require.NotNil(t, updatedOp)
	assert.NotNil(t, updatedOp.Status.LastTriggeredTime, "dry-run match must record LastTriggeredTime")
	assert.Nil(t, updatedOp.Status.LastExecutionTime, "dry-run must not set LastExecutionTime")

	entries, err := server.AlertStore.GetAlerts("", 0)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].JobInfo)
	assert.Equal(t, "N/A (DryRun)", entries[0].JobInfo.JobName)
	assert.Equal(t, "Skipped: DryRun", entries[0].JobInfo.LastExecutionStatus)
	assert.Equal(t, "busybox", entries[0].JobInfo.Image)
	assert.Equal(t, int32(0), entries[0].JobInfo.ExecutionCount)
}
