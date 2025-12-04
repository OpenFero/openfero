---
applyTo: ".github/**,charts/**,SECURITY.md"
---

# OpenFero Security Guidelines

## OSSF Scorecard Compliance

OpenFero maintains a high OSSF Scorecard rating. Follow these practices:

### GitHub Actions Security

- **SHA-pin all GitHub Actions** (format: `uses: action@SHA # v1.2.3`)
- **Minimal permissions**: Set `permissions:` for each workflow job
- **Harden runners**: Use `step-security/harden-runner` as first step
- **persist-credentials: false** in `actions/checkout`

Example:

```yaml
jobs:
  build:
    permissions:
      contents: read
    steps:
      - uses: step-security/harden-runner@v2
        with:
          egress-policy: audit
      - uses: actions/checkout@v4
        with:
          persist-credentials: false
```

### Dependency Management

- Dependabot enabled for Go modules, Docker, GitHub Actions, npm
- Review dependency updates promptly
- Use `labels` instead of `groups` for GitHub Actions updates (prevents blocking)

## Cosign Signing

### Container Images

Container images are signed via Sigstore/Cosign with keyless OIDC:

```bash
# Verify image signature
cosign verify ghcr.io/openfero/openfero:latest \
  --certificate-identity-regexp="https://github.com/OpenFero/openfero/.*" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

- Certificate identity regular expression: `https://github.com/OpenFero/openfero/.*` (note capital F)
- Transparency log: Rekor for public verification

### Helm Charts

Helm charts are also signed with Cosign:

```bash
# Verify Helm chart
cosign verify ghcr.io/openfero/openfero/charts/openfero:latest \
  --certificate-identity-regexp="https://github.com/OpenFero/openfero/.*" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

## Authentication Patterns

### Webhook Authentication

OpenFero supports multiple auth methods for the `/api/alerts` endpoint:

| Method | Flag                  | Description                     |
| ------ | --------------------- | ------------------------------- |
| None   | `--authMethod=none`   | No authentication (default)     |
| Basic  | `--authMethod=basic`  | HTTP Basic Auth                 |
| Bearer | `--authMethod=bearer` | Bearer token                    |
| OAuth2 | `--authMethod=oauth2` | OIDC/OAuth2 with JWT validation |

### Implementation

- Auth middleware: `pkg/handlers/auth.go:AuthMiddleware()`
- Validate config at startup with `validateAuthConfig()`
- Always test with all auth methods: basic, bearer, none

### Kubernetes RBAC

Minimal permissions in Helm chart:

```yaml
# Role for reading ConfigMaps (Operarii)
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch"]

# Role for creating Jobs
rules:
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["create", "get", "list", "watch"]
```

## Secrets Management

### Best Practices

1. **Never commit secrets** - Use Kubernetes Secrets or external secret managers
2. **Environment variables** - Pass secrets via env vars, not flags
3. **Secret rotation** - Support credential rotation without restart
4. **Audit logging** - Log authentication attempts (without credentials)

### Helm Values

```yaml
auth:
  enabled: true
  method: basic
  existingSecret: openfero-auth # Reference existing K8s Secret
```

## Network Security

### NetworkPolicy

Restrict ingress to OpenFero:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: openfero-ingress
spec:
  podSelector:
    matchLabels:
      app: openfero
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: monitoring
      ports:
        - port: 8080
```

### TLS

- Support TLS termination at ingress or service mesh level
- Optional: Direct TLS with cert-manager certificates
- See `docs/authentication.md` for detailed TLS/mTLS setup

## Security Scanning

### Container Scanning

- Trivy scans in CI pipeline
- Ignore file: `.trivyignore` for false positives
- Configuration: `trivy.yaml`

### Code Scanning

- CodeQL analysis enabled
- Gitleaks for secret detection (`.gitleaks.toml`)
- Dependabot security updates

## Vulnerability Reporting

See `SECURITY.md` for:

- Security policy
- Reporting process
- Response timeline
- Disclosure policy
