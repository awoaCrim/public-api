# Restore Legacy Usage Analysis Presentation

## Goal

Restore the more polished previous Usage Analysis dashboard presentation while keeping the current safe, paginated Usage Analysis API and metric semantics.

## Requirements

- Restore the previous explanatory header and right-aligned filter layout.
- Restore the prominent Actual Consumed Tokens hero with selected user/API-key context.
- Restore the request/cost summary block and colored metric-card hierarchy.
- Restore the gradient trend chart, compact token formatting, date-range context, and previous breakdown-table styling.
- Use current metrics: input, output, cache read, cache write, cache rate, request average, quota, and legacy-row disclosure.
- Keep the current filters, reverse-date validation, explicit refresh behavior, loading/error/empty states, page size, pagination, and `keepPreviousData` behavior.
- Keep the current backend response contract and server-provided summary; do not aggregate only the current page to produce totals.
- Add/restore all required UI translations for every supported locale through the sanctioned i18n script workflow.

## Out of Scope

- Backend SQL/query changes, timeout changes, billing changes, or larger page-size limits.
- Restoring the old unpaginated data behavior or obsolete cache-token field names.
- Redesigning global dashboard components.

## Acceptance Criteria

- [x] The page presents the agreed previous visual hierarchy at desktop and responsive widths.
- [x] Summary values come from the current server summary, not the visible page rows.
- [x] Separate cache-read/cache-write semantics and legacy disclosure remain visible.
- [x] Filters, manual refresh, pagination, errors, loading, and empty states still work.
- [x] Focused presentation/helper/API tests, i18n sync, typecheck, lint/format, and production build pass.
