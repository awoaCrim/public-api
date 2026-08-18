# Review Uncommitted Customization Changes

## Goal

Independently review the current uncommitted product changes on `rebuild/customizations-20260812` and identify correctness, security, compatibility, regression, and maintainability risks before any product-code commit or release.

The review is diagnostic only. It must not modify product code, stage unrelated files, commit, push, or deploy. Any remediation should be proposed in the report and handled by a follow-up implementation task unless explicitly requested later.

## Confirmed baseline

- Repository: `E:/myCode/public-api`
- Baseline: `ccd535ef` (`fix: harden concurrent quota and status updates`)
- Current status: 83 tracked files modified and approximately 220 untracked product/metadata paths; the exact inventory is generated during review.
- Existing migration notes claim functionality across request snapshots, rate limits/IP security, LLM Review, Vision, usage analysis, OpenCode Go, group authorization, channel model policies, and routing-group migration, but those claims have not yet received one independent full-scope review.
- Backend conventions are documented in `AGENTS.md`, `web/AGENTS.md`, and `.trellis/spec/backend/`.

## In scope

1. Every modified tracked product file and every untracked product source/test/config/dependency file relative to `HEAD`, grouped by feature and layer.
2. Backend and database behavior:
   - authentication, authorization, sensitive-operation proofs, audits, and session/token invalidation;
   - RPM/rate-limit/IP blacklist and permanent-ban behavior;
   - request snapshots and local encrypted storage;
   - LLM Review, token preflight/calibration, Vision interception;
   - user groups, fixed/Auto token routing, channel model policies, and compatibility migration;
   - usage/cache metrics, quota/billing paths, migrations, transactions, caches, and logging.
3. Relay and protocol boundaries, including OpenCode Go, OpenAI usage normalization, provider request DTOs, and independent `relaykit` buildability.
4. Frontend API contracts, error/loading states, permission gating, routing, i18n completeness, serialization/presence semantics, accessibility, and affected tests.
5. Validation commands and test results needed to distinguish newly introduced failures from documented baseline failures.

## Out of scope

- Editing product code or silently fixing findings.
- Committing, pushing, or deploying product changes as part of the review.
- `.trellis/`, `.agents/`, `.pi/`, workspace journals, archived task metadata, and planning documents as review targets; they may be read for context but are not product findings.
- The read-only legacy repository and production data, except where existing design notes require semantic comparison.
- Re-reviewing already committed upstream history unless it is necessary to establish whether a current uncommitted behavior is a regression.

## Review requirements

- Review all in-scope paths; if a path cannot be reviewed, list it explicitly with the reason.
- Trace security-sensitive and billing-sensitive flows across storage → service → middleware/controller → API/UI boundaries.
- Check SQLite, MySQL, and PostgreSQL compatibility wherever database or migration code is affected; check `relaykit` independence separately.
- Treat a claim as a finding only when supported by code, tests, or reproducible command output. Record confidence and distinguish baseline failures from regressions.
- Use severity levels P0 (release-blocking), P1 (high-risk correctness/security/data issue), P2 (material defect or compatibility risk), and P3 (lower-risk maintainability/test/documentation issue).
- Each finding must include severity, confidence, file and line anchor, evidence, impact, and a concrete recommendation.

## Acceptance criteria

- [x] A complete in-scope inventory is recorded, grouped by feature/layer, with exclusions and unreviewed paths explicit.
- [x] Backend, relaykit, frontend, cross-layer, security, billing, database, and test concerns are reviewed with evidence.
- [x] Focused and appropriate full-scope validation commands are run; failures are classified as baseline, environment, flaky, or change-related.
- [x] A task-local review report records all findings, including "no finding" areas where useful, and ranks remediation order.
- [x] No product source file is modified by the review; only Trellis task artifacts and the review report may change.
- [x] The final report states whether the uncommitted product changes are safe to commit/release, and lists required follow-up work.

## Blocking decisions

None. The user requested a review rather than fixes; the default is report-only and no product-code edits.
