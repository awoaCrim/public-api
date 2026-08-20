# Implementation plan: commit and deploy structured request body viewer

- [x] Verify the GitHub OAuth registration-age status in the checked-in source and remote deployed source snapshot. The current application does not read a GitHub account creation timestamp or compare account age; this is distinct from the unrelated GitHub PR anti-spam workflow setting `min-account-age: 30`.
- [x] Re-run focused tests, typecheck, affected lint/format, build check, and diff check.
- [x] Create a commit using only the four approved request-snapshot paths; verify the commit file list exactly.
- [x] Publish `main` to `origin/main` through the authenticated Git Data API fallback after normal Git transport reset the connection; verify the remote ref, parent, tree, and exactly four changed paths.
- [x] Archive committed `HEAD`, transfer it to `ssh2:/opt/newapi/staging/`, extract it, and build the unique `newapi-custom:v1.0.0-custom.10-b7123a90` image.
- [x] Back up Compose and deployment markers, replace only the application image, recreate the app container, and run the JSON/HTTP health check.
- [x] Verify container status, `/api/status`, the request-log route, and the served bundle; record rollback details.
- [x] Mark acceptance criteria complete, run final `git diff --check`, and archive this Trellis task without committing unrelated task/spec files.

Rollback points:

- Before product commit: do not stage unrelated changes.
- Before Compose update: retain the existing image and Compose backup.
- On failed health check: restore the Compose backup and previous image, then verify the old container is healthy.
