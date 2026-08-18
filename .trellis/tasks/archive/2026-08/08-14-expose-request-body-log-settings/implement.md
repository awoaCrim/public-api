# Implementation Plan: Expose Request Body Log Settings

1. Inspect the current Operations registry diff and existing section patterns.
2. Remove `RequestSnapshotSettingsSection` from the `logs` section fragment.
3. Add a dedicated `request-snapshots` registry entry using the same default-value mapping.
4. Reuse the existing `Request Snapshots` translation key and verify no locale edit is needed.
5. Add a focused registry/navigation regression test.
6. Verify:
   - focused system-settings registry test with Bun;
   - `cd web && bun run typecheck`;
   - affected-file oxlint and format checks;
   - `cd web && bun run build`;
   - `git diff --check`.

## Verification Results

- Focused Operations registry test: 2 pass, 0 fail.
- `cd web && bun run typecheck`: pass.
- Targeted oxlint and oxfmt checks: pass.
- `cd web && bun run build`: pass.
- Existing `Request Snapshots` key confirmed in all seven locales; no locale edits required.
- `git diff --check`: pass.

## Rollback

Move the existing component back under `logs` and remove the registry entry. No backend or data migration rollback is required.
