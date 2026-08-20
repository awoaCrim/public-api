# Repository audit: usage analysis and LLM review

Date: 2026-08-16

## Usage analysis

- `web/src/features/usage-analysis/index.tsx` initializes `userId` to the `all` sentinel and starts the analysis query independently of the options query.
- `web/src/features/usage-analysis/api.ts` omits `user_id` for the all-user state.
- `controller/usage_analysis.go` lists enabled users but does not identify the canonical Root user in the options response.
- `model/usage_analysis.go:applyUsageAnalysisFilters` applies the user predicate only for `UserID > 0`; an omitted user filter aggregates all users.
- `/api/usage-analysis` and `/api/usage-analysis/options` are already under `RootAuth`, `CriticalRateLimit`, and `DisableCache` in `router/api-router.go`.
- The canonical root role is `common.RoleRootUser`; `model.GetRootUser` finds the first row with that role. Initial database creation uses username `root` and that role.

## LLM review

- `service/llm_review_client.go` always builds a strict `response_format.type=json_schema` request. `TestSchemaCapability` sends the same request shape.
- `service/llm_review_payload.go` validates a fixed verdict vocabulary, category vocabulary, confidence range, and non-empty evidence. `ParseRawLLMResponse` supports string content and text-part arrays but does not normalize fences or surrounding prose.
- `service/llm_review_worker.go:runLLMReviewWorkerPass` checks only `IsLLMReviewEnabled`; it does not require `SchemaTested` or another usable capability state before claiming tasks.
- `ShouldAutoBan` independently requires `cfg.SchemaTested` and a validated result, which is the safety boundary to preserve for compatibility mode.
- `service/llm_review_policy.go` has one policy source: the administrator-managed `llm_review_setting.policy_text`. It sanitizes and refreshes the current setting before calls. An empty policy is completed as uncertain with a missing-policy schema error.
- `controller/llm_review.go` currently refuses enablement without `SchemaTested`, does not persist an explicitly empty `policy_text`, and exposes only strict-schema status.
- Production inspection on `ssh2:/opt/newapi/data/one-api.db` found `enabled=true`, `schema_tested=false`, a recent upstream HTTP 400, and no `llm_review_setting.policy_text` option row. Review tasks include missing-policy evidence. No alternate terms/policy source was found in the repository.

## Design constraints

- The selected product policy is: permit a compatibility path for non-strict models, but compatibility results are manual-review-only and can never auto-ban; policy text remains an explicit administrator prerequisite.
- Keep the existing strict path and field names backward compatible. Add explicit mode/readiness metadata rather than overloading `schema_tested` to mean a non-strict mode.
- Any fallback parsing must be deterministic and fail closed on ambiguity. Do not infer a violation from parser errors or unvalidated text.
- Frontend setting/status changes introduce user-facing strings and must follow the project's i18n skill for all supported locales.
