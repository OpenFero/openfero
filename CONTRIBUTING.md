# Contributing

Hello, I am pleased that you would like to make a contribution to OpenFero and thus become part of the community.

I hope this document will help you to get started.

## Development Workflow

### Prerequisites

Before contributing to OpenFero, ensure you have:
- Go 1.20 or higher
- Docker
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [Helm](https://helm.sh/docs/intro/install/) v3.0+
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) (for local Kubernetes development)
- [goreleaser](https://goreleaser.com/install/) (for building)
- [swag](https://github.com/swaggo/swag) (for API documentation)

### Local development

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
# Clone the repository if you haven't already
git clone https://github.com/OpenFero/openfero.git
cd openfero

# Build and deploy OpenFero to your local cluster
make build
kubectl apply -f k8s/deployment.yaml  # Adjust based on actual deployment files
```

### Build locally

OpenFero uses [goreleaser](https://github.com/goreleaser) to build, with the following command you can build locally.

```bash
goreleaser build --snapshot --clean --single-target
```

### Check for goreleaser dependencies

```bash
goreleaser healthcheck
```

### Test locally

You can test OpenFero using various methods:

#### Using curl

```bash
# Test with sample alert
curl -X POST -H "Content-Type: application/json" -d @./test/alerts.json http://localhost:8080/alerts

# Alternative test payload
curl -X POST http://localhost:8080/alert \
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

#### Using the Swagger UI

Access the Swagger UI at `http://localhost:8080/swagger/` to interact with the API directly through a web interface.

#### Using the OpenFero UI

The OpenFero UI is available at `http://localhost:8080/` and provides an overview of alerts and configurations.

### Update Swagger-Docs

```bash
swag init --generalInfo main.go --output ./pkg/docs
```

## Contributing Guide

### Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests to ensure they pass
5. Commit your changes (`git commit -m 'Add some amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Pull Request Process

1. Ensure your code follows the project's style guidelines
2. Update documentation as needed
3. Include tests for new functionality
4. Ensure the CI pipeline passes
5. Request reviews from maintainers

### Documentation Contributions

Documentation improvements are always welcome. Please follow these guidelines:

1. Use clear and concise language
2. Follow Markdown best practices
3. Include diagrams where helpful (PlantUML is preferred)
4. Place documentation in the appropriate location (usually `/docs`)

## Project Structure

- `/pkg`: Core packages and libraries
- `/docs`: Documentation files
- `/charts`: Helm charts for deployment
- `/test`: Test files and fixtures

## Code Style

Please follow Go's standard formatting and style conventions. Run `gofmt` before committing.
