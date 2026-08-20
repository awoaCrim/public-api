# Design: publish and deploy request-body viewer change

## Change boundary

The working tree contains unrelated changes. The publish set is isolated with an explicit path list and a temporary index so only the four approved request-snapshot files enter the product commit.

The deployment source is `git archive` from that committed `HEAD`, not the dirty working tree. This prevents unrelated local files from entering the Docker build context.

## Preflight and publish flow

1. Reconfirm the GitHub OAuth registration-age status from the checked-in source and the deployed source snapshot. The current evidence is negative: the GitHub user DTO has no `created_at` field, `findOrCreateOAuthUser` has no age comparison, and the only `min-account-age: 30` setting is for the GitHub PR anti-spam workflow. Do not silently add or restore an OAuth restriction in this task.
2. Re-run the focused frontend checks against the four files.
3. Stage only the four approved paths and create one product commit.
4. Verify the commit tree and working-tree status; unrelated changes must remain unstaged.
5. Push `main` to `origin/main`. If normal Git HTTPS/SSH transport is unavailable, use the authenticated GitHub Git Data API fallback and verify the resulting remote tree contains the commit contents.

## Deployment flow

1. Create an archive from the committed `HEAD` with a version derived from the new commit.
2. Transfer and extract it under `/opt/newapi` on `ssh2`.
3. Build `newapi-custom:<version>` remotely using the extracted source.
4. Back up the current Compose file and deployment markers.
5. Change only the `newapi` service `image:` line and run `docker compose up -d --force-recreate newapi`.
6. Check `/api/status` for HTTP 200 and a JSON `success=true` response, then verify the request-log route/bundle is served.
7. If the health check fails, restore the Compose backup and previous image automatically.

## Compatibility and risk

- The app continues to use the existing SQLite database because `SQL_DSN` remains unset in Compose.
- The Redis service, external network, data mount, and session configuration are unchanged.
- The deployment is reversible by restoring the recorded Compose backup and restarting the previous image.
- Existing GitHub transport failures are operational rather than product failures; any API fallback must verify tree contents and never include unrelated paths.
- The GitHub OAuth age check is read-only. The current source/deployed snapshot does not enforce an application-level registration-age limit; restoring one requires a separate scope decision and tests.
