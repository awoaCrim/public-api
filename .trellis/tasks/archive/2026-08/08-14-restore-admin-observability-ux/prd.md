# Restore Admin Observability UX

## Goal

Restore a simpler and more visible administrator experience for request-body inspection, usage analysis, and request-snapshot management without accidentally changing unrelated logging or billing behavior.

## Background

The user reported three problems after the customization deployment:

1. Viewing a saved request body currently requires verification, but the user wants the viewing flow simplified.
2. The rebuilt Usage Analysis page is visually less desirable than the previous implementation.
3. The backend request-body log management page is no longer visible.

## Confirmed Current-State Findings

- The request-body endpoint is already protected by an authenticated admin session, critical rate limiting, cache disabling, and the dedicated `request_snapshot.read` permission. Root has implicit permission.
- A second, per-view 2FA/passkey proof is enforced independently in both the frontend dialog flow and the backend controller. Successful and failed post-permission accesses are synchronously audited; successful content is not returned if audit storage is unavailable.
- The previous Usage Analysis page used a stronger dashboard hierarchy: explanatory header, right-aligned filters, a prominent “Actual Consumed Tokens” hero, request/cost summary, seven colored metric cards, gradient trend chart, compact token formatting, and a simpler breakdown table.
- The current Usage Analysis page retained safer bounded/paginated data behavior but flattened the presentation into a plain filter row, total-token card, six equal metrics (including legacy rows), simpler chart, and paginated table.
- Request Snapshot settings still exist and are rendered inside **System Settings → Operations → Log Maintenance**. The feature is therefore nested under a generic section rather than absent from the codebase, which explains why it can appear to have disappeared.

## Requirements Under Discussion

### Request-body viewing

- Only the Root super administrator may view saved request bodies.
- A Root user can open the request body directly without a per-view 2FA/passkey challenge and without separately assigning `request_snapshot.read`.
- Non-Root administrators and regular users must not see the control or retrieve the request body endpoint.
- Keep request bodies out of list responses and load them only on demand.
- Preserve synchronous access auditing, critical rate limiting, and no-cache response behavior.

### Usage Analysis UI

- Restore the complete previous visual hierarchy: explanatory header, right-aligned filter controls, prominent “Actual Consumed Tokens” hero, request/cost summary, colored metric cards, gradient trend chart, compact token formatting, and previous breakdown-table styling.
- Adapt the restored presentation to the current response model, including separate cache-read/cache-write metrics and legacy-row disclosure where relevant.
- Preserve the current bounded SQL aggregation, pagination, filters, timeout, and billing semantics.

### Request-body log management

- Add a dedicated **Request Body Logs** item under **System Settings → Operations** with its own section route.
- Reuse the existing request-snapshot settings component as the sole editor for capture, storage, capacity, retention, cleanup, and orphan-grace settings.
- Remove the component from the generic **Log Maintenance** section so the form is not duplicated.

## Child Deliverables

1. `08-14-request-snapshot-root-direct` — make request-body reads Root-only and remove the per-view security proof while retaining audit and transport safeguards.
2. `08-14-restore-usage-analysis-ui` — restore the previous Usage Analysis visual hierarchy on top of the current safe paginated API contract.
3. `08-14-expose-request-body-log-settings` — expose Request Snapshot management as its own Operations settings section.

## Constraints

- Request bodies may contain credentials, prompts, personal data, and other sensitive content.
- Do not change F-002 LLM Review snippet/payload behavior as part of this task.
- Preserve SQLite, MySQL, and PostgreSQL compatibility.
- Preserve existing usage-analysis query bounds and database-side aggregation.
- Add regression tests for visibility, authorization, and UI behavior changed by this task.

## Acceptance Criteria

- [x] The intended request-body viewing security boundary is explicitly decided and implemented.
- [x] Root administrators can open a saved request body with the agreed direct interaction flow.
- [x] Unauthorized users cannot retrieve request bodies.
- [x] Usage Analysis matches the agreed previous-page visual structure while retaining current safe data/query behavior.
- [x] Request-body log management is visible from the agreed administrator navigation location.
- [x] Frontend typecheck/build, affected-file lint/format, focused frontend tests, and focused backend authorization tests pass.

## Open Questions

- Decided: request-body viewing is Root-only and direct, with no per-view 2FA/passkey challenge; access auditing remains.
- Decided: restore the complete previous visual structure while retaining the current safe data/query behavior and pagination.
- Decided: expose request-body log management as an independent Operations settings menu item and route.
