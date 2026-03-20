# Changelog

All notable changes to OpenFero will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.18.0] - Unreleased

### Removed

- **ConfigMap-based Remediation**: The legacy ConfigMap approach has been fully removed. Use Operarius CRDs instead. See [Migration Guide](docs/operarius-crd-migration.md).
- **OAuth2 Authentication Stub**: The unimplemented OAuth2 auth method has been removed. Supported methods: `none`, `basic`, `bearer`.
- **SSE Composable**: Removed unused `useSSE.ts` frontend composable (replaced by WebSocket in v0.17.0).
- **Deprecated `/alertStore` Route**: The `/alertStore` endpoint now logs a deprecation warning and redirects to `/api/alerts`. It will be removed in a future release.

### Fixed

- Fixed syntax error in auth middleware after OAuth2 removal (missing switch closing brace).
- Fixed Swagger comment in jobs endpoint: "ConfigMaps" replaced with "Operarius CRDs".
- Removed debug `console.log` statements from frontend WebSocket, store, and component code.
- Updated Copilot instructions to match actual project structure (removed references to WorkflowView, useSSE, vue-flow).

### Changed

- **Alert Handler Tests**: Replaced 3 skipped test stubs with full implementations covering single alert POST, multiple alert POST, and malformed JSON POST.

## [0.17.0] - 2025-12-05

### Added

- **Operarius CRD as Default**: The Operarius Custom Resource Definition is now the default method for defining remediation jobs. ConfigMaps are deprecated and will be removed in v0.18.0.
- **Operarius Starter Pack**: Five production-ready Operarii for common kube-prometheus-stack alerts:
  - `KubePodCrashLooping` - Deletes crash-looping pods to trigger fresh restarts
  - `KubeDeploymentReplicasMismatch` - Restarts deployments with replica mismatches
  - `KubeJobFailed` - Cleans up failed jobs to allow re-scheduling
  - `KubeHpaMaxedOut` - Scales HPAs temporarily to handle load spikes
  - `KubeDaemonSetRolloutStuck` - Restarts stuck DaemonSet rollouts
- **Vue.js 3 SPA Frontend**: Complete rewrite of the UI using Vue.js 3, TypeScript, Pinia, and TailwindCSS 4
  - Real-time updates via WebSocket
  - Dark/Light theme support
  - Responsive design
  - Alert search and filtering
  - Job status monitoring
- **End-to-end Test Suite**: Comprehensive end-to-end tests for the alert-to-job remediation flow
- **OPENFERO_* Environment Variables**: Alert labels are now injected as `OPENFERO_LABEL_*` environment variables in remediation jobs (both ConfigMap and CRD paths)

### Changed

- **Helm Chart**: `operarius.useOperariusCRDs` now defaults to `true`
- **API Endpoints**: New `/api/ws` WebSocket endpoint for real-time updates

### Deprecated

- **ConfigMap-based Remediation**: The legacy ConfigMap approach is deprecated and has been removed in v0.18.0. See [Migration Guide](docs/operarius-crd-migration.md) for upgrade instructions.

### Fixed

- Prometheus metrics (`openfero_jobs_created_total`, `openfero_jobs_failed_total`) now increment correctly in the CRD path
- SSE/WebSocket broadcast now works correctly for CRD-created jobs
- Fixed `bitnami/kubectl` image tag in Operarius examples (use `latest` instead of version-specific tags)

## [0.16.0] - Previous Release

See [GitHub Releases](https://github.com/OpenFero/openfero/releases) for previous changelog entries.
