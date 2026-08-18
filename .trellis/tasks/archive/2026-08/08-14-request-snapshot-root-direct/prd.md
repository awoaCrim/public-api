# Make Request Snapshots Root-only Direct Access

## Goal

Allow the Root super administrator to open a saved request body directly, without Passkey/2FA or a separately assigned permission, while preserving the existing sensitive-data safeguards.

## Requirements

- `GET /api/log/:request_id/snapshot` is accessible only through `RootAuth`.
- Keep `CriticalRateLimit` and `DisableCache` on the route.
- Remove the `request_snapshot.read` permission from the delegable admin permission catalog.
- Remove the request-snapshot security-proof scope from universal 2FA/Passkey verification.
- Show the View Request Body control only to Root and only when a request ID exists.
- Clicking the control fetches immediately; no secure-verification dialog or proof header is used.
- Continue fetching only on demand and clearing decrypted body state when the details dialog closes.
- Preserve synchronous access auditing and fail closed if successful access cannot be audited.

## Out of Scope

- Snapshot capture, encryption, storage, cleanup, retention, and error-code behavior.
- LLM Review secondary verification.
- Allowing non-Root admins to view request bodies.

## Acceptance Criteria

- [x] Root can retrieve and render a stored request body without a proof token.
- [x] Admin/common users are rejected by the backend and do not see the frontend control.
- [x] The permission catalog no longer offers request snapshot read delegation.
- [x] Successful and failed Root handler-level reads retain durable audit rows.
- [x] Snapshot bytes, copy/download behavior, no-cache behavior, and state clearing remain unchanged, including invalidating late responses after close.
- [x] Focused backend and frontend regression tests pass.
