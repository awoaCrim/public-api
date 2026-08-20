# Deployment guidelines

This repository's self-hosted Docker deployment uses a committed source tree, a remote-built image, and an explicit health-check/rollback boundary. The deployment target may vary; the contract below is independent of a specific host alias.

## 1. Scope / Trigger

Apply these rules when deploying the Go application and embedded React frontend to a remote Docker Compose host, especially when the host does not build directly from the developer's working tree.

## 2. Signatures

- Source artifact: `git archive --format=tar.gz <committed-ref>`.
- Compose application service: the service whose `image:` is `newapi-custom:<tag>`.
- Health endpoint: `GET /api/status`.
- Required remote paths: a staging directory for the archive, a versioned source snapshot, a deployment backup directory, and the active Compose file.

## 3. Contracts

- The archive must be created from a committed ref and must not include unrelated uncommitted working-tree changes.
- The remote build must use a unique image tag derived from the deployed commit/version; do not overwrite the previous image tag.
- Before changing Compose, save the complete Compose file, the current image marker, and the running image identifier.
- Update only the application service's `image:` value. Preserve database/Redis environment wiring, bind mounts, ports, networks, and dependent services.
- A deployment is successful only when the recreated container is running and `/api/status` returns HTTP 200 with JSON `success=true`.
- A failed build, startup, or health check must restore the saved Compose file and recreate the previous image before reporting failure.
- Credential files, environment values, database contents, and full upstream error bodies must not be printed in deployment output.
- When the local branch contains local-only task/journal commits and normal Git transport is unavailable, an authenticated Git Data API fallback must base the published commit on the actual remote `heads/main` tree. Upload only the authorized changed blobs, create the remote commit with the current remote ref as its parent, update the ref, and verify the remote comparison contains exactly the approved paths. Do not assume the local commit SHA is already reachable on GitHub.

## 4. Validation & Error Matrix

| Condition | Expected result |
| --- | --- |
| Unrelated working-tree paths are staged | Abort before commit/push |
| Remote build fails | Leave the active Compose configuration unchanged |
| Compose update fails | Restore the saved Compose file and recreate the previous service |
| New container starts but health endpoint is not ready | Wait with a bounded timeout, then roll back |
| Health endpoint returns non-200 or `success != true` | Roll back and report the image/backup identifiers |
| Health succeeds and the expected frontend asset is served | Keep the new image and record its tag/backup |
| Git transport is unavailable but an authenticated API fallback is used | Read the actual remote `heads/main` ref, create a tree from that remote base plus only authorized blobs, verify the updated ref/parent, and compare changed paths explicitly; do not claim a normal Git push without that verification |

## 5. Good / Base / Bad Cases

- **Good:** archive the committed tree, transfer it to remote staging, build `newapi-custom:<new-tag>`, back up Compose, replace one image line, recreate only the application service, verify `/api/status`, and retain the rollback backup.
- **Base:** the remote host uses a blank `SQL_DSN` intentionally for its SQLite deployment; preserve that existing Compose behavior rather than inventing a database migration during deployment.
- **Bad:** build from the dirty working directory, stage unrelated files, replace the Compose file wholesale, delete the old image before health verification, print `.env`/DSN values, or declare success from a container `Running` status alone.

## 6. Tests Required

- Pre-push path audit: `git diff --cached --name-only` contains only the authorized files.
- Source/build validation: focused frontend tests, typecheck, lint/format checks, `bun run build:check`, and `git diff --check`.
- Remote validation: `docker compose ps`, application image inspection, HTTP status/JSON assertion for `/api/status`, bounded recent-log inspection, and a served-asset assertion for the changed frontend route/label.
- Rollback validation: if deployment fails, assert the old image is restored and `/api/status` is healthy before ending the operation.

## 7. Wrong vs Correct

### Wrong

```sh
# Builds and deploys every local change, destroys the old reference, and has no health gate.
docker build -t newapi-custom:latest .
sed -i 's#image:.*#image: newapi-custom:latest#' docker-compose.yml
docker compose up -d --force-recreate
```

### Correct

```sh
# Build from a committed archive, back up first, replace only the app image,
# verify /api/status, and restore the backup on failure.
git archive --format=tar.gz HEAD > deploy.tar.gz
cp docker-compose.yml backups/deploy-<tag>/docker-compose.yml
# transfer/archive/extract/build with a unique tag
# update only the newapi image line
# recreate newapi, then require HTTP 200 and {"success":true}
# restore the saved compose/image on any failed gate
```
