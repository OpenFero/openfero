# Webhook Authentication

OpenFero supports authentication to secure the `/alerts` webhook endpoint.

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

## Alertmanager Configuration

### Basic Auth Alertmanager

```yaml
receivers:
- name: 'openfero'
  webhook_configs:
  - url: 'http://admin:your-password-here@openfero:8080/alerts'
```

### Bearer Token Alertmanager

```yaml
receivers:
- name: 'openfero'  
  webhook_configs:
  - url: 'http://openfero:8080/alerts'
    http_config:
      authorization:
        type: Bearer
        credentials: your-token-here
```

## Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openfero
spec:
  template:
    spec:
      containers:
      - name: openfero
        args: ["--authMethod=bearer", "--authBearerToken=$(TOKEN)"]
        env:
        - name: TOKEN
          valueFrom:
            secretKeyRef:
              name: openfero-auth
              key: token
```

## Testing

```bash
# Without auth - should fail
curl -X POST http://localhost:8080/alerts -d '{}'

# With basic auth - should succeed  
curl -u admin:your-password-here -X POST http://localhost:8080/alerts -d '{}'

# With bearer token - should succeed
curl -H "Authorization: Bearer your-token-here" -X POST http://localhost:8080/alerts -d '{}'
```
