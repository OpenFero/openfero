# KubeDeploymentReplicasMismatch Operarius

Automatically triggers a rollout restart when a deployment's replicas don't match the expected count.

## Alert Definition

Source: [kube-prometheus-stack kubernetes-apps.yaml](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/templates/prometheus/rules-1.14/kubernetes-apps.yaml)

```yaml
- alert: KubeDeploymentReplicasMismatch
  annotations:
    description: Deployment {{ $labels.namespace }}/{{ $labels.deployment }} has not matched the expected number of replicas for longer than 15 minutes.
    summary: Deployment has not matched the expected number of replicas.
  expr: |
    (
      kube_deployment_spec_replicas > kube_deployment_status_replicas_available
    ) and (
      changes(kube_deployment_status_replicas_updated[10m]) == 0
    )
  for: 15m
  labels:
    severity: warning
```

## What This Does

When triggered, this Operarius:

1. Logs the deployment status (desired vs available replicas)
2. Checks for pods stuck in Pending or other non-Running states
3. Triggers a rollout restart to unstick the deployment
4. Waits for the rollout to complete (with timeout)

## Available Environment Variables

OpenFero automatically provides these from the alert labels:

| Variable              | Description                 |
| --------------------- | --------------------------- |
| `OPENFERO_NAMESPACE`  | Namespace of the deployment |
| `OPENFERO_DEPLOYMENT` | Name of the deployment      |
| `OPENFERO_SEVERITY`   | Alert severity (warning)    |
| `OPENFERO_CLUSTER`    | Cluster name                |

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
kubectl get operarius deployment-replicas-fix -n openfero -o yaml
```

## Safety

- **Safe for production**: Rollout restart is a standard Kubernetes operation
- **Graceful**: Uses rolling update strategy respecting PodDisruptionBudgets
- **Timeout**: 5 minute timeout prevents hanging indefinitely

## Rollback

If the deployment still has issues after rollout restart:

```bash
# Check deployment events
kubectl describe deployment $OPENFERO_DEPLOYMENT -n $OPENFERO_NAMESPACE

# Check for resource constraints
kubectl get events -n $OPENFERO_NAMESPACE --sort-by='.lastTimestamp'

# Manual rollback if needed
kubectl rollout undo deployment/$OPENFERO_DEPLOYMENT -n $OPENFERO_NAMESPACE
```
