# Implementation Plan: Root-only Direct Request Snapshot Access

1. Inspect relevant diffs first so existing uncommitted snapshot work is preserved.
2. Change the snapshot route to `RootAuth + CriticalRateLimit + DisableCache`.
3. Remove controller proof validation without changing audit/read/error ordering.
4. Remove the obsolete authz resource and security-proof scope, then update affected tests/comments.
5. Change the frontend gate, API loader signature, and request-body section to direct fetch.
6. Update focused frontend tests for Root-only visibility, direct click fetch, errors, byte fidelity, and close-time clearing.
7. Update focused Go tests for direct retrieval and audit fail-closed behavior.
8. Verify:
   - `gofmt` on changed Go files;
   - `go test ./controller ./middleware ./service/authz ./router`;
   - `go build ./...`;
   - focused request-snapshot frontend tests with Bun;
   - `cd web && bun run typecheck`;
   - affected-file oxlint/format checks;
   - `git diff --check`.

## Verification Results

- `go test ./controller ./router ./service/authz -count=1`: pass.
- `go build ./...`: pass.
- Focused request-snapshot frontend tests: 10 pass, 0 fail.
- `cd web && bun run typecheck`: pass.
- Affected-file oxlint and oxfmt checks: pass.
- `cd web && bun run i18n:sync`: all seven locales report zero missing, extra, and untranslated entries.
- `cd web && bun run build`: pass.
- `git diff --check`: pass; no staged files.

## Rollback

Restore the old middleware/proof chain and frontend verifier together. Do not roll back only one side, because proof requirements must remain aligned across API and UI.
