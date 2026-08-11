# Scheduled WOOP

Schedule CAST AI Workload Autoscaler policy settings with one small Kubernetes Deployment and one ConfigMap.

Scheduled WOOP keeps workloads assigned to their existing CAST AI policy. At each scheduled window it copies vertical recommendation settings from a source policy into that stable managed policy. Its name, assignment rules, and HPA settings are preserved.

## How it works

Your workloads never switch policies. They stay assigned to the existing **managed policy**. Scheduled WOOP changes only the vertical recommendation settings inside that policy by copying them from unassigned **source policies**, which act as templates.

If you currently have one policy named `Production`, keep it assigned to your workloads and create two additional, unassigned policies:

- `Business Hours`, configured for daytime scaling.
- `Off Hours`, configured for nights and weekends.

Set **When to apply changes** independently on each source policy. For example, `Business Hours` may use `IMMEDIATE` while `Off Hours` uses `DEFERRED`; Scheduled WOOP copies the active source policy's mode along with its vertical recommendation settings.

If upgrading from `0.1.0`, an existing global `config.applyType: DEFERRED` continues forcing all scheduled profiles to deferred mode. Review every source policy, then remove the global value to explicitly opt into source-controlled modes.

```text
+-------------+  +-------------+
| Business    |  | Off Hours   |
| Hours source|  | source      |
+------|------+  +------|------+
       |                |
       | 08:00-18:00    | nights and weekends
       | Monday-Friday  |
       +--------+-------+
                |
                v
       Scheduled WOOP selects
       the active source settings
                |
                | copies settings into
                v
       +--------------------------+
       | Production policy        |
       | Existing managed policy  |
       +------------|-------------+
                    | always assigned to
                    v
             Customer workloads
```

For this example, the weekly timeline is:

```text
Monday-Friday

00:00--------------08:00----------------18:00--------------24:00
       Off Hours            Business Hours         Off Hours

Saturday-Sunday

00:00------------------------------------------------------24:00
                         Off Hours
```

At 08:00 on a weekday, Scheduled WOOP copies the `Business Hours` source settings into `Production`. At 18:00, it copies the `Off Hours` source settings into `Production`. The `Production` policy keeps its name, workload assignment rules, and HPA settings throughout.

If the active source uses `IMMEDIATE`, CAST AI may restart or resize affected workloads as soon as the scheduled settings generate applicable recommendations. Use CAST AI rollout safeguards and test the transition against representative workloads before production.

If the scheduler is unavailable at a transition, the current settings remain active. When it starts again, it calculates which window is active and immediately converges to the correct settings.

## What you need

- Kubernetes 1.19+ with [Helm 3.14+](https://helm.sh/docs/intro/install/). See [compatibility](docs/COMPATIBILITY.md).
- A CAST AI cluster with Workload Autoscaler enabled.
- A CAST AI API key allowed to read and update workload scaling policies.
- For each application:
  - One **managed policy** already assigned to its workloads.
  - One or more **source policies** configured with the settings you want to schedule.

Source policies are templates. They should not themselves be managed by another schedule.

## Preflight checklist

Before installing for a customer, confirm:

- The managed policy is the only policy assigned to the target workloads.
- Every source policy is unassigned and has the intended vertical settings and apply mode.
- The managed and source policy JSON has been backed up.
- Schedule windows do not overlap, the IANA timezone is correct, and the default profile covers all other time.
- Only one Scheduled WOOP release will own each managed policy.
- A customer change-window owner and a rollback owner are present for the pilot.
- No one will edit managed assignment rules or HPA settings during a transition.
- `IMMEDIATE` sources have been accepted against a representative, low-risk workload.

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

  schedules:
    - name: production
      managedPolicyId: production-policy-id
      defaultProfile:
        name: off-hours
        policyId: off-hours-policy-id
      windows:
        - name: business-hours
          days: [Monday, Tuesday, Wednesday, Thursday, Friday]
          start: "08:00"
          end: "18:00"
          profile:
            name: business-hours
            policyId: business-hours-policy-id
```

This means:

- `Production` uses the `Business Hours` source settings from 08:00 until 18:00 on weekdays.
- `Production` uses the `Off Hours` source settings at all other times, including weekends.

Overnight windows are supported. Window starts are inclusive and ends are exclusive in the configured IANA timezone.

### 3. Install

Install the published chart:

```bash
helm upgrade --install scheduled-woop \
  oci://ghcr.io/ronakforcast/charts/scheduled-woop \
  --version 0.2.0 \
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

For the first customer pilot, observe one start and one end transition. At both boundaries verify the managed policy's recommendation settings and apply mode, then prove its ID, name, workload assignments, and HPA settings did not change. Also compare both source policies with the backups. Use the focused acceptance checklist in [TESTING.md](TESTING.md).

## How reconciliation works

Every poll, each schedule is handled independently:

1. Select the active source policy from local day/time and timezone.
2. Read the source and managed policies from CAST AI.
3. Copy only `recommendationPolicies` from the source.
4. Preserve the managed policy's name, assignment rules, and HPA settings.
5. Copy the active source policy's `IMMEDIATE` or `DEFERRED` application mode.
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

- Each source policy controls whether its scheduled settings use `IMMEDIATE` or `DEFERRED` mode.
- `IMMEDIATE` can restart or resize workloads at a scheduled transition. Configure CAST AI rollout safeguards and validate disruption behavior before using it in production.
- HPA settings are preserved, not scheduled.
- A policy cannot be both a managed target and a source template.
- Two schedules cannot manage the same policy.
- Run only one Scheduled WOOP release for a managed policy. Separate releases are not coordinated.
- Do not edit a managed policy's assignment rules or HPA settings while reconciliation is running; CAST AI's update endpoint does not expose a conditional-write mechanism used by this controller.
- Make those managed-policy edits only by pausing the Deployment, completing and verifying the edit, and starting the Deployment again. The next reconciliation still owns vertical settings.
- Policy update preservation covers the writable fields documented by the current CAST AI update API. Revalidate the API contract before upgrading across schema changes.
- External edits to managed vertical settings are overwritten on the next poll because the scheduler owns those settings.
- The process has no Kubernetes API token or RBAC permissions.
- The container runs read-only as numeric non-root UID/GID `65532`.

## Troubleshooting

- `HTTP 401/403`: verify the Secret and CAST AI API-key permissions.
- `HTTP 404`: verify cluster and policy IDs.
- Pod configuration error: confirm `castai-api-credentials` exists in the release namespace.
- Startup validation failure: check timezone, unique names/managed IDs, local times, and overlapping windows.

## Development

See [TESTING.md](TESTING.md) for the complete automated, staging acceptance, and production canary test plan.
See [OPERATIONS.md](OPERATIONS.md) for monitoring, image pinning, API-key rotation, upgrades, and rollback.
See [SECURITY.md](SECURITY.md) for the security model, release scans, and vulnerability reporting.

```bash
make all          # race tests, binary build, Helm lint/render
make image        # local container image
make helm-package # package chart under bin/
```

## License

MIT License. See [LICENSE](LICENSE).
