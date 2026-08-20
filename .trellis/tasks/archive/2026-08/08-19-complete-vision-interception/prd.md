# Complete Vision interception feature

## Goal

Finish the Vision default-prompt and Responses API work with a trustworthy threshold contract, behavior-level middleware/UI tests, and release-compliant frontend files.

## Requirements

- Accept persisted/API `phash_threshold` values only in `0..64`; reject new out-of-range values before replacing user settings.
- Treat malformed legacy runtime thresholds as the safe value `0`, avoiding broad accidental image clustering.
- At threshold `0`, perform no image download/decode/pHash calculation for perceptual grouping and keep each image in a separate group. Preserve existing exact-URL/request/LRU cache behavior.
- At thresholds `1..64`, preserve bounded pHash clustering and cache behavior.
- Prove the real `/v1/responses` middleware success path converts `input_image` to `input_text`, rewrites the reusable request body/context marker, and preserves the original client model. Preserve fail-open behavior on extraction/analysis failure.
- Preserve the canonical evidence-based default prompt across backend and frontend; blank saves normalize to that prompt and custom nonblank prompts remain unchanged.
- Add profile-card behavior tests for initial default display, blank save, threshold submission, API failure, profile refresh, and auth-store update.
- Ensure all new frontend files satisfy copyright, format, lint, i18n, accessibility, typecheck, and build rules.

## Acceptance criteria

- [x] Controller tests reject `-1` and `65` without modifying the stored settings, and accept `0` and `64` through both current and legacy settings endpoints.
- [x] Service tests prove invalid legacy thresholds normalize to `0`, threshold `0` does no pHash decoder work/grouping, and positive thresholds retain clustering.
- [x] Middleware tests protect actual Responses conversion, request-body replacement, interception marker, model preservation, partial-mutation fail-open behavior, and analysis failure.
- [x] Frontend tests protect prompt normalization, valid and invalid threshold submission, saving/failure state, accessibility, and auth-store/profile behavior from the user perspective.
- [x] Vision copyright and all affected backend/frontend checks pass; repository-wide unrelated copyright/format/lint failures are documented.

## Out of scope

- Changing Vision billing, provider selection, global cache architecture, or image security limits unrelated to threshold validation.
- Removing exact-URL/request/LRU caching when perceptual grouping is disabled.
- Committing or deploying before the sibling GitHub OAuth child also passes.
