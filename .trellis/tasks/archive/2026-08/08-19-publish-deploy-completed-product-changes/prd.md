# Publish and deploy completed product changes

## Goal

After both product children pass review, create two focused feature commits, push `origin/main`, and deploy the exact pushed committed tree to `ssh2:/opt/newapi` with health verification and rollback.

## Preconditions

- `08-19-complete-github-oauth-registration-age` has passed all checks.
- `08-19-complete-vision-interception` has passed all checks.
- No new commit was created before both preconditions were satisfied.
- Explicit final path manifests are available for both features.

## Requirements

- Create one GitHub OAuth feature commit and one Vision feature commit; exclude all additional Trellis/Pi/task/workspace/`nul` paths.
- Preserve the existing five local commits and push only `origin/main`.
- Verify the remote ref and exact changed paths; use authenticated GitHub fallback only if normal Git transport fails.
- Archive the pushed SHA, remotely build a unique image, back up the active Compose/image state, and update only `newapi`.
- Require `/api/status` HTTP 200 with JSON `success=true`, inspect bounded logs, and verify deployed feature markers.
- Roll back to the prior image/Compose state on build/startup/health failure.

## Acceptance criteria

- [x] The two commits contain exactly the approved product manifests.
- [x] `origin/main` contains the existing five commits plus the two feature commits and no unintended new paths.
- [x] The built image is uniquely tagged from the pushed SHA.
- [x] Only the Compose application image changes, with backup retained.
- [x] Health and feature-marker checks pass, or rollback restores the previous healthy image.

## Execution evidence

- GitHub OAuth commit: `58f97d2b feat(oauth): enforce GitHub registration account age` (29 approved paths).
- Vision commit: `4f91593f feat(vision): harden image interception` (12 approved paths).
- The actual remote already contained API-recreated equivalents of the five older local commits. Normal push therefore rejected the stale parallel history. A history-only merge, `526d7bd1 merge: reconcile remote deployment history`, preserved both histories without changing the reviewed tree; the remote-relative comparison contains exactly the 41 approved product paths.
- `origin/main` and local `main` both resolve to `526d7bd163dbc05c6d36ccb44a386f92ee9bb649`.
- Deployed image: `newapi-custom:v1.0.0-custom.11-526d7bd1`, built from a verified `git archive` of the pushed SHA and labeled with the full revision.
- Rollback backup: `/opt/newapi/backups/deploy-v1.0.0-custom.11-526d7bd1`; its Compose diff changes only the `newapi` image, and the prior image remains available.
- `/api/status` returned HTTP 200 with JSON `success=true`; `/system-settings/auth/github` and `/profile` returned HTTP 200; the served JS asset contains both `Minimum GitHub Account Age` and `Vision Interception` markers.

## Out of scope

- Pushing `upstream`, creating release tags, committing additional workflow metadata, changing production settings/data, or deleting the rollback image.
