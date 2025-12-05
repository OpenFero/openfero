# All-in-One Operarii Bundle

This directory contains a single YAML file with all Operarii and RBAC resources for easy installation.

## Quick Start

```bash
# Install all Operarii and RBAC
kubectl apply -f operarii-bundle.yaml

# Verify installation
kubectl get operarius -n openfero
kubectl get serviceaccounts -n openfero | grep openfero-
```

## What's Included

This bundle contains:

### Operarii (5)
- `pod-crashloop-restart` - Restarts CrashLooping pods
- `deployment-replicas-fix` - Fixes deployment replica mismatches
- `failed-job-cleanup` - Cleans up failed jobs
- `hpa-scale-increase` - Increases HPA max replicas
- `daemonset-rollout-fix` - Fixes stuck DaemonSet rollouts

### Service Accounts (5)
- `openfero-pod-restarter`
- `openfero-deployment-fixer`
- `openfero-job-cleaner`
- `openfero-hpa-scaler`
- `openfero-daemonset-fixer`

### ClusterRoles and ClusterRoleBindings (5 each)
Each Operarius has its own minimal RBAC configuration.

## Selective Installation

If you only want specific Operarii, apply them individually:

```bash
# Example: Only pod restart and job cleanup
kubectl apply -f ../KubePodCrashLooping/
kubectl apply -f ../KubeJobFailed/
```

## Uninstall

```bash
kubectl delete -f operarii-bundle.yaml
```

## Customization

To customize before applying:

1. Download the bundle
2. Edit as needed (priorities, TTLs, enabled flags)
3. Apply the modified version

```bash
curl -O https://raw.githubusercontent.com/OpenFero/openfero/main/operarios/all-in-one/operarii-bundle.yaml
# Edit operarii-bundle.yaml
kubectl apply -f operarii-bundle.yaml
```
