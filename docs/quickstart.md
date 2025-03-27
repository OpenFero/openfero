# QuickStart Guide for OpenFero

This guide will help you quickly set up OpenFero on a local Kubernetes cluster for testing and development purposes.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [Helm](https://helm.sh/docs/intro/install/) v3.0+

## 1. Create a Local Kubernetes Cluster

```bash
export KUBE_VERSION=v1.26.0
kind create cluster --image kindest/node:${KUBE_VERSION}
```

## 2. Deploy Prometheus Stack

```bash
export PROM_OPERATOR_VERSION=45.2.0
helm install mmop prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set kubeTargetVersionOverride="${KUBE_VERSION}" \
  --version=${PROM_OPERATOR_VERSION}

# Wait for Prometheus to be ready
kubectl wait --for=condition=Ready pods -l app=prometheus -n monitoring --timeout=120s
```

## 3. Deploy OpenFero

```bash
# Create namespace for OpenFero
kubectl create namespace openfero-system

# Deploy OpenFero (replace with actual installation command when available)
# TODO: Add specific installation commands for OpenFero
kubectl apply -f https://github.com/OpenFero/openfero/releases/latest/download/openfero.yaml -n openfero-system
```

## 4. Configure Alertmanager

Create an Alertmanager configuration to forward alerts to OpenFero:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: alertmanager-config
  namespace: monitoring
data:
  alertmanager.yaml: |
    global:
      resolve_timeout: 5m
    route:
      group_by: ['alertname', 'job']
      group_wait: 30s
      group_interval: 5m
      repeat_interval: 12h
      receiver: 'openfero'
    receivers:
      - name: 'openfero'
        webhook_configs:
        - url: 'http://openfero-service.openfero-system.svc:8080/alerts'
          send_resolved: true
EOF

# Restart Alertmanager to apply changes
kubectl rollout restart statefulset alertmanager-mmop-kube-prometheus-alertmanager -n monitoring
```

## 5. Create a Test Operario

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: openfero-testalert-firing
  namespace: openfero-system
data:
  job.yaml: |
    apiVersion: batch/v1
    kind: Job
    metadata:
      generateName: openfero-testalert-remediation-
      labels:
        app: openfero
        alert: testalert
    spec:
      template:
        spec:
          containers:
          - name: remediation
            image: busybox
            command: ["sh", "-c", "echo 'Executing test remediation' && sleep 10"]
          restartPolicy: Never
      backoffLimit: 1
EOF
```

## 6. Test the Setup

Send a test alert to OpenFero:

```bash
# Port-forward to access OpenFero API
kubectl port-forward svc/openfero-service -n openfero-system 8080:8080 &

# Send a test alert
curl -X POST http://localhost:8080/alerts \
  -H 'Content-Type: application/json' \
  -d '{
    "status": "firing",
    "alerts": [{
      "status": "firing",
      "labels": {
        "alertname": "TestAlert",
        "severity": "warning"
      }
    }]
  }'

# Check if the remediation job was created
kubectl get jobs -n openfero-system
```

## 7. Access the OpenFero UI

```bash
# Access the UI at http://localhost:8080/
echo "OpenFero UI available at: http://localhost:8080/"

# Access the Swagger documentation
echo "OpenFero API docs available at: http://localhost:8080/swagger/"
```

## Next Steps

- Configure real alerts from your Prometheus installation
- Create more sophisticated remediation jobs
- Explore the [full documentation](../README.md) for advanced configuration options
