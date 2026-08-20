# Design: LLM review compatibility and policy readiness

## Behavior gap and boundary

The request builder hard-codes strict JSON Schema, the capability test only proves that one shape, and the worker checks only the master switch. The implementation must place compatibility at the reviewer-client request/parse boundary and readiness at the setting/enqueue/worker boundary; it must not weaken the final auto-ban predicate.

## Setting model and readiness

Add a persisted output-mode/readiness model to `operation_setting.LLMReviewSetting` while preserving the meaning of existing fields:

- `StructuredOutputMode`: `strict_schema`, `json_object`, or `prompt_json`.
- `StructuredOutputTested`: whether the selected mode passed a live capability test.
- `StructuredOutputTestedAt`, `StructuredOutputTestedModel`, and `StructuredOutputVersion` for diagnostics.

A legacy `SchemaTested=true` row with no new metadata is interpreted as `strict_schema`. A compatibility pass sets `StructuredOutputTested=true` and leaves `SchemaTested=false`. Critical changes (base URL, model, key, private-address flag) clear both states. Add a helper that returns the effective mode and a helper that returns whether the service is ready.

Readiness requires configured endpoint/model, non-empty sanitized policy text, and either a strict or compatibility capability pass. The controller's enable validation, enqueue path, worker claim loop, and in-flight task guard must all use the same readiness helper. A stale enabled-but-untested configuration must not call the reviewer.

## Capability probe

Keep `/api/llm_review/test_schema` for compatibility, but make its implementation test structured-output capability in this order:

1. strict `json_schema`;
2. `json_object`;
3. prompt-only JSON.

Each probe uses the same deterministic sample and the same semantic verdict validation. Continue to the next mode only for a controlled unsupported/invalid structured-output result; surface authentication, network, endpoint, rate-limit, and server failures directly. Persist the first passing mode and return the selected mode plus strict/compatibility metadata. Do not persist a successful candidate until the entire test has passed.

The request builder chooses the mode from the tested configuration. Strict mode keeps the existing schema. JSON-object mode uses `response_format: {type: "json_object"}`. Prompt-only mode omits `response_format` and adds explicit key/type requirements. All modes keep tools disabled, deterministic sampling, bounded output tokens, and the current policy-only system instructions.

## Parsing and semantic validation

Separate provider-envelope extraction from content normalization:

- extract string or text-part-array content;
- trim BOM/whitespace;
- accept one fenced JSON object or one unambiguous object embedded in harmless prose;
- reject multiple/ambiguous objects and unsupported content;
- validate required verdict/category/confidence/reason/evidence fields and bounds.

Strict mode uses the strict validation contract. Compatibility modes can tolerate non-semantic additional properties but cannot omit required fields or use unsupported values. Use `common.Unmarshal`/`common.Marshal`; do not introduce direct business calls to `encoding/json`.

The worker records the selected mode on each task. A semantically valid compatibility verdict may be stored and shown to an administrator, but `ShouldAutoBan` must require both strict capability and effective `strict_schema` mode. Any parse/validation failure remains manual-review/failure and cannot be converted into a violation.

## Policy behavior

`Policy Text` is the only approved source. Add `policy_configured` to configuration/status responses and update the controller request to distinguish omitted policy from an explicit empty value. Require non-empty sanitized policy when enabling; reject clearing it while enabled. New triggers with an enabled but not-ready service become an auditable unavailable/skipped task, and the worker does not claim queued work while not ready. Preserve the existing direct no-policy uncertain completion for legacy tasks, but make its evidence/reason actionable.

No remote terms fetch or built-in policy is introduced.

## API and UI contract

Extend existing config/status responses without removing old fields:

- retain `schema_tested` and `supports_strict_json_schema` as strict-only indicators;
- add selected output mode, structured-output tested/usable state, and policy configured state;
- add task `output_mode` to detailed task responses.

The settings UI replaces strict-only wording with structured-output wording, shows strict versus compatibility status, explicitly states that compatibility mode cannot auto-ban, and warns when policy text is missing. All new UI strings must be added to `en`, `zh`, `zh-TW`, `fr`, `ja`, `ru`, and `vi` using `web/scripts/add-missing-keys.mjs`, followed by `bun run i18n:sync`.

## Database and compatibility

New task mode is an ordinary GORM model field included in the existing `AutoMigrate` list. New setting fields are option-map keys and need no table migration. Existing tasks/configurations remain readable. No dialect-specific SQL or destructive migration is allowed.

## Verification boundary

Tests must cover request shape per mode, capability fallback order and non-fallback errors, content normalization/ambiguity, readiness gates, missing policy, stale enabled configuration, task mode persistence, and the strict-only auto-ban boundary. Run focused Go tests, frontend typecheck/build/lint/i18n checks, and the full applicable repository checks before activation/finish.
