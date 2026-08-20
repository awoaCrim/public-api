# Design: finish, publish, and deploy the authorized product changes

## Scope and ownership

This parent task coordinates three independently verifiable deliverables:

1. complete the GitHub OAuth registration-age feature;
2. complete the Vision interception feature;
3. commit both finished feature sets, push `origin/main`, and deploy the exact pushed tree.

The same worktree contains unrelated Trellis/Pi/task metadata, so only one implementation writer operates at a time and every commit uses an explicit path manifest. The feature children may be implemented and checked independently, but no new commit is created until both children pass their checks.

## Change boundaries

### GitHub OAuth

The feature boundary runs from the persisted `GitHubOAuthMinimumAgeYears` option through GitHub `created_at` parsing, new-user policy enforcement, localized callback errors, and the system-settings form. Existing numeric and legacy-ID login plus authenticated bind remain ahead of or outside the registration gate.

The completion work adds behavior-level tests for provider metadata, startup/settings API state, legacy migration, callback localization, and the actual React form. Frontend integer/range errors use explicit translated messages in all seven locales; locale writes go through the required script and `bun run i18n:sync`.

### Vision

The trusted threshold boundary is `0..64`. New invalid settings are rejected before persistence. Runtime use of malformed legacy values is normalized to the safe value `0`, preventing broad accidental clustering.

At threshold `0`, `clusterImages` returns one group per image without downloading, decoding, or hashing for pHash. Existing exact-URL/request/LRU caching remains a cache concern rather than perceptual clustering. Thresholds `1..64` retain the existing pHash grouping and cache behavior.

Responses API coverage must exercise the real middleware success path: extract `input_image`, analyze/replace it as `input_text`, rewrite the reusable request body/context marker, and preserve the original client model. The profile card tests cover default prompt display, blank-save normalization, valid threshold submission, API failure, and auth-store update. New frontend files must satisfy the repository copyright header check.

## Validation and commit boundary

Each feature receives focused tests and independent review first. Then run the combined Go/frontend build gates against the complete dirty worktree. Only after both groups pass are two focused commits created:

1. GitHub OAuth feature manifest;
2. Vision feature manifest.

Staged-path and commit-path audits must prove no Trellis/Pi/task/workspace paths entered either commit. The existing five local commits remain unchanged and are pushed together with the two new commits.

## Deployment data flow

1. Push `main` only to `origin` and verify the remote ref.
2. Create `git archive --format=tar.gz <pushed-sha>` locally so the artifact cannot include remaining dirty metadata.
3. Transfer the archive to `ssh2:/opt/newapi/staging/` and extract it into a unique source snapshot.
4. Build a unique `newapi-custom:<version>-<short-sha>` image before changing Compose.
5. Save a timestamped backup containing the full Compose file, previous application image tag, and running image ID.
6. Change only the `newapi` service image reference and recreate only that service.
7. Require bounded HTTP 200 plus JSON `success=true` from `/api/status`, inspect bounded recent logs, and verify deployed assets/source markers for both features.

## Rollback

Build failure leaves Compose untouched. Any Compose, startup, or health failure restores the saved Compose file/image reference and recreates `newapi`; rollback is complete only after the previous image again passes `/api/status`. The previous image is retained until final verification succeeds.

## Compatibility and safety

- No schema or destructive data migration is introduced.
- SQLite, MySQL, and PostgreSQL option behavior remains through GORM.
- Existing GitHub login/bind and Vision fail-open relay behavior remain intact.
- Credentials, `.env`, DSNs, secrets, and full upstream bodies are never printed.
- No release tag is created and `upstream` is never pushed.
