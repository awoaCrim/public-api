# Harden LLM review structured output and terms retrieval

## Goal

Make LLM review reliable for models with different structured-output capabilities, while keeping validation, policy enforcement, auditability, and automatic-ban safety fail-closed.

## Confirmed Current Behavior

- `service/llm_review_client.go` always requests a strict `json_schema` response format with `additionalProperties: false` and a fixed required verdict schema.
- The capability test uses that same strict request and stores `SchemaTested`, but the worker currently gates only on the enabled flag. Production evidence shows an enabled configuration with `schema_tested=false` and an upstream HTTP 400.
- Raw review parsing accepts a JSON string or text-part content, while strict validation rejects code fences, extra prose, invalid values, out-of-range confidence, and empty evidence.
- Automatic bans require both a schema-passed result and a tested schema capability.
- The current policy source is the administrator-managed `Policy Text` setting. When it is empty, the worker returns an uncertain/manual-review result with `未取得条款文本`; no alternate terms source was found in the repository.

## Requirements

- Define and implement an explicit compatibility behavior for providers that reject strict `json_schema`; the service must not repeatedly retry an identical unsupported request.
- Make the configured capability/mode visible and make configuration persistence, capability testing, and worker gating agree. A stale `Enabled=true` with no currently permitted capability must not be treated as a usable review service.
- Normalize only unambiguous supported response forms (including the content representation chosen by the compatibility design), then run the existing semantic validation and bounds checks.
- Treat malformed, ambiguous, incomplete, or unsupported output as an auditable uncertain/manual-review outcome. Never infer a violation or trigger an automatic ban from a parse/validation failure.
- Keep automatic bans restricted to explicitly trusted, fully validated results and preserve the existing manual override and fail-closed behavior.
- Make policy/terms availability explicit to administrators and workers. If no approved policy source is available, do not fabricate, silently substitute, or falsely report retrieved terms; retain a clear setup error and manual-review outcome.
- Preserve existing task records and retry semantics where safe, provide actionable failure diagnostics, and maintain compatibility with existing settings and supported databases.
- Add regression tests for strict-capable models, non-strict models, malformed/fenced/part-array responses, stale enabled configuration, missing policy, and the automatic-ban safety boundary.

## Acceptance Criteria

- [ ] A model that rejects the strict structured-output request follows the approved compatibility behavior and reaches a deterministic usable or manual-review outcome without an endless identical retry loop.
- [ ] Capability status, persisted configuration, and worker behavior agree; an untested or unsupported configuration cannot silently process as if strict schema capability had passed.
- [ ] Valid supported response representations are extracted and semantically validated; invalid or ambiguous output is auditable and cannot auto-ban.
- [ ] Auto-ban remains impossible when schema/mode trust requirements are not met, even if the model's text resembles a violation verdict.
- [ ] Missing policy text is surfaced as a concrete setup condition and remains fail-closed/manual-review; no evidence claims terms were obtained when they were not.
- [ ] Existing manual review, retry, and override behavior remains intact, with focused backend tests covering the contracts above.
- [ ] Relevant Go tests and the applicable build/lint/type checks pass.

## Confirmed Product Decision

- Non-strict models may be enabled through an explicit compatibility path. The capability test may select a supported JSON mode, but compatibility-mode results are manual-review-only and can never trigger automatic banning.
- Policy text remains an explicit administrator-provided prerequisite. There is no approved alternate terms source in the repository; missing policy must be surfaced clearly and handled fail-closed rather than fabricated or silently fetched.

## Out of Scope

- Adding a new legal/compliance policy without approved source text.
- Provider-specific arbitrary adapters unrelated to structured output or policy availability.
- Relaxing the semantic verdict validation or automatic-ban safeguards to increase pass rates.

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- This child is expected to need `design.md` and `implement.md` before activation.
