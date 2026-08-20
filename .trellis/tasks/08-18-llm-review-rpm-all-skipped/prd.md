# Diagnose why RPM reviews are always skipped

## Goal

Determine why RPM-triggered LLM review records are being created as `skipped` instead of reaching the reviewer, explain the observed production behavior, and agree on the smallest safe behavior change if the skip is not intended.

## Confirmed production evidence

- Environment: `ssh2`, `/opt/newapi`, SQLite database `/opt/newapi/data/one-api.db`.
- RPM limit events are reaching the review enqueue seam. Recent application logs contain repeated `RPM limit exceeded user=245 model=deepseek-v4-flash` entries.
- For user `245`, the database currently contains:
  - `111` tasks with `status=skipped`, `skip_reason=grace_period`;
  - `18` tasks with `status=skipped`, `skip_reason=review_unavailable`;
  - `3` completed `compliant` tasks.
- The active grace row for user `245` has `grace_start_at=1787041958` and `grace_end_at=1787059958`, which is `2026-08-18 16:32:38` through `2026-08-18 21:32:38` server local time. At `2026-08-18 20:14:48`, new RPM tasks were still inside that window.
- The current settings are enabled and contain a policy, endpoint, model, and tested strict capability. The observed recent skips are therefore not primarily caused by the RPM trigger not firing.
- `service/llm_review_enqueue.go` checks `model.CheckLLMReviewGrace(...)` before readiness and enqueue. When true, it intentionally records `SkipReasonGracePeriod` and returns without creating reviewer work.
- `model/llm_review_task_db.go` opens a grace window after the configured compliant count is reached. Production settings show `compliant_limit=3` and `immune_hours=5`, matching the observed five-hour grace window.
- RPM throttling and LLM review are separate outcomes: `middleware/model-rate-limit.go` triggers the review enqueue attempt and then calls `service.WriteRateLimitError(...)`; `service/rate_limit_error.go` responds with HTTP 429, `rate_limit_error`, and `rate_limit_exceeded`.
- Production logs for user `245` confirm the RPM path returns HTTP 429 immediately after `RPM limit exceeded`; the review task being skipped does not allow the original request to continue.
- The current production option row is `rate_limit_ban_setting.enabled=true` and `rate_limit_ban_setting.max_rpm=3`. `ModelRequestRateLimitDurationMinutes` has no persisted override and therefore uses the code default of 1 minute. Historical task rows with `limit_value=5` were created before the current limit was changed to 3.
- The Redis RPM bucket is keyed as `rateLimit:rpm:<user_id>`, so the dedicated RPM limit is a rolling-window count per user across non-whitelisted models, not an independent bucket per model. The model name is used for whitelist matching and diagnostics, not for the bucket key.
- The limiter removes entries older than the current rolling window and removes the new reservation when the count would exceed `max_rpm`. Therefore fifth-and-later attempts do not extend the block; they remain 429 while the existing oldest reservations are still inside the window, then capacity returns one slot at a time as those reservations expire.

## Current behavior

The system treats the grace period as an intentional suppression window: after three compliant reviews, subsequent RPM triggers are recorded for audit but never sent to the reviewer until the five-hour window expires. Earlier `review_unavailable` records indicate there was also a readiness/configuration period before the current grace window.

## Requirements

- Explain the skip reason and ordering clearly in the admin-visible task data or diagnosis.
- Preserve fail-closed behavior for disabled users, unavailable review configuration, and other safety gates.
- Resolve whether the grace-period suppression of RPM reviews is intended product behavior or an overly broad bypass.
- If a behavior change is approved, keep the change focused and preserve auditability, retry semantics, and automatic-ban safety.

## Acceptance criteria

- [ ] A deterministic test or production query distinguishes RPM trigger delivery from grace-period suppression and readiness suppression.
- [ ] The exact skip reason and active grace expiry are visible to the operator.
- [ ] The chosen behavior for RPM triggers during grace is documented and covered by a regression test.
- [ ] Disabled/unready review and permanent user safety gates remain unchanged.
- [ ] No automatic ban can result from bypassing or changing the grace-period gate.

## Open product decision

Should RPM-triggered events during a user's compliant grace period remain audit-only skipped records, or should RPM violations bypass the grace period and enter LLM review? Recommendation: keep the grace period for ordinary repeated checks unless the product intent is specifically to review every RPM violation; if bypassing it, apply the exception only to RPM and retain the current skip behavior for token triggers and other safety gates.

## Out of scope

- Changing the configured RPM threshold, compliant count, or grace duration without an explicit product decision.
- Removing the review readiness gate or allowing untested reviewer configurations to call the upstream model.
- Relaxing automatic-ban trust requirements.
