# Research: Admin Observability UX

## Request-body access

Current flow:

`Usage log details → permission-aware control → SecureVerificationDialog → X-Security-Proof → AdminAuth → CriticalRateLimit → DisableCache → RequirePermission(request_snapshot.read) → controller proof validation → audited snapshot read`

Evidence:

- `web/src/features/usage-logs/components/dialogs/request-snapshot-section.tsx` starts Passkey/2FA verification before fetching.
- `web/src/features/usage-logs/lib/request-snapshot.ts` gates the control by `request_snapshot.read` and sends `X-Security-Proof`.
- `router/api-router.go` protects the endpoint with `AdminAuth`, `CriticalRateLimit`, `DisableCache`, and `RequirePermission`.
- `controller/requestsnapshot.go` validates the proof, checks audit storage before loading bytes, audits every handler-level result, and refuses successful content if the success audit cannot be stored.
- `service/authz/resources_requestsnapshot.go` exposes the delegable permission in the admin permission catalog.

Decision: replace the delegable permission plus secondary-proof model with the project-standard `RootAuth` boundary. Keep rate limiting, no-cache behavior, on-demand fetch, encrypted storage, and synchronous audit fail-closed behavior.

## Usage Analysis comparison

The old implementation at `E:/myCode/myapi/web/default/src/features/usage-analysis/index.tsx` used:

- “Usage Statistics” title plus explanatory copy;
- filters grouped at the upper right;
- an “Actual Consumed Tokens” hero with selected user/key context;
- request and cost summary blocks;
- colored metric cards including cache rate, average tokens, and consumed quota;
- a gradient-filled trend chart with compact token axis formatting;
- a visually lighter token-usage breakdown table.

The current implementation at `web/src/features/usage-analysis/index.tsx` introduced important safety and correctness improvements:

- server-provided summary independent of the current page;
- explicit page/page-size contract and previous-data pagination;
- separate cache-read and cache-write metrics;
- legacy-row disclosure;
- bounded backend aggregation, timeout, and filter validation.

Decision: restore the old visual hierarchy, not the old data-fetching behavior. The current API types, query bounds, summary semantics, filters, pagination, cache metric definitions, and error handling remain authoritative.

## Request Snapshot settings visibility

`RequestSnapshotSettingsSection` still exists and is rendered from `web/src/features/system-settings/operations/section-registry.tsx`, but it is nested inside the generic `logs` section after `LogSettingsSection`. The dynamic Operations route already accepts every ID registered in `OPERATIONS_SECTION_IDS`, and sidebar items are generated from the same registry.

Decision: move the existing component into a dedicated `request-snapshots` registry entry using the existing `Request Snapshots` translation key. Do not duplicate the form or add a second state owner.
