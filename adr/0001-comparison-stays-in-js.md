# 0001 — Side-by-side comparison stays in JS

Status: Accepted (2026-06-18)

## Context

The ground rule (`AGENT.md`, PLAN.md #6) is "all inspection logic lives in Go;
the frontend only renders and captures." v2 adds a side-by-side comparison of
multiple **Profiles**.

Two options:
- A Go `compareProfiles(states)` returning an aligned difference matrix +
  per-column tallies + a `differs` flag. Keeps the ground rule literally true and
  makes "what counts as a difference" unit-testable.
- Pure JS: call the existing `computeReport` once per active profile and lay the
  results out in columns.

Comparison is mechanically just N reports placed in columns. Because every
Profile scores against the *same authored content*, item ids align perfectly —
the matrix is trivial. The Go option's only real benefit is preventing JS from
growing scoring logic over time.

## Decision

Comparison is **presentation, not scoring**, and lives in **JS**. JS calls
`computeReport` per active profile and renders an aligned table: rows = items/
categories, columns = profiles, cells = severity. No new Go function.

Crucially: **no ranking, no auto-sort, no composite number.** Column order =
insertion order. The only aggregates shown are the per-severity counts the
report already produces, displayed per column, never summed across severities.
This upholds PLAN.md decision #2 ("a home isn't a number").

## Consequences

- The "no ranking / no composite" rule is now a **convention enforced by code
  review and a comment**, not by a type. Nothing structurally prevents a future
  edit from sorting columns by deal-breaker count. Guard it in review.
- No WASM boundary cost; no multi-state marshalling.
- If comparison ever needs real cross-property logic (e.g. weighting shared
  defects), revisit and move it into Go — this ADR is the precedent to overturn.
