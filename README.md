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

## Quick start

```bash
make serve   # build + http://localhost:8080
```

Requires Go 1.23+. See [`PLAN.md`](PLAN.md) for the design and [`AGENT.md`](AGENT.md)
for the build brief. Deploys to GitHub Pages via `.github/workflows/deploy.yml`.

## Status

Scaffold + full checklist content authored; core scoring/escalation/report
working. Polish and v2 (photos, multi-property compare) are tracked in
[`PLAN.md`](PLAN.md). MIT licensed.
