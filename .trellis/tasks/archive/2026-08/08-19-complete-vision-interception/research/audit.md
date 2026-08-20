# Independent audit findings

The existing default prompt and Responses replacement logic are internally consistent, but release blockers remain:

1. `phash_threshold` is not validated by the trusted backend and values above 64 can merge unrelated images and change billing-call counts.
2. Threshold `0` currently still computes pHash and merges distance-zero images despite UI copy saying disabled.
3. Middleware tests assert route-map/no-image implementation details rather than real conversion/body/model behavior.
4. The profile card lacks interaction/error/auth-store tests.
5. New Vision frontend files fail the repository copyright-header check.
6. Runtime helper/test files are untracked and require explicit staging.

Confirmed decision: `0` disables pHash computation/clustering and produces separate perceptual groups; `1..64` enables clustering. Existing exact-URL/request/LRU caches remain.
