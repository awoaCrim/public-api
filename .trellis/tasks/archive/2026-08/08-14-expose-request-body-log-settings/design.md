# Design: Dedicated Request Snapshot Settings Entry

## Registry ownership

`web/src/features/system-settings/operations/section-registry.tsx` already drives all three relevant contracts from one source:

- valid section IDs for the dynamic route;
- sidebar navigation items;
- section title and rendered content.

Add one `request-snapshots` registry entry with `titleKey: 'Request Snapshots'` and a build function that renders the existing `RequestSnapshotSettingsSection` with the same defaults currently passed inside `logs`.

Change the `logs` entry to render only `LogSettingsSection`. No new route file is required because `$section.tsx` validates against `OPERATIONS_SECTION_IDS`.

## Data and behavior

No option keys, defaults, API calls, form schema, or persistence behavior change. The settings component remains the single owner, merely reached through a clearer route.

## Tests

Add a focused Operations registry/navigation contract test that verifies:

- `request-snapshots` is a valid section ID;
- its generated URL is `/system-settings/operations/request-snapshots`;
- Log Maintenance and Request Snapshots remain distinct registry entries;
- the dedicated section resolves to the existing Request Snapshot form contract.

Avoid brittle full DOM/class snapshots.
