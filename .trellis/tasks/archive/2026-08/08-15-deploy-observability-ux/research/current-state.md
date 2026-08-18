# Current Publication and Production State

Recorded: 2026-08-15 +08:00

## Local repository

- Branch: `rebuild/customizations-20260812`.
- Remotes:
  - `origin=https://github.com/awoaCrim/public-api.git`
  - `upstream=https://github.com/QuantumNous/new-api.git`
- `origin` has no `rebuild/customizations-20260812` branch yet.
- Latest commits:
  - `1dc60d0c docs: record customization migration and deployment`
  - `da678d51 feat: rebuild customizations on latest upstream`
- The intended observability delta consists of Root-only direct request snapshot retrieval, dedicated Request Snapshots settings navigation, restored Usage Analysis presentation, focused tests, seven-locale translations, and matching docs/spec updates.
- Untracked Trellis runtime/task/archive files, `.pi/`, and `nul` coexist in the working tree and must remain outside product commits and deployment archives.

## Production server

Read-only preflight over SSH alias `ssh2`:

- Host: `VM-0-13-debian`, `x86_64`.
- Deployment root: `/opt/newapi`.
- Docker Compose: v2.27.1.
- SQLite CLI: `/usr/bin/sqlite3`.
- Container: `newapi`.
- Current image: `newapi-custom:20260814-remediated-da678d51`.
- Container state: running; restart count 0.
- Local status check: HTTP 200 from `http://127.0.0.1:3000/api/status`.
- Compose references the same current image.
- Database: `/opt/newapi/data/one-api.db`, approximately 193 MB.
- Current source tree: `/opt/newapi/src`, approximately 23 MB.
- Available disk: approximately 38 GB.
- Existing backups include `/opt/newapi/backups/deploy-20260814-204512-da678d51` and `/opt/newapi/backups/20260814-212917-remove-routing-group-3`.

## Established deployment pattern

The prior release:

1. created a source archive from exact commit `da678d51`;
2. uploaded/extracted it under `/opt/newapi`;
3. built immutable image `newapi-custom:20260814-remediated-da678d51`;
4. created a SQLite online backup and required `PRAGMA integrity_check=ok`;
5. backed up Compose, `.env`, source, and prior image metadata;
6. switched the `newapi` Compose image;
7. confirmed container running, restart count 0, and `/api/status` HTTP 200.

This task will reuse that backup-first exact-commit pattern and will not change environment values or run routing-group migration.
