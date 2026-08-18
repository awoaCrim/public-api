# Implementation Plan: Restore Usage Analysis Presentation

1. Inspect the current file diff and use the old repository page only as a read-only visual reference.
2. Keep current query/filter/page orchestration and server-summary usage intact.
3. Rebuild the header/filter row, hero, summary, metric cards, trend chart, and breakdown card to match the old visual hierarchy.
4. Extract feature-local presentational components only where they represent stable page regions and reduce the oversized page component.
5. Retain current cache-read/cache-write fields, legacy disclosure, errors, loading, empty state, reverse-date validation, refresh semantics, and pagination.
6. Add missing translations via the required temporary `add-missing-keys.mjs` workflow; run i18n sync and delete temporary scripts.
7. Add/update focused presentation and helper tests.
8. Verify:
   - focused Usage Analysis tests with Bun;
   - `cd web && bun run i18n:sync` and inspect `_reports/_sync-report.json`;
   - `cd web && bun run typecheck`;
   - affected-file oxlint and format checks;
   - `cd web && bun run build`;
   - `git diff --check`.

## Verification Results

- Focused Usage Analysis API/helper/presentation tests: 9 pass, 0 fail.
- `cd web && bun run typecheck`: pass.
- Targeted oxlint and oxfmt checks: pass.
- `cd web && bun run i18n:sync`: all seven locales report zero missing, extra, and untranslated entries.
- `cd web && bun run build`: pass.
- `git diff --check`: pass; no staged files.

## Rollback

Revert the presentation components and locale additions together. The backend and API contract require no rollback.
