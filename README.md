# Scheduled WOOP

Scheduled WOOP automatically changes CAST AI Workload Autoscaler settings by time of day.

Your workloads stay attached to one existing **managed policy**. The scheduler copies vertical recommendation settings and the apply mode from unassigned **source policies** into it. Policy identity, workload assignments, and HPA settings stay unchanged.

```text
Business Hours source (IMMEDIATE or DEFERRED) --\
                                                   > Scheduled WOOP --> Managed policy --> Workloads
Off Hours source (IMMEDIATE or DEFERRED) -------/
```

Example: use Business Hours settings from 08:00–18:00 Monday–Friday, and Off Hours settings at every other time.

## Before you install

You need:

- Kubernetes 1.19+ and Helm 3.14+.
- A CAST AI cluster with Workload Autoscaler enabled.
- A CAST AI API key that can read and update scaling policies.
- One managed policy already assigned to the workloads.
- Typically two unassigned source policies containing the settings to schedule.

Configure the vertical settings and **When to apply changes** on each source policy in CAST AI:

- `IMMEDIATE`: CAST AI may resize or restart affected workloads at the transition.
- `DEFERRED`: CAST AI waits until its normal deferred application point.

Scheduled WOOP uses whichever mode is configured on the active source policy. Test `IMMEDIATE` with a low-risk workload first.

Back up the managed and source policy JSON before the first installation. Do not assign source policies to workloads.

## Install

### 1. Create the API-key Secret

Save the key as `apikey.txt`, then run:

```bash
kubectl create namespace woop-scheduler-system
kubectl -n woop-scheduler-system create secret generic castai-api-credentials \
  --from-file=api-key=./apikey.txt
```

The key is mounted from the Secret. It is not stored in Helm values, the ConfigMap, image, or logs.

### 2. Create `values.yaml`

Replace the cluster ID and all three policy IDs:

```yaml
config:
  clusterId: your-cast-cluster-id
  timezone: Europe/Amsterdam
  pollInterval: 1m

  schedules:
    - name: production
      managedPolicyId: your-managed-policy-id
      defaultProfile:
        name: off-hours
        policyId: your-off-hours-source-policy-id
      windows:
        - name: business-hours
          days: [Monday, Tuesday, Wednesday, Thursday, Friday]
          start: "08:00"
          end: "18:00"
          profile:
            name: business-hours
            policyId: your-business-hours-source-policy-id
```

The default profile applies at nights and weekends. Times use the configured [IANA timezone](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones). Start is inclusive; end is exclusive. Overnight windows are supported.

Add more entries under `schedules` to manage multiple policies independently. Each managed policy must have its own source policies.

### 3. Install the chart

```bash
helm upgrade --install scheduled-woop \
  oci://ghcr.io/ronakforcast/charts/scheduled-woop \
  --version 0.2.0 \
  --namespace woop-scheduler-system \
  --values values.yaml \
  --wait
```

### 4. Verify

```bash
kubectl -n woop-scheduler-system get pods
kubectl -n woop-scheduler-system logs deployment/scheduled-woop-scheduled-woop -f
```

Healthy logs show `profile applied` or `policy already converged` for every schedule.

For the first pilot, watch one start and one end transition. Confirm:

- The correct source settings and `IMMEDIATE`/`DEFERRED` mode were copied.
- Managed policy ID, name, workload assignments, and HPA settings did not change.
- Source policies did not change.

## What happens at a transition

Every minute, the scheduler:

1. Selects the active source from the day, local time, and timezone.
2. Reads the source and managed policy from CAST AI.
3. Copies the source vertical recommendation settings and apply mode.
4. Preserves the managed name, assignment rules, and HPA settings.
5. Skips the update when the managed policy is already correct.
6. Reads the policy again after an update to verify it.

If the scheduler is down during a transition, CAST AI keeps the last settings. When the scheduler returns, it immediately applies the source that should be active now.

## Change or remove it

To change a schedule, edit `values.yaml` and run the same `helm upgrade` command.

Before uninstalling, wait until the source profile you want to leave active is shown as converged. Then run:

```bash
helm uninstall scheduled-woop --namespace woop-scheduler-system
```

Uninstalling does not change the current CAST AI policy settings. The API-key Secret is retained for explicit cleanup.

## Important safety rules

- Use only one scheduler for each managed policy.
- A policy cannot be both a managed policy and a source policy.
- Do not edit managed assignment rules or HPA settings while the scheduler is running. Pause the Deployment first, make and verify the edit, then resume it.
- External edits to managed vertical settings are replaced on the next poll.
- `IMMEDIATE` may restart or resize workloads. Validate customer rollout safeguards and availability expectations first.
- When upgrading from `0.1.0`, legacy `config.applyType: DEFERRED` continues forcing deferred mode. Remove it only after reviewing every source policy.

## Troubleshooting

- `401/403`: check the API key and its policy permissions.
- `404`: check the CAST cluster and policy IDs.
- Pod configuration error: check that the Secret exists in `woop-scheduler-system`.
- Startup validation error: check timezone, times, duplicate IDs, and overlapping windows.

More detail: [operations](OPERATIONS.md), [testing and pilot checklist](TESTING.md), [compatibility](docs/COMPATIBILITY.md), [validation evidence](docs/VALIDATION.md), and [security](SECURITY.md).

## Development

```bash
make all
make image
```

MIT License. See [LICENSE](LICENSE).
