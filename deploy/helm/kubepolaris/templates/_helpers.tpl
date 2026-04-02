{{/*
Expand the name of the chart.
*/}}
{{- define "synapse.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "synapse.fullname" -}}
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
{{- define "synapse.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "synapse.labels" -}}
helm.sh/chart: {{ include "synapse.chart" . }}
{{ include "synapse.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "synapse.selectorLabels" -}}
app.kubernetes.io/name: {{ include "synapse.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Backend labels
*/}}
{{- define "synapse.backend.labels" -}}
{{ include "synapse.labels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Backend selector labels
*/}}
{{- define "synapse.backend.selectorLabels" -}}
{{ include "synapse.selectorLabels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Frontend labels
*/}}
{{- define "synapse.frontend.labels" -}}
{{ include "synapse.labels" . }}
app.kubernetes.io/component: frontend
{{- end }}

{{/*
Frontend selector labels
*/}}
{{- define "synapse.frontend.selectorLabels" -}}
{{ include "synapse.selectorLabels" . }}
app.kubernetes.io/component: frontend
{{- end }}

{{/*
MySQL labels
*/}}
{{- define "synapse.mysql.labels" -}}
{{ include "synapse.labels" . }}
app.kubernetes.io/component: mysql
{{- end }}

{{/*
MySQL selector labels
*/}}
{{- define "synapse.mysql.selectorLabels" -}}
{{ include "synapse.selectorLabels" . }}
app.kubernetes.io/component: mysql
{{- end }}

{{/*
ServiceAccount name
*/}}
{{- define "synapse.serviceAccountName" -}}
{{- if .Values.backend.serviceAccount.create }}
{{- default (include "synapse.fullname" .) .Values.backend.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.backend.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image name with registry
*/}}
{{- define "synapse.image" -}}
{{- $registry := .Values.global.imageRegistry | default "" }}
{{- $repository := .repository }}
{{- $tag := .tag | default $.Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
MySQL password secret name
*/}}
{{- define "synapse.mysql.secretName" -}}
{{- if .Values.mysql.internal.existingSecret }}
{{- .Values.mysql.internal.existingSecret }}
{{- else if .Values.mysql.external.existingSecret }}
{{- .Values.mysql.external.existingSecret }}
{{- else }}
{{- include "synapse.fullname" . }}-mysql
{{- end }}
{{- end }}

{{/*
JWT secret name
*/}}
{{- define "synapse.jwt.secretName" -}}
{{- if .Values.security.existingSecret }}
{{- .Values.security.existingSecret }}
{{- else }}
{{- include "synapse.fullname" . }}-secrets
{{- end }}
{{- end }}

{{/*
Grafana secret name
*/}}
{{- define "synapse.grafana.secretName" -}}
{{- if .Values.grafana.existingSecret }}
{{- .Values.grafana.existingSecret }}
{{- else }}
{{- include "synapse.fullname" . }}-grafana
{{- end }}
{{- end }}

{{/*
Database host
*/}}
{{- define "synapse.database.host" -}}
{{- if .Values.mysql.external.enabled }}
{{- .Values.mysql.external.host }}
{{- else }}
{{- printf "%s-mysql" (include "synapse.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Database port
*/}}
{{- define "synapse.database.port" -}}
{{- if .Values.mysql.external.enabled }}
{{- .Values.mysql.external.port }}
{{- else }}
{{- print "3306" }}
{{- end }}
{{- end }}

{{/*
Database name
*/}}
{{- define "synapse.database.name" -}}
{{- if .Values.mysql.external.enabled }}
{{- .Values.mysql.external.database }}
{{- else }}
{{- .Values.mysql.internal.database }}
{{- end }}
{{- end }}

{{/*
Database username
*/}}
{{- define "synapse.database.username" -}}
{{- if .Values.mysql.external.enabled }}
{{- .Values.mysql.external.username }}
{{- else }}
{{- .Values.mysql.internal.username }}
{{- end }}
{{- end }}

{{/*
Return the proper image pull secrets
*/}}
{{- define "synapse.imagePullSecrets" -}}
{{- if .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- range .Values.global.imagePullSecrets }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}
