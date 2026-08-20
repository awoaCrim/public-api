# Repository audit: combined task

Date: 2026-08-16

The two child deliverables have independent boundaries:

1. Usage analysis: root-only route -> options response -> frontend filter initialization -> first query parameters. The current gap is that `all` is both the UI default and the backend meaning of omitted `user_id`.
2. LLM review: settings/controller -> capability request mode -> worker readiness -> response extraction/validation -> task persistence/auto-ban. The current gap is that every request assumes strict JSON Schema, while production can remain enabled without a passing test; policy text has no source beyond the manual setting.

Confirmed product decision: use compatibility output modes for non-strict reviewer models, but only strict-schema capability plus validated output may participate in automatic bans. Policy text stays administrator-provided and is required for effective review; missing policy must be surfaced clearly and handled fail-closed.

No production code is changed during planning.
