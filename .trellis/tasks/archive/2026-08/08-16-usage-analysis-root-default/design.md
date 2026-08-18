# Design: default usage analysis to Root

## Behavior gap

The UI uses `userId: 'all'` before options load, and the backend interprets the omitted `user_id` as an all-user aggregate. The fix belongs at the options-to-filter boundary, not in the aggregation formula or authorization middleware.

## API contract

Extend `GET /api/usage-analysis/options` with `root_user_id`:

- return the ID of the first enabled user holding `common.RoleRootUser`;
- return `0`/`null` when no usable Root user is available;
- keep the existing user list and all existing filter option fields unchanged;
- perform the lookup within the existing request context and timeout;
- do not expose credentials or add a role field to every user option.

The canonical role is stable even if the Root username/display name changes. The route remains Root-only.

## Frontend state flow

1. Fetch options first.
2. Keep analysis query disabled while options are loading or while initial filter state is unresolved.
3. On the first successful options response, initialize both `filters` and `appliedFilters` with `String(root_user_id)` when positive; otherwise retain `all` and set a visible `rootUnavailable` state.
4. Enable analysis query only after step 3. Its first settled query therefore contains the root ID in the normal case.
5. After initialization, user-selected filters are authoritative. Options refetches do not reset them.
6. If the root ID is absent, render the existing all-user selection plus a short warning that Root could not be resolved; never render a Root label for an all-user result.

The initialization helper should be pure and testable. The query gating must be covered by a component-level or query-boundary test; a helper test alone is insufficient because the regression is a request-order/state issue.

## Expected files

- `controller/usage_analysis.go`: query and return `root_user_id`.
- `web/src/features/usage-analysis/api.ts`: add the response field.
- `web/src/features/usage-analysis/index.tsx`: initialization guard, query `enabled`, and fallback notice.
- `web/src/features/usage-analysis/lib/usage-analysis.ts`: pure root/default helper if useful.
- `controller/usage_analysis_test.go` (or the nearest existing controller test): options contract.
- Existing frontend usage-analysis tests: initial root scope and manual filter preservation.

## Failure behavior

- Options request failure: keep analysis disabled and show the existing error; do not issue an all-user request.
- Successful options without a root ID: show all users only as an explicit safe fallback with a warning.
- User changes to All Users or another user: query behavior remains unchanged.
