# CLAUDE.md

Guidance for Claude Code working in this repo. See `AGENT.md` and `PLAN.md` for
the full brief.

## Project
Go→WASM home-inspection checklist for evaluating cheap/distressed homes. Hosted
on GitHub Pages, no server. Style matches `nulvox/fate4.tools`.

## Key facts
- Logic in Go (`internal/`), rendering in vanilla JS (`web/`).
- Content authored in `internal/checklist/checklist.json`, embedded via `//go:embed`.
- Three phases: pre-viewing diligence → first-walk DIY → inspections required.
- Output = deal-breakers + weighted concerns + escalated inspection recommendations.
  No composite score by design.
- Persistence: localStorage + JSON import/export. Report: printable HTML.

## Commands
- `make build` — compile WASM, assemble `docs/`.
- `make serve` — local server on :8080.
- `make test` — `go test ./internal/...`.

## Don'ts
- Don't add runtime dependencies (keep it pure Go + vanilla).
- Don't move scoring/escalation logic into JS.
- Don't soften the disclaimer (modal + footer + report; personal use, no warranty,
  not a substitute for a licensed inspection).
