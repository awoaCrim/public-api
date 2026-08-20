# Commit and deploy structured request body viewer

## Goal

Commit only the approved request-body viewer formatting change, publish it to `origin/main`, and deploy the resulting application to `ssh2` without including unrelated working-tree changes.

## Confirmed scope

The approved product change is limited to these four files:

- `web/src/features/usage-logs/components/__tests__/request-snapshot-section.test.tsx`
- `web/src/features/usage-logs/components/dialogs/request-snapshot-section.tsx`
- `web/src/features/usage-logs/lib/__tests__/request-snapshot.test.ts`
- `web/src/features/usage-logs/lib/request-snapshot.ts`

The change formats valid JSON for display while preserving raw copy/download behavior. The focused tests, typecheck, lint, format check, build check, and `git diff --check` have already passed before publishing.

## Additional verification scope

Confirm before publishing whether the GitHub OAuth login/registration flow still enforces the previously expected GitHub-account registration-age restriction. This is a verification item, not permission to add unrelated OAuth behavior to the request-body viewer commit.

Current repository and deployed-source inspection finds no such restriction: the GitHub user payload does not read `created_at`, the OAuth registration path only checks `RegisterEnabled`, and the only `min-account-age: 30` occurrence is the GitHub PR anti-spam workflow, not application OAuth. If the user wants that restriction restored, it must be planned as a separate product change rather than folded into this deployment.

## Requirements

- Create a product commit containing exactly the four approved files above.
- Do not stage, commit, push, archive, or deploy existing Vision, Trellis, profile, or other unrelated changes.
- Push the authorized commit to `origin/main`; if the configured Git transport is unavailable, use an authenticated equivalent and verify the remote tree.
- Build a uniquely tagged Docker image from the committed tree on `ssh2`.
- Update only the application image in `/opt/newapi/docker-compose.yml`.
- Preserve the existing SQLite/Redis Compose configuration and mounted data volume.
- Health-check the deployed application before reporting success and retain a rollback backup.
- Record the GitHub OAuth registration-age verification result and do not claim that an application-level age restriction is active when the source does not enforce one.

## Acceptance criteria

- [x] The product commit contains exactly the four approved request-snapshot paths.
- [x] Unrelated working-tree changes remain uncommitted and untouched.
- [x] `origin/main` contains the committed request-body viewer change.
- [x] The uniquely tagged image is built from the committed tree and running on `ssh2`.
- [x] `/api/status` returns HTTP 200 with `success=true` after deployment.
- [x] The request-log frontend serves the updated request-body viewer bundle.
- [x] Compose/image rollback information is recorded and usable.
- [x] GitHub OAuth registration-age status is explicitly verified and reported; the PR workflow's `min-account-age: 30` is not treated as an OAuth restriction.

## Execution evidence

- Local product commit: `b7123a90009eda0e2a0e5eec680df41c7cbe34dc`; its tree contains only the four approved paths relative to its parent.
- Remote `origin/main` was updated through the authenticated Git Data API fallback because normal Git transport reset the connection. Remote commit: `dc4712310475c49ddaff29795e806bc85ef6b0d4`, based on remote commit `800871ee50bb07cb6915576a2ae9fd8b34471cf1`; the verified remote comparison contains exactly the four approved paths.
- Deployed image: `newapi-custom:v1.0.0-custom.10-b7123a90`; `/api/status` returned HTTP 200 with `success=true`; `/usage-logs/common` returned HTTP 200; the served async bundle contained the JSON formatting code.
- Rollback backup: `/opt/newapi/backups/deploy-v1.0.0-custom.10-b7123a90/docker-compose.yml`; the Compose diff changed only the application image, and Redis remained unchanged.
- GitHub OAuth verification: no application-level registration-age check exists in the checked-in or deployed source; `min-account-age: 30` is only the PR anti-spam workflow setting.

## Out of scope

- Any Vision interception changes.
- Any Trellis/spec/task artifact changes in the product commit.
- Additional request-body viewer features or backend changes.
- Changes to database configuration, rate limits, LLM review behavior, or access control.
- Restoring or changing a GitHub OAuth registration-age restriction; if requested, that is a separate follow-up implementation task.
