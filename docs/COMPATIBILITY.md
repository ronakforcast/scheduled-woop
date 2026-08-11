# Compatibility

Scheduled WOOP uses only stable Kubernetes `v1` and `apps/v1` APIs and does not call the Kubernetes API at runtime.

Requirements:

- Kubernetes `1.19` or newer. Scheduled WOOP's pod security fields require 1.19+, which is stricter than the CAST AI Workload Autoscaler minimum.
- A supported CAST AI Workload Autoscaler installation with scaling policies enabled.
- Helm `3.14` or newer.
- Outbound HTTPS access to `api.cast.ai`, unless `castApiUrl` points to an approved private endpoint.

Directly validated:

- k3s `1.35.5+k3s1`, three-node k3d cluster.
- CAST AI Workload Autoscaler `1.7.2` in high-availability mode.
- Helm `3.17.2`.
- AMD64 and ARM64 container builds.

Kubernetes distributions and versions not listed under “Directly validated” are expected to work when supported by CAST AI, but should complete the customer acceptance checklist before production use.
