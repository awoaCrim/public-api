# Implementation plan: Vision completion

1. Load backend/frontend guidance and inspect existing middleware, user-setting, profile API, auth-store, and component-test fixtures.
2. Add failing controller/service tests for `0..64`, invalid no-mutation behavior, legacy safe normalization, and zero-threshold no-pHash grouping.
3. Implement reusable threshold validation/normalization and update controller/runtime grouping behavior.
4. Replace implementation-detail middleware tests with a real Responses success-path test and model/body/marker assertions; retain explicit fail-open coverage.
5. Add component tests for default prompt, blank save normalization, threshold payload, API failure, auth-store update, and profile refresh.
6. Correct frontend copyright headers and any accessibility/i18n issues found by affected checks.
7. Run gofmt, focused controller/middleware/service tests, frontend tests, affected lint/format, typecheck, copyright check, build check, and `git diff --check`.
8. Dispatch an independent check and fix any in-scope findings.
9. Leave the feature uncommitted until the GitHub OAuth sibling also passes; record the exact final path manifest for the publish child.
