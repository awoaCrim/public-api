# Completion audit summary

## Working tree classification

The authorized product changes are two feature groups: GitHub OAuth registration age and Vision interception. Trellis/Pi installation files, active/archived task artifacts, workspace state, caches/build output, and the accidental `nul` path are excluded from new commits.

The product groups include untracked runtime files, so explicit path staging is mandatory. A normal push also includes the five existing local commits already ahead of `origin/main`.

## GitHub OAuth gaps found by independent review

- Explicit frontend integer/range validation messages are not localized.
- Provider tests cover only the timestamp helper, not GitHub JSON through `GetUserInfo` to `OAuthUser.CreatedAt`.
- Legacy login/migration bypass of the new-registration age gate lacks a regression test.
- Startup option loading and HTTP option API publication/rejection are not tested end to end.
- The real OAuth callback envelope/localized age errors and the real React settings form lack behavior tests.

The core policy placement is correct: numeric and legacy login occur before the new-user gate, bind is routed separately, `0` disables the policy, `AddDate` provides calendar-year comparison, and unavailable metadata fails closed only for new registration.

## Vision gaps found by independent review

- `phash_threshold` has only frontend bounds; the trusted backend boundary accepts invalid values.
- UI says threshold `0` disables perceptual deduplication, but current grouping still computes pHash and merges distance-zero images.
- Existing middleware tests assert map membership/no-image behavior rather than a real Responses conversion path.
- The profile card lacks user-visible interaction/error/auth-store tests.
- New frontend Vision files fail the project copyright-header check.

Confirmed product decision: threshold `0` performs no pHash calculation/clustering and each image is a separate perceptual cluster; thresholds `1..64` enable pHash clustering. Existing exact-URL/request/LRU caches remain.

## Deployment baseline

Read-only probe confirmed `ssh2` resolves to `root@43.131.249.217:22`, application directory `/opt/newapi`, service/container `newapi`, image `newapi-custom:v1.0.0-custom.10-b7123a90`, and a responding local `/api/status` endpoint.
