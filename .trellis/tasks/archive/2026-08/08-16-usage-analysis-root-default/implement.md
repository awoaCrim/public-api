# Implementation plan: usage-analysis Root default

1. Inspect current controller tests and add a minimal options fixture covering enabled Root and non-root users; assert `root_user_id` is returned and the existing option lists remain intact.
2. Update `controller/usage_analysis.go` to resolve the canonical enabled Root role within the request timeout and return the metadata without changing route authorization or aggregation filters.
3. Extend `UsageAnalysisOptions` typing and add a pure initial-selection helper if it keeps the state transition easy to test.
4. In `UsageAnalysis`, fetch options before enabling the analysis query; initialize editable/applied filters once to the returned Root ID, or to `all` plus a visible fallback notice when no root is available.
5. Add regression tests proving the first completed query contains `user_id=<root id>`, no options failure emits an all-user query, and manual selection of another user/All Users remains functional.
6. Run `gofmt` on changed Go files, focused controller tests, frontend usage-analysis tests, frontend typecheck/lint/build, and `git diff --check`.

Do not change usage aggregation formulas, root-only middleware, or unrelated UI filters.
