# Implementation Plan: Uncommitted Customization Review

This is a read-only review task. “Implementation” means collecting evidence and producing the review report; product-code remediation is explicitly out of scope.

## Ordered checklist

1. [x] Freeze the candidate inventory from `git status --porcelain=v1 --untracked-files=all`, `git diff --name-status`, and untracked-file enumeration. Classify paths into product code, tests, configuration/dependencies, generated files, documentation, and Trellis metadata.
2. [x] Read the applicable repository contracts: root `AGENTS.md`, `web/AGENTS.md`, `.trellis/spec/backend/` indexes/guidelines, `task_plan.md`, `findings.md`, and the feature inventory.
3. [x] Review backend security/data/billing/migration paths and their tests. Trace authz, proof, audit, cache, transaction, quota, and failure behavior across layers.
4. [x] Review relay/provider/relaykit paths, including request DTO optional values, error mapping, usage propagation, stream behavior, and independent-module boundaries.
5. [x] Review frontend routes/features/types/forms/locales/tests for API contract, permissions, error/loading states, presence semantics, accessibility, and i18n consistency.
6. [x] Run validation in increasing scope:
   - focused Go tests/builds for changed backend areas;
   - root build and appropriate root test packages;
   - `cd relaykit && GOWORK=off go build ./...` and focused RelayKit tests if applicable;
   - frontend typecheck, production build, focused tests, changed-file lint/format;
   - `git diff --check`, locale/dependency checks where applicable.
7. [x] Reconcile command results with known baseline failures. Re-run suspicious failures in isolation and classify them as baseline, flaky, environmental, or change-related.
8. [x] Write `review.md` with inventory, methodology, validation results, findings (severity/confidence/path/line/evidence/impact/recommendation), unreviewed areas, and commit/release recommendation.
9. [x] Perform a final completeness pass: every in-scope path is covered or explicitly listed as unreviewed; no product file was edited; the report does not claim tests passed without current output.

Quality-check note: this is a report-only task, so findings are documented rather than auto-fixed in product code.

## Validation commands

Use the portable Go binary recorded in `findings.md` when system Go is unavailable:

```text
E:\myCode\.tools\go1.26.1\go\bin\go.exe build ./...
E:\myCode\.tools\go1.26.1\go\bin\go.exe test <focused packages> -count=1
cd relaykit && GOWORK=off E:\myCode\.tools\go1.26.1\go\bin\go.exe build ./...
cd web && bun run typecheck
cd web && bun run build
cd web && bun test <focused files>
git diff --check
```

Actual commands may be narrowed when a full command is known to reproduce documented baseline failures; all omissions must be recorded in `review.md`.

## Review gates

- Do not start product-code editing based on a finding.
- Do not stage or commit unrelated working-tree changes.
- A finding without a reproducible or source-backed evidence anchor is not final.
- If a critical issue requires a fix, stop after documenting it and propose a separate implementation task or obtain explicit scope approval.

## Rollback

Review artifacts can be removed or archived without touching product files. No product-code rollback is needed because this task must not modify product code.
