{{- define "scheduled-woop.fullname" -}}
{{- printf "%s-scheduled-woop" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "scheduled-woop.image" -}}
{{- $repository := required "image.repository is required" .Values.image.repository -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" $repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" $repository (required "image.tag is required when image.digest is empty" .Values.image.tag) -}}
{{- end -}}
{{- end -}}
