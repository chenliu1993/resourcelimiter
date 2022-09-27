{{/*
Expand the name of the chart.
*/}}
{{- define "resourcelimiter-crd.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "resourcelimiter-crd.selectorLabels" -}}
app.kubernetes.io/name: {{ include "resourcelimiter-crd.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "resourcelimiter-crd.labels" -}}
helm.sh/chart: {{ include "resourcelimiter-crd.chart" . }}
{{ include "resourcelimiter-crd.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "resourcelimiter-crd.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}