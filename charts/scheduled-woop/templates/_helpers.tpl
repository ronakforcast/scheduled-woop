{{- define "scheduled-woop.fullname" -}}
{{- printf "%s-scheduled-woop" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
