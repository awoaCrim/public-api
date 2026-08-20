# Implementation plan: publish and deploy

- [x] Re-run combined validation and inspect the final dirty path inventory.
- [x] Stage, audit, and commit the GitHub OAuth manifest.
- [x] Stage, audit, and commit the Vision manifest.
- [x] Verify history and confirm excluded Trellis/Pi/task/workspace/`nul` paths remain dirty or untracked.
- [x] Reconcile the already-diverged remote deployment history with a tree-identical merge, push only `origin/main`, and verify the remote SHA and 41-path comparison.
- [x] Revalidate remote baseline, disk space, active image, Compose service, and health without printing environment values.
- [x] Create `git archive` from the pushed SHA, transfer, verify checksum, extract, and remotely build a unique image.
- [x] Save the Compose, previous image, image ID, source SHA, target image, and archive checksum in the rollback backup.
- [x] Update only the `newapi` image and recreate only that service.
- [x] Poll direct `/api/status` HTTP/JSON health with a bounded timeout and inspect bounded startup logs.
- [x] Verify served routes/assets contain both feature markers and record the pushed SHA, image tag, and backup path.
- [x] Retain an automatic rollback trap that restores and health-checks the previous Compose state on startup or health failure.
- [x] Archive/finish the feature and deployment tasks without auto-committing task metadata.

## Verification result

- Focused Go gate: `go test ./common ./oauth ./model ./controller ./middleware ./service/vision -count=1 -p=1` — 709 passed.
- Focused frontend gate — 11 tests passed; `bun run typecheck` and `bun run build:check` passed.
- Affected lint/format/i18n/copyright checks passed. Repository-wide copyright failures remain limited to six unrelated pre-existing frontend files.
- Full serial Go run reached 1732 passes with three unrelated flaky failures in service/channel tests; the affected packages and isolated middleware suite passed.
- `origin/main`: `526d7bd163dbc05c6d36ccb44a386f92ee9bb649`.
- Image: `newapi-custom:v1.0.0-custom.11-526d7bd1` (`sha256:4c76420d51cdc13e1b39b54fd9c340dcbb4f5bde411686c11021843e42f7a204`).
- Backup: `/opt/newapi/backups/deploy-v1.0.0-custom.11-526d7bd1`.
- Health: HTTP 200 and `success=true`; GitHub OAuth and Vision markers were found in the served `/static/js/index.45b521f593.js` asset.
