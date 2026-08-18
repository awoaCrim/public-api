# Fix Routing Access and Migration

## Goal

Correct F-001, F-004, F-005, and F-006, then close the two migration integrity/status gaps found during final review, without changing unrelated routing-group behavior.

## Requirements

### Fixed and Auto Token authorization

- A non-`auto` token is fixed to its resolved token group.
- A request-level group override for a fixed token is allowed only when it equals the fixed group; a different group returns the existing stable access-denied behavior.
- An `auto` token may select an explicit valid fixed group from the user's effective access set.
- Auto Token candidate listing and mutation validation must both include active extra user grants and reject expired/revoked grants.

### Migration safety

- Failure to read `user_routing_group_grants` must propagate through preview, readiness, and strict migration.
- A read failure must produce zero migration writes and no migration-version marker.
- Existing idempotency and cross-database behavior must remain intact.
- A still-effective legacy grant whose `routing_group_id` has no corresponding legacy group row must be reported as unmappable and must block strict migration; it must never be silently dropped behind a completion marker.
- Expired orphan grants do not confer access and must not block migration.
- Migration preview/status must count only grants that would create or materially update the target `user_group_grants` row.
- After a successful idempotent migration, `PendingGrants` must be zero and `InSync` must be true when there are no other blockers or token updates.

### Usage Analysis pagination

- `page=1&page_size=1` is valid.
- Offset overflow is rejected without overflowing the validation arithmetic.

## Out of Scope

- Changing the global group catalog or migration mapping policy.
- Restoring legacy routing tables as runtime authority.
- F-002 and unrelated frontend cleanup.

## Acceptance Criteria

- [x] Fixed token + different granted group is rejected.
- [x] Fixed token + same group remains valid.
- [x] Auto token + granted requested group remains valid.
- [x] Extra granted groups can be saved as per-token Auto candidates; revoked/expired groups are rejected.
- [x] Legacy grant query failure aborts preview/readiness/strict migration with zero writes and no marker.
- [x] `page_size=1` succeeds and real offset overflow fails.
- [x] Focused controller/middleware/service/model tests pass.
- [x] Active orphan legacy grants are reported and block strict migration with zero writes and no marker.
- [x] Expired orphan legacy grants do not block migration.
- [x] Already-imported equivalent or broader target grants are not reported as pending.
- [x] A successful migration reports `PendingGrants=0` and `InSync=true` when no other blockers remain.
