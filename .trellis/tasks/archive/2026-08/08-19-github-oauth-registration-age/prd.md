# Add GitHub OAuth registration-age restriction

## Goal

Prevent newly created GitHub accounts from registering a new application user through GitHub OAuth until the configured minimum GitHub-account age has elapsed, while preserving normal login for existing GitHub-bound users and preserving OAuth binding for authenticated users.

## Confirmed repository facts

- The standard GitHub OAuth flow is implemented in `oauth/github.go` and `controller/oauth.go`.
- `oauth.GitHubProvider.GetUserInfo` currently reads only the GitHub numeric ID, login, name, and email. It does not retain the GitHub account creation timestamp.
- `controller.findOrCreateOAuthUser` distinguishes existing OAuth users from new registrations. The registration gate currently checks only `common.RegisterEnabled`.
- Existing system settings already use the persisted `Option`/`common.OptionMap` mechanism. GitHub OAuth settings are exposed through `web/src/features/system-settings/auth/oauth-section.tsx`, `web/src/features/system-settings/auth/section-registry.tsx`, and `web/src/features/system-settings/types.ts`.
- The GitHub REST user response provides a `created_at` timestamp that can support this check.
- The current GitHub OAuth flow has no application-level registration-age restriction.

## Resolved product decision

The minimum GitHub account age is a configurable non-negative integer measured in calendar years:

- Default: `1` year.
- `0`: disable the restriction.
- Positive values: require the GitHub account to be at least that many calendar years old.
- The setting is exposed in the existing OAuth system-settings UI so administrators can change the policy without rebuilding or redeploying.
- The comparison uses calendar-year arithmetic (`now.AddDate(-years, 0, 0)`), not a fixed `365 * years` day count, so leap years and calendar boundaries behave predictably.

## Requirements

- Apply the age check only when creating a new application user through GitHub OAuth.
- Existing users already linked to the GitHub account must be able to log in regardless of the current threshold.
- Authenticated OAuth bind flows must remain binding flows and must not be treated as new registration.
- The check must use the canonical GitHub account creation timestamp, not the local application's `User.CreatedAt`.
- When the restriction is enabled and the GitHub account is too new, reject registration with a translated, user-safe error message and do not create a local user.
- Missing or invalid creation metadata while the restriction is enabled must fail closed for new registration; it must not silently bypass the restriction. Existing login and authenticated binding flows must remain available.
- Existing GitHub OAuth enablement, client credential validation, CSRF state handling, legacy-ID migration, login, binding, and route authorization must remain unchanged.

## Acceptance criteria

- [x] The configured minimum age is persisted, loaded at startup, exposed in the OAuth system-settings UI, and validated at the agreed boundary.
- [x] A GitHub account younger than the configured threshold cannot create a new local user.
- [x] A GitHub account at or above the threshold can register successfully.
- [x] Existing GitHub-linked users can log in even if they are below the threshold.
- [x] GitHub OAuth binding for an authenticated user remains functional and is not blocked by the registration restriction.
- [x] The rejection response is localized and does not expose provider response details or credentials.
- [x] Focused backend and frontend tests cover threshold boundaries, disabled behavior, existing-user bypass, binding preservation, invalid metadata, settings persistence, and UI validation.
- [x] Backend tests, affected-file frontend typecheck/build/lint/format, i18n checks, and `git diff --check` pass.

## Out of scope

- Restricting Discord, OIDC, LinuxDO, Telegram, WeChat, custom OAuth, password registration, or existing local accounts.
- Changing GitHub OAuth scopes, client credentials, callback routes, provider authorization, or local user deletion behavior.
- Retroactively disabling or deleting existing users whose GitHub accounts are younger than the threshold.
- Replacing the GitHub OAuth provider or introducing a general provider-age policy before this GitHub-specific behavior is proven.

## Completion status

The persisted option, GitHub timestamp parsing, fail-closed registration check, localized errors, frontend setting field, focused regression tests, and cross-layer backend spec update are implemented. Full frontend lint/format and the full Go test suite still report unrelated pre-existing failures outside this task; affected-file checks and task-focused tests pass.
