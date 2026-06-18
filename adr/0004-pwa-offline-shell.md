# 0004 — PWA: an offline caching shim, nothing more

Status: Accepted (2026-06-18)

## Context

The tool is used at a property, often with no signal. It is already a set of
static files (html/css/js/wasm) over localStorage + IndexedDB — there is no
server or API. The *only* reason it fails offline is that the browser must fetch
the static shell. So a PWA here is purely a caching shim, not an app-platform
play.

The complication is hosting: GitHub Pages serves from the subpath
`https://<user>.github.io/inspection-checklist/`, which breaks the two classic
PWA footguns — service-worker scope and manifest/icon paths.

## Decision

- **Scope: caching shim only.** A service worker + manifest. **No** install
  prompts, push, or background sync.
- **Everything is path-relative** to survive the subpath:
  - register with a relative URL (`navigator.serviceWorker.register('sw.js')`),
    never `/sw.js` (root-scoped, 404s on Pages);
  - manifest `start_url` and `scope` = `./`, icons relative.
- **Precache the shell exactly:** `index.html`, `css/style.css`, `js/app.js`,
  `js/wasm_exec.js`, `inspect.wasm`, the manifest, and icons. **Not** user data —
  localStorage and IndexedDB already work offline.
- **Cache-first for the shell.** The shell is static; no network-first logic.
- **Cache-busting is owned by the Makefile, not a human.** `make build` stamps a
  version (git short-sha or a content hash) into `sw.js`. Each deploy ships a
  byte-different SW → browser runs `activate` → old cache purged. This is the
  piece most likely to be done wrong by hand; the build owns it so `inspect.wasm`
  can never go stale silently.

## Consequences

- Adds `sw.js`, `manifest.webmanifest`, icons, and a version-stamp step in the
  Makefile + `docs/` assembly.
- Forgetting the version stamp = users pinned to a stale wasm; the stamp must be
  part of `make build`, asserted, not optional.
- The subpath assumptions break if the app is ever served from a root domain;
  relative paths mostly survive that, the SW registration certainly does.
