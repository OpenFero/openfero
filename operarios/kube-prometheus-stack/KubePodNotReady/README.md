# KubePodNotReady Operarius

Automatically restarts pods stuck Pending, Unknown, or Running-but-never-Ready.

> **Template**: ships with `spec.enabled: false` and no scope restriction. Review, scope `alertSelector.labels` to your environment, then enable - see [the catalog warning](../README.md#%EF%B8%8F-these-are-templates-not-install-and-forget).

## Alert Definition

Source: [kube-prometheus-stack kubernetes-apps.yaml](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/templates/prometheus/rules-1.14/kubernetes-apps.yaml)

```yaml
- alert: KubePodNotReady
  annotations:
    description: Pod {{ $labels.namespace }}/{{ $labels.pod }} has been in a non-ready state for longer than 15 minutes on cluster {{ $labels.cluster }}.
    summary: Pod has been in a non-ready state for more than 15 minutes.
  expr: |-
    sum by (namespace, pod, job, cluster) (
      max by (namespace, pod, job, cluster) (
        kube_pod_status_phase{job="kube-state-metrics", phase=~"Pending|Unknown"}
        or
        (
          kube_pod_status_phase{job="kube-state-metrics", phase="Running"} == 1
          and on (namespace, pod, cluster)
          kube_pod_status_ready{job="kube-state-metrics", condition="true"} == 0
        )
      ) * on (namespace, pod, cluster) group_left() topk by (namespace, pod, cluster) (
        1, max by (namespace, pod, owner_kind, cluster) (kube_pod_owner{owner_kind!="Job"})
      )
    ) > 0
    unless on (namespace, pod, cluster)
    kube_pod_status_reason{job="kube-state-metrics", reason="SchedulingGated"} == 1
  for: 15m
  labels:
    severity: warning
```

Unlike `KubePodCrashLooping` (which targets pods actively restarting), this alert catches pods that never become ready in the first place - stuck `Pending` (e.g. unschedulable), `Unknown` (e.g. unreachable node), or a `Running` pod whose readiness probe never succeeds.

## What This Does

When triggered, this Operarius:

1. Logs the affected pod's status
2. Deletes the pod to trigger a fresh restart/reschedule by the controller
3. Reports success or failure

## Available Environment Variables

OpenFero automatically provides these from the alert labels:

| Variable             | Description                  |
| -------------------- | ----------------------------- |
| `OPENFERO_NAMESPACE` | Namespace of the affected pod |
| `OPENFERO_POD`       | Name of the affected pod      |
| `OPENFERO_SEVERITY`  | Alert severity (warning)      |
| `OPENFERO_CLUSTER`   | Cluster name                  |

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
kubectl get operarius pod-not-ready-restart -n openfero -o yaml
```

## Safety

- **Safe for production**: Deleting a pod managed by a controller (Deployment, StatefulSet, DaemonSet, etc.) will cause it to be recreated
- **Deduplication**: 5 minute TTL prevents rapid restarts of the same pod
- **Non-destructive**: Does not affect persistent data or configuration
- **No effect on the root cause**: If the pod is unschedulable (e.g. insufficient resources) or the node is genuinely unreachable, the recreated pod may return to the same state - the alert will fire again

## Rollback

If the pod continues to be not-ready after restart, investigate the root cause:

```bash
# Check pod events (unschedulable, probe failures, etc.)
kubectl describe pod $OPENFERO_POD -n $OPENFERO_NAMESPACE

# Check node status if the pod is Unknown
kubectl get nodes
```

## Customization

Adjust the deduplication TTL to control how often the same pod can be restarted:

```yaml
spec:
  deduplication:
    enabled: true
    ttl: 600 # 10 minutes instead of 5
```
