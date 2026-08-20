# Implementation plan: GitHub OAuth completion

1. Load backend, frontend, React, and i18n guidance; inspect existing option/controller/form test fixtures.
2. Add failing tests for `GetUserInfo` metadata projection, legacy-ID migration bypass, startup option load/default/fallback, option GET/invalid PUT, localized callback errors, and the real form.
3. Add the smallest provider test seam and complete backend behavior without changing login/bind ordering.
4. Add translated integer/range validation messages through the required locale script and synchronize locales.
5. Make the real form tests pass for valid values, invalid no-request behavior, and failed-save state.
6. Run gofmt, focused Go tests, frontend tests, affected lint/format, i18n sync/report, typecheck, copyright check, build check, and `git diff --check`.
7. Dispatch an independent check and fix any in-scope findings.
8. Leave the feature uncommitted until the Vision sibling also passes; record the exact final path manifest for the publish child.
