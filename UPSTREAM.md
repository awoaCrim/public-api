# Upstream Provenance

## Repository

- Upstream project: `QuantumNous/new-api`
- Upstream URL: `https://github.com/QuantumNous/new-api.git`
- Fork URL: `https://github.com/awoaCrim/public-api.git`
- Rebuild baseline commit: `ccd535ef8e50cf6e5846a59278c40b7ff59d1b7d`
- Baseline commit subject: `fix: harden concurrent quota and status updates`
- Rebuild branch: `rebuild/customizations-20260812`
- Baseline verified on: `2026-08-12`

## Local Source Inventory

The previous customized workspace is preserved read-only at `E:\myCode\myapi`.
Its first Git commit is a full workspace snapshot rather than a commit with an
upstream parent. Therefore, customization provenance must be reconstructed at
the feature level by comparing actual code, design records, tests, and current
upstream behavior. The old snapshot must not be merged wholesale into this
repository.

## Rebuild Policy

1. Prefer current upstream implementations when equivalent functionality is
   already available.
2. Reimplement local customizations against current APIs and architecture;
   avoid copying obsolete packages or entire files over upstream changes.
3. Keep each customization in a focused migration batch with targeted tests.
4. Record database compatibility for SQLite, MySQL, and PostgreSQL.
5. Do not restore the obsolete parallel `routing_groups` runtime model.
6. Do not deploy, commit, or push the rebuild without explicit approval.
