# OpenFero Signature Verification

All OpenFero releases (containers, Helm charts, and binaries) are signed using [Sigstore Cosign](https://www.sigstore.dev/) for supply chain security.

## What is Signed

- **Container Images**: All Docker images published to GHCR
- **Helm Charts**: All Helm charts published to GHCR OCI registry
- **Release Artifacts**: Checksums and SBOMs for binary releases

## Installing Cosign

### macOS

```bash
brew install cosign
```

### Linux

```bash
# Download latest release
curl -O -L "https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64"
sudo mv cosign-linux-amd64 /usr/local/bin/cosign
sudo chmod +x /usr/local/bin/cosign
```

### Windows

```powershell
# Using Chocolatey
choco install cosign

# Or download from releases
# https://github.com/sigstore/cosign/releases
```

## Verify Container Images

### Verify a specific version

```bash
cosign verify ghcr.io/openfero/openfero:v1.0.0 \
  --certificate-identity-regexp "https://github.com/OpenFero/openfero/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

### Verify latest stable release

```bash
cosign verify ghcr.io/openfero/openfero:latest \
  --certificate-identity-regexp "https://github.com/OpenFero/openfero/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

### Verify nightly builds

```bash
cosign verify ghcr.io/openfero/openfero:1.0.1-nightly.20250916 \
  --certificate-identity-regexp "https://github.com/OpenFero/openfero/.github/workflows/nightly-build.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

## Verify Helm Charts

### Verify Helm chart

```bash
cosign verify ghcr.io/openfero/openfero/charts/openfero:1.0.0 \
  --certificate-identity-regexp "https://github.com/OpenFero/openfero/.github/workflows/release-only-chart.yaml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

## Verify Release Artifacts

### Download and verify checksums

```bash
# Download release artifacts
curl -LO "https://github.com/openfero/openfero/releases/download/v1.0.0/checksums.txt"
curl -LO "https://github.com/openfero/openfero/releases/download/v1.0.0/checksums.txt.sig"
curl -LO "https://github.com/openfero/openfero/releases/download/v1.0.0/checksums.txt.pem"

# Verify checksums signature
cosign verify-blob checksums.txt \
  --signature checksums.txt.sig \
  --certificate checksums.txt.pem \
  --certificate-identity-regexp "https://github.com/OpenFero/openfero/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

### Verify SBOM (Software Bill of Materials)

```bash
# Download SBOM and signature
curl -LO "https://github.com/openfero/openfero/releases/download/v1.0.0/openfero_1.0.0_linux_amd64.tar.gz.sbom.json"
curl -LO "https://github.com/openfero/openfero/releases/download/v1.0.0/openfero_1.0.0_linux_amd64.tar.gz.sbom.json.sig"
curl -LO "https://github.com/openfero/openfero/releases/download/v1.0.0/openfero_1.0.0_linux_amd64.tar.gz.sbom.json.pem"

# Verify SBOM signature
cosign verify-blob openfero_1.0.0_linux_amd64.tar.gz.sbom.json \
  --signature openfero_1.0.0_linux_amd64.tar.gz.sbom.json.sig \
  --certificate openfero_1.0.0_linux_amd64.tar.gz.sbom.json.pem \
  --certificate-identity-regexp "https://github.com/OpenFero/openfero/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

## Kubernetes Policy Enforcement

You can enforce signature verification in Kubernetes using Sigstore Policy Controller:

### Install Policy Controller

```bash
kubectl apply -f https://github.com/sigstore/policy-controller/releases/latest/download/policy-controller-latest.yaml
```

### Create ClusterImagePolicy

```yaml
apiVersion: policy.sigstore.dev/v1beta1
kind: ClusterImagePolicy
metadata:
  name: openfero-policy
spec:
  images:
    - glob: "ghcr.io/openfero/openfero*"
  authorities:
    - keyless:
        identities:
          - issuer: "https://token.actions.githubusercontent.com"
            subject: "https://github.com/OpenFero/openfero/.github/workflows/release.yml@refs/tags/v*"
          - issuer: "https://token.actions.githubusercontent.com"
            subject: "https://github.com/OpenFero/openfero/.github/workflows/nightly-build.yml@refs/heads/main"
```

Apply the policy:

```bash
kubectl apply -f openfero-image-policy.yaml
```

Now Kubernetes will only allow signed OpenFero images to run!

## Gatekeeper Policy (Alternative)

If you're using OPA Gatekeeper:

```yaml
apiVersion: templates.gatekeeper.sh/v1beta1
kind: ConstraintTemplate
metadata:
  name: cosignverification
spec:
  crd:
    spec:
      names:
        kind: CosignVerification
      validation:
        properties:
          allowedImages:
            type: array
            items:
              type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package cosignverification

        violation[{"msg": msg}] {
          container := input.review.object.spec.containers[_]
          not startswith(container.image, "ghcr.io/openfero/openfero")
          msg := "Only signed OpenFero images are allowed"
        }
```

## Manual Verification Examples

### Using Docker content trust

```bash
# Enable content trust
export DOCKER_CONTENT_TRUST=1

# Pull will fail if image is not signed
docker pull ghcr.io/openfero/openfero:latest
```

### Inspect signatures

```bash
# Show signature details
cosign tree ghcr.io/openfero/openfero:latest

# Show certificate details
cosign verify ghcr.io/openfero/openfero:latest \
  --certificate-identity-regexp "https://github.com/OpenFero/openfero/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --output json | jq '.[]'
```

## Learn More

- [Sigstore Documentation](https://docs.sigstore.dev/)
- [Cosign GitHub Repository](https://github.com/sigstore/cosign)
- [Supply Chain Security Guide](https://slsa.dev/)
- [OSSF Scorecard](https://github.com/ossf/scorecard)

## Troubleshooting

### Common Issues

**"no matching signatures"**: The image might not be signed yet or you're using wrong certificate identity.

**"certificate identity mismatch"**: Make sure you're using the correct workflow path in the certificate identity regular expression.

**"OIDC issuer mismatch"**: Ensure you're using `https://token.actions.githubusercontent.com` as the issuer.

### Getting Help

If you have issues verifying signatures:

1. Check the [GitHub Actions workflow logs](https://github.com/openfero/openfero/actions)
2. Verify you're using the latest version of Cosign
3. Open an issue in the [OpenFero repository](https://github.com/openfero/openfero/issues)
