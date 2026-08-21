# Implementation Plan: 分组缓存与活动引用修复

## Ordered steps

1. Read `web/AGENTS.md`, the task PRD/design/research artifacts, and `.trellis/spec/backend/deployment-guidelines.md`; confirm the existing `useUpdateOption` and shared `['groups']` query contract.
2. Write the deterministic hook regression test first:
   - successful `GroupRatio` save invalidates exactly `['groups']`;
   - failed save does not invalidate;
   - successful unrelated option save does not invalidate.
3. Implement the focused frontend fix centrally in `useUpdateOption` success handling; do not add a generic rename API or scattered list refreshes.
4. Run the focused hook test, then frontend typecheck, affected-file lint/format checks, and `bun run build`.
5. Review the production migration target against ssh2: channel 53, two ability rows, token 32, and grant 237. Confirm `test` is the old value and `svip` is the intended value before writing.
6. Create and verify a readable SQLite backup. Execute the four-table migration inside one transaction with explicit predicates, affected-row assertions, and no writes to `logs`, `perf_metrics`, or `quota_data`.
7. On migration failure, roll back the transaction, retain the backup, and verify no partial active-reference update remains. Record bounded, non-sensitive operational evidence.
8. Review the complete authorized diff and run the required pre-push path audit. Commit only after validation is green; do not include unrelated working-tree changes.
9. Push the authorized commit to the intended branch/remote, then deploy from the committed ref using the deployment guideline: unique image tag, Compose/image backup, application-service-only recreation, and rollback readiness.
10. Verify deployment with `docker compose ps`, image inspection, bounded recent logs, `GET /api/status` requiring HTTP 200 and `success=true`, and a served-asset assertion for the changed frontend behavior.
11. If any deployment gate fails, restore the saved Compose/image state, recreate the previous service, and re-check `/api/status` before reporting failure.

## Validation gates

- Focused frontend hook test passes.
- `cd web && bun run typecheck` passes.
- Affected-file lint/format checks pass.
- `cd web && bun run build` passes.
- SQLite backup is readable and migration transaction checks exact authorized rows.
- Historical table counts/content remain unchanged.
- `git diff --check` passes and staged paths contain only authorized implementation/deployment files.
- Remote container and `/api/status` health checks pass after deployment.

## Scope guard

This planning artifact does not authorize production-code edits, database writes, commits, pushes, or deployment during planning. Those actions belong to the execution phase after review of this plan.
