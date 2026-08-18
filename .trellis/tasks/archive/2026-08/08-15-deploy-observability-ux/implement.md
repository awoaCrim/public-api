# Implementation Plan: Publish and Deploy Observability UX

## 1. Pre-publication verification

1. Snapshot `git status --porcelain`, current branch, remotes, and recent commit style.
2. Re-run the completed observability verification matrix:
   - focused Go tests for `controller`, `router`, and `service/authz`;
   - root `go build ./...`;
   - `cd relaykit && GOWORK=off go build ./...`;
   - focused frontend tests for request snapshots, settings navigation, and Usage Analysis;
   - `cd web && bun run typecheck`;
   - targeted oxlint/oxfmt for changed TypeScript/TSX files;
   - `cd web && bun run i18n:sync` and confirm all seven locales have zero missing/extra/untranslated keys;
   - `cd web && bun run build`;
   - `git diff --check`.
3. If verification changes generated files or reveals a defect, return to task review before committing.

## 2. Commit and initial push

1. Classify dirty files into:
   - product code/tests/i18n for the observability UX;
   - related spec/documentation to be finalized after deployment;
   - excluded Trellis runtime/task/archive assets, `.pi/`, `nul`, and unrelated files.
2. Present the mandatory one-shot commit plan to the user before any commit.
3. After confirmation, create the product commit without staging excluded files.
4. Confirm the product commit diff contains only intended files.
5. Push `rebuild/customizations-20260812` to `origin` with upstream tracking. Do not push to `upstream`.

## 3. Production preflight and backup

1. Reconfirm SSH connectivity, container health, current image, Compose path, available disk, SQLite path, and required tools.
2. Create local and remote release identifiers from the exact product commit.
3. Create a Git archive from the exact product commit and upload it to a unique staging path under `/opt/newapi`.
4. Create the timestamped backup directory.
5. Perform SQLite online `.backup`; run `PRAGMA integrity_check` on the backup and require `ok`.
6. Copy the current Compose file, `.env`, current source, and prior image/container metadata into the backup directory without printing secrets.
7. Abort before cutover if any backup step fails.

## 4. Build and cutover

1. Extract the uploaded exact-commit archive into a fresh source directory.
2. Build the immutable `newapi-custom:20260815-observability-<shortsha>` image and capture the build log.
3. Require a successful image build before editing Compose.
4. Update only the `newapi` service image reference in `/opt/newapi/docker-compose.yml`.
5. Validate Compose configuration.
6. Run `docker compose up -d newapi`.
7. Poll container state and `http://127.0.0.1:3000/api/status` for a bounded interval.
8. Verify root HTML delivery, restart count, effective image, and startup/migration logs.

## 5. Rollback gate

If cutover verification fails:

1. Restore the backed-up Compose file/image reference.
2. Recreate the `newapi` service using the previous image.
3. Confirm the previous image is running, restart count is stable, and `/api/status` returns 200.
4. Preserve failure logs and stop; do not mark deployment complete.

## 6. Post-deployment documentation and final push

1. Update `HANDOFF.md`, `docs/customization-migration-report.md`, and `progress.md` with:
   - product commit;
   - deployed image tag;
   - deployment timestamp;
   - backup path and SQLite integrity result;
   - container/status checks;
   - push state;
   - unchanged routing-group/config warnings where still applicable.
2. Retain the observability contract update in `.trellis/spec/backend/logging-guidelines.md` and the related inventory/findings changes.
3. Run `git diff --check` and review the documentation-only diff.
4. Commit the documentation/spec batch.
5. Push the final branch state to `origin`.
6. Verify the remote branch tip equals the local branch tip.

## 7. Final report

Report:

- product and documentation commit hashes;
- remote branch and tracking state;
- image tag and image ID;
- production backup path;
- health/status/restart results;
- whether rollback was needed;
- excluded local dirty files that remain uncommitted.

## Actual Results

- Product commit: `bd8b8746` (`feat: restore admin observability UX`).
- Remote: `origin/rebuild/customizations-20260812`, pushed and tracking configured.
- Production image: `newapi-custom:20260815-observability-bd8b8746`, image ID `sha256:bc3a6c51743b88aca6909bbb9d66a063eda666e2331752b7ca4dfd7cf786a794`.
- Backup: `/opt/newapi/backups/deploy-20260815-042500-bd8b8746`; SQLite `PRAGMA integrity_check` returned `ok`.
- Health: `newapi` running, restart count 0, `/api/status` HTTP 200, root page HTTP 200; no rollback required.
- Production environment, mounted data, Redis, secrets, and routing-group migration state were not changed.
