# Development Guide: Operarius CRDs

## Architecture Overview

The Operarius CRD implementation in OpenFero follows Kubernetes best practices for custom resources and extends the native Job API for automated remediation.

### Key Components

#### 1. CRD Definition (`api/v1alpha1/operarius_types.go`)

```go
// OperariusSpec defines the desired state of Operarius
type OperariusSpec struct {
    // JobTemplate defines the job to create for remediation
    JobTemplate batchv1.JobTemplateSpec `json:"jobTemplate"`
    
    // AlertSelector specifies which alerts this operarius handles
    AlertSelector AlertSelector `json:"alertSelector"`
    
    // Priority determines selection when multiple operarii match (higher wins)
    Priority int32 `json:"priority,omitempty"`
    
    // Enabled controls whether this operarius is active
    Enabled *bool `json:"enabled,omitempty"`
    
    // Deduplication prevents duplicate job execution
    Deduplication *DeduplicationConfig `json:"deduplication,omitempty"`
}
```

Key design decisions:

- **Embedded `JobTemplateSpec`**: Provides 100% compatibility with Kubernetes Job API
- **kubebuilder annotations**: Enable automatic CRD generation and validation
- **Optional fields**: Use pointers for proper defaulting behavior

#### 2. Service Layer (`pkg/services/operarius.go`)

The service layer handles:

- **Operarius discovery**: Finding matching operarii for alerts
- **Template processing**: Rendering Go templates with alert data
- **Job creation**: Creating Kubernetes Jobs from operarius templates
- **Deduplication**: Preventing duplicate job execution

#### 3. Integration Layer (`pkg/handlers/alerts.go`)

Extends existing alert handlers with operarius support:

```go
if s.OperariusService != nil && s.UseOperariusCRDs {
    // Try CRD-based approach first
    if handled, err := s.handleWithOperarius(ctx, hookMessage); err != nil {
        logger.Error("Failed to handle alert with operarius", "error", err)
    } else if handled {
        return
    }
}

// Fallback to ConfigMap approach
s.handleWithConfigMap(ctx, hookMessage)
```

## Development Workflow

### Setting Up Development Environment

1. **Install controller-tools**:
   ```bash
   go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
   ```

2. **Generate code and manifests**:
   ```bash
   make generate  # Generate DeepCopy methods
   make manifests # Generate CRD YAML
   ```

3. **Install CRDs in cluster**:
   ```bash
   make install-crds
   ```

### Code Generation Process

The project uses controller-gen for automatic code generation:

```bash
# Generate DeepCopy methods
controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

# Generate CRD manifests
controller-gen crd paths="./api/..." output:crd:artifacts:config=config/crd/bases
```

### Testing Strategy

#### Unit Tests

Test individual components in isolation:

```bash
# Test operarius service
go test ./pkg/services -v -run TestOperariusService

# Test with specific scenarios
go test ./pkg/services -v -run TestOperariusService_FindMatchingOperarius
```

#### Integration Tests

Test with real Kubernetes clients:

```bash
# Requires cluster access
go test ./pkg/services -v -tags integration
```

#### End-to-End Tests

Test complete alert-to-job workflow:

```bash
# Run comprehensive test suite
make test-e2e
```

### Debugging

#### Enable Debug Logging

```go
import "github.com/go-logr/logr"

// In your service
logger := logr.FromContextOrDiscard(ctx).WithName("operarius-service")
logger.V(1).Info("Processing alert", "alertname", alert.Labels["alertname"])
```

#### Common Issues

1. **CRD Schema Validation Errors**:
   ```bash
   # Check CRD schema
   kubectl explain operarius.spec.jobTemplate.spec
   
   # Validate against schema
   kubectl --dry-run=client apply -f operarius.yaml
   ```

2. **Template Rendering Errors**:
   ```go
   // Add debug output in template processing
   logger.V(1).Info("Template variables", "variables", templateVars)
   ```

3. **RBAC Issues**:
   ```bash
   # Check permissions
   kubectl auth can-i create jobs --as=system:serviceaccount:openfero:default
   ```

## Advanced Development Topics

### Custom Validation

Add custom validation with webhooks:

```go
//+kubebuilder:webhook:path=/validate-openfero-io-v1alpha1-operarius,mutating=false,failurePolicy=fail,sideEffects=None,groups=openfero.io,resources=operariuses,verbs=create;update,versions=v1alpha1,name=voperarius.kb.io,admissionReviewVersions=v1

func (r *Operarius) ValidateCreate() error {
    // Custom validation logic
    return r.validateOperarius()
}
```

### Status Updates

Implement status subresource:

```go
type OperariusStatus struct {
    // Conditions represent the latest available observations
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // ExecutionCount tracks how many jobs were created
    ExecutionCount int64 `json:"executionCount,omitempty"`
    
    // LastExecution records the last job creation time
    LastExecution *metav1.Time `json:"lastExecution,omitempty"`
}
```

### Performance Optimization

#### 1. Indexing

```go
// Add field indexing for efficient queries
func (r *OperariusReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&operariov1alpha1.Operarius{}).
        WithOptions(controller.Options{
            MaxConcurrentReconciles: 10,
        }).
        Complete(r)
}
```

#### 2. Caching

```go
// Cache frequently accessed operarii
type OperariusCache struct {
    operarii map[string]*operariov1alpha1.Operarius
    mutex    sync.RWMutex
    ttl      time.Duration
}
```

### Monitoring and Metrics

#### Custom Metrics

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    operariusExecutions = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "openfero_operarius_executions_total",
            Help: "Total number of operarius executions",
        },
        []string{"operarius", "status"},
    )
)
```

#### Health Checks

```go
func (s *OperariusService) HealthCheck(ctx context.Context) error {
    // Check if we can list operarii
    _, err := s.client.OperariusV1alpha1().Operariuses("").List(ctx, metav1.ListOptions{Limit: 1})
    return err
}
```

## Contributing Guidelines

### Code Style

1. **Follow Go conventions**:
   - Use gofmt and golint
   - Write clear, descriptive names
   - Add comments for exported functions

2. **Kubernetes conventions**:
   - Use structured logging
   - Follow controller-runtime patterns
   - Implement proper error handling

### Testing Requirements

1. **Unit tests**: Minimum 80% coverage
2. **Integration tests**: For Kubernetes API interactions
3. **Documentation**: Update docs for API changes

### Pull Request Process

1. **Create feature branch**: `git checkout -b feature/operarius-enhancement`
2. **Generate manifests**: `make generate manifests`
3. **Run tests**: `make test`
4. **Update docs**: Include relevant documentation
5. **Submit PR**: With clear description and tests

## API Reference

### Operarius Resource

```yaml
apiVersion: openfero.io/v1alpha1
kind: Operarius
metadata:
  name: example-operarius
  namespace: openfero
spec:
  # Required: Defines the job to create
  jobTemplate:
    spec:
      template:
        spec:
          # Standard Kubernetes Job spec
          
  # Required: Defines which alerts trigger this operarius
  alertSelector:
    alertname: "string"      # Required
    status: "firing"         # Optional: "firing" or "resolved"
    labels:                  # Optional: additional label matching
      key: "value"
      
  # Optional: Priority for selection (default: 0)
  priority: 50
  
  # Optional: Enable/disable this operarius (default: true)
  enabled: true
  
  # Optional: Deduplication configuration
  deduplication:
    enabled: true
    ttl: 300  # seconds

status:
  conditions: []
  executionCount: 0
  lastExecution: "2024-01-01T00:00:00Z"
```

### AlertSelector Reference

```go
type AlertSelector struct {
    // Alertname matches the alert name (required)
    Alertname string `json:"alertname"`
    
    // Status matches alert status: "firing" or "resolved" (optional)
    Status string `json:"status,omitempty"`
    
    // Labels provides additional label matching (optional)
    Labels map[string]string `json:"labels,omitempty"`
}
```

### Template Variables

Available in job templates:

- `{{ .Alert.Labels.* }}` - Alert labels
- `{{ .Alert.Annotations.* }}` - Alert annotations
- `{{ .HookMessage.Status }}` - Alert status
- `{{ .HookMessage.GroupKey }}` - Alert group
- `{{ .Labels.* }}` - Shorthand for alert labels
- `{{ .Status }}` - Shorthand for alert status

## Roadmap

### Short Term (Next Release)

- [ ] Admission webhooks for validation
- [ ] Status subresource implementation
- [ ] Enhanced metrics and monitoring
- [ ] Migration tooling

### Medium Term

- [ ] Multi-cluster support
- [ ] Advanced scheduling options
- [ ] Workflow orchestration
- [ ] UI integration

### Long Term

- [ ] Machine learning for remediation suggestions
- [ ] Integration with external systems
- [ ] Advanced templating engine
- [ ] Policy-based automation
