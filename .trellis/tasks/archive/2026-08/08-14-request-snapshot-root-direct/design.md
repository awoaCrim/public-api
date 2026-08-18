# Design: Root-only Direct Request Snapshot Access

## Authorization owner

The backend route becomes the single security boundary:

`Root dashboard credential → RootAuth → CriticalRateLimit → DisableCache → audited controller read`

`AdminAuth`, `RequirePermission(request_snapshot.read)`, and controller-level security-proof validation are removed. The existing `RootAuth` convention may authenticate either a valid Root dashboard session or Root PAT; no second proof is required.

## Backend changes

- In `router/api-router.go`, replace `AdminAuth + RequirePermission` with `RootAuth` while retaining rate limiting and no-cache middleware.
- In `controller/requestsnapshot.go`, remove proof constants/checks and begin directly with audit-store/read handling.
- Remove the request-snapshot permission resource/registration because a delegable permission would no longer affect the route.
- Remove `request_snapshot.read` from allowed universal security-proof scopes and update scope tests.
- Keep the audit preflight, per-result audit rows, safe error codes, exact-byte response, and success fail-closed behavior unchanged.

## Frontend changes

- `canViewRequestSnapshot` becomes a Root-role plus request-ID gate; non-Root permission maps cannot enable the control.
- `getRequestSnapshot` no longer accepts a proof token or sends `X-Security-Proof`.
- `RequestSnapshotSection` directly invokes the loader on click and removes secure-verification hook/dialog dependencies.
- Keep component-local payload state, lazy loading, retry, copy, download, and close-time clearing.

## Tests

- Update frontend gating tests to prove Root-only visibility and denial for explicitly granted non-Root admins.
- Update the section interaction test to prove one direct fetch on click and no proof parameter.
- Replace proof-gate controller tests with direct-read/error/audit tests.
- Add or update a route authorization regression so the snapshot endpoint is tied to Root authorization; retain the existing `RootAuth` middleware role test.
