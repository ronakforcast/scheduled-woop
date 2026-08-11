# Operations

## Health and metrics

The chart exposes a cluster-internal Service on port `8080`:

- `/healthz` confirms the process and HTTP server are alive.
- `/readyz` confirms startup configuration and API-client initialization completed.
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

