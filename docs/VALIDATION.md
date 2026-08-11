# Validation record

## 2026-08-11 isolated CAST AI end-to-end test

Environment:

- Kubernetes: k3s `v1.35.5+k3s1` on a three-node local k3d cluster.
- CAST AI Workload Autoscaler: enabled and healthy.
- Scheduled WOOP chart/application: preview `0.2.0`, commit `5c1b39e` plus Milestone 1 tests.
- Poll interval: 10 seconds.
- Timezone: `Europe/Amsterdam`.
- Schedule: Tuesday 09:44–09:47 local time.
- Dedicated resources: one managed policy, one `IMMEDIATE` business source, one `DEFERRED` off-hours source, and one disposable two-replica workload.

No existing CAST AI policy or customer workload was used or modified.

| Test | Result | Evidence |
|---|---|---|
| Initial convergence | Pass | Managed policy changed from overhead `0.23` to off-hours `DEFERRED`/`0.17`; read-back verified |
| Business-window start | Pass | At 09:44:04, managed policy changed to `IMMEDIATE`/`0.41`; warning and success logs emitted |
| Workload policy binding | Pass | CAST inventory showed the disposable workload bound to the dedicated managed policy with `IMMEDIATE`/`0.41` |
| Scheduler restart | Pass | Replacement pod became Ready with zero restarts and reported `policy already converged` |
| Missed end transition | Pass | Scheduler stayed at zero replicas across 09:47; old settings remained active; restart applied `DEFERRED`/`0.17` immediately |
| Invalid API key | Pass | Controller reported HTTP 401 without exposing the key; policy remained unchanged; key restoration recovered automatically |
| CAST API outage | Pass | Unreachable test endpoint produced safe connection errors; policy remained unchanged |
| Helm rollback | Pass | Rollback from revision 2 to revision 1 restored API connectivity and convergence |
| Uninstall | Pass | Managed policy retained its current off-hours settings |
| Reinstall | Pass | New release became Ready and immediately reported convergence |
| Managed identity | Pass | Policy ID and name remained unchanged across both transitions and all failure tests |
| Assignment rules | Pass | Canonical fingerprint remained `4ed5dfa2167dce4cbd2b01fc0b8e88ef35d02f21c63b898216d1b753f50ebbba` together with identity/HPA |
| HPA settings | Pass | Included in the same managed-field fingerprint and unchanged |
| Business source immutability | Pass | Canonical writable-field fingerprint remained `e731e6755eb4229cdfc0d81ce1126798351c019d55fa5669b649cd7ab7781504` |
| Off-hours source immutability | Pass | Canonical writable-field fingerprint remained `5e761b0b7de59803d6948826072469f5d435c0257b359e15bd63d692972ea3df` |
| Cleanup | Pass | Both Kubernetes namespaces and all three dedicated policies were deleted; follow-up policy search returned no matches |

The disposable workload did not have meaningful resource recommendations, so this test proves policy-mode propagation but does not claim that CAST AI restarted a production-like application. Immediate-mode disruption behavior must be accepted during the design-partner change window using that customer's rollout safeguards and availability requirements.

