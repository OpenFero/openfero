# OpenFero

**Lightweight, deterministic self-healing for Kubernetes.**

OpenFero is a play on words from the Latin *"opem fero"* ("to bring help") and *Open Source*. It turns Prometheus Alertmanager webhooks into predictable, RBAC-isolated Kubernetes Jobs—intentionally staying small, transparent, and easy to reason about.

[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/OpenFero/openfero/badge)](https://scorecard.dev/viewer/?uri=github.com/OpenFero/openfero)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/6683/badge)](https://www.bestpractices.dev/projects/6683)

## Why OpenFero

Modern operations increasingly combine probabilistic AI agents (for triage, root-cause analysis, and authoring) with deterministic building blocks for execution. Operating autonomous agents safely in production requires dedicated evaluation, observability, and guardrail stacks—complexity that OpenFero intentionally keeps out of its core.

Instead, OpenFero focuses on doing one job well as a **bounded, deterministic execution layer**:

- **Deterministic & GitOps-native:** Remediation actions are declared as `Operarius` Custom Resource Definitions (CRDs), reviewed in Git, and executed as standard Kubernetes Jobs.
- **Least-privilege by design:** OpenFero only creates Jobs; each `Operarius` references its own `ServiceAccount` scoped to the exact permissions that remediation needs.
- **Lightweight & zero-magic:** A single Go binary with an optional web UI, priority-based matching, and built-in deduplication—no external databases or complex orchestration state.
- **Built to complement AI & agent workflows:** Use AI agents at design time to turn runbooks into `Operarius` CRDs, or delegate known remediations from agentic/ChatOps workflows to OpenFero's predictable, RBAC-isolated execution boundary.

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

## Operarius Definitions

Automated remediation actions are defined declaratively using `Operarius` Kubernetes Custom Resource Definitions (CRDs), providing schema validation, priority handling, and deduplication:

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

## Security note

The service account that is installed when deploying openfero is for openfero itself. For the operarios, separate service accounts must be rolled out, which have the appropriate permissions for the remediation.

For operarios that need to interact with the Kubernetes API, it is recommended to define a suitable role for and authorize it via ServiceAccount in the job definition.

[comp-dia]: ./docs/component-diagram.png
