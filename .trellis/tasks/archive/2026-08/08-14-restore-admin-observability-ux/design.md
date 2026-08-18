# Design: Restore Admin Observability UX

## Boundary

This parent task coordinates three independently verifiable changes. It does not own direct production edits; each child owns one behavior boundary.

## Cross-child contracts

- Request snapshot capture, encryption, storage, cleanup, and audit persistence remain unchanged.
- Request snapshot retrieval remains an on-demand endpoint and never joins usage-log list payloads.
- Root-only authorization is enforced on both the backend route and frontend control; frontend hiding is convenience, not the security boundary.
- Usage Analysis continues consuming the current paginated API response and does not reintroduce client-side aggregation of incomplete pages.
- Request Snapshot settings have one form/state owner and one canonical route even after gaining a dedicated sidebar entry.

## Integration sequence

1. Change request snapshot authorization and direct-view interaction.
2. Split Request Snapshot settings into a dedicated Operations section.
3. Restore the Usage Analysis presentation and perform the final i18n synchronization.
4. Run focused child checks, then full frontend typecheck/build and affected Go package tests.

## Explicit non-goals

- No changes to request capture policy or stored snapshot format.
- No changes to F-002 LLM Review proof/payload behavior.
- No changes to Usage Analysis SQL, billing semantics, time-range bounds, timeout, or pagination API.
- No duplicate Request Snapshot settings form or shortcut on Usage Logs.
