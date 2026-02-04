{{/*
Expand the name of the chart.
*/}}
{{- define "kubepolaris.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "kubepolaris.fullname" -}}
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
{{- define "kubepolaris.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kubepolaris.labels" -}}
helm.sh/chart: {{ include "kubepolaris.chart" . }}
{{ include "kubepolaris.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kubepolaris.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubepolaris.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Backend labels
*/}}
{{- define "kubepolaris.backend.labels" -}}
{{ include "kubepolaris.labels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Backend selector labels
*/}}
{{- define "kubepolaris.backend.selectorLabels" -}}
{{ include "kubepolaris.selectorLabels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Frontend labels
*/}}
{{- define "kubepolaris.frontend.labels" -}}
{{ include "kubepolaris.labels" . }}
app.kubernetes.io/component: frontend
{{- end }}

{{/*
Frontend selector labels
*/}}
{{- define "kubepolaris.frontend.selectorLabels" -}}
{{ include "kubepolaris.selectorLabels" . }}
app.kubernetes.io/component: frontend
{{- end }}

{{/*
MySQL labels
*/}}
{{- define "kubepolaris.mysql.labels" -}}
{{ include "kubepolaris.labels" . }}
app.kubernetes.io/component: mysql
{{- end }}

{{/*
MySQL selector labels
*/}}
{{- define "kubepolaris.mysql.selectorLabels" -}}
{{ include "kubepolaris.selectorLabels" . }}
app.kubernetes.io/component: mysql
{{- end }}

{{/*
ServiceAccount name
*/}}
{{- define "kubepolaris.serviceAccountName" -}}
{{- if .Values.backend.serviceAccount.create }}
{{- default (include "kubepolaris.fullname" .) .Values.backend.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.backend.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image name with registry
*/}}
{{- define "kubepolaris.image" -}}
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
{{- define "kubepolaris.mysql.secretName" -}}
{{- if .Values.mysql.internal.existingSecret }}
{{- .Values.mysql.internal.existingSecret }}
{{- else if .Values.mysql.external.existingSecret }}
{{- .Values.mysql.external.existingSecret }}
{{- else }}
{{- include "kubepolaris.fullname" . }}-mysql
{{- end }}
{{- end }}

{{/*
JWT secret name
*/}}
{{- define "kubepolaris.jwt.secretName" -}}
{{- if .Values.security.existingSecret }}
{{- .Values.security.existingSecret }}
{{- else }}
{{- include "kubepolaris.fullname" . }}-secrets
{{- end }}
{{- end }}

{{/*
Grafana secret name
*/}}
{{- define "kubepolaris.grafana.secretName" -}}
{{- if .Values.grafana.existingSecret }}
{{- .Values.grafana.existingSecret }}
{{- else }}
{{- include "kubepolaris.fullname" . }}-grafana
{{- end }}
{{- end }}

{{/*
Database host
*/}}
{{- define "kubepolaris.database.host" -}}
{{- if .Values.mysql.external.enabled }}
{{- .Values.mysql.external.host }}
{{- else }}
{{- printf "%s-mysql" (include "kubepolaris.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Database port
*/}}
{{- define "kubepolaris.database.port" -}}
{{- if .Values.mysql.external.enabled }}
{{- .Values.mysql.external.port }}
{{- else }}
{{- print "3306" }}
{{- end }}
{{- end }}

{{/*
Database name
*/}}
{{- define "kubepolaris.database.name" -}}
{{- if .Values.mysql.external.enabled }}
{{- .Values.mysql.external.database }}
{{- else }}
{{- .Values.mysql.internal.database }}
{{- end }}
{{- end }}

{{/*
Database username
*/}}
{{- define "kubepolaris.database.username" -}}
{{- if .Values.mysql.external.enabled }}
{{- .Values.mysql.external.username }}
{{- else }}
{{- .Values.mysql.internal.username }}
{{- end }}
{{- end }}

{{/*
Return the proper image pull secrets
*/}}
{{- define "kubepolaris.imagePullSecrets" -}}
{{- if .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- range .Values.global.imagePullSecrets }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}
