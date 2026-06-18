# Project Plan — Home Inspection Checklist

A Go→WASM, GitHub-Pages-hosted web app for inspecting a home you're considering
buying — built in the style of `nulvox/fate4.tools` and `gasslands.tools` (Go
compiled to WASM, vanilla JS/CSS frontend, no server, localStorage + JSON).

This document is the handoff brief for the agent picking up implementation. The
**scaffold compiles and runs**; the content is fully authored; what's left is
polish and the items marked `TODO(next-agent)`.

> **Scope note:** built for evaluating *dirt-cheap / distressed / as-is* homes.
> Diligence skews toward title/liens/back-taxes/access and structural/systems
> deal-breakers, not lifestyle factors.

---

## 1. Settled design decisions

These were resolved in a grilling session. Don't relitigate without reason.

| # | Decision | Choice |
|---|----------|--------|
| 1 | Core model | **Stateful scored walkthrough** — tick items per property, capture notes/severity, get a report. |
| 2 | Output | **Deal-breakers + weighted concerns. NO single composite score** (a home isn't a number). |
| 3 | Phases cross-link | **Yes** — a flagged first-walk item auto-recommends a required inspection (e.g. marble/slope → structural engineer). |
| 4 | Photos | **Reserve `photoRef` field; v1 is text-notes only.** Photos = v2. |
| 5 | Report format | **Printable HTML, browser “Save as PDF”.** No PDF library. JSON export for raw data. ⚠️ v2 **supersedes the "self-contained single file" part** — photos are hydrated from IndexedDB into an ephemeral report and print-to-PDF is the portable artifact. See `adr/0002`. |
| 6 | Content source of truth | **`internal/checklist/checklist.json`, embedded into the Go binary** via `//go:embed`. Go owns scoring/escalation/report; JS only renders + captures. |
| 7 | Persistence | **localStorage** (single active inspection) + JSON import/export, like fate4.tools. |
| 8 | Disclaimer | **Blocking first-run modal (once/device) + persistent footer + reprinted on every report.** |
| 9 | License | **MIT** for the code; the in-app disclaimer governs reliance on the *output*. These don't conflict: MIT covers reuse of software, the disclaimer covers “no warranty on the advice.” |

## 2. The three phases (the content)

Authored in `internal/checklist/checklist.json`. The "unorthodox" DIY checks are
flagged `"unorthodox": true` and badged in the UI.

1. **Pre-Viewing Diligence** — desk research: title/liens/back-taxes, auction
   terms, permit history, **sex-offender registry (NSOPW)**, FEMA flood zone,
   EPA radon zone, listing/price history, **old-vs-new listing photos**, and a
   red-flag pre-brief (hazard panel brands, problem plumbing eras, reading
   system age off serial plates).
2. **First-Walk DIY Checks** (basic toolkit + DIY skills) — the unorthodox core:
   **marble on the floors → structural engineer trigger**, **IR-scope for damp /
   insulation**, **flush-everything-at-once** drainage stress test, **fridge-magnet
   pipe ID**, **outlet tester + GFCI trip**, **nose tests** (sewer gas, mould,
   oil), **fresh-paint-patch** spotting, **tile tapping**, **toilet rocking**,
   attic vermiculite, WDO screwdriver probe, etc.
3. **Inspections Required** — specialist commissions: structural engineer,
   electrician, **sewer scope**, **WDO/termite (NPMA-33)**, radon, mould, oil-tank
   sweep, asbestos/lead, roofer, chimney L2, well/septic, general home inspection.
   Items here light up automatically via escalation links.

Every item carries a `link` to a procedure/standard/how-to where a reliable one
exists (NSOPW, FEMA MSC, EPA, HUD NPMA-33, CPSC, InterNACHI, InspectAPedia).

> **TODO(next-agent):** spot-check the non-`.gov` deep links (InspectAPedia,
> InterNACHI) — slugs drift. The `.gov` links and homepages are stable.

## 3. Architecture

```
cmd/wasm/main.go        WASM entry; exposes globalThis.goInspect.{checklist,
                        computeReport,renderReport}. ALL logic is in Go.
internal/checklist/     Data model + embedded checklist.json (source of truth).
internal/scoring/       State -> Report (deal-breakers, concerns, escalations).
internal/report/        Report -> printable HTML (html/template, no deps).
web/                    index.html + css + app.js (render + capture only).
docs/                   Generated Pages output (make build).
```

Data flow: JS loads WASM → `goInspect.checklist()` returns the embedded JSON →
JS renders phases/items and captures `{severity, note}` per item in localStorage
→ on "Generate report", JS sends state JSON to `goInspect.renderReport()` → Go
computes + returns printable HTML → JS opens it for Save-as-PDF.

## 4. Build & run

```bash
make build     # GOOS=js GOARCH=wasm build + assemble docs/
make serve     # build + serve docs/ at http://localhost:8080
make test      # go test ./internal/...  (pure packages, host build)
```

CI (`.github/workflows/deploy.yml`) tests, builds, and deploys `docs/` to Pages
on push to `main`.

## 5. Current status

- ✅ Compiles: `internal/...` host build + `GOOS=js GOARCH=wasm` WASM build.
- ✅ Loads in-browser: disclaimer modal, phase tabs, all seeded content, per-item
  severity + notes, localStorage persistence, JSON import/export.
- ✅ Scoring + escalation: working baseline in `internal/scoring`.
- ✅ Printable report: minimal working template in `internal/report`.
- 🚧 Everything tagged `TODO(next-agent)` below.

## 6. Roadmap for the next agent

**Now (finish v1):**
1. Theme `web/` to match fate4.tools / gasslands.tools visual style.
2. Flesh out `internal/report` — per-phase tally table, group findings by phase,
   render notes and escalation reasons cleanly.
3. Refine `internal/scoring` weighting (see TODO in `scoring.go`): per-item
   weights, roll-up rules (e.g. several moderates in one system → a concern).
4. Add `go test` cases for `scoring.Compute` (deal-breaker surfacing, escalation
   dedup/merge) and validate `checklist.json` against the schema at test time.
5. Render the computed `Report` in-app (summary view) in addition to the
   printable popup.
6. Verify procedure links.

**v2 (scope settled 2026-06-18 — all four ship together; no v3 until the tool is
actually in use). Design decisions recorded in `adr/`; glossary in `CONTEXT.md`.**

1. **Multiple property Profiles** — clobber the v1 key, new `:v2` shape
   `{profiles:{id:{name,…,state}}, activeId}`. Name moves into the Profile.
   (`adr/0003`)
2. **Side-by-side comparison** — JS-only, N `computeReport` calls in columns.
   Aligned difference matrix, **no ranking / no composite score**. (`adr/0001`)
3. **Per-item Photos** — runtime state `photoIds:[]` (kill `photoRef` on the
   content `Item`); Blobs in IndexedDB; downscale-on-capture with a per-photo
   full-res opt-out; report hydrates `<img>` via object URLs; export carries
   base64. (`adr/0002`)
4. **PWA offline shell** — caching shim only (SW + manifest), relative paths for
   the Pages subpath, Makefile-stamped SW version for cache-busting. (`adr/0004`)

Forced order: Profiles first (schema spine), then Comparison (cheap on top),
Photos and PWA are the heavyweight cross-cutting pieces.

## 7. Disclaimer

See `DISCLAIMER.md`. It is enforced in three places (modal, footer, report).
**Personal use only; “as is”, no warranties; not a substitute for a licensed
professional inspection.** Keep it that way.
