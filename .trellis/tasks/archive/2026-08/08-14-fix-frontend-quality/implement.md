# Implementation Plan: Frontend Quality Fixes

1. Fix the listed curly-brace lint errors.
2. Replace the review evidence array-index key with a stable duplicate-safe key.
3. Convert the user-form type import.
4. Run targeted `oxlint` and `oxfmt --check`.
5. Run `bun run typecheck`, `bun run build`, and relevant focused tests.
6. Confirm no locale files or unrelated frontend files changed.

## Verification

- Targeted `oxlint` on the four F-010 files: pass.
- Targeted `oxfmt --check`: pass.
- `bun run typecheck`: pass.
- `bun run build`: pass.
- Focused LLM Review format and user-form tests: 8 pass, 0 fail.
- No locale files were edited by this task; `git diff --check` passes.
