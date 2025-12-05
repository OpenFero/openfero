# OpenFero Operarii for kube-prometheus-stack

Production-ready Operarius CRDs for common [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) alerts.

## Quick Start

1. Install OpenFero with CRD support:
   ```bash
   helm install openfero ./charts/openfero --set operarius.useOperariusCRDs=true
   ```

2. Apply all Operarii and RBAC:
   ```bash
   kubectl apply -f operarios/all-in-one/operarii-bundle.yaml
   ```

3. Wait for alerts to fire and watch remediation happen automatically.

## Available Operarii

| Alert                                                               | Priority | Action                | Safety          |
| ------------------------------------------------------------------- | -------- | --------------------- | --------------- |
| [KubePodCrashLooping](./KubePodCrashLooping/)                       | 80       | Restart pod           | Safe            |
| [KubeDeploymentReplicasMismatch](./KubeDeploymentReplicasMismatch/) | 60       | Rollout restart       | Safe            |
| [KubeJobFailed](./KubeJobFailed/)                                   | 40       | Clean up failed job   | Safe            |
| [KubeHpaMaxedOut](./KubeHpaMaxedOut/)                               | 50       | Increase max replicas | Review capacity |
| [KubeDaemonSetRolloutStuck](./KubeDaemonSetRolloutStuck/)           | 55       | Restart rollout       | Safe            |

## Environment Variables

OpenFero automatically injects all alert labels as environment variables with the `OPENFERO_` prefix.
No manual configuration needed in your Operarius specs.

### Common Variables (from kube-prometheus-stack alerts)

| Variable                           | Description           | Example                 |
| ---------------------------------- | --------------------- | ----------------------- |
| `OPENFERO_ALERTNAME`               | Name of the alert     | `KubePodCrashLooping`   |
| `OPENFERO_NAMESPACE`               | Kubernetes namespace  | `default`               |
| `OPENFERO_POD`                     | Pod name (pod alerts) | `my-app-7d4b8c9f-x2k4j` |
| `OPENFERO_CONTAINER`               | Container name        | `main`                  |
| `OPENFERO_DEPLOYMENT`              | Deployment name       | `my-app`                |
| `OPENFERO_DAEMONSET`               | DaemonSet name        | `node-exporter`         |
| `OPENFERO_JOB_NAME`                | Job name              | `backup-job`            |
| `OPENFERO_HORIZONTALPODAUTOSCALER` | HPA name              | `my-app-hpa`            |
| `OPENFERO_SEVERITY`                | Alert severity        | `warning`               |
| `OPENFERO_CLUSTER`                 | Cluster name          | `prod-us-east-1`        |

## Testing

Each Operarius includes a `test-alert.json` for manual testing:

```bash
# Port-forward to OpenFero
kubectl port-forward svc/openfero 8080:8080 -n openfero &

# Send test alert
curl -X POST http://localhost:8080/alerts \
  -H "Content-Type: application/json" \
  -d @operarios/kube-prometheus-stack/KubePodCrashLooping/test-alert.json

# Watch job creation
kubectl get jobs -n openfero -w
```

## Prerequisites

- OpenFero installed with `--useOperariusCRDs=true`
- kube-prometheus-stack installed (for real alerts)
- RBAC permissions applied (included in each Operarius folder)

## Safety Considerations

All Operarii are designed to be safe for production use:

- **Deduplication**: Prevents duplicate remediation within TTL window
- **Priority**: Higher priority Operarii take precedence
- **TTL Cleanup**: Jobs are automatically cleaned up after completion
- **Minimal RBAC**: Each Operarius has only the permissions it needs

Review the `KubeHpaMaxedOut` Operarius carefully before enabling - it modifies HPA max replicas which may have cost implications.

## Customization

Each Operarius can be customized:

1. **Adjust priority**: Change `spec.priority` to control selection order
2. **Modify deduplication TTL**: Adjust `spec.deduplication.ttl` in seconds
3. **Disable temporarily**: Set `spec.enabled: false`
4. **Customize remediation**: Modify the shell script in `spec.jobTemplate`

## Troubleshooting

### Job Not Created

1. Check Operarius is enabled:
   ```bash
   kubectl get operarius -n openfero
   ```

2. Verify alert matches selector:
   ```bash
   kubectl describe operarius <name> -n openfero
   ```

3. Check OpenFero logs:
   ```bash
   kubectl logs -l app=openfero -n openfero
   ```

### Remediation Failed

1. Check job logs:
   ```bash
   kubectl logs job/<job-name> -n openfero
   ```

2. Verify RBAC permissions:
   ```bash
   kubectl auth can-i delete pods --as=system:serviceaccount:openfero:openfero-pod-restarter -n <target-namespace>
   ```

## Additional Resources

- [Operarius CRD Documentation](../../docs/operarius-crds.md)
- [Development Guide](../../docs/operarius-development-guide.md)
- [kube-prometheus-stack Alerts](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack/templates/prometheus/rules-1.14)
