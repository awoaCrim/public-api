# Implementation plan: LLM review reliability

## 1. Capability/readiness settings

- Add explicit structured-output mode/readiness fields and constants to `setting/operation_setting/llm_review_setting.go`.
- Preserve legacy strict fields and interpret an existing strict pass as strict mode.
- Centralize configured/policy/capability readiness and reset both strict and compatibility state after critical changes.
- Update controller config/status/test endpoints and request pointer semantics for explicit empty policy.
- Require configured policy plus a passing capability mode before enabling; make stale enabled/unready workers idle.

## 2. Reviewer client and parser

- Make request construction mode-aware for strict Schema, JSON object, and prompt-only JSON while preserving deterministic settings, tools disabled, SSRF, masking, and output limits.
- Implement ordered live capability probing with controlled fallback on unsupported/invalid structured output and no fallback for auth/network/rate/server failures.
- Add a normalization boundary for fenced JSON, one unambiguous object in surrounding prose, text-part arrays, BOM/whitespace, and ambiguity rejection.
- Keep strict validation semantics and add compatibility-mode validation that still enforces required fields, vocabulary, bounds, and evidence.

## 3. Worker, enqueue, and durable task contract

- Use the centralized readiness predicate in enqueue, worker claim, and in-flight processing.
- Add `output_mode` to `LLMReviewTask`, persist it in completion, and expose it in controller/frontend detail responses.
- Keep valid compatibility verdicts auditable/manual-review-only; make `ShouldAutoBan` require effective strict mode in addition to existing gates.
- Keep missing-policy legacy completion fail-closed but make its reason/evidence actionable; add an unavailable skip reason for newly triggered work when readiness is absent.

## 4. Frontend settings and translations

- Extend `web/src/features/llm-review/types.ts` and API response types.
- Update `web/src/features/system-settings/security/llm-review-section.tsx` to show structured-output status, strict versus compatibility mode, auto-ban safety, and missing-policy warning.
- Load `i18n-translate` (already read), add all new source keys through `web/scripts/add-missing-keys.mjs`, run `bun run i18n:sync`, and remove the temporary script.

## 5. Verification

- Add/adjust `service/llm_review_client_test.go`, `service/llm_review_payload_test.go`, `service/llm_review_policy_test.go`, `service/llm_review_worker_test.go`, `controller/llm_review_test.go`, model task tests, and frontend tests where each contract is owned.
- Run `gofmt` and focused `go test` for service/controller/model.
- From `web/`, run the relevant Bun test/typecheck/lint/build/i18n commands from `package.json`.
- Run full applicable repository checks, inspect `git diff --check`, and verify no raw secrets/provider bodies are logged or returned.

## Review gates

- Capability test must prove the selected mode before enablement.
- A fallback result must never pass the strict auto-ban mode check.
- A missing policy must be visible and cannot be silently replaced by an invented/default source.
- Existing strict configurations and old task rows must remain readable and behaviorally compatible.
