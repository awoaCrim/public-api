# Push and deploy current changes

## Goal

Push the authorized completed changes to `origin/main` and deploy the resulting application to `ssh2` so the new **LLM Review Trigger Limits** settings entry is available in the running instance.

## User value

The administrator can adjust RPM, input-token, output-token, and model-whitelist thresholds in the deployed system instead of having the backend configuration hidden behind the generic option API.

## Confirmed facts

- Local `main` is three commits ahead of `origin/main`.
- The authorized publish set includes those three existing local commits plus the uncommitted LLM Review Trigger Limits UI change and its focused test/locales.
- The authorized uncommitted files are:
  - `web/src/features/system-settings/request-limits/review-trigger-limits-section.tsx`
  - `web/src/features/system-settings/security/__tests__/section-registry.test.tsx`
  - `web/src/features/system-settings/security/index.tsx`
  - `web/src/features/system-settings/security/section-registry.tsx`
  - `web/src/features/system-settings/types.ts`
  - the seven modified JSON files under `web/src/i18n/locales/`
- Uncommitted Vision, Trellis, and other unrelated changes must remain untouched and outside the deployment commit.
- The push target is `origin/main`.
- The deployment target is `ssh2`, resolving to `root@43.131.249.217:22` through the local SSH configuration.
- `ssh2` runs the `newapi` Docker Compose project from `/opt/newapi/docker-compose.yml`; the active `newapi` container uses `newapi-custom:v1.0.0-custom.8-3f83fcbb`.
- The remote deployment layout uses `/opt/newapi/src-*` source snapshots, `/opt/newapi/staging` build artifacts, `/opt/newapi/backups` deployment backups, and marker files for the last image and backup.
- The UI change has passed focused tests, typecheck, affected lint/format checks, i18n sync, and `bun run build:check`.

## Requirements

1. Audit the authorized file list before staging.
2. Preserve unrelated working-tree changes without reverting, staging, or overwriting them.
3. Run final validation for the authorized change set.
4. Create a commit containing only the authorized uncommitted files, then push `main` to `origin/main`.
5. Build a uniquely tagged custom Docker image on `ssh2` from the pushed committed tree.
6. Back up the current Compose configuration and image marker before changing the running service.
7. Recreate only the `newapi` service using the new image while preserving data, Redis, ports, environment, and network configuration.
8. Verify service health and the new settings route after deployment.
9. Roll back the Compose image reference and recreate the previous service if build/startup/health verification fails.
10. Do not expose credentials, alter database contents destructively, or modify protected project identifiers.

## Acceptance criteria

- [x] The authorized files are explicitly identified and unrelated changes remain uncommitted and untouched.
- [x] A local commit contains only the authorized system-settings/locales change.
- [x] The resulting `main` is pushed to `origin/main` through the authenticated GitHub Git Data API after the local Git HTTPS transport failed; the remote ref is `800871ee50bb07cb6915576a2ae9fd8b34471cf1`.
- [x] A uniquely tagged image built from the pushed tree is running on `ssh2`: `newapi-custom:v1.0.0-custom.9-894e93ef`.
- [x] `/api/status` is healthy after deployment and recent logs show normal startup with the LLM review worker running.
- [x] The deployed frontend returns the settings route and its served main bundle contains `review-trigger-limits` and `LLM Review Trigger Limits`.
- [x] Two initial deployment attempts were automatically rolled back after a quoting-sensitive health-check script reported failure; the final deployment succeeded with a direct JSON/HTTP health check.

## Out of scope

- Deploying or committing unrelated Vision interception, Trellis, or other working-tree changes.
- Changing RPM/token enforcement logic or the backend option API.
- Changing production credentials, server configuration, or database contents.
- Creating a release tag or publishing a public release.
