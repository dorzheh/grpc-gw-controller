{{- define "namespace" -}}
  {{- default .Values.namespace .Release.Namespace .Chart.Name -}}
{{- end -}}
