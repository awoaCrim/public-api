# Design: Remediate Review Findings

## Structure

This task is an integration parent. It owns scope and final validation; implementation belongs to four independently verifiable child tasks.

## Child boundaries

- **Routing/access/migration:** authorization resolver and controller consistency, fail-closed legacy migration, safe pagination.
- **Vision contracts:** a focused user Vision settings write boundary plus request-context propagation.
- **Usage/calibration:** reject invalid upstream cache fallback metrics before billing and bound calibration samples before persistence.
- **Frontend quality:** mechanical lint corrections only.

## Integration contracts

- Fixed token: an explicit request group may not change a non-`auto` token's fixed group.
- Auto token: candidate groups and mutation validation use the same effective access set, including active extra grants.
- Migration: any legacy-table read error is a hard error; no write or migration-version marker follows it.
- Vision settings: updating Vision must preserve all unrelated user settings.
- Vision requests: cancellation/deadline flows from parent request to outbound provider call.
- Usage: negative/absurd cache fallback values are never used as billable cache tokens.
- Calibration: only bounded non-negative samples are persisted; relative-error arithmetic cannot overflow.

## Compatibility and rollback

All changes are focused source/test edits with no destructive migration. Rollback is per child task. Existing tables and API behavior outside the corrected contracts remain unchanged.

## Integration order

1. Routing/access/migration fixes.
2. Vision contract fixes.
3. Usage/calibration hardening.
4. Frontend lint cleanup.
5. Full integration verification.
