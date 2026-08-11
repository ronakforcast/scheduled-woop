# Security

## Design

- The CAST AI API key is read from a mounted Kubernetes Secret and is never accepted through Helm values.
- The pod has no Kubernetes API token and no RBAC permissions.
- The container runs as numeric non-root UID/GID `65532`, drops all capabilities, uses `RuntimeDefault` seccomp, blocks privilege escalation, and has a read-only root filesystem.
- The metrics Service is cluster-internal and exposes no credentials or policy bodies.
- Policy errors include status codes and identifiers but not the API key or CAST response body.

## Release checks

CI runs race tests, Gitleaks repository/history scans, `govulncheck`, a Trivy high/critical image scan, and Syft SPDX SBOM generation. Builder and runtime base images are pinned by multi-architecture digest.

The 2026-08-11 preview candidate passed:

- Gitleaks and TruffleHog: no secrets.
- `govulncheck` under Go 1.26.5: no vulnerabilities.
- Trivy 0.73.0: zero high/critical findings in the runtime OS and Go binary.
- Syft 1.51.0: SPDX 2.3 SBOM generated.

## Reporting a vulnerability

Do not open a public issue containing exploit or credential details. Use GitHub's **Report a vulnerability** private security-advisory flow for this repository. Revoke and rotate any credential immediately if exposure is suspected.

