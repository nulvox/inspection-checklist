# AGENT.md — instructions for the implementing agent

Read `PLAN.md` first; it holds the settled decisions and the roadmap. This file
is the quick operational brief.

## What this repo is
Go→WASM home-inspection checklist, GitHub-Pages hosted, no server. Same shape as
`nulvox/fate4.tools`: `cmd/wasm` entry, `internal/` logic, `web/` vanilla
frontend, `docs/` build output, Makefile, localStorage + JSON.

## Ground rules
- **All inspection logic lives in Go** (`internal/`). The frontend only renders
  content and captures input. Don't move scoring/escalation into JS.
- **`internal/checklist/checklist.json` is the single source of truth** for what
  gets inspected. Edit content there, not in the UI.
- **Keep it dependency-free**: pure Go + vanilla JS/CSS. The printable report
  uses `html/template`; do not pull in a PDF library.
- **Don't weaken the disclaimer.** Modal + footer + on-report. Personal use only,
  no warranties, not a substitute for a licensed inspection.
- Pick up items tagged `TODO(next-agent)` in code and in `PLAN.md §6`.

## Build / test
```bash
make build   # WASM + assemble docs/
make serve   # http://localhost:8080
make test    # go test ./internal/...
make vet
```
Requires Go 1.23+. `wasm_exec.js` is copied from the Go toolchain by `make assets`
(the Makefile handles both `lib/wasm` and `misc/wasm` locations).

## Conventions
- Severity enum lives in `internal/checklist` and is mirrored in `web/js/app.js`
  (`SEVERITIES`). Keep them in sync.
- Escalation: a first-walk item's `escalatesTo` points at an
  inspections-required item ID. Flagging it Concern/DealBreaker surfaces that
  inspection in the report. Keep IDs stable.
- Module path is `github.com/nulvox/inspection-checklist` — rename consistently
  if the repo name changes (it appears in import paths).
