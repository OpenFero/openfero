---
applyTo: ".github/workflows/**,.goreleaser.yaml,goreleaser.dockerfile"
---

# OpenFero CI/CD Guidelines

## Workflow Structure

### Path-based Workflow Filtering

Use `tj-actions/changed-files` for conditional job execution:

```yaml
jobs:
  check-changes:
    outputs:
      go_changed: ${{ steps.changed-files-go.outputs.any_changed }}
      frontend_changed: ${{ steps.changed-files-frontend.outputs.any_changed }}
    steps:
      - uses: tj-actions/changed-files@v45
        id: changed-files-go
        with:
          files: |
            **/*.go
            go.mod
            go.sum
      - uses: tj-actions/changed-files@v45
        id: changed-files-frontend
        with:
          files: |
            frontend/**

  test-go:
    needs: check-changes
    if: needs.check-changes.outputs.go_changed == 'true'

  test-frontend:
    needs: check-changes
    if: needs.check-changes.outputs.frontend_changed == 'true'
```

**Rationale**: Path-filtered workflows prevent PRs from being blocked when non-matching files change.

## Build Pipeline

### Frontend Build (runs first)

```yaml
frontend-build:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-node@v4
      with:
        node-version-file: frontend/.nvmrc
        cache: "npm"
        cache-dependency-path: frontend/package-lock.json
    - run: npm ci
      working-directory: frontend
    - run: npm run build
      working-directory: frontend
    - uses: actions/upload-artifact@v4
      with:
        name: frontend-dist
        path: frontend/dist
```

### Go Build (depends on frontend)

```yaml
go-build:
  needs: frontend-build
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/download-artifact@v4
      with:
        name: frontend-dist
        path: frontend/dist
    - uses: actions/setup-go@v5
      with:
        go-version-file: go.mod
    - run: go build -o openfero .
```

## Testing

### Go Tests

```yaml
test-go:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version-file: go.mod
    - run: go test -v -race ./...
```

### Frontend Tests

```yaml
test-frontend:
  runs-on: ubuntu-latest
  defaults:
    run:
      working-directory: frontend
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-node@v4
      with:
        node-version-file: frontend/.nvmrc
        cache: "npm"
        cache-dependency-path: frontend/package-lock.json
    - run: npm ci
    - run: npm run type-check
    - run: npm run lint
    - run: npm run test:unit
```

## Release Workflow

### Git Workflow

1. **Feature Development**: Feature-Branch -> PR -> main
2. **Release**: Update `CHANGELOG.md` + `frontend/package.json`, then tag:
   ```bash
   git tag vX.Y.Z && git push origin vX.Y.Z
   ```
3. **Hotfix** (if main has moved on): Create branch from tag, fix, tag again:
   ```bash
   git checkout -b hotfix/v0.17.1 v0.17.0
   # make fix, commit
   git tag v0.17.1 && git push origin hotfix/v0.17.1 --tags
   ```

### Trigger

```yaml
on:
  push:
    tags:
      - "v*"
```

### Steps

1. **Frontend build** - `npm run build`
2. **GoReleaser** - Multi-platform binaries + container images
3. **Cosign** - Sign containers and Helm charts
4. **Helm** - Publish to `ghcr.io/openfero/openfero/charts/openfero`

### GoReleaser Configuration

```yaml
# .goreleaser.yaml
builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64

dockers_v2:
  - image_templates:
      - "ghcr.io/openfero/openfero:{{ .Tag }}"
    build_flag_templates:
      - "--pull"
      - "--label=org.opencontainers.image.source=https://github.com/OpenFero/openfero"
```

## Dockerfile

### Multi-stage Build

```dockerfile
# Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Build Go binary
FROM golang:1.22-alpine AS backend
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -o openfero .

# Final image
FROM gcr.io/distroless/static-debian12
COPY --from=backend /app/openfero /
ENTRYPOINT ["/openfero"]
```

## Changelog Categories

Defined in `.github/release.yml`:

```yaml
changelog:
  categories:
    - title: Features
      labels: [enhancement]
    - title: Bug Fixes
      labels: [bug]
    - title: Dependencies
      labels: [dependencies]
    - title: CI
      labels: [github_actions, automation]
    - title: Docker
      labels: [docker]
    - title: Documentation
      labels: [documentation]
```

## Dependabot Configuration

```yaml
# .github/dependabot.yml
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    labels: ["dependencies"]

  - package-ecosystem: "npm"
    directory: "/frontend"
    labels: ["dependencies", "frontend"]

  - package-ecosystem: "github-actions"
    directory: "/"
    labels: ["github_actions"]

  - package-ecosystem: "docker"
    directory: "/"
    labels: ["docker"]
```

## Local Development

```bash
# Run frontend dev server
cd frontend && npm run dev

# Run Go backend (separate terminal)
go run . --operariusNamespace=default

# Or use goreleaser for production-like build
goreleaser build --snapshot --clean --single-target
```

## Artifact Embedding

Go embeds the frontend build output:

```go
//go:embed frontend/dist/*
var frontendFS embed.FS

func SPAHandler() http.Handler {
    dist, _ := fs.Sub(frontendFS, "frontend/dist")
    return http.FileServer(http.FS(dist))
}
```
