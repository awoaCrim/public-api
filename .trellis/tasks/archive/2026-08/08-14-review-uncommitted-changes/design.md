# Design: Uncommitted Customization Review

## Review boundary

The review compares the working tree with `HEAD` and treats tracked modifications plus untracked product files as the candidate change set. Trellis metadata, task artifacts, journals, and planning documents are context only and are excluded from product findings.

The review is read-only with respect to product code. Findings and validation notes are written to task artifacts, not to the implementation files being reviewed.

## Evidence model

For every finding, preserve:

- severity (`P0`–`P3`) and confidence;
- exact file and line anchor, including both sides of a cross-layer contract when relevant;
- observed code/test/command evidence;
- user-visible or operational impact;
- minimal remediation recommendation;
- whether the issue is new, baseline, environment-specific, or unresolved.

A behavior described only in planning notes is not accepted as proof that the implementation is correct. The implementation, tests, migrations, and runtime wiring must agree.

## Review tracks

### Track A — Backend security, data, and billing

Inspect authentication/authorization/proof scopes, request snapshot access, IP/rate limiting, permanent-ban invalidation, LLM Review policy/worker, token preflight, Vision, group access, routing migration, transactions, locks, quotas, structured logs, and database migrations. Trace each sensitive flow end to end and check the project’s JSON, database, quota, and test conventions.

### Track B — Relay and protocol boundaries

Inspect OpenCode Go registration and conversion, OpenAI usage/cache normalization, stream/non-stream seams, optional pointer DTO fields, provider error mapping, billing usage propagation, `relaykit` imports, and independent-module build/test behavior.

### Track C — Frontend and API contracts

Inspect new/changed routes, API types, query/mutation behavior, form presence semantics, loading/error/empty states, permission gating, i18n across seven locales, accessibility, tests, generated route output, and changed-file lint/type behavior. Use `web/AGENTS.md` as the frontend contract.

### Track D — Integration and verification

Reconcile the tracks against the actual diff, migration registration, router wiring, feature flags/defaults, cache invalidation, tests, dependency changes, and documented baseline failures. Look for gaps where one layer assumes a contract that another layer does not enforce.

## Severity policy

- **P0**: exploitable security issue, data corruption/loss, negative billing, or release-blocking failure.
- **P1**: high-probability authorization, privacy, accounting, migration, or protocol correctness defect.
- **P2**: material user-visible regression, cross-database incompatibility, operational failure, or missing important regression coverage.
- **P3**: maintainability, documentation, non-critical test, or low-impact consistency issue.

## Validation strategy

Use the repository’s portable Go tool when needed, run focused tests for changed packages first, then run appropriate root and frontend checks. Required independent validation includes:

- root Go build/tests relevant to changed backend packages;
- `cd relaykit && GOWORK=off go build ./...` for the nested module;
- frontend `bun run typecheck`, production build, focused tests, and changed-file lint/format where available;
- `git diff --check` and dependency/locale checks where relevant.

Known baseline failures from `findings.md` and `task_plan.md` must be reproduced or explicitly separated from new failures.

## Deliverable

Create `.trellis/tasks/08-14-review-uncommitted-changes/review.md` with the complete inventory, validation matrix, findings, severity ranking, unreviewed areas, and release/commit recommendation. Do not edit product source code.
