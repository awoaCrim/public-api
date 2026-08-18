# Implementation Plan: Configurable Daily Check-in Balance Threshold

## Execution Rules

- Remain in Trellis planning until the user explicitly approves the final planning summary.
- Do not run `task.py start`, dispatch implementation agents, or edit product code before that approval.
- Preserve the unrelated dirty `.trellis`, `.agents`, `.pi`, `nul`, and `.trellis/spec` paths already present in the working tree.
- Do not commit, push, or deploy unless separately authorized.

## Phase A — Planning and Activation Gate

1. Re-read `prd.md`, `design.md`, and this plan top-to-bottom.
2. Confirm the resolved product decisions:
   - N is entered as the active display-currency amount;
   - the saved numeric value is dynamically reinterpreted after display-currency/rate changes;
   - TOKENS mode interprets N as USD;
   - N must be strictly positive;
   - threshold enforcement is independent and disabled by default.
3. Validate the task context and manifests:

```text
python ./.trellis/scripts/task.py validate daily-signin-balance-threshold
```

4. After explicit approval, activate the task with:

```text
python ./.trellis/scripts/task.py start daily-signin-balance-threshold
```

5. Load the backend/frontend guidance through the applicable Trellis implementation workflow before edits.

## Phase B — Backend Configuration and Currency Boundary

1. Extend `setting/operation_setting/checkin_setting.go` with the independent switch and positive display-unit threshold.
2. Add a single operation-setting helper that maps the current display mode/rate to an effective raw-quota `decimal.Decimal` threshold. Cover USD, CNY, CUSTOM, and TOKENS→USD.
3. Add strict option-update validation for the new threshold key before database persistence. Preserve a safe positive default for malformed legacy values loaded at startup.
4. Add the backend check-in i18n key and translations in `i18n/keys.go` and `i18n/locales/{en,zh-CN,zh-TW}.yaml`.
5. Keep the change free of database schema/migration work.

## Phase C — Check-in Enforcement and Controller Contract

1. Add stable model sentinel errors for threshold rejection and authoritative balance-read failure.
2. In `model.UserCheckin`, after the existing enabled/duplicate checks and before reward/record side effects, read `GetUserQuota(userID, true)` when the threshold switch is enabled.
3. Compare the authoritative quota to the decimal effective threshold with `>=`; reject before creating a record or incrementing quota.
4. Map the sentinel conditions in `controller.DoCheckin` to localized/generic dashboard API responses without exposing database details.
5. Preserve existing MySQL/PostgreSQL transaction handling, SQLite rollback behavior, unique daily constraint, reward range, logging, and response success shape.

## Phase D — Admin Settings UI

1. Extend `BillingSettings`, billing defaults, and the Check-in section registry with the two new option keys.
2. Update `CheckinSettingsSection` with:
   - independent threshold switch;
   - positive decimal validation;
   - dynamic current-unit label;
   - explicit USD fallback explanation in TOKENS mode;
   - save/reset behavior through the existing option update hook.
3. Add the new user-facing keys to all seven frontend locales using the i18n skill and run the repository i18n synchronization check.
4. Keep profile/check-in API payload shape unchanged unless a focused UI requirement proves a status field is necessary.

## Phase E — Regression Tests

1. Add operation-setting conversion/validation tests for all display modes, dynamic rates, positive-only values, and TOKENS→USD.
2. Add model check-in tests for disabled bypass, below/equal/above threshold, no side effect on rejection, balance read failure, duplicate check-in, and existing reward/accounting behavior.
3. Add controller tests for localized threshold rejection and safe balance-read failure response where the current test harness supports it.
4. Add frontend settings validation/unit-label coverage using the existing system-settings test conventions; do not add a coverage-only test.
5. Ensure tests restore global settings and database/cache state.

## Phase F — Focused Verification

Run the applicable focused checks:

```text
gofmt -w <changed Go files>
go test ./setting/operation_setting ./model ./controller -count=1
go build ./...
cd relaykit && GOWORK=off go build ./...
cd ..
git diff --check
```

From `web/`:

```text
bun test <affected tests>
bun run typecheck
bun run build:check
bunx oxlint -c .oxlintrc.json <changed frontend files>
bunx oxfmt <changed frontend files> --check
bun run i18n:sync
```

Record unrelated pre-existing full-suite lint/format/test failures without broadening this task.

## Phase G — Full Quality Check and Review

1. Run focused model/controller tests again after final edits and inspect persisted/response behavior.
2. Run the repository's known applicable root tests/build checks.
3. Run a fresh-context `trellis-check` review against `prd.md`, `design.md`, the staged diff, cross-layer currency conversion, and SQLite/MySQL/PostgreSQL compatibility.
4. Resolve actionable findings, rerun affected tests, and update the acceptance checklist in `prd.md` only after verification.
5. Run `git diff --check` and inspect the final diff for unrelated changes, protected identifiers, invalid currency conversion, and accidental production/config edits.

## Rollback Points

- Before backend edits: revert only this task's files if the configuration contract is wrong.
- Before frontend edits: backend conversion and threshold tests must pass for all four display modes.
- Before final commit: all focused tests and cross-layer review must pass.
- If an implementation is rejected: disable the new switch or deploy the previous code; no database rollback is needed.
