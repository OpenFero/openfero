package services

import (
	"testing"

	"github.com/OpenFero/openfero/pkg/alertstore"
	"github.com/OpenFero/openfero/pkg/alertstore/memory"
	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/models"
	"go.uber.org/zap"
)

func init() {
	// Suppress debug logging in benchmarks to avoid I/O noise
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	_ = log.SetConfig(cfg)
}

func benchmarkAlertStoreAlert(alertName string) models.Alert {
	return models.Alert{
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
			"summary":     "Alert " + alertName + " is firing",
			"description": "Detailed description for benchmarking",
			"runbook_url": "https://runbooks.example.com/" + alertName,
		},
		StartsAt: "2026-02-17T10:00:00.000Z",
	}
}

// --- SaveAlert benchmarks ---

func BenchmarkSaveAlert(b *testing.B) {
	store := memory.NewMemoryStore(1000)
	_ = store.Initialize()
	defer store.Close()
	alert := benchmarkAlertStoreAlert("KubeQuotaAlmostFull")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		SaveAlert(store, alert, "firing")
	}
}

func BenchmarkSaveAlertWithJobInfo(b *testing.B) {
	store := memory.NewMemoryStore(1000)
	_ = store.Initialize()
	defer store.Close()
	alert := benchmarkAlertStoreAlert("KubeQuotaAlmostFull")
	jobInfo := &alertstore.JobInfo{
		OperariusName:       "openfero-KubeQuotaAlmostFull-firing",
		JobName:             "KubeQuotaAlmostFull",
		Namespace:           "openfero",
		Image:               "remediation-worker:latest",
		ExecutionCount:      5,
		LastExecutedJobName: "openfero-KubeQuotaAlmostFull-firing-abc123",
		LastExecutionStatus: "Successful",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		SaveAlertWithJobInfo(store, alert, "firing", jobInfo)
	}
}

func BenchmarkSaveAlert_StoreAtCapacity(b *testing.B) {
	store := memory.NewMemoryStore(100)
	_ = store.Initialize()
	defer store.Close()

	// Fill the store to capacity
	for i := 0; i < 100; i++ {
		alert := benchmarkAlertStoreAlert("PreFillAlert")
		SaveAlert(store, alert, "firing")
	}

	alert := benchmarkAlertStoreAlert("KubeQuotaAlmostFull")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		SaveAlert(store, alert, "firing")
	}
}

func BenchmarkCheckAlertStatus(b *testing.B) {
	statuses := []string{"firing", "resolved", "invalid", "pending", ""}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		CheckAlertStatus(statuses[i%len(statuses)])
	}
}
