# How does OpenFero work?

The following is an example of how OpenFero creates a Kubernetes job based on the alert received and the job definition from the configmap.

This should help you to better understand the behavior of OpenFero.

Below is the alert that was sent to OpenFero.

```json
{
  "version": "4",
  "groupKey": "{}/{alertname=\"KubeQuotaAlmostFull\"}:{alertname=\"KubeQuotaAlmostFull\", severity=\"info\", stage=\"dev\", zone=\"dmz\"}",
  "status": "firing",
  "receiver": "openfero",
  "groupLabels": {
    "alertname": "KubeQuotaAlmostFull",
    "severity": "info",
    "stage": "dev",
    "zone": "dmz"
  },
  "commonLabels": {
    "alertname": "KubeQuotaAlmostFull",
    "cluster": "dev-dmz",
    "container": "kube-state-metrics",
    "endpoint": "http",
    "pod": "mmop-kube-state-metrics-79fb6b966c-xrkgb",
    "prometheus": "monitoring/mmop-kube-prometheus-stack-prometheus",
    "resourcequota": "std-quota",
    "service": "mmop-kube-state-metrics",
    "severity": "info",
    "stage": "dev",
    "zone": "dmz"
  },
  "commonAnnotations": {
    "runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubequotaalmostfull",
    "summary": "Namespace quota is going to be full."
  },
  "externalURL": "http://alertmanager.example.com",
  "alerts": [
    {
      "labels": {
        "alertname": "KubeQuotaAlmostFull",
        "cluster": "dev-dmz",
        "container": "kube-state-metrics",
        "endpoint": "http",
        "namespace": "namespace",
        "pod": "mmop-kube-state-metrics-79fb6b966c-xrkgb",
        "prometheus": "monitoring/mmop-kube-prometheus-stack-prometheus",
        "resource": "requests.cpu",
        "resourcequota": "std-quota",
        "service": "mmop-kube-state-metrics",
        "severity": "info",
        "stage": "dev",
        "zone": "internal"
      },
      "annotations": {
        "description": "Namespace be is using 92.85% of its requests.cpu quota.",
        "runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubequotaalmostfull",
        "summary": "Namespace quota is going to be full."
      },
      "startsAt": "2021-10-25T12:01:24.29524738Z",
      "EndsAt": "0001-01-01T00:00:00Z"
    }
  ]
}
```

In combination with the following Operarios configuration...

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: openfero-kubequotaalmostfull-firing
  labels:
    app: openfero
data:
  KubeQuotaAlmostFull: |-
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: openfero-kubequotaalmostfull-firing
      labels:
        app: openfero
    spec:
      parallelism: 1
      completions: 1
      template:
          labels:
            app: openfero
          spec:
            containers:
            - name: python-job
              image: ubuntu:latest
              args:
              - bash
              - -c
              - |-
                echo "Hallo Welt"
            imagePullPolicy: Always
            restartPolicy: Never
```

Openfero would deploy a job as follows:

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: openfero-kubequotaalmostfull-firing
  labels:
    app: openfero
data:
  KubeQuotaAlmostFull: |-
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: openfero-kubequotaalmostfull-firing
      labels:
        app: openfero
    spec:
      parallelism: 1
      completions: 1
      template:
          labels:
            app: openfero
          spec:
            containers:
            - name: python-job
              image: ubuntu:latest
              args:
              - bash
              - -c
              - |-
                echo "Hallo Welt"
              env:
              - name: OPENFERO_ALERTNAME
                value: "KubeQuotaAlmostFull"
              - name: OPENFERO_CLUSTER
                value: "dev-dmz"
              - name: OPENFERO_CONTAINER
                value: "kube-state-metrics"
              - name: OPENFERO_ENDPOINT
                value: "http"
              # ... and so on for all other labels
              - name: OPENFERO_ZONE
                value: "internal"
            imagePullPolicy: Always
            restartPolicy: Never
```

In words, OpenFero takes the labels from the alert or alerts (if Alertmanager sends multiple alerts in the event) and adds them to the job as environment variables.

This allows you to make more specific decisions in the Operarios logic based on the information in the labels.

## Securing the Webhook /alerts Endpoint

OpenFero allows you to secure its `/alerts` POST endpoint using a bearer token. This prevents unauthorized parties from triggering jobs.

To enable authentication, start OpenFero with the `--authToken` command-line flag, providing a secret token:

```bash
./openfero --authToken="your-very-secret-token"
```

If the `--authToken` flag is not provided or is set to an empty string, authentication for the `/alerts` endpoint will be disabled, and OpenFero will accept requests without checking for a token.

### Sending Authenticated Requests

When authentication is enabled, clients sending alerts to the `/alerts` endpoint must include an `Authorization` header with the bearer token.

Here's an example using `curl`:

```bash
curl -X POST \
  http://localhost:8080/alerts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-very-secret-token" \
  -d '{
        "status": "firing",
        "receiver": "openfero",
        "alerts": [
          {
            "labels": {
              "alertname": "TestAlert",
              "service": "MyService"
            },
            "annotations": {
              "summary": "This is a test alert for authenticated endpoint"
            }
          }
        ]
      }'
```

Make sure to replace `your-very-secret-token` with the actual token you configured when starting OpenFero. If the token is missing or incorrect, OpenFero will respond with a `401 Unauthorized` error.
