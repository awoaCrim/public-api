# Configurable daily sign-in balance threshold

## Goal

Allow administrators to optionally stop daily check-in rewards for users whose current remaining balance has reached a configured threshold. When the feature is enabled, a user may check in only while their current balance is strictly below `N`; when disabled, existing check-in behavior remains unchanged.

## Confirmed Repository Facts

- Daily check-in is already implemented by `model.UserCheckin` in `model/checkin.go` and is exposed by `controller.DoCheckin`.
- Existing check-in settings are registered as `checkin_setting` with `enabled`, `min_quota`, and `max_quota` in `setting/operation_setting/checkin_setting.go`.
- The existing admin UI is `web/src/features/system-settings/general/checkin-settings-section.tsx`, under Billing → Check-in Rewards, and persists settings through the existing option-update flow.
- The user's remaining balance is represented by `model.User.Quota`; `model.GetUserQuota` supports a database-backed read and a Redis-backed read. Check-in currently checks only feature enablement and once-per-day status before awarding quota.
- The check-in award is applied atomically with the daily record for MySQL/PostgreSQL and with explicit rollback for SQLite; the threshold check must not weaken those duplicate-check and quota-update guarantees.

## In Scope

- Add an optional check-in balance-threshold switch and a configurable threshold to the existing check-in setting.
- Enforce the threshold in the backend check-in mutation path using the user's current remaining balance.
- Keep the existing behavior when the switch is disabled.
- Expose and persist the new settings in the existing administrator Check-in Settings section.
- Return a clear, localized failure message when the threshold prevents a check-in.
- Add regression coverage for disabled/enabled behavior, boundary comparison, balance read failures, and existing once-per-day behavior.

## Out of Scope

- Changing the meaning or amount of the existing random check-in reward.
- Applying the threshold to registration rewards, invitations, top-ups, API requests, or any operation other than daily check-in.
- Automatically revoking or changing already-issued check-in rewards.
- Changing the user's displayed balance or quota accounting model.
- Adding a new database table or migration; the setting should use the existing option configuration mechanism.

## Acceptance Criteria

- [x] Administrators can enable/disable the balance threshold independently of the existing check-in switch.
- [x] When disabled, users retain the current check-in behavior regardless of balance.
- [x] When enabled, a user with balance `>= N` is rejected before a check-in record or reward is created; a user with balance `< N` can proceed if all existing checks pass.
- [x] A balance exactly equal to `N` is rejected, matching the requested “not below N” rule.
- [x] The backend evaluates the threshold against the current balance and fails safely if that balance cannot be read; it must not grant a reward on an unknown balance.
- [x] Existing daily uniqueness, quota update atomicity/rollback, and no-threshold error behavior remain intact.
- [x] The administrator UI validates and saves the threshold using the chosen unit and clearly explains the comparison rule.
- [x] The user-facing threshold rejection message is translated in all supported frontend locales, with backend/API error behavior remaining stable.
- [x] Focused backend tests, frontend typecheck/build/lint/format checks, i18n synchronization, and `git diff --check` pass.

## Verification Notes

- Frontend targeted tests, typecheck, production build, targeted lint/format checks, i18n synchronization, and `git diff --check` passed.
- Go 1.26.5 was installed with winget. Focused backend tests passed for `setting/operation_setting`, `model`, and `controller`; `go build ./...` and `cd relaykit && GOWORK=off go build ./...` also passed.
- Trellis task context validation passed.
- Committed as `fc5623af` and pushed to `origin/rebuild/customizations-20260812`.
- Deployed image `newapi-custom:20260816-034834-checkin-fc5623af` to `/opt/newapi`; backup is `/opt/newapi/backups/20260816-034834-checkin-fc5623af`, SQLite integrity check returned `ok`, and no rollback was needed.
- Post-deployment checks passed: container running with restart count 0, `/api/status` HTTP 200, and root page HTTP 200.

## Resolved Product Decisions

1. `N` is entered in the user's currently configured display-currency unit rather than the internal quota integer. The supported display configuration can be USD, CNY, or a custom currency.
2. The saved numeric value is dynamically interpreted using the current display currency and exchange rate. Changing the display currency or rate therefore changes the effective internal threshold; the saved number itself does not change.
3. When the site is configured for token-only display (`TOKENS`), `N` is interpreted as USD. The threshold remains active instead of being disabled; the administrator UI must label this fallback clearly.
4. `N` must be strictly greater than zero. The existing check-in enable switch remains the supported way to disable all check-ins.

## Technical Notes

- The repository's frontend converts raw quota to the configured display currency through `formatQuotaWithCurrency`: raw quota → USD-equivalent using `QuotaPerUnit` → configured exchange rate. Backend currency conversion uses the same `QuotaPerUnit`, USD exchange rate, and custom-currency exchange rate concepts.
- The recommended implementation is to reuse the existing `checkin_setting` option object and Check-in Settings UI rather than introducing a parallel setting surface.
- The comparison is performed before creating the daily record and before awarding quota, using an authoritative database read (`GetUserQuota(userID, true)`) so stale Redis state cannot incorrectly allow a threshold-protected check-in.

## Notes

- Requirements have converged in the Trellis planning phase; product code must not be changed until the final planning summary is explicitly approved and the task is started.
- Complex-task artifacts are complete: `design.md`, `implement.md`, `implement.jsonl`, and `check.jsonl`.
