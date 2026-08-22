{{- define "kargo-observability.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kargo-observability.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "kargo-observability.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kargo-observability.labels" -}}
app.kubernetes.io/name: {{ include "kargo-observability.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: delivery-kit
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "kargo-observability.prefix" -}}
{{- .Values.metricPrefix | default "kargo" -}}
{{- end -}}
