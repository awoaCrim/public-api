# Implementation plan: push and deploy to ssh2

## Preflight

1. Confirm branch, remote, and authorized file list.
2. Run the focused frontend test, typecheck, affected lint/format check, `bun run i18n:sync`, and `git diff --check`.
3. Inspect the staged diff and verify no Vision/Trellis/unrelated paths are included.

## Publish

4. Stage only the authorized system-settings and locale files.
5. Create a commit describing the visible settings-entry fix.
6. Verify the commit contains only the authorized files.
7. Push `main` to `origin/main`.

## Remote build and deploy

8. Create a tar archive from the pushed committed tree and transfer it to `ssh2:/opt/newapi/staging/`.
9. On `ssh2`, extract the archive into a new `/opt/newapi/src-*` snapshot and build a uniquely tagged `newapi-custom` image with Docker.
10. Save a timestamped backup of `/opt/newapi/docker-compose.yml` and the current image marker.
11. Update the Compose image tag and recreate only the `newapi` service.
12. Wait for the container and health endpoint to become ready.
13. Verify the deployed source/image tag, `/api/status`, recent logs, and the presence of the new security settings route in the embedded frontend bundle.

## Rollback points

- Before local commit: discard no changes; remove staging only if the staged path audit fails.
- Before push: abort if staged paths differ from the authorized list.
- Before remote Compose update: retain the old Compose file and image tag.
- After failed startup/health check: restore the old Compose file and run `docker compose up -d --force-recreate newapi`.

## Validation commands

- `cd web && bun test src/features/system-settings/security/__tests__/section-registry.test.tsx`
- `cd web && bun run typecheck`
- `cd web && bunx oxlint -c .oxlintrc.json <affected files>`
- `cd web && bunx oxfmt --check <affected files>`
- `cd web && bun run build:check`
- `git diff --check`
- Remote: `docker compose -f /opt/newapi/docker-compose.yml ps`, `curl -fsS http://127.0.0.1:3000/api/status`, and bounded `docker logs` inspection.

Do not run `task.py start` until this plan is explicitly approved; after approval, implementation starts with the preflight path audit.

## Execution result

- Preflight frontend test, typecheck, affected lint/format, i18n sync, build check, and diff checks passed.
- The authorized UI change was committed as `894e93ef` with exactly 12 authorized paths; unrelated working-tree changes remained unstaged.
- Direct `git push` over the configured HTTPS remote failed because the GitHub host connection was reset/timed out. The authenticated GitHub Git Data API was used as a fallback to recreate the four-commit content chain and update `origin/main` to `800871ee50bb07cb6915576a2ae9fd8b34471cf1`.
- The source archive was transferred to `ssh2:/opt/newapi/staging/`, built successfully, and deployed as `newapi-custom:v1.0.0-custom.9-894e93ef`.
- The first two deployment health-check attempts were rolled back automatically because the nested remote-shell check was quote-sensitive. The final deployment used a direct HTTP/JSON check and succeeded.
- Final verification: `newapi` is running, `/api/status` returns HTTP 200 with `success=true`, the route returns HTTP 200, and the served main JavaScript contains `review-trigger-limits` and `LLM Review Trigger Limits`.
