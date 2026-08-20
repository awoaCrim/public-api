# Structure request body in viewer

## Goal

When a Root administrator views a captured request body from usage-log details, display valid JSON in a readable structured form instead of one-line/minified text, without changing the captured bytes or download behavior.

## Confirmed current behavior

- The target component is `web/src/features/usage-logs/components/dialogs/request-snapshot-section.tsx`.
- `web/src/features/usage-logs/lib/request-snapshot.ts:snapshotBytesToText` currently decodes bytes to UTF-8 text and the component renders that text directly in `<pre>`.
- Copy currently uses the displayed text, while download uses the original decoded bytes.
- Existing request-snapshot tests cover exact text rendering, arbitrary binary byte preservation, access gating, stale responses, and backend error handling.

## Requirements

- For captured bodies whose decoded content is valid JSON, format the displayed content with stable two-space indentation.
- Keep copy behavior compatible with the existing request-body copy action; copied content should remain the original decoded text rather than silently changing the request payload.
- Keep download behavior byte-exact and unchanged.
- For invalid JSON, plain text, empty content, or binary content, fall back to the existing decoded text display without throwing or hiding the body.
- Do not change request snapshot access control, backend storage, audit behavior, request identity checks, or error handling.
- Keep the formatting logic in the request-snapshot utility boundary so the component does not own JSON parsing rules.

## Acceptance criteria

- [x] A valid JSON request body is displayed with readable indentation and line breaks.
- [x] Copy returns the original decoded request body, not the formatted display projection.
- [x] Download still uses the exact captured bytes and content type.
- [x] Invalid JSON and non-JSON content still render using the current fallback behavior.
- [x] Existing stale-response, access-control, error, and state-clearing behavior remains unchanged.
- [x] Focused request-snapshot utility/component tests, frontend typecheck, affected lint/format checks, build check, and `git diff --check` pass.

## Change boundary

Expected files:

- `web/src/features/usage-logs/lib/request-snapshot.ts`: add the pure display-formatting projection beside byte decoding.
- `web/src/features/usage-logs/lib/__tests__/request-snapshot.test.ts`: cover valid JSON formatting and fallback cases.
- `web/src/features/usage-logs/components/dialogs/request-snapshot-section.tsx`: use the formatted projection only for rendered content while preserving raw copy/download data.
- `web/src/features/usage-logs/components/__tests__/request-snapshot-section.test.tsx`: verify the visible formatted output and existing user actions.

No new user-facing copy or locale keys are required.

## Out of scope

- Reformatting content on the backend or rewriting stored snapshots.
- Changing the copy action to copy formatted JSON.
- Adding an interactive JSON tree/editor, syntax highlighting, field folding, or search.
- Changing request-body permissions or security-proof behavior.
