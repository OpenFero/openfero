# Webhook Authentication and Security

OpenFero supports multiple authentication methods to secure the `/alerts` webhook endpoint. This guide covers authentication setup, TLS configuration, and security best practices.

## Table of Contents

- [Quick Setup](#quick-setup)
- [Alertmanager Configuration](#alertmanager-configuration)
- [Kubernetes Deployment](#kubernetes-deployment)
- [TLS Configuration](#tls-configuration)
- [Mutual TLS (mTLS)](#mutual-tls-mtls)
- [Security Best Practices](#security-best-practices)
- [Helm Chart Configuration](#helm-chart-configuration)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)

## Quick Setup

### Basic Auth

```bash
./openfero --authMethod=basic --authBasicUser=admin --authBasicPass=your-password-here
```

### Bearer Token

```bash
./openfero --authMethod=bearer --authBearerToken=your-token-here
```

### No Auth (Default)

```bash
./openfero  # No authentication required
```

> **Warning**: Running without authentication is not recommended for production environments.

## Alertmanager Configuration

### Basic Auth

```yaml
receivers:
  - name: "openfero"
    webhook_configs:
      - url: "http://openfero:8080/alerts"
        http_config:
          basic_auth:
            username: admin
            password_file: /etc/alertmanager/openfero-password
```

Alternative with inline credentials (not recommended for production):

```yaml
receivers:
  - name: "openfero"
    webhook_configs:
      - url: "http://admin:your-password-here@openfero:8080/alerts"
```

### Bearer Token

```yaml
receivers:
  - name: "openfero"
    webhook_configs:
      - url: "http://openfero:8080/alerts"
        http_config:
          authorization:
            type: Bearer
            credentials_file: /etc/alertmanager/openfero-token
```

## Kubernetes Deployment

### Using Kubernetes Secrets

```yaml
# Create secret
apiVersion: v1
kind: Secret
metadata:
  name: openfero-auth
  namespace: openfero
type: Opaque
stringData:
  username: alertmanager
  password: your-secure-password-here  # Use: openssl rand -base64 32
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openfero
spec:
  template:
    spec:
      containers:
        - name: openfero
          args:
            - "--authMethod=basic"
            - "--authBasicUser=$(AUTH_USERNAME)"
            - "--authBasicPass=$(AUTH_PASSWORD)"
          env:
            - name: AUTH_USERNAME
              valueFrom:
                secretKeyRef:
                  name: openfero-auth
                  key: username
            - name: AUTH_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: openfero-auth
                  key: password
```

## TLS Configuration

TLS encryption is essential for production deployments to protect credentials in transit.

### Option 1: Self-Signed Certificates (Development/Testing)

#### Generate Certificates

```bash
# Create a directory for certificates
mkdir -p certs && cd certs

# Generate CA key and certificate
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 365 -key ca.key -out ca.crt \
  -subj "/CN=OpenFero CA"

# Generate server key and CSR
openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr \
  -subj "/CN=openfero.openfero.svc.cluster.local"

# Create server certificate signed by CA
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt \
  -extfile <(printf "subjectAltName=DNS:openfero,DNS:openfero.openfero.svc,DNS:openfero.openfero.svc.cluster.local,IP:127.0.0.1")
```

#### Create Kubernetes Secret

```bash
kubectl create secret tls openfero-tls \
  --cert=certs/server.crt \
  --key=certs/server.key \
  -n openfero

kubectl create secret generic openfero-ca \
  --from-file=ca.crt=certs/ca.crt \
  -n openfero
```

#### Alertmanager Configuration with TLS

```yaml
receivers:
  - name: "openfero"
    webhook_configs:
      - url: "https://openfero.openfero.svc.cluster.local:8443/alerts"
        http_config:
          basic_auth:
            username: alertmanager
            password_file: /etc/alertmanager/openfero-password
          tls_config:
            ca_file: /etc/alertmanager/openfero-ca.crt
            server_name: openfero.openfero.svc.cluster.local
```

### Option 2: cert-manager (Production)

[cert-manager](https://cert-manager.io/) automates certificate management in Kubernetes.

#### Install cert-manager

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

#### Create Issuer

```yaml
# Self-signed issuer for internal services
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: openfero-selfsigned
  namespace: openfero
spec:
  selfSigned: {}
---
# CA Certificate
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: openfero-ca
  namespace: openfero
spec:
  isCA: true
  commonName: openfero-ca
  secretName: openfero-ca-secret
  privateKey:
    algorithm: ECDSA
    size: 256
  issuerRef:
    name: openfero-selfsigned
    kind: Issuer
---
# CA Issuer using the generated CA
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: openfero-ca-issuer
  namespace: openfero
spec:
  ca:
    secretName: openfero-ca-secret
```

#### Create Server Certificate

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: openfero-server
  namespace: openfero
spec:
  secretName: openfero-tls
  duration: 8760h  # 1 year
  renewBefore: 720h  # 30 days
  subject:
    organizations:
      - OpenFero
  commonName: openfero
  dnsNames:
    - openfero
    - openfero.openfero.svc
    - openfero.openfero.svc.cluster.local
  issuerRef:
    name: openfero-ca-issuer
    kind: Issuer
```

## Mutual TLS (mTLS)

mTLS provides the highest level of security by requiring both client and server to present certificates.

### Create Client Certificate

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: alertmanager-client
  namespace: monitoring  # Alertmanager namespace
spec:
  secretName: alertmanager-client-tls
  duration: 8760h
  renewBefore: 720h
  subject:
    organizations:
      - Alertmanager
  commonName: alertmanager
  usages:
    - client auth
  issuerRef:
    name: openfero-ca-issuer
    kind: Issuer
    group: cert-manager.io
```

### Alertmanager mTLS Configuration

```yaml
receivers:
  - name: "openfero"
    webhook_configs:
      - url: "https://openfero.openfero.svc.cluster.local:8443/alerts"
        http_config:
          tls_config:
            ca_file: /etc/alertmanager/openfero-ca.crt
            cert_file: /etc/alertmanager/client.crt
            key_file: /etc/alertmanager/client.key
            server_name: openfero.openfero.svc.cluster.local
```

### Mount Certificates in Alertmanager

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alertmanager
spec:
  template:
    spec:
      containers:
        - name: alertmanager
          volumeMounts:
            - name: openfero-ca
              mountPath: /etc/alertmanager/openfero-ca.crt
              subPath: ca.crt
              readOnly: true
            - name: client-certs
              mountPath: /etc/alertmanager/client.crt
              subPath: tls.crt
              readOnly: true
            - name: client-certs
              mountPath: /etc/alertmanager/client.key
              subPath: tls.key
              readOnly: true
      volumes:
        - name: openfero-ca
          secret:
            secretName: openfero-ca-secret
        - name: client-certs
          secret:
            secretName: alertmanager-client-tls
```

## Security Best Practices

### Production Checklist

- [ ] **Enable TLS** - Never expose HTTP endpoints in production
- [ ] **Use strong passwords** - Minimum 32 characters, generated randomly
- [ ] **Store secrets securely** - Use Kubernetes Secrets or external secret managers
- [ ] **Rotate credentials regularly** - At least every 90 days
- [ ] **Limit network access** - Use NetworkPolicies to restrict traffic
- [ ] **Enable audit logging** - Track authentication attempts
- [ ] **Use mTLS for highest security** - When compliance requires it

### Generate Strong Passwords

```bash
# Generate a 32-character random password
openssl rand -base64 32

# Generate a 64-character hex token
openssl rand -hex 32
```

### NetworkPolicy Example

Restrict who can access OpenFero:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: openfero-ingress
  namespace: openfero
spec:
  podSelector:
    matchLabels:
      app: openfero
  policyTypes:
    - Ingress
  ingress:
    - from:
        # Only allow Alertmanager
        - namespaceSelector:
            matchLabels:
              name: monitoring
          podSelector:
            matchLabels:
              app: alertmanager
        # And Prometheus (for metrics scraping)
        - namespaceSelector:
            matchLabels:
              name: monitoring
          podSelector:
            matchLabels:
              app: prometheus
      ports:
        - protocol: TCP
          port: 8080
```

### Secret Management with External Secrets

For production, consider using [External Secrets Operator](https://external-secrets.io/):

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: openfero-auth
  namespace: openfero
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: openfero-auth
  data:
    - secretKey: username
      remoteRef:
        key: openfero/auth
        property: username
    - secretKey: password
      remoteRef:
        key: openfero/auth
        property: password
```

## Helm Chart Configuration

### Basic Auth with TLS

```yaml
# values.yaml
auth:
  enabled: true
  method: basic
  basic:
    username: alertmanager
    # Reference existing secret
    existingSecret: openfero-auth
    secretKey: password

tls:
  enabled: true
  # Use cert-manager
  certManager:
    enabled: true
    issuerRef:
      name: letsencrypt-prod
      kind: ClusterIssuer
  # Or use existing secret
  existingSecret: openfero-tls
```

### Full Production Example

```yaml
# values.yaml for production deployment
replicaCount: 3

auth:
  enabled: true
  method: basic
  basic:
    existingSecret: openfero-auth

tls:
  enabled: true
  certManager:
    enabled: true
    issuerRef:
      name: internal-ca
      kind: ClusterIssuer

networkPolicy:
  enabled: true
  allowedNamespaces:
    - monitoring

podDisruptionBudget:
  enabled: true
  minAvailable: 2

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

## Testing

### Test Authentication

```bash
# Without auth - should return 401
curl -X POST http://localhost:8080/alerts \
  -H "Content-Type: application/json" \
  -d '{"alerts":[]}'
# Expected: 401 Unauthorized

# With basic auth - should return 200
curl -u alertmanager:your-password \
  -X POST http://localhost:8080/alerts \
  -H "Content-Type: application/json" \
  -d '{"alerts":[]}'
# Expected: 200 OK

# With bearer token - should return 200
curl -H "Authorization: Bearer your-token" \
  -X POST http://localhost:8080/alerts \
  -H "Content-Type: application/json" \
  -d '{"alerts":[]}'
# Expected: 200 OK
```

### Test TLS

```bash
# Test with self-signed CA
curl --cacert ca.crt \
  -u alertmanager:password \
  -X POST https://localhost:8443/alerts \
  -H "Content-Type: application/json" \
  -d '{"alerts":[]}'

# Test mTLS
curl --cacert ca.crt \
  --cert client.crt \
  --key client.key \
  -X POST https://localhost:8443/alerts \
  -H "Content-Type: application/json" \
  -d '{"alerts":[]}'
```

### Verify Certificate

```bash
# Check server certificate
openssl s_client -connect localhost:8443 -CAfile ca.crt

# Verify certificate chain
openssl verify -CAfile ca.crt server.crt
```

## Troubleshooting

### Common Issues

#### 401 Unauthorized

**Symptoms:** All requests return 401

**Solutions:**
1. Verify credentials are correct
2. Check if auth method matches (basic vs bearer)
3. Ensure secrets are mounted correctly

```bash
# Debug: Check if secret exists
kubectl get secret openfero-auth -n openfero -o yaml

# Debug: Check environment variables in pod
kubectl exec -it deploy/openfero -n openfero -- env | grep AUTH
```

#### Certificate Errors

**Symptoms:** `x509: certificate signed by unknown authority`

**Solutions:**
1. Ensure CA certificate is correctly configured in Alertmanager
2. Verify server certificate DNS names match the URL
3. Check certificate expiration

```bash
# Check certificate details
openssl x509 -in server.crt -text -noout

# Verify certificate is valid
openssl verify -CAfile ca.crt server.crt
```

#### Connection Refused on TLS Port

**Symptoms:** Cannot connect to port 8443

**Solutions:**
1. Verify TLS is enabled in OpenFero configuration
2. Check if certificates are mounted correctly
3. Verify port is exposed in Service

```bash
# Check OpenFero logs
kubectl logs -l app=openfero -n openfero

# Verify TLS secret exists
kubectl get secret openfero-tls -n openfero
```

### Enable Debug Logging

```bash
./openfero --logLevel=debug --authMethod=basic ...
```

This will log authentication attempts and failures for debugging.
