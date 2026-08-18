# Implementation Plan: Routing Access and Migration Fixes

1. Add regression tests for fixed-token request override semantics in the resolver and Playground/distributor boundary.
2. Update `ResolveGroupSelection` to reject cross-group overrides for fixed tokens while preserving Auto behavior.
3. Add controller tests for extra-grant Auto candidate saves and revoked/expired rejection.
4. Update `setTokenAutoGroups` to validate against effective user access.
5. Add a migration query-failure test that asserts error propagation, zero writes, and no version marker.
6. Return and propagate the legacy grant scan error.
7. Add Usage Analysis tests for `page_size=1` and exact overflow boundaries; replace overflow-prone arithmetic.
8. Run focused tests:
   - `go test ./service ./controller ./middleware ./model -count=1`
   - relevant narrower `-run` selectors during iteration.
9. Run `gofmt` on changed Go files and `git diff --check`.
10. Add regression tests for an active orphan legacy grant and an expired orphan grant.
11. Report active orphan grants as readiness blockers and include them in migration status without changing the legacy tables.
12. Add target-aware grant preview tests for missing, equivalent, broader, and update-required target grants.
13. Reuse the grant merge/update semantics when deciding whether a grant is pending, then verify a successful rerun reports `PendingGrants=0` and `InSync=true`.
14. Re-run the focused migration tests, root build, and `git diff --check`.

## Verification

- Fixed/Auto selection, migration-query failure, strict migration, Auto candidate, and Usage Analysis focused tests: pass.
- `go test ./service -run 'Test(ResolveGroupSelectionFixedAutoRequestedAndRevoked|RoutingGroupMigrationGrantReadFailureAbortsWithoutWritesOrMarker|StrictMigration|RoutingGroupMigrationReadiness)' -count=1`: pass.
- `go test ./controller -run 'Test(AddTokenAcceptsActiveExtraGrantAsAutoGroupCandidate|AddTokenRejectsExpiredExtraGrantAsAutoGroupCandidate|AddTokenRejectsRevokedExtraGrantAsAutoGroupCandidate|GetTokenAutoGroupsIncludesActiveExtraGrant|ParseUsageAnalysisQuery)' -count=1`: pass.
- `go test ./middleware -run '^TestDistributeRejectsPlaygroundGroupOverrideForFixedToken$' -count=1`: pass.
- `gofmt` and `git diff --check`: pass.
- Broader package failures remain classified as unrelated baselines in the parent task; this child did not broaden into those failures.
- Added active/expired orphan-grant regression coverage: active references block strict migration with zero writes/no marker; expired references do not block.
- Grant preview now batches target-grant reads and reports only rows that would be created or materially updated using the same source/expiry merge semantics as the write path.
- Added missing/equivalent/broader/update-required target-state coverage; after migration, `PendingGrants=0` and `InSync=true`.
- `go test ./service -run 'Test(MigrateRoutingGroupCompatibilityData|PreviewRoutingGroupMigration|StrictMigration|RoutingGroupMigration)' -count=1`: pass.
- `go test ./controller -run 'RoutingGroupMigration' -count=1`: pass.
- `go build ./...`, RelayKit independent build, `gofmt`, and `git diff --check`: pass.
- Full `go test ./service -count=1` still fails only the previously classified channel-affinity shared-global-state tests; all routing migration tests pass within that run.

## Rollback points

The resolver, controller validation, migration error propagation, and pagination fixes are independent commits/diffs and can be reverted separately before final integration if a regression appears.
