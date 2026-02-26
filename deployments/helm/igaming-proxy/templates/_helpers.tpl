{{/*
Expand the name of the chart.
*/}}
{{- define "igaming-proxy.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "igaming-proxy.fullname" -}}
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
Common labels
*/}}
{{- define "igaming-proxy.labels" -}}
helm.sh/chart: {{ include "igaming-proxy.name" . }}-{{ .Chart.Version }}
{{ include "igaming-proxy.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "igaming-proxy.selectorLabels" -}}
app.kubernetes.io/name: {{ include "igaming-proxy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
