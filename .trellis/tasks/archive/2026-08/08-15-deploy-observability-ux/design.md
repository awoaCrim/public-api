# Design: Publish and Deploy Observability UX

## Boundaries

This task publishes already completed code. It must not broaden product behavior. The main boundaries are:

1. **Source boundary** — deployment input is an archive generated from an exact Git product commit, never the dirty working directory.
2. **Commit boundary** — product code/tests/i18n are committed separately from post-deployment documentation. Trellis runtime/task files and unrelated untracked files remain excluded.
3. **Remote boundary** — push only the current branch to `origin`; never push to `upstream`.
4. **Production boundary** — modify only the `/opt/newapi` release source/image and Compose image reference required for this cutover. Keep `.env`, mounted data, Redis, secrets, and unrelated services unchanged.

## Publication Flow

1. Re-run the task-scoped verification matrix against the current working tree.
2. Review all dirty paths and prepare the Trellis-required one-shot commit plan.
3. Commit the observability product code/tests/i18n as one coherent product commit.
4. Push the product commit to `origin/rebuild/customizations-20260812` and set branch tracking.
5. Generate a source archive from that exact commit. Because `git archive <commit>` reads the committed tree, untracked Trellis and local runtime assets cannot enter the image.
6. Deploy and verify production.
7. Update handoff/progress/report/spec documentation with actual deployment facts, commit it, and push the final branch state.

## Production Data Flow

```text
exact Git product commit
        |
        v
local git archive -> secure copy to ssh2 staging path
        |
        v
extract new source directory -> docker build new immutable tag
        |
        v
backup SQLite + compose + env + source + old image metadata
        |
        v
update compose image reference -> docker compose up -d newapi
        |
        v
container inspect + /api/status + root HTML + startup log checks
```

The existing mounted `/opt/newapi/data` directory and existing environment file remain attached to the replacement container.

## Image and Release Naming

Use an immutable image tag containing the date, purpose, and short product commit, for example:

`newapi-custom:20260815-observability-<shortsha>`

Use a matching timestamped backup directory:

`/opt/newapi/backups/deploy-<YYYYMMDD-HHMMSS>-<shortsha>`

The final values must be recorded after deployment rather than guessed in advance.

## Backup Contract

Before Compose is modified:

- run SQLite online `.backup` into the new backup directory;
- run `PRAGMA integrity_check` against the backup and require the result `ok`;
- copy `docker-compose.yml` and `.env` without displaying secret values;
- preserve the current `/opt/newapi/src` tree or archive;
- record current container image, image ID, inspect output, and current health/status;
- retain the prior Docker image locally for immediate rollback.

A failed backup or failed integrity check blocks cutover.

## Cutover and Rollback

Build the new image completely before changing Compose. The production service is a single `newapi` container, so `docker compose up -d newapi` may cause a brief interruption; prebuilding minimizes it.

After cutover, poll the local status endpoint for a bounded interval. Success requires:

- container state `running`;
- restart count 0;
- `/api/status` HTTP 200;
- root page responds successfully;
- no fatal startup or migration errors in the new container logs.

If any condition fails:

1. restore the backed-up Compose file or prior image reference;
2. run `docker compose up -d newapi`;
3. verify the previous image is running and `/api/status` is 200;
4. retain failed-build/startup logs for diagnosis;
5. report deployment failure without claiming completion.

## Compatibility and Operational Constraints

- The observability delta does not introduce a new schema migration; normal startup migration remains enabled but no unrelated migration command is run.
- Existing SQLite, Redis, secrets, ports, volumes, restart policy, and environment values remain unchanged.
- Strict routing-group migration stays untouched.
- Request Snapshot capture remains default-off unless production already configured it otherwise; deployment does not change settings.
- No secret file content is emitted into logs or chat.

## Documentation Strategy

Current semantic documentation changes remain part of this delivery. After production verification, update the deployment status in `HANDOFF.md`, `docs/customization-migration-report.md`, and `progress.md` with the actual product commit, image tag, backup path, health result, and push result. Commit and push this documentation after deployment so the remote branch is the authoritative handoff state.
