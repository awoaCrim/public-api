# Technical Design: Configurable Daily Check-in Balance Threshold

## 1. Design Summary

Extend the existing `checkin_setting` configuration and Check-in Rewards admin section with an independent balance-threshold switch and a positive numeric threshold. The threshold is entered as the currently configured display-currency amount, but is stored as the entered number without normalization. At check-in time, the backend dynamically converts that number using the current display mode and exchange settings:

- `USD`: threshold is interpreted as USD.
- `CNY`: threshold is interpreted as CNY and divided by the current USD→CNY rate.
- `CUSTOM`: threshold is interpreted as the configured custom currency and divided by its current USD→custom rate.
- `TOKENS`: threshold is interpreted as USD, as explicitly decided by the user.

The effective threshold is compared against the user's authoritative database quota before any check-in record or reward is created. The comparison uses decimal arithmetic so fractional currency thresholds do not move the inclusive boundary because of an intermediate integer rounding step.

The existing check-in switch remains the master switch for the feature. The new threshold switch defaults to disabled, preserving current behavior for existing installations.

## 2. Configuration and Cross-Layer Contract

### 2.1 Backend setting

Extend `setting/operation_setting/checkin_setting.go`:

- `BalanceThresholdEnabled bool` serialized as `balance_threshold_enabled`.
- `BalanceThreshold float64` serialized as `balance_threshold`, defaulting to `1.0` while the threshold switch is disabled.

The existing `config.GlobalConfig` registration and flattened option mechanism will expose these as:

- `checkin_setting.balance_threshold_enabled`
- `checkin_setting.balance_threshold`

No database table or migration is required; values continue to live in the existing `options` table.

The option-update boundary must reject non-finite, zero, and negative threshold values. Runtime/config loading should retain a safe positive default when an invalid legacy value is encountered rather than activating a malformed threshold. This keeps the product fail-closed for the check-in mutation without making a bad persisted value silently grant rewards.

### 2.2 Currency conversion owner

Keep conversion logic in the operation-setting/domain boundary rather than duplicating it in the controller and model. Add a focused helper that returns the effective threshold in raw quota units as a `decimal.Decimal`, using:

```text
threshold display value
  -> divide by active display rate (USD: 1, CNY: USDExchangeRate,
     CUSTOM: CustomCurrencyExchangeRate, TOKENS: 1)
  -> multiply by common.QuotaPerUnit
```

The helper must normalize invalid/non-positive exchange rates to the same safe fallback used by existing display conversion. It must not round to `int` before the model compares it with `User.Quota`.

### 2.3 Check-in mutation contract

`model.UserCheckin` remains the single business mutation entry point. Its order becomes:

```text
check-in enabled?
  -> already checked in today?
  -> if threshold switch enabled:
       authoritative DB quota read
       read failure -> sentinel error; no side effect
       quota >= effective threshold -> threshold sentinel; no side effect
  -> calculate reward
  -> existing DB-specific record/reward update path
```

Use `GetUserQuota(userID, true)` so Redis lag cannot make a user pass a threshold they have already crossed in the primary database. Preserve the existing unique daily index, MySQL/PostgreSQL transaction path, and SQLite rollback path.

A threshold rejection must occur before `Checkin` creation and before quota increment. The model returns stable sentinel errors; it does not write to Gin contexts.

### 2.4 API error contract

Add a backend i18n key for the threshold rejection in `i18n/keys.go` and the three backend locale YAML files. In `controller.DoCheckin`:

- map the threshold sentinel to the localized check-in threshold message;
- map an authoritative-balance read failure to the existing generic database error contract and log the internal error with request context;
- preserve the existing response envelope and HTTP 200 business-error convention;
- leave unrelated legacy check-in error messages unchanged unless a test requires a narrow correction.

The frontend check-in card continues to display `res.message`, so no second client-side interpretation of this backend error is necessary.

## 3. Frontend Design

### 3.1 Settings payload

Extend `BillingSettings`, `defaultBillingSettings`, and the Check-in section registry with:

- `'checkin_setting.balance_threshold_enabled': boolean`
- `'checkin_setting.balance_threshold': number`

`CheckinSettingsSection` receives the two values, persists them through `useUpdateOption`, and keeps the existing per-field update behavior.

### 3.2 Admin UX

Add an independent switch such as “Enable check-in balance threshold.” When enabled, show a decimal number input with `step="any"` and strict `> 0` validation. The description must state that users can check in only when their current balance is below the configured value.

Use the existing `getCurrencyDisplay`/`getCurrencyLabel` configuration source for the unit label:

- USD, CNY, or the custom currency label follows the current display configuration.
- TOKENS renders an explicit USD fallback label and explanatory text because the backend interprets `N` as USD in this mode.

Changing the site's display currency or exchange rate changes only the displayed meaning of the saved number; the persisted numeric value is unchanged. No client-side conversion should be applied when saving the field.

### 3.3 Internationalization

Add all new administrator descriptions, labels, validation text, and threshold rejection copy through the existing i18n workflow. Frontend locale files remain flat JSON with English source keys and must be synchronized across `en`, `zh`, `zh-TW`, `fr`, `ru`, `ja`, and `vi`. Backend check-in error copy is added to `en.yaml`, `zh-CN.yaml`, and `zh-TW.yaml`.

## 4. Validation and Error Handling

- Frontend schema: threshold is finite and strictly positive; empty, zero, negative, and non-numeric values are rejected before save.
- Backend option update: the new threshold key is validated before persistence; malformed values cannot overwrite the valid in-memory setting.
- Runtime check-in: if threshold is enabled and balance cannot be read, return a safe generic error and do not create a check-in row or grant quota.
- Do not include raw database errors in the user-facing response.
- Do not use float-to-int casts for the effective threshold comparison; compare decimal raw-quota values with the integer balance converted to decimal.

## 5. Tests

### Backend

Add focused tests for:

1. Currency conversion table: USD, CNY, custom currency, and TOKENS/USD fallback.
2. Dynamic interpretation: changing the display type or exchange rate changes the effective threshold while the configured numeric value remains unchanged.
3. Strict threshold validation: positive values accepted; zero, negative, NaN, and infinity rejected.
4. `UserCheckin`: threshold disabled bypasses the guard; balance below threshold succeeds; balance exactly equal to or above threshold is rejected; rejection leaves no row and no quota mutation; balance read failure fails safely; once-per-day behavior remains unchanged.
5. Controller mapping: threshold rejection is localized and balance-read failures use the safe database error contract.

Use deterministic fixtures and `require`/`assert` as required by the backend quality guidelines. Restore global settings, database handles, database type, cache flags, and locale state in test cleanup.

### Frontend

Add focused coverage for the settings payload/validation and the TOKENS→USD unit label fallback if the existing component harness supports it. At minimum, run typecheck/build and targeted lint/format checks on the changed settings component and verify all seven locale files with `i18n:sync`.

## 6. Compatibility, Operations, and Rollback

- Existing installations keep threshold enforcement off by default.
- Existing check-in API response shape and once-per-day behavior remain unchanged except for the new localized rejection condition.
- Existing option rows are additive and safe for SQLite, MySQL, and PostgreSQL; no schema migration is introduced.
- No user balance, reward amount, registration reward, invitation reward, top-up, or relay billing behavior changes.
- Rollback is code/config based: remove or disable the new option keys, or deploy the previous image. No data migration rollback is needed.
- Deployment is out of scope unless separately authorized; normal implementation verification must not alter production settings or data.
