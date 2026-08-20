# Design: focused commits and committed-tree deployment

## Commit construction

Use explicit manifest staging. Before each commit, compare `git diff --cached --name-only` with the approved sorted manifest. After each commit, inspect `git show --name-only` and ensure unrelated dirty files remain unstaged.

## Push

Push `main` only to `origin/main` and verify the remote SHA. If Git HTTPS transport fails, use authenticated GitHub MCP/Git Data operations based on the actual remote head and verify parent/ref/path comparison; never call the GitHub API unauthenticated.

## Deployment

Create the source archive from the verified pushed SHA. Build a unique remote image before editing Compose. Back up the full Compose file, previous tag, and image ID. Replace only the `newapi` service image, recreate it, and perform a bounded direct HTTP/JSON health loop. Verify source/frontend markers for `GitHubOAuthMinimumAgeYears`, the localized GitHub age field, the Vision default prompt/threshold behavior, and Responses support.

## Rollback

Any post-backup failure restores the saved Compose file and previous image, recreates `newapi`, and requires the previous `/api/status` health check to pass. Keep both the backup and prior image after success.
