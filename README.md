# Home Inspection Checklist

A client-side web app for inspecting a home you're thinking about buying —
especially **cheap, distressed, or as-is** properties. Go compiled to WebAssembly,
vanilla JS/CSS frontend, hosted on GitHub Pages. No server, no accounts, runs
entirely in your browser. Built in the style of
[fate4.tools](https://github.com/nulvox/fate4.tools) and gasslands.tools.

> ⚠️ **Personal use only. Provided “as is”, no warranties. Not a substitute for a
> licensed professional inspection.** See [`DISCLAIMER.md`](DISCLAIMER.md).

## What it does

Walks you through three phases and turns your findings into a decision:

1. **Pre-Viewing Diligence** — title/liens/back-taxes, auction terms, permit
   history, sex-offender registry, flood & radon zones, listing history.
2. **First-Walk DIY Checks** — what you can check yourself with a basic toolkit:
   roll a marble to find a sloping floor, IR-scope for hidden damp, flush
   everything at once to stress the drains, magnet-test the pipes, trip every
   GFCI, and trust your nose for sewer gas and mould.
3. **Inspections Required** — the specialist commissions (sewer scope, WDO/termite
   NPMA-33, structural engineer, …), which light up automatically when a DIY
   finding warrants them.

Output is **deal-breakers first**, then weighted concerns, then the inspections
your findings escalate to — printable to PDF and exportable as JSON. No fake
“87/100” score: a house isn't a number.

## Multiple properties, photos, offline

- **Property profiles** — inspect several houses, switch between them, and keep
  each one's findings separate. Saved in your browser (localStorage).
- **Side-by-side comparison** — line properties up in columns to see where they
  differ. Deliberately **no ranking and no combined score** — it surfaces
  differences, it doesn't crown a winner.
- **Per-item photos** — attach as many photos as you like to any finding;
  they're downscaled by default (with a full-res opt-out) and stored locally in
  IndexedDB. They appear in the printable report.
- **Works offline** — it's a PWA: load it once and it runs at the property with
  no signal. Your data never leaves the device.

> The generated report is a live page, not a saved file — use your browser's
> **Print → Save as PDF** to keep a copy (that's also what bundles the photos in).
> Move data between devices with **Export** / **Import**.

## Quick start

```bash
make serve   # build + http://localhost:8080
```

Requires Go 1.23+. See [`PLAN.md`](PLAN.md) for the design and [`AGENT.md`](AGENT.md)
for the build brief. Deploys to GitHub Pages via `.github/workflows/deploy.yml`.

## Status

Full checklist content authored; scoring/escalation/report working; v2 shipped —
property profiles, side-by-side comparison, per-item photos, and offline PWA
install. Design decisions are recorded in [`adr/`](adr/) with a glossary in
[`CONTEXT.md`](CONTEXT.md). MIT licensed.
