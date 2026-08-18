# Remediate Review Findings

## Goal

Fix the confirmed defects from `.trellis/tasks/08-14-review-uncommitted-changes/review.md` so the customization branch preserves token authorization, migration safety, Vision usability/cancellation, usage billing safety, calibration integrity, pagination correctness, and affected frontend quality gates.

## Scope

### In scope

- F-001: fixed-group tokens must not switch to another group through Playground request overrides.
- F-003: Vision settings must be saveable without resubmitting unrelated notification settings.
- F-004: legacy routing-grant query failures must abort preview/readiness/migration and must never write a completion marker.
- F-005: Usage Analysis pagination must safely support `page_size=1` and reject only true offset overflow.
- F-006: Auto Token candidate validation must use the same effective user-group access as the candidate-list API.
- F-007: Vision subrequests must inherit request cancellation and deadlines.
- F-008: invalid negative or oversized cache-token fallback values must not reach billing arithmetic.
- F-009: calibration relative-error calculation must not overflow or accept absurd token samples.
- F-010: affected LLM Review and user-form frontend files must pass targeted lint.

### Explicitly out of scope

- F-002: the user explicitly accepted the current LLM Review request-snippet behavior; do not change it in this task tree.
- Unrelated baseline lint, format, copyright, Bun runner, channel-affinity, or Windows HTTP/2 failures.
- Broad refactors or changes to protected project identity/metadata.

## Requirements

- Preserve SQLite, MySQL, and PostgreSQL compatibility.
- Preserve the current dashboard/API response envelopes unless a child task explicitly defines a new focused endpoint.
- Preserve explicit zero/false values and omitted-field semantics in request DTOs.
- Add regression tests at the boundary protected by every fix.
- Work with the existing uncommitted customization changes; do not revert unrelated user work.

## Task map

1. `08-14-fix-routing-access-migration`: F-001, F-004, F-005, F-006.
2. `08-14-fix-vision-contracts`: F-003, F-007.
3. `08-14-harden-usage-calibration`: F-008, F-009.
4. `08-14-fix-frontend-quality`: F-010.

## Acceptance Criteria

- [x] All four child tasks meet their acceptance criteria and focused validation commands pass.
- [x] Root Go build and relevant focused Go tests pass.
- [x] `relaykit` remains independently buildable when its public DTO is in the affected tree.
- [x] Frontend typecheck, production build, and affected-file lint pass.
- [x] Full-suite failures are rerun and classified as fixed, baseline, environment-specific, or unresolved.
- [x] F-002 remains unchanged.
- [x] No unrelated working-tree changes are reverted or included in any later commit without explicit approval.
