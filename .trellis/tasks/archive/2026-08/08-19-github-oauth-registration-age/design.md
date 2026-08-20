# Design: GitHub OAuth registration-age restriction

## Scope and boundary

The restriction belongs at the new-user branch of `controller.findOrCreateOAuthUser`. The existing-user lookup and the authenticated bind flow must remain ahead of the check so the policy affects only creation of a new local user through GitHub OAuth.

The implementation will not change provider authorization scopes, callback routes, route authorization, or the local `User.CreatedAt` field. It will carry GitHub's account creation timestamp from the provider response into the OAuth user snapshot and evaluate it only when a new GitHub-linked user would otherwise be inserted.

## Configuration contract

Add the persisted system option `GitHubOAuthMinimumAgeYears`:

- default runtime and persisted-map value: `1`;
- `0` disables the restriction;
- positive values require the GitHub account to be at least that many calendar years old;
- accepted values are whole numbers in the safe range `0..100`;
- invalid values are rejected by the update boundary and are not published as active runtime state;
- an invalid legacy database value falls back to the safe default and emits the existing startup/update diagnostic path.

The value will be represented as an `int` in runtime configuration and as the existing string value in `model.Option`/`common.OptionMap`. The shared parser/validator will be used by both option loading and the controller update path so startup, API updates, and the frontend agree on the same contract.

## Cross-layer data flow

```text
GitHub GET /user response
  -> gitHubUser.created_at raw string
  -> optional *time.Time in oauth.OAuthUser
  -> HandleOAuth login intent
  -> existing numeric/legacy binding lookup
  -> registration-enabled check
  -> GitHub age check
  -> local user creation and binding
```

`gitHubUser.CreatedAt` will remain a raw string during JSON decoding. The provider will parse it separately with RFC3339 parsing. A missing or malformed value will be logged as provider metadata diagnostic and represented as `nil`, rather than making `GetUserInfo` fail. This preserves existing GitHub login and authenticated binding behavior even if the optional metadata is unavailable.

`OAuthUser` will gain an optional `CreatedAt *time.Time` field. Other OAuth providers will leave it nil and will not be subject to the GitHub-only check.

## Registration decision

Add a pure, testable calendar-age decision helper in the controller layer, conceptually:

```text
checkGitHubRegistrationAge(now, createdAt, minimumAgeYears) -> nil or typed registration error
```

Rules:

1. If the configured value is `0`, allow registration without requiring metadata.
2. If the configured value is positive and `CreatedAt` is nil, return a typed metadata-unavailable error.
3. Compute `cutoff := now.AddDate(-minimumAgeYears, 0, 0)`.
4. Allow creation when `createdAt <= cutoff`; reject when `createdAt > cutoff`.
5. Use the request-time clock and the timestamp's instant, not a fixed 365-day duration and not the application's local user creation time.

`findOrCreateOAuthUser` calls this helper only after both the new provider ID and the legacy GitHub login ID have been checked. Therefore:

- existing GitHub-linked users bypass the threshold;
- deleted-user and legacy-ID migration behavior stays unchanged;
- a new registration is rejected before `GetMaxUserId`, email availability side effects, or any database insert;
- `handleOAuthBind` is unaffected because bind flows return before `findOrCreateOAuthUser`.

Use typed errors for the two user-visible outcomes:

- too-new account: translated message includes the configured calendar-year threshold;
- missing/invalid metadata: translated generic message says GitHub account age could not be verified, without exposing provider payloads or credentials.

`HandleOAuth` maps these errors through `common.ApiErrorI18n`, just like the existing registration-disabled and account-status errors. The consumed auth flow remains consumed, and no local user is created on either rejection path.

## Backend option persistence

- `common/constants.go`: add the option key, default, maximum, and runtime variable beside the existing GitHub OAuth settings.
- `model.InitOptionMap`: publish the default value before loading database overrides.
- `model.validateOptionValue`/`updateOptionMap`: parse and validate the value before active-map publication; on invalid startup data, restore the default runtime/map value and return the diagnostic error.
- `controller.UpdateOption`: retain the generic option endpoint and add the same strict validation at the request boundary so malformed numbers, negative values, decimals, and out-of-range values cannot be saved.
- No schema migration is needed because `Option` already stores arbitrary key/value pairs and GORM supports the existing SQLite, MySQL, and PostgreSQL paths.

## Frontend settings

Extend the existing OAuth settings shape and registry wiring with a numeric `GitHubOAuthMinimumAgeYears` field. The GitHub tab will display:

- a whole-number input with `min=0`, `max=100`, and `step=1`;
- a description explaining that `0` disables the restriction and positive values are calendar years;
- the default value `1` when the option is absent or cannot be parsed.

Use the existing `safeNumberFieldProps` adapter and Zod validation. `normalizeFormValues` will submit the numeric option through the existing `useUpdateOption` mutation; no new API endpoint or settings transport is required. All user-visible labels, descriptions, and validation messages will be added to the supported frontend locale files according to the i18n skill.

## Test strategy

### Backend

- shared option parser: default, zero, positive values, whitespace, negative/decimal/overflow/out-of-range rejection;
- option loading/update: valid value changes runtime state and `OptionMap`; invalid values do not publish invalid state and preserve the safe fallback;
- GitHub timestamp parsing: valid RFC3339, empty, and malformed metadata;
- calendar boundary helper: exact cutoff allowed, one instant newer rejected, leap-year/calendar boundary behavior, disabled mode, and unavailable metadata fail-closed;
- OAuth registration path: too-new and unavailable metadata create no local user, old/exactly-old accounts create successfully, existing numeric/legacy users bypass the check, and bind flow remains independent;
- translated controller errors contain only the safe localized message and no raw GitHub response details.

### Frontend

- settings defaults and option parsing use `1` when absent;
- valid zero/positive whole-year values can be edited and submitted;
- negative, fractional, and out-of-range values fail client validation;
- the GitHub tab exposes the setting with an accessible label and the `0`-disables description;
- existing OAuth fields and save/reset behavior remain intact.

## Compatibility and rollback

The change is additive. Existing installations receive a default one-year policy on initialization, so GitHub accounts that are less than one calendar year old will no longer create new local users unless an administrator sets the option to `0`. Existing linked users and bind flows remain compatible. Rollback consists of reverting the code and locale changes; the extra `Option` row is harmless to older code because unknown options are already tolerated by the generic option map.
