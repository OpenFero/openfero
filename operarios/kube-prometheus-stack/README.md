# OpenFero Operarii for kube-prometheus-stack

Operarius CRD **templates** for common [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) alerts - a starting point to adapt, not ready-to-run production jobs.

## ⚠️ These Are Templates, Not "Install and Forget"

An alert name alone doesn't tell you whether "restart the pod" / "increase max replicas" / "delete the job" is the *right* reaction for a given workload - that depends on what the workload actually is and who owns it. For that reason:

- Every Operarius here ships with **`enabled: false`** by default. Nothing runs until you consciously flip it to `true`.
- Every `alertSelector` includes a commented-out example for scoping the Operarius to specific workloads (e.g. by `namespace`) instead of matching every occurrence of that alert cluster-wide.
- Treat the bundled `jobTemplate` shell script as a starting point to review and adjust for your own environment, not a black box to trust blindly.

See [Customization](#customization) below for how to scope a selector, including a caveat about which labels are actually available to match on.

## Quick Start

1. Install OpenFero with CRD support:
   ```bash
   helm install openfero ./charts/openfero --set operarius.useOperariusCRDs=true
   ```

2. Apply the Operarii and RBAC you want (they come disabled):
   ```bash
   kubectl apply -f operarios/all-in-one/operarii-bundle.yaml
   ```

3. For each Operarius you actually want active: review its remediation script, scope its `alertSelector` (see [Customization](#customization)), then set `spec.enabled: true`.

4. Wait for alerts to fire and watch remediation happen automatically.

## Available Operarii

| Alert                                                               | Priority | Action                | Safety          |
| ------------------------------------------------------------------- | -------- | --------------------- | --------------- |
| [KubePodCrashLooping](./KubePodCrashLooping/)                       | 80       | Restart pod           | Safe            |
| [KubePodNotReady](./KubePodNotReady/)                               | 75       | Restart pod           | Safe            |
| [KubeContainerWaiting](./KubeContainerWaiting/)                     | 65       | Restart pod           | Safe            |
| [KubeDeploymentReplicasMismatch](./KubeDeploymentReplicasMismatch/) | 60       | Rollout restart       | Safe            |
| [KubeDaemonSetRolloutStuck](./KubeDaemonSetRolloutStuck/)           | 55       | Restart rollout       | Safe            |
| [KubeHpaMaxedOut](./KubeHpaMaxedOut/)                               | 50       | Increase max replicas | Review capacity |
| [KubeJobFailed](./KubeJobFailed/)                                   | 40       | Clean up failed job   | Safe            |
| [KubeJobNotCompleted](./KubeJobNotCompleted/)                       | 35       | Clean up stuck job    | Review before enabling |

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
| `OPENFERO_REASON`                  | Waiting reason        | `ImagePullBackOff`      |
| `OPENFERO_SEVERITY`                | Alert severity        | `warning`               |
| `OPENFERO_CLUSTER`                 | Cluster name          | `prod-us-east-1`        |

## Testing

Each Operarius includes a `test-alert.json` for manual testing. Remember the Operarius must be scoped and `spec.enabled: true` first (see [Quick Start](#quick-start)) - a disabled Operarius will not react to this:

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

## Testing with Real kube-prometheus-stack Alerts

Sending a `test-alert.json` payload directly to `/alerts` (above) verifies OpenFero's matching and job-creation logic, but doesn't exercise real Prometheus rule evaluation or Alertmanager routing. To test against a real kube-prometheus-stack with alerts actually firing:

```bash
make test-e2e-kube-prometheus-stack
```

This spins up a real kube-prometheus-stack (Prometheus Operator + Alertmanager) in the E2E Kind cluster, provokes each alert's real underlying condition (e.g. a pod with an invalid image tag for `KubeContainerWaiting`), and verifies the matching remediation Job is created and the condition is resolved. See `test/e2e/kubeprometheusstack_test.go`.

Since most of these alerts default to `for:` durations of 15m-1h, the test suite overrides them via kube-prometheus-stack's `customRules.<AlertName>.for` Helm value (see `test/e2e/testdata/kube-prometheus-stack-values.yaml`) so the real alert still fires off the real expression, just faster. Useful if you're adding a 9th Operarius and want to verify it end-to-end without waiting hours for a real Prometheus rule to fire.

## Prerequisites

- OpenFero installed with `--useOperariusCRDs=true`
- kube-prometheus-stack installed (for real alerts)
- RBAC permissions applied (included in each Operarius folder)

## Safety Considerations

The remediation actions themselves follow safe, non-destructive patterns (restarting controller-managed pods, deleting jobs, etc.), and:

- **Disabled by default**: nothing runs until you review and enable an Operarius
- **Deduplication**: Prevents duplicate remediation within TTL window
- **Priority**: Higher priority Operarii take precedence
- **TTL Cleanup**: Jobs are automatically cleaned up after completion
- **Minimal RBAC**: Each Operarius has only the permissions it needs

That said, "safe action" isn't the same as "correct reaction for your workload" - see the [warning above](#️-these-are-templates-not-install-and-forget). Review `KubeHpaMaxedOut` and `KubeJobNotCompleted` especially carefully: the former can mask a real bug as "needs more capacity" and has cost implications, the latter deletes a still-active job and its running pods.

## Customization

Each Operarius can be customized:

1. **Scope the selector (do this first)**: Uncomment and adjust the `labels` example under `alertSelector` to restrict the Operarius to specific workloads instead of matching every occurrence of the alert cluster-wide, e.g.:
   ```yaml
   alertSelector:
     alertname: KubePodCrashLooping
     status: firing
     labels:
       namespace: my-app-namespace
   ```
   Any label already present on the incoming alert works here (`namespace`, `severity`, etc.). Arbitrary Kubernetes object labels (e.g. a custom `team: platform` label on your Deployment) are **not** automatically available to match on - kube-state-metrics only exposes labels it's explicitly configured to allow (`--metric-labels-allowlist`), and the alerting rule itself would need to join on them. `namespace`-based scoping is the practical option that works out of the box with the stock alerts.
2. **Enable**: Once scoped and reviewed, set `spec.enabled: true`
3. **Adjust priority**: Change `spec.priority` to control selection order
4. **Modify deduplication TTL**: Adjust `spec.deduplication.ttl` in seconds
5. **Customize remediation**: Modify the shell script in `spec.jobTemplate`

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
