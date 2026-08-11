#!/usr/bin/env bash
set -euo pipefail

chart="charts/scheduled-woop"
rendered="$(helm template production "$chart" --namespace woop-scheduler-system)"

require_text() {
  if ! grep -Fq -- "$1" <<<"$rendered"; then
    echo "expected rendered chart to contain: $1" >&2
    exit 1
  fi
}

reject_text() {
  if grep -Fq -- "$1" <<<"$rendered"; then
    echo "expected rendered chart not to contain: $1" >&2
    exit 1
  fi
}

require_text "automountServiceAccountToken: false"
require_text "type: Recreate"
require_text "containerPort: 8080"
require_text "path: /healthz"
require_text "path: /readyz"
require_text "kind: Service"
require_text "name: metrics"
require_text "runAsNonRoot: true"
require_text "runAsUser: 65532"
require_text "runAsGroup: 65532"
require_text "readOnlyRootFilesystem: true"
require_text "allowPrivilegeEscalation: false"
require_text 'capabilities: {drop: ["ALL"]}'
require_text "secretName: castai-api-credentials"
require_text "checksum/config:"
require_text "kind: ConfigMap"

reject_text "kind: ServiceAccount"
reject_text "kind: Role"
reject_text "kind: ClusterRole"
reject_text "api-key:"

if helm template production "$chart" --namespace woop-scheduler-system --set existingSecret= >/dev/null 2>&1; then
  echo "expected an empty existingSecret to fail rendering" >&2
  exit 1
fi

digest="sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
digest_rendered="$(helm template production "$chart" --namespace woop-scheduler-system --set-string image.digest="$digest")"
if ! grep -Fq -- "image: \"ghcr.io/ronakforcast/scheduled-woop@$digest\"" <<<"$digest_rendered"; then
  echo "expected image digest to override the tag" >&2
  exit 1
fi

alerts_rendered="$(helm template production "$chart" --namespace woop-scheduler-system --set monitoring.alerts.enabled=true)"
for expected in "kind: PrometheusRule" "ScheduledWoopUnavailable" "ScheduledWoopReconciliationFailures"; do
  if ! grep -Fq -- "$expected" <<<"$alerts_rendered"; then
    echo "expected enabled alerts to contain: $expected" >&2
    exit 1
  fi
done

echo "Helm security contract passed"
