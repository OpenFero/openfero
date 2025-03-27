# Installation Recommendations

Since remediation often needs to be close to the system that needs the remediation, it is recommended to run the OpenFero component close to the system. The component itself has a small resource footprint and can run in parallel to the observed system in the namespace.

## Deployment Patterns

### Pattern 1: OpenFero Per Namespace

![Shows 3 kubernetes namespaces monitoring, namespaceA, namespaceB. In monitoring runs prometheus and alertmanager. Alertmanager notifies the OpenFero instances in namespaceA and namespaceB.][recommendation-1]

In this pattern:
- Prometheus and Alertmanager run in a dedicated monitoring namespace
- Each application namespace has its own OpenFero instance
- OpenFero remediation jobs run in the same namespace as the application they need to remediate
- Each namespace can have customized remediation strategies

**Benefits:**
- Clear separation of concerns
- Namespace-specific permissions for remediation actions
- Isolated failure domains
- Customized remediation for each application's needs

**Implementation:**

1. Deploy Prometheus and Alertmanager in the monitoring namespace:
```bash
kubectl create namespace monitoring
helm install prometheus prometheus-community/kube-prometheus-stack --namespace monitoring
```

2. Deploy OpenFero in each application namespace:
```bash
kubectl create namespace namespace-a
kubectl create namespace namespace-b

# Deploy OpenFero in namespace-a
kubectl apply -f openfero-deployment.yaml -n namespace-a

# Deploy OpenFero in namespace-b
kubectl apply -f openfero-deployment.yaml -n namespace-b
```

3. Configure Alertmanager to send alerts to OpenFero instances:
```yaml
receivers:
  - name: 'openfero-namespace-a'
    webhook_configs:
      - url: 'http://openfero.namespace-a.svc:8080/alerts'
  - name: 'openfero-namespace-b'
    webhook_configs:
      - url: 'http://openfero.namespace-b.svc:8080/alerts'

route:
  group_by: ['namespace']
  routes:
    - match:
        namespace: namespace-a
      receiver: 'openfero-namespace-a'
    - match:
        namespace: namespace-b
      receiver: 'openfero-namespace-b'
```

### Pattern 2: Centralized OpenFero (Alternative Approach)

For smaller deployments or when starting with OpenFero, a centralized deployment can be simpler:

- Single OpenFero instance in a dedicated namespace
- RBAC configuration that allows cross-namespace remediation
- Simpler alertmanager configuration

## Resource Requirements

OpenFero has minimal resource requirements:
- 50m CPU (request), 100m (limit)
- 64Mi memory (request), 128Mi (limit)

For production deployments, adjust these values based on the number of alerts you expect to handle.

[recommendation-1]: openfero-installation-recommendation-1.png
