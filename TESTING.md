# Production test plan

This plan separates tests that run in CI from acceptance tests that require a real Kubernetes cluster and dedicated CAST AI policies. Never run acceptance tests against a policy serving production workloads for the first time.

## Test environments

- **CI:** Go tests, race detector, binary build, Helm lint/render, Helm security contract, and container build.
- **Staging acceptance:** A CAST AI-enabled non-production cluster with three dedicated policies: one managed test policy and two unassigned source policies.
- **Production canary:** One low-risk workload and dedicated managed/source policies before broader rollout.

Record the chart version, image digest, cluster ID, policy IDs, timezone, tester, start/end timestamps, and evidence for every staging or canary run. Never record the API key.

## Automated test catalog

The tables below define the release target, not a claim that every row already passes. Current CI implementation covers the schedule boundary, validation, reconciliation, HTTP, and Helm safety cases. The isolated acceptance results are recorded in [docs/VALIDATION.md](docs/VALIDATION.md). Public artifact checks and the design-partner immediate-disruption acceptance remain release/pilot gates.

### Schedule selection

| ID | Test | Expected result |
|---|---|---|
| SCH-01 | One minute before a window | Default profile selected |
| SCH-02 | Exact start minute | Window profile selected; start is inclusive |
| SCH-03 | One minute before the end | Window profile remains selected |
| SCH-04 | Exact end minute | Default profile selected; end is exclusive |
| SCH-05 | Day not listed in the window | Default profile selected |
| SCH-06 | Overnight window before start | Default profile selected |
| SCH-07 | Overnight window at start | Window profile selected |
| SCH-08 | Overnight window after midnight | Window belongs to the previous start day |
| SCH-09 | Overnight window at end | Default profile selected |
| SCH-10 | Week wrap, such as Sunday night into Monday | Correct profile across week boundary |
| SCH-11 | Same local schedule in winter and summer | IANA timezone applies the correct UTC offset |
| SCH-12 | Overlapping ordinary windows | Configuration rejected |
| SCH-13 | Ordinary window overlapping an overnight continuation | Configuration rejected |

### Configuration validation

| ID | Test | Expected result |
|---|---|---|
| CFG-01 | Valid minimal configuration | Loads with a `1m` poll default; apply mode comes from the active source policy |
| CFG-02 | Unknown YAML field | Rejected instead of silently ignored |
| CFG-03 | Missing configuration file | Clear startup failure |
| CFG-04 | Malformed YAML | Clear parse failure |
| CFG-05 | Missing cluster ID | Rejected |
| CFG-06 | Invalid IANA timezone | Rejected |
| CFG-07 | Poll interval below 10 seconds | Rejected |
| CFG-08 | Invalid poll duration | Rejected |
| CFG-09 | Legacy global `config.applyType: DEFERRED` | Accepted and continues forcing deferred mode until removed as an explicit migration step |
| CFG-09A | Any other global `config.applyType` | Rejected with guidance to configure each source policy |
| CFG-10 | No schedules | Rejected |
| CFG-11 | Blank or duplicate schedule name | Rejected |
| CFG-12 | Blank or duplicate managed policy ID | Rejected |
| CFG-13 | Blank source profile name or ID | Rejected |
| CFG-14 | Managed policy also used as any source | Rejected |
| CFG-15 | Blank or duplicate window name | Rejected |
| CFG-16 | Window without days | Rejected |
| CFG-17 | Invalid weekday | Rejected |
| CFG-18 | Non-`HH:MM` or out-of-range time | Rejected |
| CFG-19 | Equal start and end | Rejected rather than interpreted as 24 hours |

### Reconciliation safety

| ID | Test | Expected result |
|---|---|---|
| REC-01 | Default period | Default source settings copied |
| REC-02 | Active scheduled window | Window source settings copied |
| REC-03 | Source says `IMMEDIATE` or `DEFERRED` | The source mode is copied to the managed policy and nested recommendations |
| REC-04 | Managed policy name | Preserved exactly |
| REC-05 | Managed assignment rules | Preserved exactly |
| REC-06 | Managed HPA settings | Preserved exactly |
| REC-07 | Desired state already active | No PUT is issued |
| REC-08 | Settings differ | Exactly one PUT followed by read-back verification |
| REC-09 | Malformed source recommendations | No write; error returned |
| REC-10 | Source GET fails | No managed-policy write |
| REC-11 | Managed GET fails | No managed-policy write |
| REC-12 | Update fails | Error returned; no false success |
| REC-13 | Read-back differs after update | Verification failure reported |
| REC-14 | One of multiple schedules fails | Other schedules continue independently |
| REC-15 | Multiple schedules fail | Errors aggregated without stopping the process |
| REC-16 | Process starts after a missed transition | Current active profile is applied immediately |
| REC-17 | Context is cancelled | Runner exits cleanly |
| REC-18 | Repeated polls after convergence | No repeated writes |

### CAST AI API boundary

| ID | Test | Expected result |
|---|---|---|
| API-01 | API key contains surrounding whitespace | Whitespace trimmed; correct header sent |
| API-02 | GET policy | Correct cluster/policy endpoint and response decoding |
| API-03 | PUT policy | Complete preserved payload sent |
| API-04 | HTTP 401/403/404 | No retry; status-only safe error |
| API-05 | GET receives HTTP 429 | Retried up to three total attempts |
| API-06 | HTTP 5xx then success | Retries and succeeds |
| API-07 | Persistent HTTP 5xx | Stops after three total attempts |
| API-08 | Transport failure | Retried up to three attempts |
| API-09 | Cancelled context | Cancellation returned promptly |
| API-10 | Malformed JSON | Decode error returned; no update |
| API-11 | Missing required policy fields | Response rejected |
| API-12 | Invalid API URL | Client creation rejected |
| API-13 | Empty API key | Client creation rejected |
| API-14 | Sensitive response body or key | Not included in errors or logs |
| API-15 | PUT receives transport error, 429, or 5xx | Not retried in the same reconciliation; next poll reads current state before deciding whether to write again |

### Helm and Kubernetes contract

| ID | Test | Expected result |
|---|---|---|
| K8S-01 | Helm lint and render | Successful |
| K8S-02 | Empty `existingSecret` | Render fails |
| K8S-03 | API key storage | Secret referenced; key absent from chart output and ConfigMap |
| K8S-04 | Service-account token | Disabled |
| K8S-05 | RBAC resources | None rendered |
| K8S-06 | User and group | Numeric non-root `65532` |
| K8S-07 | Privilege escalation | Disabled |
| K8S-08 | Linux capabilities | All dropped |
| K8S-09 | Root filesystem | Read-only |
| K8S-10 | Seccomp | `RuntimeDefault` |
| K8S-11 | ConfigMap update | Pod-template checksum changes and causes rollout |
| K8S-12 | Resource requests and limits | Rendered from values |
| K8S-13 | Public image architecture | AMD64 and ARM64 manifests available |
| K8S-14 | Public first-time installation | Chart and image pull without registry credentials |
| K8S-15 | Deployment upgrade | `Recreate` strategy prevents old and new schedulers writing concurrently |

Run the automated suite with:

```bash
make all
docker build -t scheduled-woop:test .
```

## Staging acceptance tests

Prepare policies with deliberately distinguishable CPU/memory settings:

- `acceptance-managed`: assigned only to a disposable test workload.
- `acceptance-business`: unassigned source policy.
- `acceptance-off-hours`: unassigned source policy.

Back up the full JSON of all three policies before testing. Use short windows at least five minutes in the future; do not rely on changing the host clock.

| ID | Test | Procedure and expected result |
|---|---|---|
| ACC-01 | Fresh install | Follow only the public README; release becomes Ready without private registry authentication |
| ACC-02 | Initial convergence | Start during default period; managed recommendations match off-hours source within one poll |
| ACC-03 | Start transition | At window start, managed recommendations match business source within one poll |
| ACC-04 | End transition | At window end, managed recommendations match off-hours source within one poll |
| ACC-04A | Mixed apply modes | Business source `IMMEDIATE` and off-hours source `DEFERRED` are each copied at their respective transitions |
| ACC-04B | Immediate disruption control | Immediate transition follows the customer's configured CAST AI rollout behavior, PDBs, and availability expectations |
| ACC-05 | Assignment stability | Before/after both transitions, managed assignment rules and assigned workload are unchanged |
| ACC-06 | HPA stability | Before/after both transitions, HPA settings are byte-for-byte equivalent after JSON normalization |
| ACC-07 | Policy identity | Managed policy ID and name remain unchanged |
| ACC-08 | Source immutability | Both source policies remain unchanged |
| ACC-09 | Idempotency | Leave one profile active for three polls; only the first poll may issue a PUT |
| ACC-10 | Scheduler down at start | Scale Deployment to zero before start, restore after start; current business profile converges immediately |
| ACC-11 | Scheduler down at end | Repeat across end; current off-hours profile converges immediately |
| ACC-12 | Pod restart | Delete pod during stable window; replacement converges without duplicate harmful changes |
| ACC-13 | Invalid source ID | One schedule logs 404 while another valid schedule still converges |
| ACC-14 | Revoked API key | 401/403 is logged without key material; policies remain unchanged |
| ACC-15 | CAST AI temporary outage | Existing settings remain active; convergence resumes after recovery |
| ACC-16 | Rate limiting | Controller backs off/retries and does not crash-loop |
| ACC-17 | ConfigMap schedule change | Helm upgrade rolls the pod and new schedule becomes authoritative |
| ACC-18 | Invalid config upgrade | New pod rejects configuration; existing CAST settings are not changed |
| ACC-19 | Uninstall | Current settings remain unchanged; API-key Secret remains for explicit cleanup |
| ACC-20 | Reinstall | Controller reads current state and converges to the active profile |
| ACC-21 | Two independent schedules | Each managed policy follows only its own sources |
| ACC-22 | Overnight window | Both start-day and next-day continuation transition correctly |
| ACC-23 | DST spring-forward | No duplicate or unsafe write; active local-time profile is correct after jump |
| ACC-24 | DST fall-back | Repeated local hour remains idempotent and correct |
| ACC-25 | Human edit procedure | Operational controls prevent managed assignment/HPA edits while the scheduler is reconciling |
| ACC-26 | Source edit between source GET and managed PUT | Applied snapshot is consistent and next poll converges to the latest source |
| ACC-27 | Accidental second scheduler | Installation inventory and change controls ensure only one release owns each managed policy |

## Production canary and operational tests

| ID | Test | Pass criteria |
|---|---|---|
| OPS-01 | Least-privilege API key | Key can read/update WOOP policies but cannot perform unrelated administrative actions |
| OPS-02 | Secret visibility | Secret absent from Git, Helm values, rendered manifests, events, and logs |
| OPS-03 | Log usefulness | Success, no-op, schedule name, profile, managed policy ID, and safe errors are observable |
| OPS-04 | Alerting | Alert fires on sustained reconciliation errors or pod unavailability |
| OPS-05 | One-hour failure | Existing WOOP settings remain effective while scheduler is unavailable |
| OPS-06 | Node drain | Pod reschedules and converges on another node |
| OPS-07 | Kubernetes upgrade/restart | Deployment returns Ready and reconciles correctly |
| OPS-08 | 24-hour soak | No crash, unbounded memory growth, repeated PUTs, or unexpected policy drift |
| OPS-09 | API latency | Reconciliation remains stable with slow responses below the 15-second client timeout |
| OPS-10 | Audit trail | CAST AI audit events match expected transition writes only |
| OPS-11 | Rollback | Previous chart version can be installed and intended source profile restored |
| OPS-12 | Credential rotation | Replacing Secret and restarting pod uses new key without policy drift |
| OPS-13 | Image integrity | Deployed image digest matches the approved release digest |
| OPS-14 | Multi-architecture canary | Approved digest starts on every production node architecture in use |

## Release gate

Do not approve broad production rollout until:

1. CI and image build are green for the exact release commit.
2. All staging acceptance cases pass with saved evidence.
3. The production canary completes both a start and end transition.
4. Assignment rules, HPA settings, policy identity, and source policies are proven unchanged.
5. Alerts, rollback, API-key rotation, and ownership/on-call procedures are verified.
6. CAST AI confirms the full-update schema and preservation behavior for all current writable policy fields.
