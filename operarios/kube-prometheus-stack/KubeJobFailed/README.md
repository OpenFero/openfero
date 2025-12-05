# KubeJobFailed Operarius

Automatically cleans up failed Kubernetes Jobs to clear alerts.

## Alert Definition

Source: [kube-prometheus-stack kubernetes-apps.yaml](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/templates/prometheus/rules-1.14/kubernetes-apps.yaml)

```yaml
- alert: KubeJobFailed
  annotations:
    description: Job {{ $labels.namespace }}/{{ $labels.job_name }} failed to complete. Removing failed job after investigation should clear this alert.
    summary: Job failed to complete.
  expr: kube_job_failed > 0
  for: 15m
  labels:
    severity: warning
```

## What This Does

When triggered, this Operarius:

1. Logs the failed job details
2. Retrieves and displays pod logs for debugging
3. Deletes the failed job to clear the alert
4. Reports success

This is useful for cleaning up jobs that have failed and are no longer needed, preventing alert fatigue.

## Available Environment Variables

OpenFero automatically provides these from the alert labels:

| Variable | Description |
|----------|-------------|
| `OPENFERO_NAMESPACE` | Namespace of the failed job |
| `OPENFERO_JOB_NAME` | Name of the failed job |
| `OPENFERO_SEVERITY` | Alert severity (warning) |
| `OPENFERO_CLUSTER` | Cluster name |

## Installation

```bash
kubectl apply -f rbac.yaml
kubectl apply -f operarius.yaml
```

## Testing

```bash
# Send test alert
curl -X POST http://localhost:8080/alerts \
  -H "Content-Type: application/json" \
  -d @test-alert.json

# Watch job creation
kubectl get jobs -n openfero -w

# Check Operarius status
kubectl get operarius failed-job-cleanup -n openfero -o yaml
```

## Safety

- **Safe for production**: Only removes failed jobs, not running ones
- **Logs preserved**: Pod logs are captured before deletion
- **Deduplication**: 10 minute TTL prevents rapid re-cleanup

## Important Note

This Operarius removes failed jobs without manual investigation. Consider:

1. Setting up log aggregation (e.g., Loki, Elasticsearch) before enabling
2. Adjusting the deduplication TTL to allow time for investigation
3. Disabling for critical jobs that require manual review

## Rollback

Jobs that have been deleted cannot be restored. Ensure you have:

- Centralized logging for job output
- Job definitions in version control
- Monitoring for job failures before enabling auto-cleanup
