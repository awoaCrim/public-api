# Design: Restore Usage Analysis Presentation

## Data boundary

`index.tsx` remains the owner of filter state, applied filters, page state, React Query calls, query parameters, errors, and derived trend data. The current `UsageAnalysisData.summary`, `rows`, `trend`, `total`, and `bucket_seconds` fields remain authoritative.

The old repository is a visual oracle only. Its client-side row summation and unpaginated assumptions must not be copied.

## Component structure

Keep query orchestration in `web/src/features/usage-analysis/index.tsx` and move stable presentation regions into feature-local components when needed to keep each responsibility readable:

- summary/hero and metric cards;
- trend chart;
- breakdown table plus pagination.

These components receive typed, already-normalized props and do not fetch data or duplicate API types.

## Visual adaptation

- Restore the old header description and filter alignment.
- Restore the token hero, selected-user/key context, request/cost block, shadows, icon colors, and compact secondary values.
- Derive average tokens as `summary.total_tokens / summary.request_count`, guarded for zero requests.
- Use current cache-read and cache-write fields in cards and chart.
- Keep legacy rows as a clear disclosure rather than letting them contaminate cache metrics.
- Restore chart gradient, stroke widths, compact Y-axis formatting, date-range caption, tooltip, and legend.
- Retain current table columns and pagination controls while applying the old card/table visual hierarchy.

## i18n

Reintroduced old labels that are absent from current locales must be added to all seven locale files only through `web/scripts/add-missing-keys.mjs`, followed by `bun run i18n:sync`. Existing translation keys should be reused where they already express the intended text.

## Tests

- Preserve API query and trend/filter helper tests.
- Add a focused component/presentation regression covering the visible hero, key metric labels, trend/breakdown landmarks, and pagination controls using explicit fixture data.
- Assert user-visible behavior and semantic roles/text, not full Tailwind class snapshots.
