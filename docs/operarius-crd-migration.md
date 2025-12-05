# Operarius CRD Migration Guide

## Overview

This guide covers the migration from ConfigMap-based remediation jobs to Operarius Custom Resource Definitions (CRDs) in OpenFero.

## Benefits of Operarius CRDs

### 1. Schema Validation

- **Compile-time validation** of job definitions
- **IDE support** with autocompletion
- **kubectl explain** support for built-in documentation
- **Admission webhooks** for advanced validation

### 2. API Compatibility

- **100% Kubernetes Job API compatibility** - uses embedded `JobTemplateSpec`
- **Native Kubernetes workflows** - standard kubectl, YAML, etc.
- **Declarative management** with GitOps support

### 3. Enhanced Features

- **Priority-based selection** when multiple operarii match
- **Deduplication** to prevent duplicate job execution
- **Status tracking** with conditions and execution history
- **Metrics integration** via kube-state-metrics

## Migration Steps

### Step 1: Install the CRDs

```bash
# Generate and install CRDs
make manifests
make install-crds

# Verify installation
kubectl get crd operariuses.openfero.io
```

### Step 2: Convert ConfigMaps to Operarius CRDs

#### Before (ConfigMap)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: openfero-podcrashlooping-firing
  labels:
    app: openfero
data:
  image: bitnami/kubectl:latest
  command: |
    kubectl delete pod {{ .Alert.Labels.pod }} -n {{ .Alert.Labels.namespace }}
```

#### After (Operarius CRD)

```yaml
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: podcrashlooping-operarius
  namespace: openfero
spec:
  alertSelector:
    alertname: PodCrashLooping
    status: firing

  jobTemplate:
    spec:
      backoffLimit: 2
      ttlSecondsAfterFinished: 600
      template:
        spec:
          restartPolicy: Never
          serviceAccountName: openfero-remediation
          containers:
            - name: pod-restarter
              image: bitnami/kubectl:latest
              command:
                - /bin/sh
                - -c
                - kubectl delete pod {{ .Alert.Labels.pod }} -n {{ .Alert.Labels.namespace }}

  priority: 50
  enabled: true
```

### Step 3: Enable CRD Mode

Update your OpenFero deployment to use Operarius CRDs:

```go
// In main.go or your initialization code
operariusService := services.NewOperariusService(kubeClient)

server := &handlers.Server{
    KubeClient:       kubeClient,
    AlertStore:       alertStore,
    OperariusService: operariusService,
    UseOperariusCRDs: true,  // Enable CRD mode
}
```

### Step 4: Test the Migration

1. **Apply test Operarius**:

   ```bash
   kubectl apply -f config/samples/openfero_v1alpha1_operarius_podrestart.yaml
   ```

2. **Trigger a test alert** and verify job creation

3. **Check Operarius status**:

   ```bash
   kubectl get operarius -A
   kubectl describe operarius podcrashlooping-operarius
   ```

### Step 5: Remove Old ConfigMaps

Once CRD mode is working:

```bash
# List existing ConfigMaps
kubectl get configmap -l app=openfero -A

# Remove old ConfigMaps (after verification)
kubectl delete configmap openfero-podcrashlooping-firing
```

## Template Variables

Both ConfigMap and CRD approaches support the same template variables:

### Alert Data

- `{{ .Alert.Labels.* }}` - Alert labels (e.g., `{{ .Alert.Labels.pod }}`)
- `{{ .Alert.Annotations.* }}` - Alert annotations
- `{{ .HookMessage.Status }}` - Alert status (firing/resolved)
- `{{ .HookMessage.GroupKey }}` - Alert group key
- `{{ .Labels.* }}` - Shorthand for alert labels
- `{{ .Status }}` - Shorthand for alert status

### Example Usage

```yaml
env:
  - name: POD_NAME
    value: "{{ .Alert.Labels.pod }}"
  - name: NAMESPACE
    value: "{{ .Alert.Labels.namespace }}"
  - name: ALERT_STATUS
    value: "{{ .Status }}"
```

## Advanced Features

### Priority-Based Selection

When multiple Operarii match an alert, the one with the highest priority is selected:

```yaml
spec:
  priority: 100 # Higher priority operarius
  alertSelector:
    alertname: DiskSpaceLow
    status: firing
---
spec:
  priority: 50 # Lower priority fallback
  alertSelector:
    alertname: DiskSpaceLow
    status: firing
```

### Deduplication

Prevent duplicate job execution for the same alert group:

```yaml
spec:
  deduplication:
    enabled: true
    ttl: 300 # Don't create duplicate jobs for 5 minutes
```

### Label Matching

Match alerts based on additional labels:

```yaml
spec:
  alertSelector:
    alertname: KubeQuotaAlmostFull
    status: firing
    labels:
      severity: warning
      team: platform
```

## Monitoring and Observability

### Check Operarius Status

```bash
# List all operarii
kubectl get operarius -A

# Detailed view with execution count
kubectl get operarius -o wide

# Describe specific operarius
kubectl describe operarius quota-operarius
```

### View Execution History

```bash
# Check created jobs
kubectl get jobs -l openfero.io/managed-by=openfero

# View job details
kubectl describe job <job-name>
```

### Monitor with kubectl

```bash
# Watch for new operarius resources
kubectl get operarius -w

# Monitor job creation
kubectl get jobs -l openfero.io/managed-by=openfero -w
```

## Troubleshooting

### Common Issues

1. **CRD not found**

   ```bash
   # Install CRDs
   make install-crds
   ```

2. **No matching Operarius**

   - Check alert selector criteria
   - Verify operarius is enabled
   - Check logs: `kubectl logs -l app=openfero`

3. **Job not created**

   - Check deduplication settings
   - Verify RBAC permissions
   - Check template variables are valid

4. **Template rendering errors**
   - Validate Go template syntax
   - Ensure referenced alert labels exist
   - Check logs for template errors

### Debug Commands

```bash
# Check CRD definition
kubectl explain operarius.spec

# Validate operarius YAML
kubectl --dry-run=client apply -f operarius.yaml

# Check recent events
kubectl get events --sort-by=.metadata.creationTimestamp

# View operarius conditions
kubectl get operarius -o jsonpath='{.status.conditions}'
```

## Rollback Plan

If you need to rollback to ConfigMap mode:

1. **Disable CRD mode**:

   ```go
   server.UseOperariusCRDs = false
   ```

2. **Restore ConfigMaps** from backup

3. **Keep CRDs installed** for future migration

## Best Practices

### 1. Naming Convention

- Use descriptive names: `kubequota-remediation-operarius`
- Include alert name: `podcrashlooping-operarius`
- Avoid special characters

### 2. Resource Management

```yaml
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: remediation
              resources:
                requests:
                  memory: "64Mi"
                  cpu: "50m"
                limits:
                  memory: "128Mi"
                  cpu: "100m"
```

### 3. Job Cleanup

```yaml
spec:
  jobTemplate:
    spec:
      ttlSecondsAfterFinished: 3600 # Clean up after 1 hour
      activeDeadlineSeconds: 300 # 5 minute timeout
```

### 4. Error Handling

```yaml
spec:
  jobTemplate:
    spec:
      backoffLimit: 3 # Retry up to 3 times
```

## Migration Checklist

- [ ] CRDs installed and verified
- [ ] Test Operarius created and working
- [ ] Alert routing tested
- [ ] Template variables validated
- [ ] Deduplication configured
- [ ] RBAC permissions verified
- [ ] Monitoring in place
- [ ] Documentation updated
- [ ] Team trained on new approach
- [ ] Rollback plan tested
- [ ] Old ConfigMaps removed

## Support

For issues with the migration:

1. Check the [troubleshooting section](#troubleshooting)
2. Review OpenFero logs: `kubectl logs -l app=openfero`
3. Validate your Operarius YAML: `kubectl --dry-run=client apply -f operarius.yaml`
4. Open an issue on GitHub with relevant logs and YAML
