# Fix Frontend Quality Findings

## Goal

Resolve F-010 so the affected new/modified frontend files pass targeted lint without changing UI behavior.

## Requirements

- Add required braces in LLM Review formatting and column render branches.
- Replace unstable array-index keys in the review detail evidence list with a stable data-derived key that tolerates duplicate text.
- Convert the affected user-form import to a true type-only import.
- Do not suppress lint rules or refactor unrelated frontend files.

## Out of Scope

- Existing full-repository lint/format/copyright baseline failures outside F-010.
- UI redesign, translation changes, or F-002.

## Acceptance Criteria

- [x] Targeted `oxlint` passes for all F-010 files.
- [x] Targeted `oxfmt --check` passes.
- [x] Frontend typecheck and production build pass.
- [x] Relevant LLM Review/user-form tests remain green or runner failures are classified as existing environment limitations.
