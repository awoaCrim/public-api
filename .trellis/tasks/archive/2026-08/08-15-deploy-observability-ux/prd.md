# Commit, Push, and Deploy Observability UX

## Goal

Publish and deploy the completed administrator observability UX changes so the production `ssh2` instance runs the reviewed Root-only request-body viewer, dedicated Request Snapshots settings entry, and restored Usage Analysis presentation.

## Background

- The observability implementation is complete and locally verified but remains uncommitted.
- The current branch is `rebuild/customizations-20260812`.
- `origin` is `https://github.com/awoaCrim/public-api.git`; the branch does not yet exist on `origin`.
- Production is the existing `ssh2` Docker Compose deployment under `/opt/newapi`.
- Production currently runs `newapi-custom:20260814-remediated-da678d51`; the `newapi` container is healthy, has restart count 0, and `GET http://127.0.0.1:3000/api/status` returns 200.
- Production uses SQLite at `/opt/newapi/data/one-api.db`; `sqlite3` and Docker Compose v2 are available.
- The prior deployment pattern built an image from an exact Git commit archive, created a consistent SQLite backup plus compose/environment/source rollback assets, switched the Compose image, and verified health.
- The working tree also contains untracked Trellis runtime/task assets, `.pi/`, and `nul`; these are not product deliverables and must not enter product commits or deployment archives.

## Requirements

1. Re-verify the completed observability changes before publishing.
2. Commit only the observability product code, tests, translations, and directly related documentation/spec updates. Exclude unrelated/untracked Trellis runtime assets, archived task metadata, `.pi/`, and `nul`.
3. Preserve the reviewed security and data contracts:
   - request snapshot reads remain Root-only, direct, audited, rate-limited, and non-cacheable;
   - request bodies remain on-demand and absent from usage-log list payloads;
   - Usage Analysis retains bounded SQL aggregation, pagination, filter, timeout, and billing semantics;
   - Request Snapshots settings remain a single canonical Operations section.
4. Push `rebuild/customizations-20260812` to `origin` and establish upstream tracking. Do not push to `upstream` and do not create a PR unless separately requested.
5. Deploy an image built from the exact reviewed product commit to the existing `ssh2` production Compose stack.
6. Before switching the container, create a timestamped deployment backup containing:
   - an online SQLite `.backup` of `one-api.db` with `PRAGMA integrity_check = ok`;
   - the current Compose file;
   - the current `.env` without printing its contents;
   - the current source directory or source archive;
   - the prior image/container metadata needed for rollback.
7. Build the replacement image before changing Compose, then perform the shortest practical single-container cutover.
8. If build, startup, migration, or health verification fails, restore the previous Compose/image and confirm production health.
9. After a successful cutover, verify container state, restart count, `/api/status`, root HTML delivery, and startup logs for fatal/migration errors.
10. Record the final product commit, image tag, backup path, deployment result, and push state in the existing handoff/progress/report documentation, then push the final documentation commit.
11. Do not alter production environment variables, enable optional features, run strict routing-group migration, change the unresolved routing-group mapping, or perform unrelated database/data cleanup.

## Out of Scope

- Creating a pull request or merging the branch.
- Pushing to the protected upstream repository.
- Changing Caddy, `SESSION_COOKIE_SECURE`, `TRUSTED_PROXIES`, secrets, or optional feature settings.
- Running the strict routing-group migration or changing legacy group data.
- Committing Trellis runtime/bootstrap files, archived task directories, `.pi/`, or `nul`.
- Refactoring or adding product behavior beyond the already reviewed observability changes.

## Acceptance Criteria

- [x] Focused Go/frontend tests, frontend typecheck/build/i18n checks, root and relaykit builds, and `git diff --check` pass with only documented baseline exceptions.
- [x] The product commit contains the intended observability files and no unrelated runtime/task artifacts.
- [x] `origin/rebuild/customizations-20260812` exists and the local branch tracks it.
- [x] A timestamped production backup exists and its SQLite integrity check reports `ok`.
- [x] Production runs a newly tagged image built from the exact product commit.
- [x] The `newapi` container is running with restart count 0 after cutover.
- [x] `GET /api/status` returns HTTP 200 and the root page is served successfully.
- [x] Startup logs show no fatal build/startup/migration failure attributable to this release.
- [x] No rollback was needed; the previous image remains available for rollback.
- [x] Existing production environment/configuration and routing-group migration state are unchanged.
- [x] Handoff/progress/report documentation records the final commit, image, backup, health checks, and push state.
- [x] The final documentation commit is pushed to `origin`.

## Blocking Open Questions

None. The user's request to push and deploy, the existing branch/remote, and the established `ssh2` production target determine the intended scope. The deployment will use the same backup-first, exact-commit, fail-rollback pattern as the prior release.
