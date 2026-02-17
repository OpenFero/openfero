package memory

import (
	"fmt"
	"testing"

	"github.com/OpenFero/openfero/pkg/alertstore"
	log "github.com/OpenFero/openfero/pkg/logging"
	"go.uber.org/zap"
)

func init() {
	// Initialize logger for benchmarks and suppress debug logging to avoid I/O noise
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	_ = log.SetConfig(cfg)
}

func benchmarkAlertEntry(alertName string) alertstore.Alert {
	return alertstore.Alert{
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

func prepopulateStore(store *MemoryStore, count int) {
	alerts := []string{
		"KubeQuotaAlmostFull", "PodCrashLooping", "DiskSpaceAlmostFull",
		"KubeDeploymentReplicasMismatch", "KubeHpaMaxedOut", "KubeJobFailed",
		"NodeNotReady", "TargetDown", "PrometheusRuleFailures", "WatchdogAlert",
	}
	statuses := []string{"firing", "resolved"}

	for i := 0; i < count; i++ {
		alert := benchmarkAlertEntry(alerts[i%len(alerts)])
		alert.Labels["pod"] = fmt.Sprintf("pod-%d", i)
		status := statuses[i%2]
		_ = store.SaveAlert(alert, status)
	}
}

// --- MemoryStore.SaveAlert benchmarks ---

func BenchmarkMemoryStore_SaveAlert(b *testing.B) {
	store := NewMemoryStore(10000)
	_ = store.Initialize()
	defer store.Close()
	alert := benchmarkAlertEntry("KubeQuotaAlmostFull")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = store.SaveAlert(alert, "firing")
	}
}

func BenchmarkMemoryStore_SaveAlertWithJobInfo(b *testing.B) {
	store := NewMemoryStore(10000)
	_ = store.Initialize()
	defer store.Close()
	alert := benchmarkAlertEntry("KubeQuotaAlmostFull")
	jobInfo := &alertstore.JobInfo{
		OperariusName:       "openfero-KubeQuotaAlmostFull-firing",
		JobName:             "KubeQuotaAlmostFull",
		Namespace:           "openfero",
		Image:               "worker:latest",
		ExecutionCount:      5,
		LastExecutedJobName: "job-abc123",
		LastExecutionStatus: "Successful",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = store.SaveAlertWithJobInfo(alert, "firing", jobInfo)
	}
}

func BenchmarkMemoryStore_SaveAlert_AtCapacity(b *testing.B) {
	store := NewMemoryStore(500)
	_ = store.Initialize()
	defer store.Close()
	prepopulateStore(store, 500)
	alert := benchmarkAlertEntry("NewAlert")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = store.SaveAlert(alert, "firing")
	}
}

// --- MemoryStore.GetAlerts benchmarks ---

func BenchmarkMemoryStore_GetAlerts_NoFilter_100(b *testing.B) {
	benchmarkGetAlerts(b, 100, "", 0)
}

func BenchmarkMemoryStore_GetAlerts_NoFilter_1000(b *testing.B) {
	benchmarkGetAlerts(b, 1000, "", 0)
}

func BenchmarkMemoryStore_GetAlerts_WithFilter_100(b *testing.B) {
	benchmarkGetAlerts(b, 100, "KubeQuotaAlmostFull", 0)
}

func BenchmarkMemoryStore_GetAlerts_WithFilter_1000(b *testing.B) {
	benchmarkGetAlerts(b, 1000, "KubeQuotaAlmostFull", 0)
}

func BenchmarkMemoryStore_GetAlerts_WithLimit_1000(b *testing.B) {
	benchmarkGetAlerts(b, 1000, "", 50)
}

func BenchmarkMemoryStore_GetAlerts_FilterAndLimit_1000(b *testing.B) {
	benchmarkGetAlerts(b, 1000, "DiskSpace", 20)
}

func benchmarkGetAlerts(b *testing.B, storeSize int, query string, limit int) {
	b.Helper()
	store := NewMemoryStore(storeSize + 100)
	_ = store.Initialize()
	defer store.Close()
	prepopulateStore(store, storeSize)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = store.GetAlerts(query, limit)
	}
}

// --- alertMatchesQuery benchmarks ---

func BenchmarkAlertMatchesQuery_ByAlertname(b *testing.B) {
	entry := alertstore.AlertEntry{
		Alert:  benchmarkAlertEntry("KubeQuotaAlmostFull"),
		Status: "firing",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		alertMatchesQuery(entry, "kubequota")
	}
}

func BenchmarkAlertMatchesQuery_ByLabel(b *testing.B) {
	entry := alertstore.AlertEntry{
		Alert:  benchmarkAlertEntry("KubeQuotaAlmostFull"),
		Status: "firing",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		alertMatchesQuery(entry, "platform")
	}
}

func BenchmarkAlertMatchesQuery_NoMatch(b *testing.B) {
	entry := alertstore.AlertEntry{
		Alert:  benchmarkAlertEntry("KubeQuotaAlmostFull"),
		Status: "firing",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		alertMatchesQuery(entry, "nonexistent")
	}
}

func BenchmarkAlertMatchesQuery_WithJobInfo(b *testing.B) {
	entry := alertstore.AlertEntry{
		Alert:  benchmarkAlertEntry("KubeQuotaAlmostFull"),
		Status: "firing",
		JobInfo: &alertstore.JobInfo{
			OperariusName: "openfero-KubeQuotaAlmostFull-firing",
			JobName:       "KubeQuotaAlmostFull",
			Image:         "worker:latest",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		alertMatchesQuery(entry, "worker")
	}
}

// --- Concurrent access benchmarks ---

func BenchmarkMemoryStore_SaveAlert_Parallel(b *testing.B) {
	store := NewMemoryStore(10000)
	_ = store.Initialize()
	defer store.Close()
	alert := benchmarkAlertEntry("KubeQuotaAlmostFull")

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = store.SaveAlert(alert, "firing")
		}
	})
}

func BenchmarkMemoryStore_GetAlerts_Parallel(b *testing.B) {
	store := NewMemoryStore(1100)
	_ = store.Initialize()
	defer store.Close()
	prepopulateStore(store, 1000)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = store.GetAlerts("KubeQuota", 10)
		}
	})
}

func BenchmarkMemoryStore_Mixed_ReadWrite_Parallel(b *testing.B) {
	store := NewMemoryStore(5000)
	_ = store.Initialize()
	defer store.Close()
	prepopulateStore(store, 1000)
	alert := benchmarkAlertEntry("NewAlertForBenchmark")

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%3 == 0 {
				// Every 3rd operation is a write
				_ = store.SaveAlert(alert, "firing")
			} else {
				// Read operations
				_, _ = store.GetAlerts("KubeQuota", 10)
			}
			i++
		}
	})
}
