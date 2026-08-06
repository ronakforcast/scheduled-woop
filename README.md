# Scheduled WOOP

Schedule CAST AI Workload Autoscaler policy settings with one small Kubernetes Deployment and one ConfigMap.

Scheduled WOOP keeps workloads assigned to their existing CAST AI policy. At each scheduled window it copies vertical recommendation settings from a source policy into that stable managed policy. Its name, assignment rules, and HPA settings are preserved.

## What you need

- Kubernetes with [Helm 3](https://helm.sh/docs/intro/install/).
- A CAST AI cluster with Workload Autoscaler enabled.
- A CAST AI API key allowed to read and update workload scaling policies.
- For each application:
  - One **managed policy** already assigned to its workloads.
  - One or more **source policies** configured with the settings you want to schedule.

Source policies are templates. They should not themselves be managed by another schedule.

## Five-minute installation

### 1. Create a namespace and API-key Secret

Save the CAST AI key in a local file named `apikey.txt`, then run:

```bash
kubectl create namespace woop-scheduler-system
kubectl -n woop-scheduler-system create secret generic castai-api-credentials \
  --from-file=api-key=./apikey.txt
```

The key is mounted directly from the Secret. It is never placed in Helm values, a ConfigMap, an image, or a log.

### 2. Create your values file

Download [examples/values.yaml](examples/values.yaml) or copy it from a clone of this repository. Replace the placeholder cluster and policy IDs.

```yaml
config:
  clusterId: your-cast-cluster-id
  timezone: Europe/Prague
  pollInterval: 1m
  applyType: DEFERRED

  schedules:
    - name: application-one
      managedPolicyId: policy-a
      defaultProfile:
        name: off-hours
        policyId: policy-c
      windows:
        - name: business-hours
          days: [Monday, Tuesday, Wednesday, Thursday, Friday]
          start: "08:00"
          end: "18:00"
          profile:
            name: daytime
            policyId: policy-b

    - name: application-two
      managedPolicyId: policy-d
      defaultProfile:
        name: normal
        policyId: policy-f
      windows:
        - name: batch-window
          days: [Saturday, Sunday]
          start: "20:00"
          end: "06:00"
          profile:
            name: batch
            policyId: policy-e
```

This means:

- Managed policy A uses B during business hours and C otherwise.
- Managed policy D uses E during the batch window and F otherwise.

Overnight windows are supported. Window starts are inclusive and ends are exclusive in the configured IANA timezone.

### 3. Install

Install the published chart:

```bash
helm upgrade --install scheduled-woop \
  oci://ghcr.io/ronakforcast/charts/scheduled-woop \
  --version 0.1.0 \
  --namespace woop-scheduler-system \
  --values examples/values.yaml \
  --wait
```

Or install from a clone:

```bash
git clone https://github.com/ronakforcast/scheduled-woop.git
cd scheduled-woop
helm upgrade --install scheduled-woop ./charts/scheduled-woop \
  --namespace woop-scheduler-system \
  --values examples/values.yaml \
  --wait
```

### 4. Verify

```bash
kubectl -n woop-scheduler-system get pods
kubectl -n woop-scheduler-system logs deployment/scheduled-woop-scheduled-woop -f
```

Healthy logs show either `profile applied` or `policy already converged` for each schedule.

## How reconciliation works

Every poll, each schedule is handled independently:

1. Select the active source policy from local day/time and timezone.
2. Read the source and managed policies from CAST AI.
3. Copy only `recommendationPolicies` from the source.
4. Preserve the managed policy's name, assignment rules, and HPA settings.
5. Force `DEFERRED` application.
6. Skip the write if it is already correct.
7. Otherwise update once and read back to verify.

If one schedule fails, the remaining schedules still run.

## Change a schedule

Edit the values file and repeat the Helm upgrade command. A ConfigMap checksum automatically rolls the pod. On startup, missed transitions converge immediately.

## Uninstall safely

Uninstalling leaves the currently active CAST AI settings unchanged. To leave a specific profile active, make it the default profile, upgrade, wait for `policy already converged`, and then run:

```bash
helm uninstall scheduled-woop --namespace woop-scheduler-system
```

The API-key Secret is intentionally retained. Delete it separately if no longer needed.

## Safety and limitations

- Only `DEFERRED` mode is supported.
- HPA settings are preserved, not scheduled.
- A policy cannot be both a managed target and a source template.
- Two schedules cannot manage the same policy.
- External edits to managed vertical settings are overwritten on the next poll because the scheduler owns those settings.
- The process has no Kubernetes API token or RBAC permissions.
- The container runs read-only as numeric non-root UID/GID `65532`.

## Troubleshooting

- `HTTP 401/403`: verify the Secret and CAST AI API-key permissions.
- `HTTP 404`: verify cluster and policy IDs.
- Pod configuration error: confirm `castai-api-credentials` exists in the release namespace.
- Startup validation failure: check timezone, unique names/managed IDs, local times, and overlapping windows.

## Development

```bash
make all          # race tests, binary build, Helm lint/render
make image        # local container image
make helm-package # package chart under bin/
```

## License

MIT License. See [LICENSE](LICENSE).
