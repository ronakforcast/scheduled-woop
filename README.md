# Scheduled WOOP

Scheduled WOOP schedules CAST AI Workload Autoscaler policy settings for predictable business and off-hours traffic. Workloads remain assigned to one managed policy while the active source policy supplies its vertical recommendation settings and `IMMEDIATE` or `DEFERRED` mode.

> **Demo screenshot placeholder:** Add a CAST AI policy transition or terminal verification image here.

## Features

- Schedules business-hours, off-hours, overnight, and weekend profiles.
- Supports multiple managed policies and independent schedules.
- Uses each source policy's `IMMEDIATE` or `DEFERRED` mode.
- Preserves managed policy identity, assignments, and HPA settings.
- Leaves source policies unchanged.
- Converges to the correct profile after a restart or missed transition.
- Installs as a small Kubernetes Deployment, ConfigMap, and Service through Helm.

## Tech stack

- **Application:** Go
- **Deployment:** Kubernetes and Helm
- **Configuration:** YAML
- **Integration:** CAST AI API
- **Container:** Distroless Linux, multi-architecture (`amd64` and `arm64`)

## Getting started

### Prerequisites

- Kubernetes 1.19 or newer
- Helm 3.14 or newer
- `kubectl` configured for the target cluster
- CAST AI Workload Autoscaler enabled
- A CAST AI API key that can read and update scaling policies
- One managed policy assigned to the workloads
- Two unassigned source policies for the example schedule

### Policy setup

For a Business Hours/Off Hours schedule, use:

1. **Managed policy** — assigned to the workloads.
2. **Business Hours source policy** — unassigned.
3. **Off Hours source policy** — unassigned.

```text
Business Hours source (unassigned) --\
                                       > Scheduled WOOP --> Managed policy --> Workloads
Off Hours source (unassigned) -------/                         |
                                                                +-- ID/name preserved
                                                                +-- assignments preserved
                                                                +-- HPA settings preserved
```

### 1. Create the API-key Secret

Save the key locally as `apikey.txt` and run:

```bash
kubectl create namespace woop-scheduler-system --dry-run=client -o yaml | kubectl apply -f -
kubectl -n woop-scheduler-system create secret generic castai-api-credentials \
  --from-file=api-key=./apikey.txt
```

No `.env` file is required for the Helm installation. The chart supplies the runtime environment, and the API key is mounted directly from the Kubernetes Secret.

Runtime environment variables used by the container are:

```text
CONFIG_PATH=/etc/scheduled-woop/config.yaml
CAST_API_KEY_FILE=/etc/scheduled-woop-secret/api-key
CAST_API_URL=https://api.cast.ai
LISTEN_ADDRESS=:8080
```

### 2. Create `values.yaml`

Replace the cluster and policy ID placeholders:

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

Start times are inclusive, end times are exclusive, and `timezone` must be an IANA timezone. Overnight windows are supported.

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

## Usage

The example configuration produces this schedule:

```text
Weekdays
00:00 -------- 08:00 ---------------- 18:00 -------- 24:00
   Off Hours          Business Hours         Off Hours

Weekends
00:00 ------------------------------------------------ 24:00
                         Off Hours
```

Assume the source policies contain:

- **Business Hours:** `P99`, 3-day lookback, `IMMEDIATE`.
- **Off Hours:** `P75`, 3-hour lookback, `DEFERRED`.

```text
Shortly after 08:00 on a weekday
Business Hours source selected
        +--> Managed policy receives P99 + 3-day lookback + IMMEDIATE
        +--> CAST AI uses those settings for recommendations
        +--> Applicable changes may resize/restart workloads immediately

Shortly after 18:00 and during weekends
Off Hours source selected
        +--> Managed policy receives P75 + 3-hour lookback + DEFERRED
        +--> CAST AI uses those settings for recommendations
        +--> No immediate workload rollout is triggered by the switch
```

The Business Hours profile typically provides more headroom, while the Off Hours profile follows recent lower usage and may produce lower recommendations. Actual CPU and memory changes depend on observed usage and CAST AI's recommendation.

Throughout the schedule, the managed policy ID, name, workload assignments, and HPA settings remain unchanged. Source policies are read-only. While the scheduler and CAST AI API are available, transitions occur within one polling interval; missed transitions converge after restart.

### Update or uninstall

Edit `values.yaml` and rerun the Helm installation command to change a schedule.

Before uninstalling, wait until the profile you want to leave active is converged:

```bash
helm uninstall scheduled-woop --namespace woop-scheduler-system
```

Uninstalling leaves the active CAST AI settings and API-key Secret unchanged.

## Safety

- Back up all policies before the first installation.
- Use only one scheduler for each managed policy.
- Do not use a managed policy as a source policy.
- Do not edit managed assignments or HPA settings while the scheduler is running.
- Test `IMMEDIATE` transitions on a low-risk workload first.
- Upgrading from `0.1.0`: legacy `config.applyType: DEFERRED` forces deferred mode until removed.

## Governance

### Tests

Development requires Go 1.26+, Helm, Bash, and Docker.

```bash
make all
make image
```

See [TESTING.md](TESTING.md) for automated and customer acceptance coverage.

### Contributing

Create a focused branch, include tests or documentation for the change, run `make all`, and open a pull request. Never commit CAST AI API keys, customer policy exports, or other credentials.

### Support and security

- [Operations and rollback](OPERATIONS.md)
- [Compatibility](docs/COMPATIBILITY.md)
- [Validation evidence](docs/VALIDATION.md)
- [Security policy](SECURITY.md)

### License

MIT License. See [LICENSE](LICENSE).
