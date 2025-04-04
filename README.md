# OpenFero

Open Fero is a little play on words from the Latin "opem fero", which means "to help" and the term "OpenSource". Hence the name "openfero". The scope of OpenFero is a framework for self-healing in a cloud-native environment.

[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/OpenFero/openfero/badge)](https://scorecard.dev/viewer/?uri=github.com/OpenFero/openfero) [![OpenSSF Best Practices](https://www.bestpractices.dev/projects/6683/badge)](https://www.bestpractices.dev/projects/6683)

## Table of Contents

- [OpenFero](#openfero)
  - [Table of Contents](#table-of-contents)
  - [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Installation](#installation)
      - [Quick Start with Kind (for local development)](#quick-start-with-kind-for-local-development)
      - [Production Installation](#production-installation)
    - [Testing the Installation](#testing-the-installation)
      - [Using the Swagger UI](#using-the-swagger-ui)
      - [Using the OpenFero UI](#using-the-openfero-ui)
      - [Using curl](#using-curl)
  - [Architecture](#architecture)
    - [Component Diagram](#component-diagram)
    - [Installation Recommendations](#installation-recommendations)
  - [Operarios Definitions](#operarios-definitions)
    - [Example-Names](#example-names)
    - [Operarios-Example](#operarios-example)
  - [Security Note](#security-note)
  - [Contributing](#contributing)
  - [License](#license)

## Getting Started

### Prerequisites

Before installing OpenFero, ensure you have:
- A running Kubernetes cluster (v1.20+)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) configured to interact with your cluster
- [Helm](https://helm.sh/docs/intro/install/) v3.0+ installed (if using Helm for installation)
- Prometheus and Alertmanager deployed in your cluster

### Installation

You can install OpenFero in multiple ways:

#### Quick Start with Kind (for local development)

```bash
# Set versions
export KUBE_VERSION=v1.26.0
export PROM_OPERATOR_VERSION=45.2.0

# Create a local cluster
brew install kind
kind create cluster --image kindest/node:${KUBE_VERSION}

# Deploy Prometheus and Alertmanager
helm install mmop prometheus-community/kube-prometheus-stack --namespace default --set kubeTargetVersionOverride="${KUBE_VERSION}" --version=${PROM_OPERATOR_VERSION}

# Deploy OpenFero
# TODO: Add OpenFero deployment commands
```

#### Production Installation

For production environments, we recommend installing OpenFero in the same namespace as your application. See the [Installation Recommendations](./docs/installation-recommendations/README.md) for detailed deployment patterns.

### Testing the Installation

You can test if OpenFero is working properly in multiple ways:

#### Using the Swagger UI

Access the Swagger UI at `http://openfero-service:8080/swagger/` to interact with the API directly through a web interface. The Swagger UI provides a complete documentation of all available endpoints and allows you to test them directly.

#### Using the OpenFero UI

The OpenFero UI is available at `http://openfero-service:8080/` and provides:

- Overview of all received alerts and their current status
- Configuration viewer for operarios definitions

#### Using curl

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

## Architecture

### Component Diagram

OpenFero integrates with Prometheus and Alertmanager to provide automated remediation for your applications:

![Shows the Prometheus, Alertmanager components and that Alertmanager notifies the OpenFero component so that OpenFero starts the jobs via Kubernetes API.][comp-dia]

### Installation Recommendations

For detailed installation patterns and recommendations, see the [Installation Recommendations](./docs/installation-recommendations/README.md) document.

## Operarios Definitions

The operarios definitions are stored in the namespace in ConfigMaps with the naming convention `openfero-<alertname>-<status>`.

### Example-Names

- `openfero-KubeQuotaAlmostReached-firing`
- `openfero-KubeQuotaAlmostReached-resolved`

### Operarios-Example

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: openfero-kubequotaalmostfull-firing
  labels:
    app: openfero
spec:
  parallelism: 1
  completions: 1
  template:
    labels:
      app: openfero
    spec:
      containers:
        - name: python-job
          image: python:latest
          args:
            - bash
            - -c
            - |-
              echo "Hallo Welt"
      imagePullPolicy: Always
      restartPolicy: Never
      serviceAccount: <desired-sa>
      serviceAccountName: <desired-sa>
```

## Security Note

The service account that is installed when deploying openfero is for openfero itself. For the operarios, separate service accounts must be rolled out, which have the appropriate permissions for the remediation.

For operarios that need to interact with the Kubernetes API, it is recommended to define a suitable role for and authorize it via ServiceAccount in the job definition.

## Contributing

See our [Contributing Guide](./CONTRIBUTING.md) for details on how to contribute to OpenFero.

## License

OpenFero is licensed under the [Apache License 2.0](./LICENSE).

[comp-dia]: ./docs/component-diagram.png
