# Scheduled WOOP

Schedule CAST AI Workload Autoscaler policy settings by time of day.

## How it works

Workloads remain assigned to their existing managed policy. Scheduled WOOP selects a source policy for the current time and copies its vertical recommendation settings and `IMMEDIATE`/`DEFERRED` mode into the managed policy.

```text
Business Hours source (unassigned) --\
                                       > Scheduled WOOP --> Managed policy --> Workloads
Off Hours source (unassigned) -------/                         |
                                                                +-- ID/name preserved
                                                                +-- assignments preserved
                                                                +-- HPA settings preserved
```

Expected flow for the example schedule:

```text
Weekdays
00:00 -------- 08:00 ---------------- 18:00 -------- 24:00
   Off Hours          Business Hours         Off Hours

Weekends
00:00 ------------------------------------------------ 24:00
                         Off Hours
```

While the scheduler and CAST AI API are available, each boundary is applied within one polling interval. Source policies are read-only and are never modified. If the scheduler misses a boundary, it applies the currently active source after restarting.

## Policy setup

For a Business Hours/Off Hours schedule, use three policies:

1. **Managed policy** — assigned to the workloads.
2. **Business Hours source policy** — unassigned.
3. **Off Hours source policy** — unassigned.

## Install

Requirements: Kubernetes 1.19+, Helm 3.14+, and a CAST AI API key that can read and update scaling policies.

### 1. Create the Secret

```bash
kubectl create namespace woop-scheduler-system
kubectl -n woop-scheduler-system create secret generic castai-api-credentials \
  --from-file=api-key=./apikey.txt
```

### 2. Create `values.yaml`

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

This applies Business Hours settings from 08:00–18:00 on weekdays and Off Hours settings at all other times. Start is inclusive, end is exclusive, and the timezone must be an IANA timezone. Overnight windows are supported.

Add more `schedules` for additional managed policies. Source policies may be reused when the desired settings are identical.

### 3. Install

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

Healthy logs show `profile applied` or `policy already converged` for each schedule.

## Safety

- Back up all three policies before the first installation.
- Use only one scheduler for each managed policy.
- Do not use a managed policy as a source policy.
- Do not edit managed assignment or HPA settings while the scheduler is running.
- `IMMEDIATE` can resize or restart workloads at a transition; test it on a low-risk workload first.
- If the scheduler misses a transition, it converges to the currently active profile when it restarts.
- Upgrading from `0.1.0`: legacy `config.applyType: DEFERRED` keeps forcing deferred mode until removed.

## Update or uninstall

Edit `values.yaml` and rerun the Helm command to change the schedule.

Before uninstalling, wait until the profile you want to leave active is converged:

```bash
helm uninstall scheduled-woop --namespace woop-scheduler-system
```

Uninstalling leaves the current CAST AI settings and API-key Secret unchanged.

## Troubleshooting

- `401/403`: check the API key and permissions.
- `404`: check the cluster and policy IDs.
- Configuration error: check the Secret, timezone, policy IDs, times, and overlapping windows.

More detail: [operations](OPERATIONS.md), [testing](TESTING.md), [compatibility](docs/COMPATIBILITY.md), [validation](docs/VALIDATION.md), and [security](SECURITY.md).

MIT License. See [LICENSE](LICENSE).
