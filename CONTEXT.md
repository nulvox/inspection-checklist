# Context / Glossary

The shared vocabulary for the home-inspection checklist. This is a glossary, not a
spec — terms only, no implementation detail. ADRs (in `adr/`) record decisions.

> Note: `docs/` is the GitHub Pages **build output** (`make build` assembles it)
> and is overwritten on every build. Design docs live at the repo root, never in
> `docs/`.

## Authored content
The checklist of *what to inspect* — phases, categories, items, links, weights,
escalations. Authored in `internal/checklist/checklist.json`, embedded read-only
into the Go binary via `//go:embed`. The user cannot edit it; it ships with the
app. Contrast **Runtime state**.

## Runtime state
What the user *found* during a walkthrough: per item, a `{severity, note,
photoIds}` record. Captured in the browser, never compiled in. This is the only
thing that is per-property and persisted/exported. Photos are runtime state, not
authored content — which is why captured photos do not live on the content
`Item`.

## Profile
One named property's inspection: a name, optional address, timestamps, and its
**Runtime state**. New in v2. v1 had a single, unnamed, implicit inspection; v2
holds many Profiles keyed by id, with one `activeId`. The property name lives
inside the Profile (v1 kept it in a stray DOM input that was never persisted).

## Photo
A user-captured image attached to an item within a Profile. Stored as an opaque
`photoId` in **Runtime state**; the image bytes live in IndexedDB, not
localStorage. An item may have several (`photoIds` is an array).

## Comparison
A side-by-side view of several **Profiles**: an aligned table, rows = items,
columns = profiles, cells = severity, surfacing where properties *differ*. It is
explicitly **not a ranking** — no sort-by-goodness, no composite number; column
order is insertion order. Pure presentation, rendered in JS from per-profile
reports (`adr/0001`). Upholds the "a home isn't a number" rule.

## Severity
The status a user assigns to an item: unchecked, na, ok, minor, moderate,
concern, dealbreaker. Drives scoring/escalation. Defined in
`internal/checklist`, mirrored in the frontend.

## Escalation
A first-walk item flagged Concern-or-worse surfaces a recommended
inspections-required item (via `escalatesTo`). Owned by Go scoring.
