# Implementation plan: GitHub OAuth registration-age restriction

## Ordered work

1. **Load package guidance and establish a clean diff boundary**
   - Read the backend Trellis specs and the frontend/i18n skills before editing Go or React/locale files.
   - Confirm all existing working-tree changes are unrelated and remain untouched.
   - Identify the exact current option, OAuth, and settings test fixtures to reuse.

2. **Add the configuration contract**
   - Add the GitHub minimum-age option key, default `1`, safe maximum `100`, and runtime variable in `common`.
   - Add strict shared parsing/validation for whole calendar-year values in the range `0..100`.
   - Initialize the option in `model.InitOptionMap`, load it from `Option`, and update the runtime value only after validation.
   - Ensure invalid persisted values fall back to the default without leaving an invalid `OptionMap` value active.
   - Validate the generic option update request before persistence and preserve the existing option API envelope.

3. **Carry GitHub account creation metadata**
   - Extend `oauth.OAuthUser` with an optional creation timestamp.
   - Decode GitHub `created_at` as a raw string and parse it separately so malformed optional metadata does not break existing login or binding.
   - Add focused parser/provider tests for valid, missing, and malformed timestamps.

4. **Enforce the policy only for new GitHub registration**
   - Add typed controller errors and translated backend keys for too-new accounts and unverifiable account age.
   - Add a pure calendar-year decision helper using `time.Time.AddDate` and an injectable `now` value for deterministic tests.
   - Call it in `findOrCreateOAuthUser` only after numeric and legacy provider-ID lookups, and before local-user side effects or inserts.
   - Map the typed errors through the existing localized OAuth response path.
   - Leave existing-user login, legacy migration, authenticated binding, provider scopes, and route authorization unchanged.

5. **Expose and validate the setting in the OAuth UI**
   - Extend `AuthSettings`, the default settings, section registry, OAuth form schema, flattened defaults, normalization, and GitHub tab.
   - Use `safeNumberFieldProps` with integer/min/max constraints and explain that `0` disables the policy.
   - Add all new user-facing strings and validation messages to `en`, `zh`, `zh-TW`, `fr`, `ja`, `ru`, and `vi` through the project i18n workflow.
   - Add focused frontend tests for default parsing, field accessibility, valid submission, and invalid values.

6. **Run focused verification and review the cross-layer contract**
   - Run `gofmt` on changed Go files.
   - Run focused backend tests for `common`, `model`, `oauth`, and `controller` (including option persistence and OAuth registration behavior).
   - Run the affected frontend tests if present, then `bun run typecheck`, affected-file lint, `bun run format:check`, `bun run i18n:sync`, and `bun run build:check` from `web/`.
   - Run `git diff --check` and inspect the final diff for unchanged authorization, binding, and existing OAuth behavior.

## Planned verification commands

```text
# Backend
 gofmt -w <changed Go files>
 go test ./common ./model ./oauth ./controller

# Frontend
 cd web
 bun run typecheck
 bun run lint -- <affected files>       # or the repository-equivalent affected-file invocation
 bun run format:check
 bun run i18n:sync
 bun run build:check

# Repository
 git diff --check
```

The exact focused test selectors will be confirmed from the repository after the test files are added; no test command will be assumed if the current package layout requires a narrower selector.

## Review gates

- Do not start implementation until this plan is approved and the Trellis task is moved from `planning` to `in_progress`.
- Before reporting completion, verify that no database insert occurs on a rejected new registration, that existing users and bind flows bypass the age check, and that malformed provider metadata fails closed only for new registration.
- Do not commit changes unless the user explicitly requests a commit.
