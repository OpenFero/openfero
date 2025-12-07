---
applyTo: "**"
---

# OpenFero General Development Guidelines

## Project Overview

OpenFero is a Kubernetes-native self-healing framework that receives Alertmanager webhooks and executes remediation Jobs defined in Operarius CRDs. It acts as a bridge between Prometheus/Alertmanager and Kubernetes Jobs.

**Tech Stack**:

- **Backend**: Go 1.x, Kubernetes client-go, Prometheus metrics
- **Frontend**: Vue.js 3 SPA, TypeScript, Vite, Pinia, vue-flow
- **Infrastructure**: Helm charts, Docker containers
- **Realtime**: Server-Sent Events (SSE) for live updates

**Architecture Flow**:

```text
Alertmanager webhook → OpenFero /api/alerts → Operarius lookup → K8s Job creation → SSE broadcast → Vue.js UI update
```

## Critical Naming Conventions

### Operarius Naming Pattern

Operarius CRDs MUST follow this exact format:

```text
openfero-<alertname>-<status>
```

- `<alertname>`: Sanitized alert name from Prometheus (case-sensitive)
- `<status>`: Either `firing` or `resolved`
- Examples: `openfero-KubeQuotaAlmostReached-firing`, `openfero-DiskSpaceAlmostFull-resolved`

### Job Labels & Deduplication

- Jobs are labeled with `openfero.io/group-key` (hashed groupKey from Alertmanager)
- OpenFero deduplicates jobs using groupKey before creation
- Jobs automatically get TTL and label selector from OpenFero's configuration

## File Ownership & Responsibilities

### Backend (Go)

- `main.go`: Initialization, flag parsing, HTTP route registration
- `pkg/handlers/`: HTTP handlers (API endpoints, SSE, auth)
- `pkg/services/`: Business logic (alert processing, job creation)
- `pkg/kubernetes/`: K8s client, informers, job operations
- `pkg/alertstore/`: Alert storage abstraction + implementations
- `pkg/models/`: Data structures (HookMessage, Alert, AlertStoreEntry)

### Frontend (Vue.js)

- `frontend/src/views/`: Page components (AlertsView, JobsView, WorkflowView)
- `frontend/src/components/`: Reusable UI components
- `frontend/src/stores/`: Pinia stores (alerts, jobs, workflow)
- `frontend/src/composables/`: Vue composables (useSSE, useTheme)
- `frontend/src/api/`: API client functions

### Infrastructure

- `charts/openfero/`: Helm chart with CRDs, RBAC, deployment
- `operarios/`: Example worker images (documented in operarios/readme)

## Conventions to Follow

1. **Sanitize inputs**: Always use `utils.SanitizeInput()` for alertnames and status
2. **Defer Close()**: Always defer closing resources: `defer r.Body.Close()`
3. **Mutex safety**: Lock AlertStore mutations with `mutex.Lock()`
4. **Goroutines**: Use `go services.CreateResponseJob()` for non-blocking job creation
5. **Environment variables**: Alert labels injected as env vars in Jobs via `AddLabelsAsEnvVars()`
6. **No emojis**: Do not use emojis in code, comments, documentation, or commit messages - keep it professional and plain text

## Questions to Ask When Making Changes

- Does this change affect Operarius naming? (Breaking change!)
- Do Jobs need new labels or env vars? (Update `kubernetes/jobs.go`)
- Is authentication involved? (Test with all auth methods: basic, bearer, none)
- Does this add new metrics? (Register in `metadata.go`)
- Will this change affect OSSF Scorecard? (Check pinned actions, permissions)
- Should this be documented? (Update `docs/` or Swagger comments)
- Does the frontend need to be updated? (Check API contract changes)
