# Operations

## Health and metrics

The chart exposes a cluster-internal Service on port `8080`:

- `/healthz` confirms the process and HTTP server are alive.
- `/readyz` confirms startup configuration, API-client initialization, and the HTTP listener completed. It intentionally remains Ready during CAST AI authentication or availability failures so metrics remain scrapeable; use reconciliation metrics and alerts for dependency health.
- `/metrics` exposes Prometheus text metrics for successful/failed reconciliations, successful policy PUTs, and the last successful reconciliation time.

To inspect locally:

```bash
kubectl -n woop-scheduler-system port-forward service/scheduled-woop-scheduled-woop 8080:8080
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/metrics
```

Services have Prometheus scrape annotations by default. If the Prometheus Operator and `PrometheusRule` CRD are installed, enable the example availability and sustained-failure alerts:

```yaml
monitoring:
  alerts:
    enabled: true
```

Verify the alert route during the pilot by temporarily using an invalid API key, observing the release-scoped reconciliation alert, and then rotating back to the valid key.

## Pin the image digest

For production, pin the approved multi-architecture manifest digest:

```yaml
image:
  repository: ghcr.io/ronakforcast/scheduled-woop
  digest: sha256:replace-with-approved-digest
```

When `digest` is set, it takes precedence over `tag`.

## Rotate the CAST AI API key

Create the replacement key with the same least-privilege policy read/update permissions. Then replace the Secret and restart the Deployment:

```bash
kubectl -n woop-scheduler-system create secret generic castai-api-credentials \
  --from-file=api-key=./new-apikey.txt \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n woop-scheduler-system rollout restart deployment/scheduled-woop-scheduled-woop
kubectl -n woop-scheduler-system rollout status deployment/scheduled-woop-scheduled-woop
```

Verify `/readyz` and the reconciliation logs, then revoke the old key. Never put either key in Helm values.

## Upgrade and rollback

Before an upgrade, save the active values and the full JSON for every managed and source policy. Upgrade with an explicit chart version and image digest.

```bash
helm get values scheduled-woop -n woop-scheduler-system > scheduled-woop-values-backup.yaml
helm history scheduled-woop -n woop-scheduler-system

helm upgrade scheduled-woop \
  oci://ghcr.io/ronakforcast/charts/scheduled-woop \
  --version VERSION \
  --namespace woop-scheduler-system \
  --values values.yaml \
  --wait
```

To roll back the Kubernetes release:

```bash
helm rollback scheduled-woop REVISION -n woop-scheduler-system --wait
```

Helm rollback does not restore CAST AI policy settings already applied by a previous schedule. To restore a desired profile, configure it as the default source, run a Helm upgrade, and wait for `policy already converged` before completing the rollback.

### Version 0.1 migration

An existing global `config.applyType: DEFERRED` remains a safety override. Review every source policy first, then remove that value only when you intend each source policy's own `IMMEDIATE` or `DEFERRED` mode to take effect.

## Pilot ownership and escalation

Record these names in the customer change ticket before installation:

- Customer change-window owner: approves start, stop, and workload impact.
- Rollback owner: holds the policy backups and can pause the Deployment and restore the intended source profile.
- CAST AI solution owner: reviews scheduler logs and policy state.
- Customer incident contact: receives availability or workload-impact escalation.

On reconciliation failures, pause the Deployment if policy state is uncertain, preserve logs and policy JSON, and contact the CAST AI solution owner. For suspected credential exposure, revoke the key immediately and follow [SECURITY.md](SECURITY.md). This preview has no implied 24/7 support channel; response targets must be agreed in the customer change ticket.
