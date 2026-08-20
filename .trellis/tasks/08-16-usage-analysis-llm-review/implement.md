# Implementation plan: usage analysis and LLM review reliability

## Preconditions and review gates

- Keep the parent task in planning until both child designs and plans are reviewed.
- Implement the children independently; do not start the parent as the code-writing target.
- Do not alter unrelated existing worktree changes.
- Before frontend edits, use the loaded `i18n-translate` workflow and write locale changes only through its script.

## Ordered work

1. **Usage-analysis child**
   - Add canonical Root metadata to the options endpoint and a focused controller contract test.
   - Update frontend options types and initial filter/query gating.
   - Add root-default, first-query, fallback, and manual-filter regression coverage.
   - Run focused backend/controller tests and frontend usage-analysis tests/typecheck.
2. **LLM settings/readiness**
   - Add output-mode capability fields/helpers and update critical-change reset logic.
   - Update controller config/status/test endpoints, explicit policy clearing semantics, and enable/readiness validation.
   - Add stale-enabled/unready and missing-policy tests.
3. **LLM client compatibility**
   - Implement the three request modes and capability probe order without changing SSRF/key masking behavior.
   - Implement deterministic content normalization and mode-aware verdict validation.
   - Add client/payload tests for request shapes, fenced/prose/part-array content, ambiguity, and fallback/non-fallback status handling.
4. **Worker/task/audit path**
   - Gate enqueue and worker claims on the shared readiness predicate.
   - Persist selected output mode on tasks and expose it in task detail.
   - Keep compatibility results manual-review-only and preserve the strict auto-ban gate.
   - Add worker regressions for compatibility success, malformed output, stale readiness, missing policy, and no auto-ban.
5. **Frontend settings/i18n**
   - Update types/API and settings status copy for strict versus compatibility modes and missing policy.
   - Add/translate all new keys with `add-missing-keys.mjs`; run `bun run i18n:sync` and inspect the report.
6. **Integration verification**
   - Run `gofmt` on changed Go files.
   - Run focused Go tests for `service`, `controller`, `model` as applicable.
   - Run frontend tests, typecheck, lint, build, and i18n checks using Bun scripts discovered from `web/package.json`.
   - Run the repository's applicable full checks; inspect `git diff --check` and all changed files.

## Review gates

- After usage child: verify the first settled query cannot be all-user in the normal root-present path and root-only authorization is untouched.
- After LLM client: verify fallback parsing never calls `ShouldAutoBan` with a trusted strict mode.
- Before final check: verify no direct `encoding/json` business calls, no dialect-specific migration, no unmasked upstream/body logging, and all new frontend keys exist in seven locales.
- Final quality check must review cross-layer contracts, existing user changes, tests, and production backward compatibility before any commit.

## Rollback points

- If the options contract causes an API/UI mismatch, revert only the root metadata/gating changes while retaining existing aggregation behavior.
- If compatibility probing is unstable, disable compatibility readiness through the setting predicate and preserve strict mode; do not remove auto-ban gates.
- If task-mode migration causes a database issue, stop before deployment and remove only the new model field/change path after confirming old task fields remain intact. No destructive reset or migration rollback is permitted.
