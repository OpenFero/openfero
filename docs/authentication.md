# Webhook Authentication

OpenFero supports authentication to secure the `/alerts` webhook endpoint.

## Quick Setup

### Basic Auth
```bash
./openfero --authMethod=basic --authBasicUser=admin --authBasicPass=secret123
```

### Bearer Token  
```bash
./openfero --authMethod=bearer --authBearerToken=my-api-key-12345
```

### No Auth (Default)
```bash
./openfero  # No authentication required
```

## Alertmanager Configuration

### Basic Auth
```yaml
receivers:
- name: 'openfero'
  webhook_configs:
  - url: 'http://admin:secret123@openfero:8080/alerts'
```

### Bearer Token
```yaml
receivers:
- name: 'openfero'  
  webhook_configs:
  - url: 'http://openfero:8080/alerts'
    http_config:
      authorization:
        type: Bearer
        credentials: my-api-key-12345
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
curl -u admin:secret123 -X POST http://localhost:8080/alerts -d '{}'

# With bearer token - should succeed
curl -H "Authorization: Bearer my-api-key-12345" -X POST http://localhost:8080/alerts -d '{}'
```
