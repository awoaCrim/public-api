# Design: push and deploy to ssh2

## Scope boundary

Publish the existing three local commits that are ahead of `origin/main`, then commit only the LLM Review Trigger Limits UI change and its focused test/locales. Do not stage or alter the unrelated Vision, Trellis, or other working-tree changes.

The push target is `origin/main`. The runtime target is `root@ssh2:/opt/newapi`, where the `newapi` Compose project currently runs `newapi-custom:v1.0.0-custom.8-3f83fcbb`.

## Change-set proof

Before staging:

- record `git diff --name-only` for the authorized UI files;
- verify the three existing commits with `git log origin/main..main --oneline`;
- verify unrelated paths remain unstaged;
- run `git diff --check`.

The deployment commit must contain exactly:

- `web/src/features/system-settings/request-limits/review-trigger-limits-section.tsx`;
- `web/src/features/system-settings/security/__tests__/section-registry.test.tsx`;
- `web/src/features/system-settings/security/index.tsx`;
- `web/src/features/system-settings/security/section-registry.tsx`;
- `web/src/features/system-settings/types.ts`;
- the seven modified locale JSON files under `web/src/i18n/locales/`.

## Deployment flow

1. Push local `main` to `origin/main` after committing the authorized UI files.
2. Create a source archive from the resulting committed HEAD. This excludes unrelated uncommitted files while preserving the exact pushed tree.
3. Copy the archive to `/opt/newapi/staging/` on `ssh2`.
4. On `ssh2`, extract into a new timestamped/versioned source directory beside the existing snapshots.
5. Build a new image remotely with a unique tag derived from the pushed commit, for example `newapi-custom:v1.0.0-custom.<short-sha>`.
6. Back up `/opt/newapi/docker-compose.yml` and the current image marker before changing the Compose image reference.
7. Update only the `image:` value for the `newapi` service, preserving ports, environment variables, volumes, network, and Redis configuration.
8. Run `docker compose -f /opt/newapi/docker-compose.yml up -d --force-recreate newapi` and wait for the container to become running/healthy.
9. Verify the application health endpoint and inspect recent container logs for startup/migration errors.
10. Verify the deployed bundle contains the new settings route/label. If deployment fails, restore the prior Compose file/image and recreate the old container.

## Safety and rollback

- Never print or copy `/opt/newapi/.env`, database DSNs, session secrets, or other credential files.
- Do not run database-destructive commands.
- Preserve the existing `/opt/newapi/data` bind mount and `newapi-redis` service.
- The rollback point is the saved Compose file plus the previous image tag recorded before deployment.
- If the build or health check fails, stop before deleting the old image and restore the previous Compose configuration.
