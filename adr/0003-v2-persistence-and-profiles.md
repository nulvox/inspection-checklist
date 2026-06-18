# 0003 — v2 persistence schema: many named Profiles

Status: Accepted (2026-06-18)

## Context

v1 persists a **single, unnamed** inspection at localStorage key
`inspection-checklist:v1` as a bare item-map `{itemId:{severity,note}}`. The
property name lived in a DOM input that was never persisted. v2 needs many named
**Profiles** with a side-by-side comparison.

Going from a bare map to a `{profiles, activeId}` object changes the top-level
*type*, so an in-place version field is awkward (a bare map has no safe place for
a `schemaVersion` sibling). Normally this calls for a versioned key + lazy
migration. **But there are no real users**, so backward-compat is not required.

## Decision

- **Clobber.** No migration. Adopt a new key `inspection-checklist:v2` and define
  the shape clean:
  ```
  { profiles: { "<id>": { name, address?, createdAt, updatedAt,
                          state: { itemId: {severity, note, photoIds:[]} } } },
    activeId: "<id>" }
  ```
- The **property name moves into the Profile** and is persisted, fixing the v1
  quirk where it was a non-persisted DOM input.
- The `state` sub-map remains exactly what `scoring.Compute` consumes — Go is
  untouched. JS reframes load/save as "load/save the active Profile's state."
- Image Blobs live in IndexedDB (see ADR 0002), not in this tree.

## Consequences

- Any v1 localStorage value is ignored (acceptably — no users).
- One key holds all profiles; load is one synchronous read. Photos load lazily
  from IndexedDB by `photoId`.
- Export/import is a full snapshot across all profiles (see ADR 0002), doubling
  as backup, device-transfer, and property-sharing.
