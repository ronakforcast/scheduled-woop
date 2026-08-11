# Changelog

## 0.2.0

- Copy `IMMEDIATE` or `DEFERRED` apply mode from the active source policy.
- Preserve the legacy global `config.applyType: DEFERRED` override until customers explicitly remove it.
- Avoid retrying policy PUT requests within one reconciliation; the next poll reads current state first.
- Use `Recreate` deployment updates to avoid overlapping scheduler pods.
- Add production test catalog, API failure coverage, scheduling boundaries, and Helm security contract tests.

## 0.1.0

- Initial preview release with globally forced deferred application mode.
