{{/*
Expand the name of the chart.
*/}}
{{- define "openfero.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "openfero.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "openfero.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "openfero.labels" -}}
helm.sh/chart: {{ include "openfero.chart" . }}
{{- if .Values.commonLabels }}
{{ toYaml .Values.commonLabels}}
{{- end }}
{{ include "openfero.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "openfero.selectorLabels" -}}
app.kubernetes.io/name: {{ include "openfero.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "openfero.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "openfero.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Determine if alertStoreType should be set
*/}}
{{- define "openfero.shouldSetAlertStoreType" -}}
{{- $customArgsHasAlertStoreType := false -}}
{{- range .Values.customArgs -}}
  {{- if contains "alertStoreType" . -}}
    {{- $customArgsHasAlertStoreType = true -}}
  {{- end -}}
{{- end -}}
{{- if and (not $customArgsHasAlertStoreType) (or .Values.autoscaling.enabled (gt (int .Values.replicaCount) 1)) }}true{{- end }}
{{- end }}
