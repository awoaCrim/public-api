# Expose Request Body Log Settings

## Goal

Make Request Snapshot management discoverable as an independent item under System Settings → Operations.

## Requirements

- Add a dedicated Operations section ID and sidebar item for Request Snapshots.
- Use a canonical route under `/system-settings/operations/<section>`.
- Reuse the existing `RequestSnapshotSettingsSection` unchanged as the sole form/state owner.
- Remove Request Snapshot settings from the generic Log Maintenance section so the form is not duplicated.
- Keep all existing default values, option keys, validation, save/reset behavior, and Root-only System Settings boundary.
- Reuse the existing `Request Snapshots` i18n key unless implementation reveals a product-copy mismatch.

## Out of Scope

- Adding a Usage Logs shortcut.
- Changing snapshot capture/storage settings or backend option APIs.
- Creating a second management page or migrating option values.

## Acceptance Criteria

- [x] Operations navigation visibly contains a Request Snapshots item.
- [x] The item opens its own canonical Operations route and renders the existing settings form.
- [x] Log Maintenance no longer contains the Request Snapshot form.
- [x] Existing settings load/save/reset behavior is unchanged.
- [x] Focused registry/navigation tests, typecheck, lint/format, and build pass.
