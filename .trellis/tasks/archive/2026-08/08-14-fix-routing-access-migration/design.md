# Design: Routing Access and Migration Fixes

## Authorization contract

`service.ResolveGroupSelection` remains the single resolver. Its requested-group branch becomes token-mode aware:

- fixed token: requested group empty or equal to fixed group only;
- auto token: requested group may be any non-`auto` group in effective access;
- all invalid selections preserve sentinel errors so middleware can return the established 403 contract.

`controller.setTokenAutoGroups` resolves effective access by user ID and validates every submitted group against that set plus the existing ratio/catalog requirement. Listing and mutation therefore share the same source of truth.

## Migration contract

Change `scanLegacyRoutingGrants` to return `error`. `migrationScan` propagates it. Preview, readiness, regular migration, and strict migration already propagate `migrationScan` errors and therefore fail closed. The regression test must inject a query failure and assert no grants/tokens/options are written.

For orphaned legacy grant references, the scan records a dedicated unmappable-grant preview containing the user ID and missing routing-group ID. Only grants that are still effective at scan time block readiness; expired grants are non-authoritative and are ignored. Strict migration checks this blocker before any writes or migration marker update.

Grant preview/status is target-aware. After merging duplicate legacy rows by `(user_id, group_key)`, compare the incoming expiry/source effect with the existing `user_group_grants` row using the same merge semantics as `applyLegacyRoutingGrant`. Include an item in `GrantImports` only when the migration would create the target row or materially update its source/expiry. This keeps dry-run status idempotent and makes `PendingGrants` describe real remaining work.

## Pagination contract

Validate the eventual offset `(page-1)*pageSize` without computing `maxInt+1`. Compare `page-1` with `maxInt/pageSize` after page and size normalization.

## Compatibility

Use existing GORM operations and sentinels; no schema change. The target comparison uses GORM queries and existing source/expiry semantics, preserving SQLite/MySQL/PostgreSQL compatibility. Tests run on SQLite and inspect behavior that is dialect-independent.
