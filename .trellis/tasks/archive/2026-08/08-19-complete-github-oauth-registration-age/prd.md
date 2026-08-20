# Complete GitHub OAuth registration-age feature

## Goal

Close the audited validation, data-flow, localization, and behavior-test gaps so the existing GitHub OAuth registration-age implementation is safe and release-ready.

## Requirements

- Keep the option default at one calendar year, allow integer values `0..100`, and use `0` to disable the new-registration restriction.
- Reject invalid option updates before persistence and runtime publication; startup loading must publish the default, load valid records, and safely normalize invalid legacy records.
- Prove GitHub `created_at` JSON flows through `GetUserInfo` into `OAuthUser.CreatedAt`; missing/malformed optional metadata remains `nil` so existing login/bind can continue.
- Apply the age check only immediately before creating a new local GitHub user. Numeric-ID login, legacy-ID login/migration, and authenticated bind must remain available without age metadata.
- Keep missing/malformed age metadata fail-closed for new registration when enabled and return distinct localized callback errors without leaking provider details.
- Add explicit translated frontend validation for integer values in `0..100` and behavior-test the real OAuth settings form, including valid save, invalid no-request, and failed-save state.
- Write frontend locale changes only through the required translation script and synchronize all seven locales.

## Acceptance criteria

- [x] Provider-level tests prove valid/missing/malformed GitHub `created_at` behavior through `GetUserInfo`.
- [x] Controller tests prove too-new/unavailable rejection without creation, exact cutoff, disabled policy, numeric/legacy login, migration, bind, and localized callback envelopes.
- [x] Model/controller option tests prove default startup publication, valid startup load, invalid fallback, GET exposure, invalid PUT rejection, and no mutation of the last valid DB/runtime/map value.
- [x] The real frontend form shows localized validation, accepts `0` and valid integers, rejects negative/fractional/over-100 input without a request, and preserves unsaved state after API failure.
- [x] All backend/frontend locales and affected checks pass; the repository-wide copyright check remains blocked only by sibling/unrelated files.

## Out of scope

- Applying account-age policy to other OAuth providers or existing local users.
- Changing GitHub scopes, credentials, callback routes, or general OAuth authorization.
- Committing or deploying before the sibling Vision child also passes.
