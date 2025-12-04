# OpenFero

Open Fero is a little play on words from the Latin "opem fero", which means "to help" and the term "OpenSource". Hence the name "openfero". The scope of OpenFero is a framework for self-healing in a cloud-native environment.

[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/OpenFero/openfero/badge)](https://scorecard.dev/viewer/?uri=github.com/OpenFero/openfero)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/6683/badge)](https://www.bestpractices.dev/projects/6683)

## Getting started

The recommended method is installation via helm chart.

```bash
helm pull oci://ghcr.io/openfero/openfero/charts/openfero --version 0.2.1
helm install openfero oci://ghcr.io/openfero/openfero/charts/openfero --version 0.2.1
```

### Testing the Installation

You can test if OpenFero is working properly in multiple ways:

#### Using the Swagger UI

Access the Swagger UI at `http://openfero-service:8080/swagger/` to interact with the API directly through a web interface. The Swagger UI provides a complete documentation of all available endpoints and allows you to test them directly.

#### Using the OpenFero UI

The OpenFero UI is available at `http://openfero-service:8080/` and provides:

- Overview of all received alerts and their current status
- Configuration viewer for operarios definitions

#### Using cURL

```bash
curl -X POST http://openfero-service:8080/alert \
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
```

## Component-Diagram

![Shows the Prometheus, Alertmanager components and that Alertmanager notifies the OpenFero component so that OpenFero starts the jobs via Kubernetes API.][comp-dia]

## Operarios Definitions

OpenFero supports two approaches for defining automated remediation actions:

### 1. Operarius CRDs (Recommended)

The modern approach uses Kubernetes Custom Resource Definitions (CRDs) that provide schema validation, priority handling, and enhanced features:

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

  priority: 50
  enabled: true
```

**Benefits of Operarius CRDs:**

- Schema validation and IDE support
- Priority-based selection when multiple operarii match
- Deduplication to prevent duplicate job execution
- Status tracking and monitoring
- 100% Kubernetes Job API compatibility
- GitOps-friendly declarative management

See the [Operarius CRD Documentation](docs/operarius-crds.md) for complete details.

### 2. ConfigMaps (Legacy)

The traditional approach stores operarios definitions in ConfigMaps with the naming convention `openfero-<alertname>-<status>`:

#### Example Names

- `openfero-KubeQuotaAlmostReached-firing`
- `openfero-KubeQuotaAlmostReached-resolved`

#### ConfigMap Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: openfero-kubequotaalmostfull-firing
  labels:
    app: openfero
data:
  image: python:latest
  command: |
    echo "Hello World - Alert: {{ .Alert.Labels.alertname }}"
```

**Migration:** Existing ConfigMap-based operarios can be migrated to CRDs. See the [Migration Guide](docs/operarius-crd-migration.md) for step-by-step instructions.

## Security note

The service account that is installed when deploying openfero is for openfero itself. For the operarios, separate service accounts must be rolled out, which have the appropriate permissions for the remediation.

For operarios that need to interact with the Kubernetes API, it is recommended to define a suitable role for and authorize it via ServiceAccount in the job definition.

[comp-dia]: ./docs/component-diagram.png
