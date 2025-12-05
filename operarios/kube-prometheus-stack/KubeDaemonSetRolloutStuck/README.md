# KubeDaemonSetRolloutStuck Operarius

Automatically fixes stuck DaemonSet rollouts by restarting the rollout.

## Alert Definition

Source: [kube-prometheus-stack kubernetes-apps.yaml](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/templates/prometheus/rules-1.14/kubernetes-apps.yaml)

```yaml
- alert: KubeDaemonSetRolloutStuck
  annotations:
    description: DaemonSet {{ $labels.namespace }}/{{ $labels.daemonset }} has not finished or progressed for at least 15m.
    summary: DaemonSet rollout is stuck.
  expr: |
    (
      kube_daemonset_status_current_number_scheduled != kube_daemonset_status_desired_number_scheduled
    ) or (
      kube_daemonset_status_number_misscheduled != 0
    ) or (
      kube_daemonset_status_updated_number_scheduled != kube_daemonset_status_desired_number_scheduled
    ) or (
      kube_daemonset_status_number_available != kube_daemonset_status_current_number_scheduled
    )
  for: 15m
  labels:
    severity: warning
```

## What This Does

When triggered, this Operarius:

1. Logs the DaemonSet status
2. Identifies non-running pods
3. Deletes stuck pods to force recreation
4. Triggers a rollout restart
5. Waits for rollout completion

## Available Environment Variables

OpenFero automatically provides these from the alert labels:

| Variable | Description |
|----------|-------------|
| `OPENFERO_NAMESPACE` | Namespace of the DaemonSet |
| `OPENFERO_DAEMONSET` | Name of the DaemonSet |
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
kubectl get operarius daemonset-rollout-fix -n openfero -o yaml
```

## Safety

- **Safe for production**: Rollout restart respects update strategy
- **Graceful**: Pods are deleted according to DaemonSet's maxUnavailable setting
- **Timeout**: 5 minute timeout prevents hanging indefinitely

## Rollback

If the DaemonSet is still stuck after restart:

```bash
# Check DaemonSet events
kubectl describe daemonset $OPENFERO_DAEMONSET -n $OPENFERO_NAMESPACE

# Check node issues
kubectl get nodes -o wide

# Manual rollback if needed
kubectl rollout undo daemonset/$OPENFERO_DAEMONSET -n $OPENFERO_NAMESPACE
```
