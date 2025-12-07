---
applyTo: "**/*.go,go.mod,go.sum"
---

# OpenFero Backend Development Guidelines

## Go Code Patterns

### Alert Processing Workflow

1. **Webhook Reception**: `pkg/handlers/alerts.go:AlertsPostHandler()` receives Alertmanager JSON
2. **Status Validation**: Only `firing` and `resolved` statuses are processed
3. **Operarius Lookup**: Find matching Operarius CRD based on alert labels
4. **Job Creation**: `pkg/services/operarius.go:CreateJobFromOperarius()` creates Job in destination namespace
5. **Alert Storage**: Saved to AlertStore (memory or memberlist) with JobInfo metadata
6. **SSE Broadcast**: Notify connected frontend clients of new alerts/job status

### Kubernetes Client Architecture

- **Informers**: Operarius and Job informers watch resources with label selectors
- **Stores**: In-memory caches (`OperariusStore`, `JobStore`) for fast lookups
- **Namespace separation**: `operariusNamespace` (operarios definitions) vs `jobDestinationNamespace` (where Jobs run)

### Error Handling & Logging

- Use `go.uber.org/zap` structured logging exclusively
- Pattern: `log.Error("message", zap.String("key", value), zap.Error(err))`
- Always log alertname, status, operarius name, and job name for traceability

## API Endpoints

### JSON API (for Vue.js Frontend)

| Method | Path            | Description                              |
| ------ | --------------- | ---------------------------------------- |
| GET    | `/api/alerts`   | List alerts with optional `?q=` search   |
| POST   | `/api/alerts`   | Receive Alertmanager webhook             |
| GET    | `/api/jobs`     | List job definitions from Operarius CRDs |
| GET    | `/api/workflow` | Get workflow data (alerts + job status)  |
| GET    | `/api/events`   | SSE endpoint for realtime updates        |

### System Endpoints

| Method | Path         | Description        |
| ------ | ------------ | ------------------ |
| GET    | `/healthz`   | Liveness probe     |
| GET    | `/readiness` | Readiness probe    |
| GET    | `/metrics`   | Prometheus metrics |
| GET    | `/swagger/*` | API documentation  |

### SPA Handler

| Method | Path | Description                               |
| ------ | ---- | ----------------------------------------- |
| GET    | `/*` | Serve Vue.js SPA (fallback to index.html) |

## Server-Sent Events (SSE)

### Implementation Pattern

```go
func (s *Server) SSEHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    // Subscribe to events
    eventChan := s.EventBroker.Subscribe()
    defer s.EventBroker.Unsubscribe(eventChan)

    for {
        select {
        case event := <-eventChan:
            fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
            flusher.Flush()
        case <-r.Context().Done():
            return
        }
    }
}
```

### Event Types

- `alert:new` - New alert received
- `alert:updated` - Alert status changed
- `job:created` - Remediation job created
- `job:succeeded` - Job completed successfully
- `job:failed` - Job failed

## Development Commands

### Local Testing

```bash
# Build with goreleaser (single platform for speed)
goreleaser build --snapshot --clean --single-target

# Test webhook endpoint
curl -X POST -H "Content-Type: application/json" -d @./test/alerts.json http://localhost:8080/api/alerts

# Update Swagger docs after API changes
swag init --generalInfo main.go --output ./pkg/docs
```

### Running Tests

```bash
# All tests
go test -v ./...

# With race detection
go test -v -race ./...

# Specific package
go test -v ./pkg/services
```

## Extending AlertStore

Implement `pkg/alertstore/alertstore.go:Store` interface:

- `SaveAlert()` and `SaveAlertWithJobInfo()` for storing
- `GetAlerts(query, limit)` for retrieval (supports search)
- `Initialize()` and `Close()` for lifecycle
- Current implementations: `memory` (default), `memberlist` (clustered)

## Adding Prometheus Metrics

- Register in `pkg/metadata/metadata.go:AddMetricsToPrometheusRegistry()`
- Use `prometheus.Counter` or `prometheus.Gauge` types
- Example: `JobsCreatedTotal`, `JobsFailedTotal`

## Authentication Patterns

- Webhook supports: Basic Auth, Bearer Token, OAuth2, or None (default)
- Auth middleware: `pkg/handlers/auth.go:AuthMiddleware()`
- Validate config at startup with `validateAuthConfig()`

## Troubleshooting

### "Operarius not found"

- Check naming: `openfero-<alertname>-{firing|resolved}`
- Verify Operarius has correct label selector (matches OpenFero's `--labelSelector`)
- Check namespace: Operarius must be in `--operariusNamespace`

### "Job already exists for group"

- Expected behavior: OpenFero deduplicates based on Alertmanager groupKey
- Jobs with same groupKey are skipped if still active
- Check Job labels: `openfero.io/group-key` contains hashed groupKey
