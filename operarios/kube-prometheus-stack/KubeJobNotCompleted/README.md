# KubeJobNotCompleted Operarius

Automatically cleans up Kubernetes Jobs that have been running far longer than expected.

> **Template**: ships with `spec.enabled: false` and no scope restriction. This deletes a still-**active** job and its running pods - some jobs are legitimately long-running. Review, scope `alertSelector.labels` to your environment, then enable - see [the catalog warning](../README.md#%EF%B8%8F-these-are-templates-not-install-and-forget).

## Alert Definition

Source: [kube-prometheus-stack kubernetes-apps.yaml](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/templates/prometheus/rules-1.14/kubernetes-apps.yaml)

```yaml
- alert: KubeJobNotCompleted
  annotations:
    description: Job {{ $labels.namespace }}/{{ $labels.job_name }} is taking more than {{ "43200" | humanizeDuration }} to complete on cluster {{ $labels.cluster }}.
    summary: Job did not complete in time
  expr: |-
    time() - max by (namespace, job_name, cluster) (kube_job_status_start_time{job="kube-state-metrics"}
      and
    kube_job_status_active{job="kube-state-metrics"} > 0) > 43200
  labels:
    severity: warning
```

This complements `KubeJobFailed`: that Operarius cleans up jobs that have already **failed** (exited non-zero). This one targets jobs that are still **active** (running pods) but have been running for an unreasonably long time (default threshold: 12 hours) - typically a hung process, a stuck retry loop, or a job that will never terminate on its own.

## What This Does

When triggered, this Operarius:

1. Logs the stuck job's details
2. Retrieves and displays pod logs for debugging (from the still-running pod)
3. Deletes the job - and its still-running pods - to clear the alert
4. Reports success

## Available Environment Variables

OpenFero automatically provides these from the alert labels:

| Variable             | Description               |
| -------------------- | -------------------------- |
| `OPENFERO_NAMESPACE` | Namespace of the stuck job |
| `OPENFERO_JOB_NAME`  | Name of the stuck job      |
| `OPENFERO_SEVERITY`  | Alert severity (warning)   |
| `OPENFERO_CLUSTER`   | Cluster name               |

## Installation

```bash
kubectl apply -f rbac.yaml
kubectl apply -f operarius.yaml
```

This applies the Operarius **disabled**. Edit `operarius.yaml` to scope `alertSelector.labels` to your environment (see the commented example inside), set `spec.enabled: true`, then re-apply.

## Testing

```bash
# Send test alert
curl -X POST http://localhost:8080/alerts \
  -H "Content-Type: application/json" \
  -d @test-alert.json

# Watch job creation
kubectl get jobs -n openfero -w

# Check Operarius status
kubectl get operarius stuck-job-cleanup -n openfero -o yaml
```

## Safety

- **Important note**: unlike `KubeJobFailed`, this Operarius deletes a job that is still **active**, terminating any pods it currently has running. Only enable this if you are confident that a job running past the alert threshold (12h by default) is genuinely stuck, not simply long-running by design.
- **Logs preserved**: pod logs are captured before deletion
- **Deduplication**: 1 hour TTL (longer than `KubeJobFailed`'s) to reduce the chance of repeatedly killing a legitimately slow job while it is being investigated

## Important Note

This Operarius terminates a job while it is still running. Consider:

1. Raising the alert's `for`/threshold (or disabling this Operarius) for namespaces that run legitimately long batch jobs
2. Setting up log aggregation (e.g., Loki, Elasticsearch) before enabling
3. Disabling for critical or non-idempotent jobs that must not be interrupted mid-run

## Rollback

Jobs (and their running pods) that have been deleted cannot be restored. Ensure you have:

- Centralized logging for job output
- Job definitions in version control so the job can be re-triggered
- An understanding of whether the job is idempotent/safe to interrupt before enabling auto-cleanup
