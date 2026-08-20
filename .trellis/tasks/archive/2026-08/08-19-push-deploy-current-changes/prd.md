# Push and deploy authorized changes

## Goal

Finish every authorized in-scope change to its acceptance and verification boundary before creating any new commit, then publish the reviewed commits to `origin/main` and deploy the exact pushed committed tree to the existing remote Docker Compose application with a health gate and rollback path.

## User value

The requested changes become available in the running instance without accidentally publishing unrelated, incomplete, or Trellis-generated working-tree files.

## Confirmed repository facts

- The current branch is `main` and is five commits ahead of `origin/main`.
- The push remote is `origin` (`https://github.com/awoaCrim/public-api.git`); `upstream` points to the protected project repository and is not the push target.
- The five local commits already on `main` are:
  - `b7123a90 feat(web): format request snapshot JSON display`
  - `894e93ef feat: expose llm review trigger limits`
  - `45245dc2 chore: record journal`
  - `47dd2379 chore(task): archive 08-16-usage-analysis-root-default`
  - `31515293 test: harden usage analysis query regression`
- The working tree contains two product feature groups plus workflow/task metadata:
  - GitHub OAuth registration-age implementation and tests;
  - Vision interception/default-prompt/Responses API changes and tests;
  - Trellis/Pi workflow installation files, specs, active and archived task artifacts, and workspace state.
- Independent read-only audits found that neither product group should be committed unchanged yet.
  - GitHub OAuth still needs localized numeric validation errors and stronger provider metadata, legacy-ID, startup/settings API, callback localization, and real form tests.
  - Vision still needs trusted-boundary validation for `phash_threshold`, a resolved and accurate threshold-`0` contract, real middleware/component behavior tests, and copyright-header compliance for new frontend files.
- The product source groups contain untracked runtime files, so they must be staged from explicit path manifests; `git add -u` would produce incomplete builds.
- The deployment spec requires a committed source archive, a unique remote image tag, a backup of the active Compose/image state, an update to only the application image, an HTTP/JSON health check, and rollback on build/startup/health failure.
- The production target has been revalidated read-only as `ssh2` (`root@43.131.249.217:22`), `/opt/newapi`, Compose service/container `newapi`, active image `newapi-custom:v1.0.0-custom.10-b7123a90`, and a responding `/api/status` endpoint.

## Requirements

1. Establish an explicit authorized completion and publish set before editing or staging anything.
2. Finish each in-scope product feature rather than merely making its current tests pass:
   - close the audited GitHub OAuth registration-age contract and test gaps;
   - close the audited Vision validation, behavior, test, and frontend-header gaps.
3. Preserve all non-authorized working-tree changes; do not revert, delete, or silently include them.
4. Keep the GitHub OAuth and Vision changes independently reviewable and validated before commit.
5. Run the applicable focused and repository-level validation for the completed change set before commit and deployment; document unrelated pre-existing failures instead of hiding them.
6. Create two focused commits containing only the authorized finished GitHub OAuth and Vision manifests.
7. Review the five existing local commits that will be pushed with `main`; do not rewrite published history without a separate explicit decision.
8. Push only to `origin/main`; do not push to `upstream`.
9. Build the deployment artifact from the pushed committed tree, not from the dirty working directory.
10. Back up the remote Compose file and current image marker before changing the running service.
11. Update only the `newapi` application image and preserve database, Redis, environment, volumes, ports, and networks.
12. Require the recreated service and `/api/status` to pass bounded health checks before declaring success.
13. Verify the deployed GitHub OAuth settings behavior and Vision-related frontend/server assets through observable checks that do not require destructive production data changes.
14. Restore the previous Compose/image state if remote build, startup, or health verification fails.
15. Do not expose credentials, modify database contents destructively, or alter protected project identifiers.

## Acceptance Criteria

- [x] The approved publish set is documented by path and separated from unrelated working-tree changes.
- [x] GitHub OAuth registration age has localized `0..100` integer validation, complete provider-to-policy/settings/callback/form regression coverage, and preserves existing numeric/legacy login and bind flows.
- [x] Vision rejects new thresholds outside `0..64`, safely handles legacy invalid values, performs no pHash work or clustering at `0`, and has behavior-level middleware/component regression coverage.
- [x] Both product groups pass their focused Go/frontend tests, affected-file lint/format/copyright checks, typecheck, i18n sync, builds, and the applicable repository-level gates before any new commit is created.
- [x] The resulting two feature commits contain exactly their approved manifests; additional Trellis/Pi/task/workspace paths remain outside the new commits.
- [x] The resulting commit history and pushed `origin/main` ref contain the existing five commits plus the two reviewed feature commits, and no other newly committed paths.
- [x] The deployment image is built from the pushed commit and has a unique tag.
- [x] The remote Compose configuration changes only the application image, with a rollback backup retained.
- [x] The new service is running and `/api/status` returns HTTP 200 with JSON `success=true`.
- [x] The deployed frontend/source assets visibly contain the GitHub OAuth setting and Vision behavior markers corresponding to the pushed commit.
- [x] No credentials, unrelated files, or destructive database operations are included.

## Execution evidence

- Product commits: `58f97d2b` (GitHub OAuth, 29 paths) and `4f91593f` (Vision, 12 paths).
- The remote had already recreated the five older local commits under alternate SHAs during prior Git Data API deployments. The tree-identical merge `526d7bd1` reconciles those histories while preserving the reviewed content; the comparison against the previous remote head contains exactly the 41 approved product paths.
- Pushed ref: `origin/main` = `526d7bd163dbc05c6d36ccb44a386f92ee9bb649`; `upstream` was not modified.
- Deployment: `newapi-custom:v1.0.0-custom.11-526d7bd1`, built from the checksum-verified archive of that pushed SHA.
- Rollback state: `/opt/newapi/backups/deploy-v1.0.0-custom.11-526d7bd1`; only the Compose application image differs, and the previous image remains retained.
- Runtime checks: `/api/status` HTTP 200 with `success=true`; the GitHub OAuth settings route and profile route return HTTP 200; the served JS contains `Minimum GitHub Account Age` and `Vision Interception`.
- Unrelated Trellis/Pi/task/workspace and `nul` paths remain outside the product commits; no credentials or destructive database operations were used.

## Confirmed publish scope

The user selected the product-only scope:

- Finish, validate, commit, push, and deploy the GitHub OAuth registration-age feature.
- Finish, validate, commit, push, and deploy the Vision interception/default-prompt/Responses API feature.
- Exclude Trellis/Pi installation files, active task/workspace metadata, archived task records, and the accidental `nul` path from new commits and the deployment publish manifest.
- Do not implement the separate unresolved RPM grace-period task as part of this deployment.
- Preserve and push the five existing local `main` commits; two already contain Trellis journal/task history, but no additional workflow metadata will be committed in this task.
- For Vision, `phash_threshold=0` disables perceptual-hash calculation and clustering. Each image remains a separate cluster; existing exact-URL/cache behavior is preserved. Values `1..64` enable pHash clustering.

### GitHub OAuth product path manifest

- `common/github_oauth.go`
- `common/github_oauth_test.go`
- `controller/oauth.go`
- `controller/github_oauth_registration_age_test.go`
- `i18n/keys.go`
- `i18n/locales/en.yaml`
- `i18n/locales/zh-CN.yaml`
- `i18n/locales/zh-TW.yaml`
- `model/option.go`
- `model/github_oauth_option_test.go`
- `oauth/github.go`
- `oauth/github_test.go`
- `oauth/types.go`
- `web/src/features/system-settings/auth/index.tsx`
- `web/src/features/system-settings/auth/oauth-section.tsx`
- `web/src/features/system-settings/auth/section-registry.tsx`
- `web/src/features/system-settings/auth/github-oauth-age.ts`
- `web/src/features/system-settings/auth/__tests__/github-oauth-age.test.ts`
- `web/src/features/system-settings/auth/__tests__/oauth-section-github-age.test.tsx`
- `web/package.json`
- `web/bun.lock`
- `web/src/features/system-settings/types.ts`
- the seven source locale files under `web/src/i18n/locales/`

### Vision product path manifest

- `controller/user.go`
- `controller/user_vision_setting_test.go`
- `middleware/vision_intercept.go`
- `middleware/vision_intercept_test.go`
- `service/vision/intercept.go`
- `service/vision/vision.go`
- `service/vision/vision_test.go`
- `web/src/features/profile/components/vision-interception-card.tsx`
- `web/src/features/profile/lib/index.ts`
- `web/src/features/profile/lib/vision.ts`
- `web/src/features/profile/lib/__tests__/vision.test.ts`
- any focused component test added to verify the Vision card behavior

The two manifests overlap only through repository-wide validation; they must be committed as separate feature commits.

## Task map and ordering

- `08-19-complete-github-oauth-registration-age`: close and verify the GitHub OAuth feature gaps.
- `08-19-complete-vision-interception`: close and verify the Vision feature gaps.
- `08-19-publish-deploy-completed-product-changes`: after both feature children pass review, create the two focused product commits, push `origin/main`, and deploy the exact pushed tree.

The first two children are independently verifiable but use the same working tree and therefore run with one writer at a time. The publish/deploy child is blocked until both feature children have passed their full check gates. No new product commit is created before both feature groups are complete.

## Out of Scope

- Pushing to `upstream`.
- Implementing unresolved planning-only behavior from other active tasks, including changing whether RPM-triggered reviews bypass the compliant grace period, unless separately authorized.
- Committing additional Trellis/Pi installation, workspace, active task, or archived task metadata.
- Destructive schema/data migrations, credential rotation, or production configuration changes.
- Creating a release tag or public release unless separately requested.

