# Migration Guide: ConfigMaps to Operarius CRDs

## Overview

Starting with v0.18.0, OpenFero exclusively uses **Operarius CRDs** for defining remediation jobs. The legacy ConfigMap-based approach has been removed.

If you are already using Operarius CRDs (the default since v0.17.0), no action is required.

## Migration Steps

### 1. Identify Existing ConfigMaps

List any OpenFero-related ConfigMaps in your cluster:

```bash
kubectl get configmap -l app=openfero -A
```

Or use the Makefile helper:

```bash
make migration-check
```

### 2. Create Equivalent Operarius CRDs

For each ConfigMap, create a corresponding Operarius CRD. The naming convention is:

```text
openfero-<alertname>-<status>
```

**Example ConfigMap** (legacy):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: openfero-KubeQuotaAlmostReached-firing
  labels:
    app: openfero
data:
  image: bitnami/kubectl:latest
  command: "kubectl get quota -A"
```

**Equivalent Operarius CRD**:

```yaml
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: openfero-KubeQuotaAlmostReached-firing
  namespace: openfero
spec:
  alertSelector:
    alertName: KubeQuotaAlmostReached
    status: firing
  enabled: true
  jobTemplate:
    spec:
      backoffLimit: 0
      template:
        spec:
          restartPolicy: Never
          containers:
            - name: remediation
              image: bitnami/kubectl:latest
              command: ["kubectl", "get", "quota", "-A"]
```

### 3. Apply CRDs and Remove ConfigMaps

```bash
# Apply the new Operarius CRD
kubectl apply -f operarius.yaml

# Verify it was created
kubectl get operarius

# Remove the old ConfigMap
kubectl delete configmap openfero-KubeQuotaAlmostReached-firing
```

### 4. Verify

After migration, confirm that:

- `kubectl get operarius` lists all your remediation rules
- `make migration-check` reports no remaining OpenFero ConfigMaps
- Alerts trigger remediation jobs as expected

## Key Differences

| Feature | ConfigMap (removed) | Operarius CRD |
|---|---|---|
| Schema validation | None | Full CRD validation |
| Alert matching | Name convention only | `alertSelector` with labels |
| Deduplication | Not supported | Built-in TTL-based dedup |
| Priority | Not supported | `spec.priority` field |
| Enable/disable | Delete ConfigMap | `spec.enabled: false` |
| Template variables | Not supported | Go templates in job spec |
| Status tracking | None | `.status` subresource |

## Further Reading

- [Operarius CRD Reference](operarius-crds.md)
- [Operarius Development Guide](operarius-development-guide.md)
- Example CRDs in [docs/samples/](samples/)
