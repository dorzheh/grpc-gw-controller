{{/*
  Resolve the name of the common image repository.
  The value for .Values.repository is used by default,
  unless either override mechanism is used.

  - .Values.repository  : override default image repository for all images
  - .Values.repositoryOverride : override global and default image repository on a per image basis
*/}}
{{- define "repository" -}}
  {{if .Values.repositoryOverride }}
    {{- printf "%s" .Values.repositoryOverride -}}
  {{else}}
    {{- default .Values.repository .Values.repository -}}
  {{end}}
{{- end -}}


{{/*
  Resolve the image repository secret token.
  The value for .Values.repositoryCred is used:
  repositoryCred:
    user: user
    password: password
    mail: email (optional)
*/}}
{{- define "repository.secret" -}}
  {{- $repo := include "repository" . }}
  {{- $cred := .Values.repositoryCred }}
  {{- $mail := default "@" $cred.mail }}
  {{- $auth := printf "%s:%s" $cred.user $cred.password | b64enc }}
  {{- printf "{\"%s\":{\"username\":\"%s\",\"password\":\"%s\",\"email\":\"%s\",\"auth\":\"%s\"}}" $repo $cred.user $cred.password $mail $auth | b64enc -}}
{{- end -}}
