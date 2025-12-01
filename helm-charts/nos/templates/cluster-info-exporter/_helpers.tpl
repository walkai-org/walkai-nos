{{/*
*********************************************************************
* Cluster Info Exporter
*********************************************************************
*/}}

{{- define "clusterInfoExporter.isEnabled" -}}
{{- and .Values.clusterInfoExporter.enabled (ne (trim (default "" .Values.clusterInfoExporter.config.endpoint)) "") -}}
{{- end -}}

{{- define "clusterInfoExporter.name" -}}
{{- default (printf "%s-%s" .Chart.Name "cluster-info-exporter") .Values.clusterInfoExporter.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "clusterInfoExporter.fullname" -}}
{{- if .Values.clusterInfoExporter.fullnameOverride }}
{{- .Values.clusterInfoExporter.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name "cluster-info-exporter" | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "clusterInfoExporter.labels" -}}
helm.sh/chart: {{ include "nos.chart" . }}
{{ include "clusterInfoExporter.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: nos
app.kubernetes.io/component: cluster-info-exporter
{{- end }}

{{- define "clusterInfoExporter.selectorLabels" -}}
app.kubernetes.io/name: {{ include "clusterInfoExporter.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "clusterInfoExporter.configMapName" -}}
{{- printf "%s-config" (include "clusterInfoExporter.fullname" .) -}}
{{- end -}}

{{- define "clusterInfoExporter.serviceAccountName" -}}
{{- if .Values.clusterInfoExporter.serviceAccount.name }}
{{- .Values.clusterInfoExporter.serviceAccount.name -}}
{{- else -}}
{{- include "clusterInfoExporter.fullname" . -}}
{{- end -}}
{{- end -}}

{{- define "clusterInfoExporter.secretName" -}}
{{- if .Values.clusterInfoExporter.secret.existingSecret }}
{{- .Values.clusterInfoExporter.secret.existingSecret -}}
{{- else if .Values.clusterInfoExporter.secret.name -}}
{{- .Values.clusterInfoExporter.secret.name -}}
{{- else -}}
{{- printf "%s-secret" (include "clusterInfoExporter.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
*********************************************************************
* API Client
*********************************************************************
*/}}

{{- define "clusterInfoExporter.apiClientNamespace" -}}
{{- default "walkai" .Values.clusterInfoExporter.apiClient.namespace -}}
{{- end -}}

{{- define "clusterInfoExporter.apiClientServiceAccountName" -}}
{{- default "api-client" .Values.clusterInfoExporter.apiClient.serviceAccountName -}}
{{- end -}}

{{- define "clusterInfoExporter.apiClientTokenSecretName" -}}
{{- default "api-client-permanent-token" .Values.clusterInfoExporter.apiClient.tokenSecretName -}}
{{- end -}}

{{- define "clusterInfoExporter.discoveryClusterRoleName" -}}
{{- default "discovery-minimal" .Values.clusterInfoExporter.apiClient.discoveryClusterRoleName -}}
{{- end -}}

{{- define "clusterInfoExporter.discoveryClusterRoleBindingName" -}}
{{- default "discovery-minimal-for-api-client" .Values.clusterInfoExporter.apiClient.discoveryClusterRoleBindingName -}}
{{- end -}}

{{- define "clusterInfoExporter.adminRoleBindingName" -}}
{{- default "api-client-admin" .Values.clusterInfoExporter.apiClient.adminRoleBindingName -}}
{{- end -}}

{{- define "clusterInfoExporter.adminClusterRoleName" -}}
{{- default "admin" .Values.clusterInfoExporter.apiClient.adminClusterRoleName -}}
{{- end -}}
