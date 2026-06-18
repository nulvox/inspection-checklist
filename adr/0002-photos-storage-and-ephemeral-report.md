# 0002 — Photo storage, and the report is no longer self-contained

Status: Accepted (2026-06-18) — supersedes PLAN.md decision #5

## Context

v2 adds per-item **Photos**. Phone photos are multi-MB. PLAN.md decision #5 said
the report is a "self-contained, offline, single printable HTML file," which
forces photos to embed as base64 data URIs — inflating reports to tens of MB and
shoving image bytes across the WASM boundary into Go's `report.Render` and back.

The user reframed: **print-to-PDF is the shareable artifact**, so the report HTML
does not itself need to be a saveable self-contained file.

## Decision

1. **Photos are runtime state, not authored content.** The reserved `photoRef`
   field on `checklist.Item` is **removed**. Per-item runtime state becomes
   `{severity, note, photoIds: []}` (an item may have several photos).
2. **Storage split:** localStorage holds the whole Profile tree (incl.
   `photoIds`); **IndexedDB holds the image Blobs**, keyed by `photoId`.
   localStorage is ~5MB and synchronous — unfit for image bytes.
3. **Report is hydrated, not embedded.** Go renders `<img data-photo-id>`
   placeholders; JS sets `img.src = URL.createObjectURL(blob)` from IndexedDB in
   the opened report window. **Go never touches image bytes.** Print → Save as
   PDF flattens the rendered images into the portable artifact.
4. **Downscale at capture** (~1280px JPEG, ~150–300KB) by default, with a
   **per-photo "keep full resolution" opt-out** for the shots that matter.
5. **Export carries photos.** Since the report is now ephemeral, the export is
   the only portability path. It is a **`.zip` written by hand with STORED
   (uncompressed) entries** — no zip library, staying dependency-free; JPEGs
   don't compress further anyway. Layout: `inspection.json`
   (`{schema, exportedAt, profiles:{id:{...,state}}, photos:{id:{mime,fullRes,file}}}`)
   plus `photos/<id>.<ext>` raw bytes. This avoids base64's ~33% bloat and keeps
   photos browsable as real files. Import **merges by id** (adds new, updates
   existing — never nukes) and still accepts the earlier base64-in-JSON export
   and legacy `{property,state}` / bare-map JSON.

## Consequences

- The report HTML is **session-bound**: object URLs die with the document.
  **File → Save Page As yields broken images** — only Print → PDF preserves a
  copy. The report UI must tell the user to print-to-PDF to keep it.
- Print fidelity (margins, page breaks across photos) is now the browser's job.
- Photo-heavy full-res exports can be several MB — accepted; downscaling and the
  per-photo opt-out keep the common case bearable.
- `scoring.State` / `Compute` are **unchanged**: the `state` sub-map stays
  byte-identical; photos live in a sibling map, never inline.
