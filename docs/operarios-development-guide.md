# Operarios Development Guide

This guide defines requirements and best practices for developing **Operarios** (worker images) that integrate with OpenFero.

## What is an Operarios?

An **Operarios** (Latin for "worker") is a container image designed to execute remediation tasks in response to Prometheus/Alertmanager alerts. OpenFero creates Kubernetes Jobs using these images when alerts are triggered.

## Table of Contents

- [Quick Start](#quick-start)
- [Operarius CRD Requirements](#operarius-crd-requirements)
- [Environment Variables](#environment-variables)
- [Exit Codes](#exit-codes)
- [Best Practices](#best-practices)
- [Example Implementations](#example-implementations)
- [Testing Your Operarios](#testing-your-operarios)

## Quick Start

### Minimal Example

```yaml
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: disk-cleanup
  namespace: openfero
spec:
  alertSelector:
    alertname: DiskSpaceLow
    status: firing
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: cleanup
            image: your-registry/disk-cleanup:v1.0.0
            env:
            - name: DRY_RUN
              value: "false"
          restartPolicy: Never
```

## Operarius CRD Requirements

### Naming Convention

While Operarius CRDs can have any name, it is recommended to follow a descriptive pattern:

```text
<alertname>-<status>
```

**Examples:**

- `diskspacelow-firing`
- `kubequotaalmostfull-resolved`
- `podcrashlooping-firing`

### Alert Selector

The `alertSelector` field is **mandatory** and determines which alerts trigger this Operarius:

```yaml
spec:
  alertSelector:
    alertname: DiskSpaceLow  # Must match Prometheus alert name exactly
    status: firing           # Either 'firing' or 'resolved'
    labels:                  # Optional: Match specific alert labels
      severity: critical
```

### Job Template

The `jobTemplate` field embeds a standard Kubernetes `JobTemplateSpec`. This means you can use any feature available in standard Kubernetes Jobs:

- `initContainers`
- `volumes` / `volumeMounts`
- `serviceAccountName`
- `securityContext`
- `resources` (requests/limits)
- `affinity` / `tolerations`

```yaml
spec:
  jobTemplate:
    spec:
      ttlSecondsAfterFinished: 3600  # Auto-cleanup
      template:
        spec:
          serviceAccountName: remediation-sa
          containers:
          - name: worker
            image: your-image:tag
          restartPolicy: Never
```

## Environment Variables

### Automatic Alert Label Injection

OpenFero **automatically injects all alert labels** as environment variables into your container. Each label is:

1. Prefixed with `OPENFERO_`
2. Converted to uppercase
3. Made available to your container

**Example Alert:**

```json
{
  "labels": {
    "alertname": "DiskSpaceLow",
    "severity": "warning",
    "instance": "node-1",
    "mountpoint": "/var/log"
  }
}
```

**Resulting Environment Variables:**

```bash
OPENFERO_ALERTNAME=DiskSpaceLow
OPENFERO_SEVERITY=warning
OPENFERO_INSTANCE=node-1
OPENFERO_MOUNTPOINT=/var/log
```

### Accessing Environment Variables

**Bash:**

```bash
#!/bin/bash
echo "Alert: ${OPENFERO_ALERTNAME}"
echo "Severity: ${OPENFERO_SEVERITY}"
echo "Instance: ${OPENFERO_INSTANCE}"
```

**Python:**

```python
import os

alertname = os.getenv('OPENFERO_ALERTNAME')
severity = os.getenv('OPENFERO_SEVERITY')
instance = os.getenv('OPENFERO_INSTANCE')

print(f"Processing alert: {alertname} on {instance}")
```

**Go:**

```go
import "os"

alertname := os.Getenv("OPENFERO_ALERTNAME")
severity := os.Getenv("OPENFERO_SEVERITY")
instance := os.Getenv("OPENFERO_INSTANCE")
```

### Custom Environment Variables

You can define additional environment variables in your Operarius CRD:

```yaml
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: worker
            image: your-image:tag
            env:
            - name: CUSTOM_SETTING
              value: "my-value"
            - name: DRY_RUN
              value: "false"
```

## Exit Codes

Exit codes are **critical** for OpenFero to determine job success and trigger cleanup mechanisms.

### Required Exit Code Behavior

| Exit Code | Meaning | Kubernetes Status | Action                                  |
| --------- | ------- | ----------------- | --------------------------------------- |
| `0`       | Success | Succeeded         | Job marked complete, TTL cleanup starts |
| `1-255`   | Failure | Failed            | Job marked failed, alerts may trigger   |

### Implementation Examples

**Bash:**

```bash
#!/bin/bash
set -e  # Exit on any error

# Your remediation logic
if cleanup_disk_space; then
    echo "Cleanup successful"
    exit 0
else
    echo "Cleanup failed"
    exit 1
fi
```

**Python:**

```python
import sys

try:
    perform_remediation()
    print("Remediation successful")
    sys.exit(0)
except Exception as e:
    print(f"Remediation failed: {e}", file=sys.stderr)
    sys.exit(1)
```

**Go:**

```go
package main

import (
    "fmt"
    "os"
)

func main() {
    if err := performRemediation(); err != nil {
        fmt.Fprintf(os.Stderr, "Remediation failed: %v\n", err)
        os.Exit(1)
    }
    fmt.Println("Remediation successful")
    os.Exit(0)
}
```

## Best Practices

### 1. Idempotency

Your Operarios should be **safe to run multiple times** without causing issues:

```python
# Good: Check current state first
def cleanup_old_logs():
    old_logs = find_logs_older_than(days=7)
    if not old_logs:
        print("No old logs found, nothing to do")
        return
    
    for log in old_logs:
        if os.path.exists(log):
            os.remove(log)
```

### 2. Logging

Use structured logging for better debugging:

```python
import json
import sys

def log(level, message, **kwargs):
    entry = {
        "level": level,
        "message": message,
        "alert": os.getenv("OPENFERO_ALERTNAME"),
        **kwargs
    }
    print(json.dumps(entry), file=sys.stderr)

log("info", "Starting remediation", instance=os.getenv("OPENFERO_INSTANCE"))
```

### 3. Timeouts

Set reasonable timeouts to prevent hung jobs:

```yaml
spec:
  jobTemplate:
    spec:
      activeDeadlineSeconds: 600  # 10 minute timeout
      template:
        spec:
          containers:
          - name: worker
            image: your-image:tag
```

### 4. Resource Limits

Always define resource requests and limits:

```yaml
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: worker
            image: your-image:tag
            resources:
              requests:
                memory: "128Mi"
                cpu: "100m"
              limits:
                memory: "256Mi"
                cpu: "500m"
```

### 5. Security

Follow security best practices:

```yaml
spec:
  jobTemplate:
    spec:
      template:
        spec:
          securityContext:
            runAsNonRoot: true
            runAsUser: 1000
            fsGroup: 1000
          containers:
          - name: worker
            image: your-image:tag
            securityContext:
              allowPrivilegeEscalation: false
              readOnlyRootFilesystem: true
              capabilities:
                drop:
                - ALL
```

### 6. Job TTL

OpenFero automatically adds a TTL (default: 300 seconds) to jobs. You can override this in your Operarius CRD:

```yaml
spec:
  jobTemplate:
    spec:
      ttlSecondsAfterFinished: 60  # Cleanup 60s after completion
```

### 7. Retry Policy

Configure appropriate retry behavior:

```yaml
spec:
  jobTemplate:
    spec:
      backoffLimit: 3  # Retry up to 3 times
      template:
        spec:
          restartPolicy: Never  # Required for Jobs
```

## Example Implementations

### Example 1: Disk Cleanup (Bash)

**Dockerfile:**

```dockerfile
FROM alpine:3.18
RUN apk add --no-cache bash findutils
COPY cleanup.sh /usr/local/bin/cleanup.sh
RUN chmod +x /usr/local/bin/cleanup.sh
USER 1000:1000
ENTRYPOINT ["/usr/local/bin/cleanup.sh"]
```

**cleanup.sh:**

```bash
#!/bin/bash
set -euo pipefail

echo "Starting disk cleanup for ${OPENFERO_INSTANCE:-unknown}"
echo "Alert: ${OPENFERO_ALERTNAME}"
echo "Mountpoint: ${OPENFERO_MOUNTPOINT:-/tmp}"

# Find and delete logs older than 7 days
OLD_LOGS=$(find "${OPENFERO_MOUNTPOINT}" -name "*.log" -mtime +7 2>/dev/null || true)

if [ -z "$OLD_LOGS" ]; then
    echo "No old logs found"
    exit 0
fi

echo "Found old logs:"
echo "$OLD_LOGS"

# Delete old logs
echo "$OLD_LOGS" | xargs -r rm -f

echo "Cleanup completed successfully"
exit 0
```

### Example 2: Scale Deployment (Python)

**Dockerfile:**

```dockerfile
FROM python:3.11-alpine
RUN pip install kubernetes
COPY scale_deployment.py /app/scale_deployment.py
USER 1000:1000
WORKDIR /app
ENTRYPOINT ["python", "scale_deployment.py"]
```

**scale_deployment.py:**

```python
#!/usr/bin/env python3
import os
import sys
from kubernetes import client, config

def scale_deployment():
    alertname = os.getenv('OPENFERO_ALERTNAME')
    deployment = os.getenv('OPENFERO_DEPLOYMENT')
    namespace = os.getenv('OPENFERO_NAMESPACE', 'default')
    
    if not deployment:
        print("ERROR: OPENFERO_DEPLOYMENT not set", file=sys.stderr)
        sys.exit(1)
    
    print(f"Scaling deployment: {deployment} in namespace: {namespace}")
    
    try:
        config.load_incluster_config()
        apps_v1 = client.AppsV1Api()
        
        # Get current deployment
        dep = apps_v1.read_namespaced_deployment(deployment, namespace)
        current_replicas = dep.spec.replicas
        
        # Scale up by 1
        new_replicas = current_replicas + 1
        dep.spec.replicas = new_replicas
        
        # Update deployment
        apps_v1.patch_namespaced_deployment(deployment, namespace, dep)
        
        print(f"Scaled {deployment} from {current_replicas} to {new_replicas}")
        sys.exit(0)
        
    except Exception as e:
        print(f"ERROR: Failed to scale deployment: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    scale_deployment()
```

### Example 3: Terraform Remediation (Go)

**Dockerfile:**

```dockerfile
FROM hashicorp/terraform:1.5
COPY remediate /usr/local/bin/remediate
RUN chmod +x /usr/local/bin/remediate
USER 1000:1000
ENTRYPOINT ["/usr/local/bin/remediate"]
```

**main.go:**

```go
package main

import (
    "fmt"
    "os"
    "os/exec"
)

func main() {
    alertname := os.Getenv("OPENFERO_ALERTNAME")
    workspace := os.Getenv("OPENFERO_WORKSPACE")
    
    fmt.Printf("Processing alert: %s\n", alertname)
    fmt.Printf("Workspace: %s\n", workspace)
    
    // Run terraform apply
    cmd := exec.Command("terraform", "apply", "-auto-approve")
    cmd.Dir = workspace
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    
    if err := cmd.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Terraform apply failed: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Println("Remediation completed successfully")
    os.Exit(0)
}
```

## Testing Your Operarios

### 1. Local Testing with Docker

Test your image locally before deploying:

```bash
# Run with sample environment variables
docker run --rm \
  -e OPENFERO_ALERTNAME=DiskSpaceLow \
  -e OPENFERO_SEVERITY=warning \
  -e OPENFERO_INSTANCE=node-1 \
  your-registry/disk-cleanup:v1.0.0

# Check exit code
echo $?  # Should be 0 for success
```

### 2. Kubernetes Test Job

Create a test job to verify functionality:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: test-operarios
spec:
  template:
    spec:
      containers:
      - name: test
        image: your-registry/disk-cleanup:v1.0.0
        env:
        - name: OPENFERO_ALERTNAME
          value: "DiskSpaceLow"
        - name: OPENFERO_SEVERITY
          value: "warning"
      restartPolicy: Never
```

```bash
kubectl apply -f test-job.yaml
kubectl logs job/test-operarios
kubectl get job test-operarios -o yaml
```

### 3. Integration Testing

Test the complete flow with OpenFero:

```bash
# Send test alert to OpenFero
curl -X POST http://openfero:8080/alerts \
  -H "Content-Type: application/json" \
  -d @test-alert.json
```

**test-alert.json:**

```json
{
  "version": "4",
  "groupKey": "test-group",
  "status": "firing",
  "receiver": "openfero",
  "alerts": [{
    "status": "firing",
    "labels": {
      "alertname": "DiskSpaceLow",
      "severity": "warning",
      "instance": "node-1",
      "mountpoint": "/var/log"
    },
    "annotations": {
      "description": "Disk space is running low"
    }
  }]
}
```

## Common Patterns

### Pattern 1: Dry Run Mode

Always support a dry-run mode for testing:

```python
DRY_RUN = os.getenv("DRY_RUN", "false").lower() == "true"

if DRY_RUN:
    print("DRY RUN: Would delete file:", filepath)
else:
    os.remove(filepath)
    print("Deleted file:", filepath)
```

### Pattern 2: Conditional Remediation

Check conditions before acting:

```bash
#!/bin/bash
THRESHOLD=${THRESHOLD:-80}
CURRENT=$(df -h "${OPENFERO_MOUNTPOINT}" | awk 'NR==2 {print $5}' | sed 's/%//')

if [ "$CURRENT" -lt "$THRESHOLD" ]; then
    echo "Disk usage ${CURRENT}% is below threshold ${THRESHOLD}%"
    exit 0
fi

echo "Disk usage ${CURRENT}% exceeds threshold, cleaning up..."
# Perform cleanup
```

### Pattern 3: Multi-Step Remediation

Break complex tasks into steps:

```python
def remediate():
    steps = [
        ("Check prerequisites", check_prerequisites),
        ("Backup data", backup_data),
        ("Perform cleanup", cleanup),
        ("Verify results", verify),
    ]
    
    for step_name, step_func in steps:
        print(f"Step: {step_name}")
        try:
            step_func()
            print(f"✓ {step_name} completed")
        except Exception as e:
            print(f"✗ {step_name} failed: {e}")
            sys.exit(1)
    
    print("All steps completed successfully")
    sys.exit(0)
```

## Troubleshooting

### Job Not Created

**Check Operarius CRD:**

```bash
kubectl get operarius -n openfero
```

Verify that an Operarius exists with an `alertSelector` matching your alert.

### Environment Variables Missing

Verify alert labels are present in Alertmanager webhook:

```bash
kubectl logs -l app=openfero -f
```

Look for: `"Adding labels as environment variables"`

### Job Keeps Running

- Set `activeDeadlineSeconds` in Job spec
- Ensure your container exits with proper exit code
- Check for infinite loops or hung processes

### Job Immediately Fails

- Check container logs: `kubectl logs job/job-name`
- Verify image exists and is pullable
- Check RBAC permissions if accessing Kubernetes API

## Additional Resources

- [OpenFero Documentation](../README.md)
- [Kubernetes Jobs Documentation](https://kubernetes.io/docs/concepts/workloads/controllers/job/)
- [Example Operarius CRDs](./examples/)
- [Authentication Guide](./authentication.md)

## Contributing

Have you built an Operarios that others might find useful? Consider contributing it to the community:

1. Document your use case
2. Include a Dockerfile and example Operarius CRD
3. Open a pull request to the `operarios/` directory

## Questions?

- [GitHub Issues](https://github.com/OpenFero/openfero/issues)
