# Implementation plan: finish all authorized changes before commit

## Phase A: complete GitHub OAuth

1. Activate `08-19-complete-github-oauth-registration-age` and load backend/frontend/i18n guidance.
2. Add failing tests for the audited provider metadata, legacy-ID login, startup/settings API, callback localization, and real form contracts.
3. Implement the smallest changes required to pass those contracts, including localized integer/range validation.
4. Run focused Go/frontend/i18n/type/lint/format/build checks and an independent Trellis check.
5. Leave the completed files uncommitted until Vision also passes.

## Phase B: complete Vision

6. Activate `08-19-complete-vision-interception` with one writer in the same worktree.
7. Add failing tests for threshold rejection and legacy normalization, threshold-zero no-pHash behavior, real Responses middleware rewriting/model preservation, and profile-card interactions.
8. Implement threshold `0..64` validation, safe runtime normalization, zero-threshold separate grouping, testable middleware behavior, and copyright-compliant frontend files.
9. Run focused Go/frontend/type/lint/format/copyright/build checks and an independent Trellis check.
10. Run combined validation across both complete feature sets. Do not commit if either feature or an affected-file gate fails.

## Phase C: commit and publish

11. Activate `08-19-publish-deploy-completed-product-changes` only after both feature children pass.
12. Audit the two path manifests and inspect secrets/protected identifiers.
13. Stage and commit only the GitHub OAuth manifest; verify `git diff --cached --name-only` and the resulting commit paths.
14. Stage and commit only the Vision manifest; repeat the path audit.
15. Verify unrelated Trellis/Pi/task/workspace files remain uncommitted and the local history is the existing five commits plus exactly two feature commits.
16. Push `main` to `origin/main`. If HTTPS Git transport fails, use only the authenticated GitHub MCP/Git Data fallback based on the actual remote `heads/main`, and explicitly verify parent/ref/path comparison.

## Phase D: committed-tree deployment

17. Revalidate `ssh2:/opt/newapi`, the `newapi` service, current image, disk capacity, and healthy rollback baseline without exposing environment values.
18. Create and transfer a source archive from the verified pushed SHA.
19. Extract to a unique remote source directory and build a unique image tag.
20. Back up the full Compose file, previous image tag, and running image ID.
21. Update only the `newapi` service image and recreate only that service.
22. Poll `/api/status` with a bounded timeout; require HTTP 200 and JSON `success=true`.
23. Inspect bounded recent logs and verify the GitHub OAuth setting plus Vision markers in the deployed source/frontend assets.
24. On failure, restore the Compose backup, recreate the previous image, and require healthy rollback before reporting.

## Validation gates

### GitHub OAuth

- `gofmt` on affected Go files.
- Focused `go test` for `common`, `oauth`, `model`, and `controller` contracts.
- Focused frontend form/helper tests.
- Affected-file `oxlint` and `oxfmt --check`.
- `bun run i18n:sync`, report inspection, typecheck, copyright check, and `bun run build:check`.

### Vision

- `gofmt` on affected Go files.
- Focused controller, middleware, and `service/vision` tests, including behavior-level middleware tests.
- Focused helper/card tests.
- Affected-file `oxlint` and `oxfmt --check`.
- Typecheck, copyright check, and `bun run build:check`.

### Combined/repository

- `go test ./common ./oauth ./model ./controller ./middleware ./service/vision -count=1`.
- `go build ./...`.
- `cd relaykit && GOWORK=off go build ./...` when affected APIs are compiled through the root/relaykit boundary.
- Applicable full Go/frontend checks; unrelated pre-existing failures must be reproduced and documented.
- `git diff --check` before staging and after both commits.

## Rollback points

- Before commit: preserve all user changes; unstage only the explicit staged manifest if an audit fails.
- Before push: abort if commit paths or history differ from the approved set.
- Before Compose edit: retain the old image and backup.
- After failed deployment: restore backup/image and health-check the rollback.
