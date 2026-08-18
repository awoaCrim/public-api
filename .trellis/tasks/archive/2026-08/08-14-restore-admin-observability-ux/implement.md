# Implementation Plan: Restore Admin Observability UX

1. Complete `08-14-request-snapshot-root-direct` and verify Root-only direct retrieval plus audit behavior.
2. Complete `08-14-expose-request-body-log-settings` and verify the dedicated Operations navigation contract.
3. Complete `08-14-restore-usage-analysis-ui` and verify the restored visual landmarks, pagination, errors, and translations.
4. Perform an integration review across route authorization, frontend visibility, settings navigation, and Usage Analysis data flow.
5. Run final checks:
   - affected Go tests and `go build ./...`;
   - focused frontend tests;
   - `cd web && bun run typecheck`;
   - affected-file oxlint/format checks;
   - `cd web && bun run i18n:sync` and inspect the report;
   - `cd web && bun run build`;
   - `git diff --check`.
6. Confirm no unrelated logging, billing, database, LLM Review, or deployment files changed.

## Verification Results

- Focused cross-feature frontend tests: 23 pass, 0 fail.
- `cd web && bun run typecheck`: pass.
- All affected TypeScript/TSX files pass targeted oxlint and oxfmt checks.
- `cd web && bun run i18n:sync`: all seven locales report zero missing, extra, and untranslated entries.
- `cd web && bun run build`: pass.
- `go test ./controller ./router ./service/authz -count=1`: pass.
- `go build ./...`: pass.
- `cd relaykit && GOWORK=off go build ./...`: pass.
- `git diff --check`: pass; no staged files.
- Final integration review additionally guards filter-scope placeholder reuse and rejects mismatched snapshot response IDs.

## Rollback points

- Authorization child can be reverted independently because the snapshot response/storage contract is unchanged.
- Settings-navigation child only changes registry composition and can be reverted without option migration.
- Usage Analysis child only changes frontend presentation/i18n and can be reverted without backend rollback.
