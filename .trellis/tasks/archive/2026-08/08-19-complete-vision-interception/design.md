# Design: close Vision release gaps

## Threshold domain boundary

Define the pHash threshold domain in `service/vision` as `0..64` with reusable validation/normalization behavior. The controller validates before replacing `settings.Vision`; invalid API input returns a user-safe 400-style error and leaves all user settings unchanged. Runtime middleware normalizes legacy invalid values to `0` as defense in depth.

`clusterImages(entries, 0)` builds one group per valid image entry immediately and never invokes the pHash decoder. Exact URL and request/LRU caches remain available later in `LookupCachedDescription`/`AnalyzeImage`; the disabled contract concerns perceptual grouping only. Positive values keep current pHash grouping.

## Middleware behavior test seam

Use the real middleware path and controlled image-analysis boundary so tests can assert the rewritten JSON body, `KeyRequestBody`, `vision_intercepted`, and unchanged `model`. Do not test only private route maps or a no-image early return. Retain fail-open semantics by keeping the original body untouched until all replacements succeed.

## Frontend behavior

Keep `normalizeVisionPrompt` as the frontend projection of the backend default. Add a component-level test in the feature `__tests__` directory using user-facing labels and async outcome assertions. Ensure threshold input is constrained and API rejection does not update auth state or report success.

## Compatibility

No DTO schema change is required. Existing valid settings remain unchanged. Legacy invalid thresholds become safe disabled perceptual grouping at runtime and are rejected on the next attempted save until corrected.
