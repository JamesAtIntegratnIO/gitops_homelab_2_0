{{- define "delivery-agent.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "delivery-agent.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "delivery-agent.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "delivery-agent.labels" -}}
app.kubernetes.io/name: {{ include "delivery-agent.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: delivery-kit
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "delivery-agent.selectorLabels" -}}
app.kubernetes.io/name: {{ include "delivery-agent.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "delivery-agent.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "delivery-agent.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/* Digest wins over tag: a moving tag is how an unpinned image silently
     changes underneath you, which is the class of incident this whole kit
     exists to prevent. */}}
{{- define "delivery-agent.image" -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) -}}
{{- end -}}
{{- end -}}

{{/* Fail early and by name, rather than rendering something that cannot run. */}}
{{- define "delivery-agent.validate" -}}
{{- if not .Values.image.repository }}{{ fail "delivery-agent: image.repository is required" }}{{ end -}}
{{- if not .Values.git.owner }}{{ fail "delivery-agent: git.owner is required" }}{{ end -}}
{{- if not .Values.git.repo }}{{ fail "delivery-agent: git.repo is required" }}{{ end -}}
{{- if not .Values.git.repoURL }}{{ fail "delivery-agent: git.repoURL is required" }}{{ end -}}
{{- if not .Values.git.existingSecret }}{{ fail "delivery-agent: git.existingSecret is required -- this chart never creates a Secret" }}{{ end -}}
{{- if not .Values.llm.provider }}{{ fail "delivery-agent: llm.provider is required (openai or anthropic); there is deliberately no default" }}{{ end -}}
{{- if not .Values.llm.model }}{{ fail "delivery-agent: llm.model is required" }}{{ end -}}
{{- if and (eq .Values.llm.provider "openai") (not .Values.llm.baseURL) }}{{ fail "delivery-agent: llm.baseURL is required for the openai provider" }}{{ end -}}
{{- if not .Values.triage.allowPaths }}{{ fail "delivery-agent: triage.allowPaths is empty, so the agent could never apply a fix" }}{{ end -}}
{{- end -}}
