# Current-state research

## Frontend

- The shared groups query uses React Query key `['groups']`.
- Its current cache behavior permits stale group data to remain visible for approximately 5 minutes when a successful GroupRatio save does not invalidate the shared query.
- The intended fix boundary is the centralized `useUpdateOption` success path, with no generic rename API.

## ssh2 production data

- Current active GroupRatio values are `default`, `svip`, and `vip`.
- Confirmed active stale references to migrate from `test` to `svip`:
  - `channels`: channel ID 53;
  - `abilities`: two records;
  - `tokens`: token ID 32;
  - `user_group_grants`: grant ID 237.
- Historical data to retain unchanged:
  - `logs`: 189 records;
  - `perf_metrics`: 12 records;
  - `quota_data`: 13 records.

## Operational constraints

- The production database is SQLite for this migration path.
- Take a readable SQLite backup before any write.
- Perform the authorized active-reference updates in one transaction, validate exact affected rows, and roll back on any mismatch or failure.
- Historical logs and metrics are retained for audit/performance/quota analysis even if they contain legacy `test` references.
- Deployment must follow `.trellis/spec/backend/deployment-guidelines.md`: build from a committed ref, back up Compose/image state, recreate only the application service, verify `/api/status`, and restore the previous image/config on failure.
