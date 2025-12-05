# KubeHpaMaxedOut Operarius

Automatically increases HPA max replicas when the autoscaler is maxed out.

## Alert Definition

Source: [kube-prometheus-stack kubernetes-apps.yaml](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/templates/prometheus/rules-1.14/kubernetes-apps.yaml)

```yaml
- alert: KubeHpaMaxedOut
  annotations:
    description: HPA {{ $labels.namespace }}/{{ $labels.horizontalpodautoscaler }} has been running at max replicas for longer than 15 minutes.
    summary: HPA is running at max replicas.
  expr: |
    kube_horizontalpodautoscaler_status_current_replicas
      ==
    kube_horizontalpodautoscaler_spec_max_replicas
  for: 15m
  labels:
    severity: warning
```

## What This Does

When triggered, this Operarius:

1. Logs the current HPA status
2. Calculates a new max replicas value (current + 50%, rounded up)
3. Patches the HPA with the increased max replicas
4. Reports the change

## Available Environment Variables

OpenFero automatically provides these from the alert labels:

| Variable | Description |
|----------|-------------|
| `OPENFERO_NAMESPACE` | Namespace of the HPA |
| `OPENFERO_HORIZONTALPODAUTOSCALER` | Name of the HPA |
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
kubectl get operarius hpa-scale-increase -n openfero -o yaml
```

## Safety Considerations

**Review before enabling in production!**

This Operarius automatically increases capacity which may have:

- **Cost implications**: More replicas = more resource usage = higher costs
- **Resource constraints**: Cluster may not have capacity for more pods
- **Runaway scaling**: Consider setting a hard cap in the HPA or this script

## Customization

Adjust the scale factor by modifying the SCALE_FACTOR in the script:

```yaml
# Current: 50% increase
NEW_MAX=$(( CURRENT_MAX + CURRENT_MAX / 2 ))

# Alternative: Fixed increment
NEW_MAX=$(( CURRENT_MAX + 5 ))
```

## Rollback

To revert to original max replicas:

```bash
kubectl patch hpa $OPENFERO_HORIZONTALPODAUTOSCALER -n $OPENFERO_NAMESPACE \
  --type='json' -p='[{"op": "replace", "path": "/spec/maxReplicas", "value": <original-value>}]'
```

## Recommendations

1. Set up cluster autoscaling to ensure capacity
2. Configure resource quotas to prevent runaway costs
3. Consider setting a maximum ceiling in the script
4. Review HPA scaling metrics after enabling
