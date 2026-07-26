# KubeContainerWaiting Operarius

Automatically restarts pods with a container stuck in a waiting state (e.g. `ImagePullBackOff`, `ErrImagePull`, `InvalidImageName`) for an extended period.

## Alert Definition

Source: [kube-prometheus-stack kubernetes-apps.yaml](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/templates/prometheus/rules-1.14/kubernetes-apps.yaml)

```yaml
- alert: KubeContainerWaiting
  annotations:
    description: 'pod/{{ $labels.pod }} in namespace {{ $labels.namespace }} on container {{ $labels.container}} has been in waiting state for longer than 1 hour. (reason: "{{ $labels.reason }}") on cluster {{ $labels.cluster }}.'
    summary: Pod container waiting longer than 1 hour
  expr: kube_pod_container_status_waiting_reason{reason!="CrashLoopBackOff", job="kube-state-metrics"} > 0
  for: 1h
  labels:
    severity: warning
```

Note: `KubePodCrashLooping` already covers the `CrashLoopBackOff` reason, so this alert (and this Operarius) targets the remaining waiting reasons - most commonly `ImagePullBackOff`, `ErrImagePull`, `CreateContainerConfigError`, and `InvalidImageName`.

## What This Does

When triggered, this Operarius:

1. Logs the waiting pod/container details and the waiting reason
2. Deletes the pod to force a fresh reschedule (and image pull retry)
3. Reports success or failure

This is useful when a transient registry outage or a since-fixed image reference left a pod stuck waiting.

## Available Environment Variables

OpenFero automatically provides these from the alert labels:

| Variable             | Description                       |
| -------------------- | ---------------------------------- |
| `OPENFERO_NAMESPACE` | Namespace of the affected pod      |
| `OPENFERO_POD`       | Name of the affected pod           |
| `OPENFERO_CONTAINER` | Container stuck in waiting state   |
| `OPENFERO_REASON`    | Waiting reason (e.g. `ImagePullBackOff`) |
| `OPENFERO_SEVERITY`  | Alert severity (warning)           |
| `OPENFERO_CLUSTER`   | Cluster name                       |

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
kubectl get operarius container-waiting-restart -n openfero -o yaml
```

## Safety

- **Safe for production**: Deleting a pod managed by a controller (Deployment, StatefulSet, etc.) will cause it to be recreated
- **Deduplication**: 5 minute TTL prevents rapid restarts of the same pod
- **Non-destructive**: Does not affect persistent data or configuration
- **No effect on a genuinely bad image**: If the image reference is permanently invalid, the pod will simply return to the same waiting state after being recreated - the alert will fire again

## Rollback

If the pod continues waiting after restart, investigate the root cause:

```bash
# Check pod events
kubectl describe pod $OPENFERO_POD -n $OPENFERO_NAMESPACE

# Check the image reference actually being requested
kubectl get pod $OPENFERO_POD -n $OPENFERO_NAMESPACE -o jsonpath='{.spec.containers[*].image}'
```

## Customization

Adjust the deduplication TTL to control how often the same pod can be restarted:

```yaml
spec:
  deduplication:
    enabled: true
    ttl: 600 # 10 minutes instead of 5
```
