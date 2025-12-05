# KubePodCrashLooping Operarius

Automatically restarts pods stuck in CrashLoopBackOff state.

## Alert Definition

Source: [kube-prometheus-stack kubernetes-apps.yaml](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/templates/prometheus/rules-1.14/kubernetes-apps.yaml)

```yaml
- alert: KubePodCrashLooping
  annotations:
    description: 'Pod {{ $labels.namespace }}/{{ $labels.pod }} ({{ $labels.container }}) is in waiting state (reason: "CrashLoopBackOff").'
    summary: Pod is crash looping.
  expr: max_over_time(kube_pod_container_status_waiting_reason{reason="CrashLoopBackOff"}[5m]) >= 1
  for: 15m
  labels:
    severity: warning
```

## What This Does

When triggered, this Operarius:

1. Logs the CrashLooping pod details
2. Deletes the pod to trigger a fresh restart by the controller
3. Reports success or failure

This is useful when a pod is stuck in CrashLoopBackOff due to transient issues that may resolve with a fresh start.

## Available Environment Variables

OpenFero automatically provides these from the alert labels:

| Variable             | Description                     |
| -------------------- | ------------------------------- |
| `OPENFERO_NAMESPACE` | Namespace of the crashing pod   |
| `OPENFERO_POD`       | Name of the crashing pod        |
| `OPENFERO_CONTAINER` | Container that is crash looping |
| `OPENFERO_SEVERITY`  | Alert severity (warning)        |
| `OPENFERO_CLUSTER`   | Cluster name                    |

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
kubectl get operarius pod-crashloop-restart -n openfero -o yaml
```

## Safety

- **Safe for production**: Deleting a pod managed by a controller (Deployment, StatefulSet, etc.) will cause it to be recreated
- **Deduplication**: 5 minute TTL prevents rapid restarts of the same pod
- **Non-destructive**: Does not affect persistent data or configuration

## Rollback

If the pod continues to crash after restart, investigate the root cause:

```bash
# Check pod events
kubectl describe pod $OPENFERO_POD -n $OPENFERO_NAMESPACE

# Check previous container logs
kubectl logs $OPENFERO_POD -n $OPENFERO_NAMESPACE --previous
```

## Customization

Adjust the deduplication TTL to control how often the same pod can be restarted:

```yaml
spec:
  deduplication:
    enabled: true
    ttl: 600  # 10 minutes instead of 5
```
