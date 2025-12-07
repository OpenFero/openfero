# OpenFero Operarius CRDs

This document provides comprehensive information about OpenFero's Operarius Custom Resource Definitions (CRDs) for automated alert remediation.

## Table of Contents

- [Overview](#overview)
- [Getting Started](#getting-started)
- [API Reference](#api-reference)
- [Examples](#examples)
- [Development Guide](#development-guide)

## Overview

Operarius CRDs provide a Kubernetes-native way to define automated remediation actions for Prometheus alerts in OpenFero. They are the standard way to define remediation jobs, offering proper schema validation, enhanced features, and better integration with Kubernetes tooling.

### Key Benefits

- **Schema Validation**: Compile-time validation with IDE support
- **Kubernetes API Compatibility**: 100% compatible with Kubernetes Job specifications
- **Priority-based Selection**: Handle multiple matching operarii intelligently
- **Deduplication**: Prevent duplicate job execution
- **Status Tracking**: Monitor execution history and conditions
- **GitOps Ready**: Declarative management with version control

## Getting Started

### Prerequisites

- Kubernetes cluster with admin access
- OpenFero deployed and running
- kubectl configured

### Installation

1. **Install the CRDs**:

   ```bash
   cd /path/to/openfero
   make install-crds
   ```

2. **Verify installation**:

   ```bash
   kubectl get crd operariuses.openfero.io
   kubectl explain operarius
   ```

3. **Create your first operarius**:

   ```bash
   kubectl apply -f config/samples/openfero_v1alpha1_operarius_podrestart.yaml
   ```

### Basic Example

Here's a simple operarius that restarts crashlooping pods:

```yaml
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: pod-restart-operarius
  namespace: openfero
spec:
  alertSelector:
    alertname: PodCrashLooping
    status: firing

  jobTemplate:
    spec:
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
```

## API Reference

### Operarius

| Field                | Type                   | Description                      | Required |
| -------------------- | ---------------------- | -------------------------------- | -------- |
| `spec.jobTemplate`   | `JobTemplateSpec`      | Kubernetes job template          | Yes      |
| `spec.alertSelector` | `AlertSelector`        | Alert matching criteria          | Yes      |
| `spec.priority`      | `int32`                | Selection priority (higher wins) | No       |
| `spec.enabled`       | `*bool`                | Enable/disable operarius         | No       |
| `spec.deduplication` | `*DeduplicationConfig` | Deduplication settings           | No       |

### AlertSelector

| Field       | Type                | Description                          | Required |
| ----------- | ------------------- | ------------------------------------ | -------- |
| `alertname` | `string`            | Alert name to match                  | Yes      |
| `status`    | `string`            | Alert status: "firing" or "resolved" | No       |
| `labels`    | `map[string]string` | Additional label matching            | No       |

### DeduplicationConfig

| Field     | Type    | Description                  | Required |
| --------- | ------- | ---------------------------- | -------- |
| `enabled` | `bool`  | Enable deduplication         | No       |
| `ttl`     | `int32` | Deduplication TTL in seconds | No       |

### Template Variables

Available in job templates:

- `{{ .Alert.Labels.* }}` - Alert labels (e.g., `{{ .Alert.Labels.pod }}`)
- `{{ .Alert.Annotations.* }}` - Alert annotations
- `{{ .HookMessage.Status }}` - Alert status ("firing" or "resolved")
- `{{ .HookMessage.GroupKey }}` - Alert group key
- `{{ .Labels.* }}` - Shorthand for alert labels
- `{{ .Status }}` - Shorthand for alert status

## Examples

### Pod Restart Operarius

Restart crashlooping pods automatically:

```yaml
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: pod-restart-operarius
  namespace: openfero
spec:
  alertSelector:
    alertname: PodCrashLooping
    status: firing
    labels:
      severity: warning

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
                - |
                  echo "Restarting pod {{ .Alert.Labels.pod }} in namespace {{ .Alert.Labels.namespace }}"
                  kubectl delete pod {{ .Alert.Labels.pod }} -n {{ .Alert.Labels.namespace }}
              env:
                - name: ALERT_NAME
                  value: "{{ .Alert.Labels.alertname }}"
                - name: POD_NAME
                  value: "{{ .Alert.Labels.pod }}"
                - name: NAMESPACE
                  value: "{{ .Alert.Labels.namespace }}"

  priority: 50
  enabled: true
  deduplication:
    enabled: true
    ttl: 300
```

### Quota Management Operarius

Scale down deployments when quota is exceeded:

```yaml
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: quota-scale-down-operarius
  namespace: openfero
spec:
  alertSelector:
    alertname: KubeQuotaAlmostFull
    status: firing
    labels:
      resource: "requests.memory"

  jobTemplate:
    spec:
      backoffLimit: 1
      ttlSecondsAfterFinished: 3600
      template:
        spec:
          restartPolicy: Never
          serviceAccountName: openfero-quota-manager
          containers:
            - name: quota-manager
              image: bitnami/kubectl:latest
              command:
                - /bin/sh
                - -c
                - |
                  NAMESPACE="{{ .Alert.Labels.namespace }}"
                  echo "Scaling down non-essential deployments in namespace $NAMESPACE"
                  kubectl scale deployment --replicas=0 -l priority=low -n $NAMESPACE
              resources:
                requests:
                  memory: "64Mi"
                  cpu: "50m"
                limits:
                  memory: "128Mi"
                  cpu: "100m"

  priority: 100
  enabled: true
```

### Disk Cleanup Operarius

Clean up disk space when usage is high:

```yaml
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: disk-cleanup-operarius
  namespace: openfero
spec:
  alertSelector:
    alertname: NodeDiskSpaceFillingUp
    status: firing
    labels:
      severity: warning

  jobTemplate:
    spec:
      backoffLimit: 3
      activeDeadlineSeconds: 300
      ttlSecondsAfterFinished: 1800
      template:
        spec:
          restartPolicy: Never
          nodeSelector:
            kubernetes.io/hostname: "{{ .Alert.Labels.instance }}"
          hostPID: true
          hostNetwork: true
          containers:
            - name: disk-cleaner
              image: alpine:latest
              securityContext:
                privileged: true
              command:
                - /bin/sh
                - -c
                - |
                  echo "Cleaning up disk space on node {{ .Alert.Labels.instance }}"
                  # Clean Docker images
                  docker system prune -f
                  # Clean logs older than 7 days
                  find /var/log -name "*.log" -mtime +7 -delete
                  # Clean temp files
                  find /tmp -type f -mtime +1 -delete
              volumeMounts:
                - name: docker-sock
                  mountPath: /var/run/docker.sock
                - name: host-logs
                  mountPath: /var/log
                - name: host-tmp
                  mountPath: /tmp
          volumes:
            - name: docker-sock
              hostPath:
                path: /var/run/docker.sock
            - name: host-logs
              hostPath:
                path: /var/log
            - name: host-tmp
              hostPath:
                path: /tmp

  priority: 75
  enabled: true
  deduplication:
    enabled: true
    ttl: 1800 # 30 minutes
```

### Multi-Priority Example

Handle the same alert with different priorities:

```yaml
# High priority - immediate action for critical alerts
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: critical-pod-restart-operarius
  namespace: openfero
spec:
  alertSelector:
    alertname: PodCrashLooping
    status: firing
    labels:
      severity: critical
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: Never
          priorityClassName: system-cluster-critical
          containers:
            - name: immediate-restart
              image: bitnami/kubectl:latest
              command: ["/bin/sh", "-c"]
              args:
                - kubectl delete pod {{ .Alert.Labels.pod }} -n {{ .Alert.Labels.namespace }} --force --grace-period=0
  priority: 100
  enabled: true

---
# Lower priority - graceful restart for warning alerts
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: warning-pod-restart-operarius
  namespace: openfero
spec:
  alertSelector:
    alertname: PodCrashLooping
    status: firing
    labels:
      severity: warning
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: Never
          containers:
            - name: graceful-restart
              image: bitnami/kubectl:latest
              command: ["/bin/sh", "-c"]
              args:
                - kubectl delete pod {{ .Alert.Labels.pod }} -n {{ .Alert.Labels.namespace }}
  priority: 50
  enabled: true
```

| Deduplication     | Manual    | Automatic                  |
| Status tracking   | None      | Comprehensive status       |
| kubectl explain   | No        | Yes                        |
| GitOps friendly   | Basic     | Advanced                   |

## Best Practices

### Resource Management

Always specify resource limits:

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

### Job Cleanup

Configure automatic cleanup:

```yaml
spec:
  jobTemplate:
    spec:
      ttlSecondsAfterFinished: 3600 # Clean up after 1 hour
      activeDeadlineSeconds: 300 # 5 minute timeout
      backoffLimit: 3 # Retry up to 3 times
```

### Security

Use dedicated service accounts:

```yaml
spec:
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: openfero-remediation
          securityContext:
            runAsNonRoot: true
            runAsUser: 1000
```

### Error Handling

Include error handling in scripts:

```bash
#!/bin/bash
set -euo pipefail

POD_NAME="{{ .Alert.Labels.pod }}"
NAMESPACE="{{ .Alert.Labels.namespace }}"

if [[ -z "$POD_NAME" || -z "$NAMESPACE" ]]; then
    echo "Error: Missing required alert labels"
    exit 1
fi

echo "Restarting pod $POD_NAME in namespace $NAMESPACE"
kubectl delete pod "$POD_NAME" -n "$NAMESPACE" || {
    echo "Failed to delete pod"
    exit 1
}
```

## Troubleshooting

### Common Issues

1. **No matching operarius found**:

   - Check alert selector criteria
   - Verify operarius is enabled
   - Check logs: `kubectl logs -l app=openfero`

2. **Template rendering errors**:

   - Validate Go template syntax
   - Ensure referenced alert labels exist
   - Test with `kubectl --dry-run=client`

3. **RBAC permission denied**:
   - Check service account permissions
   - Verify RBAC configuration
   - Test with: `kubectl auth can-i create jobs --as=system:serviceaccount:openfero:default`

### Debug Commands

```bash
# List all operarii
kubectl get operarius -A

# Describe operarius with details
kubectl describe operarius <name>

# Check generated CRD schema
kubectl explain operarius.spec.jobTemplate.spec

# Validate operarius YAML
kubectl --dry-run=client apply -f operarius.yaml

# Monitor job creation
kubectl get jobs -l openfero.io/managed-by=openfero -w

# Check recent events
kubectl get events --sort-by=.metadata.creationTimestamp
```

## Next Steps

- Read the [Migration Guide](operarius-crd-migration.md) for detailed migration instructions
- Check the [Development Guide](operarius-development-guide.md) for contributing to operarius CRDs
- Explore more examples in the `config/samples/` directory
- Join the OpenFero community for support and discussions
