// app.js — frontend glue. Boots the Go/WASM module, renders the authored
// checklist content, captures per-item state, and asks Go to compute the report
// / render the printable HTML.
//
// Division of labour: ALL inspection logic (scoring, escalation, report HTML)
// lives in Go. This file only renders content, captures input, and lays things
// out. The one piece of "logic" here is presentation-only: the comparison view
// (adr/0001) is N computeReport calls placed side by side — deliberately no
// ranking, no composite score, columns in insertion order.
//
// v2 storage (adr/0003): many named Profiles under one key. Image bytes live in
// IndexedDB (adr/0002); localStorage holds only profile metadata + photo ids.

const STORAGE_KEY = "inspection-checklist:v2";
const DISCLAIMER_KEY = "inspection-checklist:disclaimer-accepted";
const THEME_KEY = "inspection-checklist:theme";
const EXPORT_SCHEMA = "inspection-checklist/v2";
const SEVERITIES = ["unchecked", "na", "ok", "minor", "moderate", "concern", "dealbreaker"];
const MAX_PHOTO_PX = 1280; // longest edge for downscaled captures (adr/0002)
const PHOTO_QUALITY = 0.82;

let checklist = null;   // authored content (from Go)
let db = loadDB();      // { profiles: {id: Profile}, activeId }
let activePhase = null;
let fullResNext = false; // per-capture "keep full resolution" opt-out

// ---- boot ---------------------------------------------------------------

async function boot() {
  initTheme();
  showDisclaimer();
  const go = new Go(); // from wasm_exec.js
  const resp = await fetch("inspect.wasm");
  const { instance } = await WebAssembly.instantiateStreaming(resp, go.importObject);
  go.run(instance); // registers globalThis.goInspect, then blocks (select{})

  checklist = JSON.parse(goInspect.checklist());
  activePhase = checklist.phases[0].id;
  renderProfiles();
  renderTabs();
  renderPhase();
  wireToolbar();
}

window.addEventListener("DOMContentLoaded", boot);

// ---- theme --------------------------------------------------------------
// The page follows the OS theme by default. The toggle sets an explicit
// data-theme override (persisted) that wins over the OS preference.

function currentTheme() {
  return document.documentElement.dataset.theme ||
    (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
}
function applyThemeButton() {
  const b = document.getElementById("btn-theme");
  if (b) b.textContent = currentTheme() === "dark" ? "☀ Light" : "🌙 Dark";
}
function initTheme() {
  const saved = localStorage.getItem(THEME_KEY);
  if (saved) document.documentElement.dataset.theme = saved;
  applyThemeButton();
}
function toggleTheme() {
  const next = currentTheme() === "dark" ? "light" : "dark";
  document.documentElement.dataset.theme = next;
  localStorage.setItem(THEME_KEY, next);
  applyThemeButton();
}

// ---- disclaimer ---------------------------------------------------------

function showDisclaimer() {
  const modal = document.getElementById("disclaimer-modal");
  if (localStorage.getItem(DISCLAIMER_KEY) === "yes") return;
  modal.hidden = false;
  document.getElementById("disclaimer-accept").addEventListener("click", () => {
    localStorage.setItem(DISCLAIMER_KEY, "yes");
    modal.hidden = true;
  });
}

// ---- profiles + persistence ---------------------------------------------
// db = { profiles: { id: { name, address, createdAt, updatedAt, state } }, activeId }
// state = { itemId: { severity, note, photoIds:[] } } — the state sub-map is
// exactly what goInspect.computeReport / renderReport consume; Go never sees the
// profile envelope.

function newId() {
  return (crypto.randomUUID && crypto.randomUUID()) ||
    "p-" + Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
}

function freshProfile(name) {
  const now = new Date().toISOString();
  return { name: name || "Unlabeled property", address: "", createdAt: now, updatedAt: now, state: {} };
}

function freshDB() {
  const id = newId();
  return { profiles: { [id]: freshProfile() }, activeId: id };
}

function loadDB() {
  try {
    const raw = JSON.parse(localStorage.getItem(STORAGE_KEY));
    if (raw && raw.profiles && raw.activeId && raw.profiles[raw.activeId]) return raw;
  } catch { /* fall through to a fresh db */ }
  return freshDB();
}
function saveDB() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(db));
}

function activeProfile() { return db.profiles[db.activeId]; }
function activeState() { return activeProfile().state; }

function touchProfile() {
  activeProfile().updatedAt = new Date().toISOString();
}

function setItem(id, patch) {
  const st = activeState();
  st[id] = Object.assign({ severity: "unchecked", note: "", photoIds: [] }, st[id], patch);
  touchProfile();
  saveDB();
}

function switchProfile(id) {
  if (!db.profiles[id]) return;
  db.activeId = id;
  saveDB();
  renderProfiles();
  renderPhase();
  hidePanels();
}

function addProfile() {
  const name = (prompt("Name this property (address or label):", "") || "").trim();
  const id = newId();
  db.profiles[id] = freshProfile(name);
  db.activeId = id;
  saveDB();
  renderProfiles();
  renderPhase();
  hidePanels();
}

async function deleteProfile() {
  const ids = Object.keys(db.profiles);
  if (ids.length === 1) { alert("Can't delete the only property. Create another first."); return; }
  const p = activeProfile();
  if (!confirm(`Delete "${p.name}" and all its findings and photos? This cannot be undone.`)) return;
  await deletePhotos(photoIdsOf(p.state));
  delete db.profiles[db.activeId];
  db.activeId = Object.keys(db.profiles)[0];
  saveDB();
  renderProfiles();
  renderPhase();
  hidePanels();
}

function renameActive(name) {
  activeProfile().name = name;
  touchProfile();
  saveDB();
  renderProfileSelect();
}

function renderProfiles() {
  renderProfileSelect();
  const nameInput = document.getElementById("profile-name");
  if (nameInput) nameInput.value = activeProfile().name;
}

function renderProfileSelect() {
  const sel = document.getElementById("profile-select");
  if (!sel) return;
  sel.innerHTML = "";
  for (const [id, p] of Object.entries(db.profiles)) {
    const o = document.createElement("option");
    o.value = id;
    o.textContent = p.name || "Unlabeled property";
    if (id === db.activeId) o.selected = true;
    sel.appendChild(o);
  }
}

// ---- IndexedDB photo store ----------------------------------------------
// One object store: photos keyed by id -> { blob, mime, fullRes, createdAt }.
// Only image bytes live here; everything else is in localStorage.

const IDB_NAME = "inspection-checklist";
const IDB_STORE = "photos";
let _idb = null;

function openIDB() {
  if (_idb) return Promise.resolve(_idb);
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(IDB_NAME, 1);
    req.onupgradeneeded = () => req.result.createObjectStore(IDB_STORE);
    req.onsuccess = () => { _idb = req.result; resolve(_idb); };
    req.onerror = () => reject(req.error);
  });
}

function idbTx(mode, fn) {
  return openIDB().then(dbi => new Promise((resolve, reject) => {
    const tx = dbi.transaction(IDB_STORE, mode);
    const store = tx.objectStore(IDB_STORE);
    const out = fn(store);
    tx.oncomplete = () => resolve(out && out.result !== undefined ? out.result : out);
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);
  }));
}

function putPhoto(id, record) { return idbTx("readwrite", s => s.put(record, id)); }
function getPhotoRecord(id) {
  return openIDB().then(dbi => new Promise((resolve, reject) => {
    const tx = dbi.transaction(IDB_STORE, "readonly");
    const req = tx.objectStore(IDB_STORE).get(id);
    req.onsuccess = () => resolve(req.result || null);
    req.onerror = () => reject(req.error);
  }));
}
function deletePhotos(ids) {
  if (!ids.length) return Promise.resolve();
  return openIDB().then(dbi => new Promise((resolve, reject) => {
    const tx = dbi.transaction(IDB_STORE, "readwrite");
    const store = tx.objectStore(IDB_STORE);
    ids.forEach(id => store.delete(id));
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
  }));
}

function photoIdsOf(state) {
  const out = [];
  for (const v of Object.values(state || {})) {
    if (v && Array.isArray(v.photoIds)) out.push(...v.photoIds);
  }
  return out;
}

// ---- photo capture + downscale ------------------------------------------

// downscale draws the image onto a canvas capped at MAX_PHOTO_PX and returns a
// JPEG Blob. Full-res captures skip this and store the original bytes.
function processImage(file) {
  if (fullResNext) return Promise.resolve({ blob: file, mime: file.type || "image/jpeg", fullRes: true });
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => {
      const scale = Math.min(1, MAX_PHOTO_PX / Math.max(img.width, img.height));
      const w = Math.round(img.width * scale), h = Math.round(img.height * scale);
      const canvas = document.createElement("canvas");
      canvas.width = w; canvas.height = h;
      canvas.getContext("2d").drawImage(img, 0, 0, w, h);
      URL.revokeObjectURL(img.src);
      canvas.toBlob(
        b => b ? resolve({ blob: b, mime: "image/jpeg", fullRes: false })
               : reject(new Error("canvas encode failed")),
        "image/jpeg", PHOTO_QUALITY);
    };
    img.onerror = () => reject(new Error("could not decode image"));
    img.src = URL.createObjectURL(file);
  });
}

async function addPhotos(itemId, files) {
  const st = activeState();
  const cur = (st[itemId] && st[itemId].photoIds) || [];
  const ids = cur.slice();
  for (const file of files) {
    if (!file.type.startsWith("image/")) continue;
    try {
      const { blob, mime, fullRes } = await processImage(file);
      const id = newId();
      await putPhoto(id, { blob, mime, fullRes, createdAt: new Date().toISOString() });
      ids.push(id);
    } catch (e) { console.error("photo capture failed", e); }
  }
  setItem(itemId, { photoIds: ids });
  rerenderItem(itemId);
}

async function removePhoto(itemId, photoId) {
  const st = activeState();
  const ids = ((st[itemId] && st[itemId].photoIds) || []).filter(x => x !== photoId);
  setItem(itemId, { photoIds: ids });
  await deletePhotos([photoId]);
  rerenderItem(itemId);
}

// ---- rendering ----------------------------------------------------------

function renderTabs() {
  const nav = document.getElementById("phase-tabs");
  nav.innerHTML = "";
  for (const phase of checklist.phases) {
    const b = document.createElement("button");
    b.textContent = phase.name;
    b.setAttribute("aria-selected", String(phase.id === activePhase));
    b.addEventListener("click", () => { activePhase = phase.id; renderTabs(); renderPhase(); });
    nav.appendChild(b);
  }
}

function renderPhase() {
  const root = document.getElementById("phases");
  const phase = checklist.phases.find(p => p.id === activePhase);
  root.innerHTML = "";

  const blurb = document.createElement("p");
  blurb.className = "phase-blurb";
  blurb.textContent = phase.blurb;
  root.appendChild(blurb);

  for (const cat of phase.categories) {
    const section = document.createElement("section");
    section.className = "category";
    const h = document.createElement("h3");
    h.textContent = cat.name;
    section.appendChild(h);
    for (const item of cat.items) section.appendChild(renderItem(item));
    root.appendChild(section);
  }
}

function rerenderItem(itemId) {
  const old = document.querySelector(`.item[data-item-id="${CSS.escape(itemId)}"]`);
  if (!old) return;
  const item = findItem(itemId);
  if (item) old.replaceWith(renderItem(item));
}

function findItem(itemId) {
  for (const p of checklist.phases)
    for (const c of p.categories)
      for (const it of c.items) if (it.id === itemId) return it;
  return null;
}

function renderItem(item) {
  const st = activeState()[item.id] || {};
  const el = document.createElement("div");
  el.className = "item";
  el.dataset.itemId = item.id;
  el.dataset.sev = st.severity || "unchecked";

  const title = document.createElement("div");
  title.className = "item-title";
  title.textContent = item.title;
  el.appendChild(title);

  if (item.detail) {
    const d = document.createElement("div");
    d.className = "item-detail";
    d.textContent = item.detail;
    el.appendChild(d);
  }

  const meta = document.createElement("div");
  meta.className = "item-meta";

  // severity selector
  const sel = document.createElement("select");
  const cur = st.severity || "unchecked";
  for (const s of SEVERITIES) {
    const o = document.createElement("option");
    o.value = s; o.textContent = s;
    if (s === cur) o.selected = true;
    sel.appendChild(o);
  }
  sel.dataset.sev = cur;
  sel.addEventListener("change", () => {
    sel.dataset.sev = sel.value;
    el.dataset.sev = sel.value;
    setItem(item.id, { severity: sel.value });
  });
  meta.appendChild(sel);

  if (item.unorthodox) meta.appendChild(badge("DIY trick", "diy"));
  if (item.tool) meta.appendChild(badge("🧰 " + item.tool, "tool"));
  if (item.escalatesTo) meta.appendChild(badge("↑ escalates", "escalates"));
  if (item.link && item.link.url) {
    const a = document.createElement("a");
    a.href = item.link.url; a.target = "_blank"; a.rel = "noopener";
    a.textContent = item.link.label || "procedure";
    meta.appendChild(a);
  }
  el.appendChild(meta);

  // note
  const note = document.createElement("textarea");
  note.placeholder = "Notes…";
  note.value = st.note || "";
  note.addEventListener("input", () => setItem(item.id, { note: note.value }));
  el.appendChild(note);

  el.appendChild(renderPhotos(item.id, st.photoIds || []));
  return el;
}

function renderPhotos(itemId, photoIds) {
  const wrap = document.createElement("div");
  wrap.className = "item-photos";

  const bar = document.createElement("div");
  bar.className = "photo-bar";

  const add = document.createElement("button");
  add.type = "button";
  add.className = "photo-add";
  add.textContent = "📷 Add photo";
  const input = document.createElement("input");
  input.type = "file";
  input.accept = "image/*";
  input.capture = "environment";
  input.multiple = true;
  input.hidden = true;
  add.addEventListener("click", () => { fullResNext = fullEl.checked; input.click(); });
  input.addEventListener("change", () => {
    if (input.files.length) addPhotos(itemId, Array.from(input.files));
    input.value = "";
  });

  const fullLabel = document.createElement("label");
  fullLabel.className = "photo-fullres";
  const fullEl = document.createElement("input");
  fullEl.type = "checkbox";
  fullLabel.appendChild(fullEl);
  fullLabel.appendChild(document.createTextNode(" full-res"));
  fullLabel.title = "Store the next photo at full resolution (larger). Default downscales to ~1280px.";

  bar.appendChild(add);
  bar.appendChild(input);
  bar.appendChild(fullLabel);
  wrap.appendChild(bar);

  if (photoIds.length) {
    const grid = document.createElement("div");
    grid.className = "photo-grid";
    for (const id of photoIds) grid.appendChild(thumb(itemId, id));
    wrap.appendChild(grid);
  }
  return wrap;
}

function thumb(itemId, photoId) {
  const fig = document.createElement("div");
  fig.className = "thumb";
  const img = document.createElement("img");
  img.alt = "finding photo";
  getPhotoRecord(photoId).then(rec => {
    if (!rec) return;
    const url = URL.createObjectURL(rec.blob);
    img.src = url;
    img.addEventListener("load", () => URL.revokeObjectURL(url), { once: true });
    if (rec.fullRes) fig.appendChild(badge("full-res", "tool"));
  });
  const del = document.createElement("button");
  del.type = "button";
  del.className = "thumb-del";
  del.textContent = "✕";
  del.title = "Remove photo";
  del.addEventListener("click", () => removePhoto(itemId, photoId));
  fig.appendChild(img);
  fig.appendChild(del);
  return fig;
}

function badge(text, cls) {
  const b = document.createElement("span");
  b.className = "badge " + cls;
  b.textContent = text;
  return b;
}

// ---- toolbar actions ----------------------------------------------------

function wireToolbar() {
  document.getElementById("profile-select").addEventListener("change", e => switchProfile(e.target.value));
  document.getElementById("profile-name").addEventListener("input", e => renameActive(e.target.value));
  document.getElementById("btn-new-profile").addEventListener("click", addProfile);
  document.getElementById("btn-del-profile").addEventListener("click", deleteProfile);

  document.getElementById("btn-summary").addEventListener("click", toggleSummary);
  document.getElementById("btn-compare").addEventListener("click", toggleCompare);
  document.getElementById("btn-report").addEventListener("click", openReport);
  document.getElementById("btn-export").addEventListener("click", exportJSON);
  document.getElementById("btn-import").addEventListener("click", () =>
    document.getElementById("import-file").click());
  document.getElementById("import-file").addEventListener("change", importJSON);
  document.getElementById("btn-reset").addEventListener("click", resetActive);
  document.getElementById("btn-theme").addEventListener("click", toggleTheme);
}

function hidePanels() {
  document.getElementById("summary").hidden = true;
  document.getElementById("compare").hidden = true;
}

function reportMeta() {
  return JSON.stringify({ Property: activeProfile().name || "Unlabeled property" });
}

// openReport renders the printable HTML in Go, then hydrates the src-less
// <img data-photo-id> placeholders from IndexedDB before the user prints to PDF
// (adr/0002 — the report is ephemeral; Save-as-PDF is the keepable artifact).
async function openReport() {
  const html = goInspect.renderReport(JSON.stringify(activeState()), reportMeta());
  const w = window.open("", "_blank");
  if (!w) { alert("Pop-up blocked — allow pop-ups to open the report."); return; }
  w.document.write(html);
  w.document.close();
  const imgs = w.document.querySelectorAll("img[data-photo-id]");
  for (const img of imgs) {
    const rec = await getPhotoRecord(img.getAttribute("data-photo-id"));
    if (rec) img.src = URL.createObjectURL(rec.blob);
  }
}

// ---- in-app summary -----------------------------------------------------
// All scoring/escalation/roll-up logic stays in Go: we ask goInspect to
// compute the report and only render the returned JSON here.

const SEV_LABELS = {
  dealbreaker: "Deal-breaker", concern: "Concern",
  moderate: "Moderate", minor: "Minor",
};
const TALLY_COLS = ["dealbreaker", "concern", "moderate", "minor"];

function toggleSummary() {
  const panel = document.getElementById("summary");
  if (!panel.hidden) { panel.hidden = true; return; }
  document.getElementById("compare").hidden = true;
  renderSummary(panel);
  panel.hidden = false;
  panel.scrollIntoView({ behavior: "smooth", block: "start" });
}

function renderSummary(panel) {
  const report = JSON.parse(goInspect.computeReport(JSON.stringify(activeState())));
  panel.innerHTML = "";

  const head = el("div", "summary-head");
  head.appendChild(el("h2", null, "Summary — " + activeProfile().name));
  const close = el("button", "summary-close", "✕ Close");
  close.type = "button";
  close.setAttribute("aria-label", "Close summary");
  close.addEventListener("click", () => { panel.hidden = true; });
  head.appendChild(close);
  panel.appendChild(head);

  panel.appendChild(countsRow(report));

  if ((report.dealBreakers || []).length) {
    panel.appendChild(findingGroup("⛔ Deal-breakers", report.dealBreakers));
  } else {
    panel.appendChild(el("p", "summary-none", "✓ No deal-breakers flagged."));
  }

  if ((report.rollUps || []).length) {
    const sec = el("section", "summary-section");
    sec.appendChild(el("h3", null, "▲ Concern clusters"));
    for (const ru of report.rollUps) {
      const d = el("div", "summary-finding concern");
      d.appendChild(el("strong", null, ru.categoryName));
      d.appendChild(el("p", "summary-sub",
        "Moderate findings: " + (ru.contributing || []).join("; ")));
      sec.appendChild(d);
    }
    panel.appendChild(sec);
  }

  if ((report.recommendations || []).length) {
    const sec = el("section", "summary-section");
    sec.appendChild(el("h3", null, "🔍 Inspections to commission"));
    for (const rec of report.recommendations) {
      const d = el("div", "summary-finding rec");
      d.appendChild(el("strong", null, rec.item.title));
      if ((rec.triggeredBy || []).length) {
        d.appendChild(el("p", "summary-sub",
          "Triggered by: " + rec.triggeredBy.join("; ")));
      }
      sec.appendChild(d);
    }
    panel.appendChild(sec);
  }

  for (const sev of ["concern", "moderate", "minor"]) {
    const list = report[sev === "concern" ? "concerns" : sev] || [];
    if (list.length) panel.appendChild(findingGroup(SEV_LABELS[sev] + "s", list));
  }

  panel.appendChild(tallyTable(report.phases || []));
}

function countsRow(report) {
  const row = el("div", "summary-counts");
  const counts = {
    dealbreaker: (report.dealBreakers || []).length,
    concern: (report.concerns || []).length,
    moderate: (report.moderate || []).length,
    minor: (report.minor || []).length,
  };
  for (const sev of TALLY_COLS) {
    const chip = el("div", "summary-chip " + sev);
    chip.appendChild(el("span", "summary-chip-n", String(counts[sev])));
    chip.appendChild(el("span", "summary-chip-l", SEV_LABELS[sev]));
    row.appendChild(chip);
  }
  return row;
}

function findingGroup(heading, findings) {
  const sec = el("section", "summary-section");
  sec.appendChild(el("h3", null, heading));
  for (const f of findings) {
    const d = el("div", "summary-finding " + f.severity);
    d.appendChild(el("strong", null, f.title));
    if (f.note) d.appendChild(el("p", "summary-note", f.note));
    sec.appendChild(d);
  }
  return sec;
}

function tallyTable(phases) {
  const sec = el("section", "summary-section");
  sec.appendChild(el("h3", null, "By phase"));
  const table = el("table", "summary-table");
  const thead = el("tr");
  thead.appendChild(el("th", null, "Phase"));
  for (const sev of TALLY_COLS) thead.appendChild(el("th", "num", SEV_LABELS[sev]));
  table.appendChild(thead);
  for (const p of phases) {
    const tr = el("tr");
    tr.appendChild(el("td", null, p.name));
    for (const sev of TALLY_COLS) {
      tr.appendChild(el("td", "num", String((p.counts && p.counts[sev]) || 0)));
    }
    table.appendChild(tr);
  }
  sec.appendChild(table);
  return sec;
}

// ---- comparison ---------------------------------------------------------
// Presentation only (adr/0001): one computeReport per profile, laid out in
// columns IN INSERTION ORDER. No ranking, no sort control, no composite number —
// the only aggregates are the per-severity counts Go already produces. The
// difference matrix highlights rows where the properties disagree.

function toggleCompare() {
  const panel = document.getElementById("compare");
  if (!panel.hidden) { panel.hidden = true; return; }
  document.getElementById("summary").hidden = true;
  renderCompare(panel);
  panel.hidden = false;
  panel.scrollIntoView({ behavior: "smooth", block: "start" });
}

function renderCompare(panel) {
  panel.innerHTML = "";
  const head = el("div", "summary-head");
  head.appendChild(el("h2", null, "Compare properties"));
  const close = el("button", "summary-close", "✕ Close");
  close.type = "button";
  close.addEventListener("click", () => { panel.hidden = true; });
  head.appendChild(close);
  panel.appendChild(head);

  const cols = Object.entries(db.profiles).map(([id, p]) => ({
    id, name: p.name || "Unlabeled property",
    rep: JSON.parse(goInspect.computeReport(JSON.stringify(p.state))),
  }));

  if (cols.length < 2) {
    panel.appendChild(el("p", "summary-sub",
      "Add a second property (＋ New) to compare. Comparison surfaces where properties differ — it does not rank them."));
    return;
  }

  panel.appendChild(el("p", "summary-sub",
    "Side by side, in the order you added them. No score, no ranking — a home isn't a number."));

  // Counts table: one row per severity bucket, one column per property.
  const counts = el("table", "summary-table compare-table");
  const hr = el("tr");
  hr.appendChild(el("th", null, ""));
  for (const c of cols) hr.appendChild(el("th", null, c.name));
  counts.appendChild(hr);
  const rows = [
    ["Deal-breakers", r => (r.dealBreakers || []).length, "dealbreaker"],
    ["Concerns", r => (r.concerns || []).length, "concern"],
    ["Moderate", r => (r.moderate || []).length, "moderate"],
    ["Minor", r => (r.minor || []).length, "minor"],
    ["Inspections to commission", r => (r.recommendations || []).length, "rec"],
  ];
  for (const [label, fn, cls] of rows) {
    const tr = el("tr");
    tr.appendChild(el("td", "compare-rowhead " + cls, label));
    for (const c of cols) tr.appendChild(el("td", "num", String(fn(c.rep))));
    counts.appendChild(tr);
  }
  panel.appendChild(counts);

  // Difference matrix: every item flagged in at least one property, by phase.
  const flagged = {}; // itemId -> { title, phase, perCol: {colId: severity} }
  for (const c of cols) {
    for (const sev of ["dealBreakers", "concerns", "moderate", "minor"]) {
      for (const f of c.rep[sev] || []) {
        (flagged[f.itemId] ||= { title: f.title, phase: f.phase, perCol: {} }).perCol[c.id] = f.severity;
      }
    }
  }

  const sec = el("section", "summary-section");
  sec.appendChild(el("h3", null, "Where they differ"));
  let any = false;
  for (const phase of checklist.phases) {
    const items = Object.entries(flagged).filter(([, v]) => v.phase === phase.id);
    if (!items.length) continue;
    any = true;
    sec.appendChild(el("h4", "compare-phase", phase.name));
    const t = el("table", "summary-table compare-table");
    const tr0 = el("tr");
    tr0.appendChild(el("th", null, "Item"));
    for (const c of cols) tr0.appendChild(el("th", "num", c.name));
    t.appendChild(tr0);
    for (const [, v] of items) {
      const sevs = cols.map(c => v.perCol[c.id] || "");
      const differs = new Set(sevs).size > 1;
      const tr = el("tr");
      if (differs) tr.className = "compare-differs";
      tr.appendChild(el("td", null, v.title));
      for (const s of sevs) {
        const td = el("td", "num");
        if (s) {
          const b = el("span", "badge " + s, SEV_LABELS[s] || s);
          td.appendChild(b);
        } else td.textContent = "—";
        tr.appendChild(td);
      }
      t.appendChild(tr);
    }
    sec.appendChild(t);
  }
  if (!any) sec.appendChild(el("p", "summary-sub", "Nothing flagged in any property yet."));
  panel.appendChild(sec);
}

// el is a tiny DOM helper: tag, optional className, optional text.
function el(tag, cls, text) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text != null) e.textContent = text;
  return e;
}

// ---- export / import ----------------------------------------------------
// Export is a full snapshot of ALL profiles + their photos as a .zip: an
// inspection.json (profiles + a photo manifest) plus photos/<id>.<ext> holding
// the raw image bytes. It's the only portability path now that the report is
// ephemeral (adr/0002). The ZIP is written by hand with STORED (uncompressed)
// entries — JPEGs don't compress further, so this stays dependency-free (no zip
// library) while avoiding base64's ~33% bloat and keeping the photos browsable
// as real files. Import accepts this .zip, the older base64-in-JSON export, and
// legacy {property,state} / bare-map JSON.

function base64ToBlob(b64, mime) {
  const bin = atob(b64);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new Blob([bytes], { type: mime || "image/jpeg" });
}

function photoExt(mime) {
  return mime === "image/png" ? "png" : mime === "image/webp" ? "webp" : "jpg";
}

// --- minimal stored-only ZIP (no compression, no dependencies) ---
const CRC_TABLE = (() => {
  const t = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = (c & 1) ? (0xEDB88320 ^ (c >>> 1)) : (c >>> 1);
    t[n] = c >>> 0;
  }
  return t;
})();
function crc32(bytes) {
  let c = 0xFFFFFFFF;
  for (let i = 0; i < bytes.length; i++) c = CRC_TABLE[(c ^ bytes[i]) & 0xFF] ^ (c >>> 8);
  return (c ^ 0xFFFFFFFF) >>> 0;
}

// buildZip writes entries [{name, bytes}] as a STORED archive and returns a Blob.
function buildZip(entries) {
  const enc = new TextEncoder();
  const parts = [];      // local headers + data, in order
  const central = [];    // central directory records
  let offset = 0;        // running offset of the next local header
  const DOS_DATE = 0x21; // 1980-01-01, time 0 — deterministic, harmless
  for (const e of entries) {
    const name = enc.encode(e.name);
    const data = e.bytes;
    const crc = crc32(data);
    const lh = new Uint8Array(30 + name.length);
    const ld = new DataView(lh.buffer);
    ld.setUint32(0, 0x04034b50, true);   // local file header sig
    ld.setUint16(4, 20, true);           // version needed
    ld.setUint16(8, 0, true);            // method: 0 = stored
    ld.setUint16(12, DOS_DATE, true);    // mod date
    ld.setUint32(14, crc, true);
    ld.setUint32(18, data.length, true); // compressed size
    ld.setUint32(22, data.length, true); // uncompressed size
    ld.setUint16(26, name.length, true);
    lh.set(name, 30);
    parts.push(lh, data);

    const ch = new Uint8Array(46 + name.length);
    const cd = new DataView(ch.buffer);
    cd.setUint32(0, 0x02014b50, true);   // central dir header sig
    cd.setUint16(4, 20, true);           // version made by
    cd.setUint16(6, 20, true);           // version needed
    cd.setUint16(10, 0, true);           // method stored
    cd.setUint16(14, DOS_DATE, true);
    cd.setUint32(16, crc, true);
    cd.setUint32(20, data.length, true);
    cd.setUint32(24, data.length, true);
    cd.setUint16(28, name.length, true);
    cd.setUint32(42, offset, true);      // offset of local header
    ch.set(name, 46);
    central.push(ch);
    offset += lh.length + data.length;
  }
  let centralSize = 0;
  for (const c of central) centralSize += c.length;
  const eocd = new Uint8Array(22);
  const ed = new DataView(eocd.buffer);
  ed.setUint32(0, 0x06054b50, true);     // EOCD sig
  ed.setUint16(8, central.length, true); // entries on this disk
  ed.setUint16(10, central.length, true);// total entries
  ed.setUint32(12, centralSize, true);
  ed.setUint32(16, offset, true);        // central dir offset
  return new Blob([...parts, ...central, eocd], { type: "application/zip" });
}

// readZip extracts a STORED archive into { name: Uint8Array }.
function readZip(buf) {
  const bytes = new Uint8Array(buf);
  const dv = new DataView(buf);
  let p = bytes.length - 22;
  while (p >= 0 && dv.getUint32(p, true) !== 0x06054b50) p--;
  if (p < 0) throw new Error("not a zip (no end-of-central-directory record)");
  const count = dv.getUint16(p + 10, true);
  let cd = dv.getUint32(p + 16, true);
  const files = {};
  for (let i = 0; i < count; i++) {
    if (dv.getUint32(cd, true) !== 0x02014b50) break;
    const nameLen = dv.getUint16(cd + 28, true);
    const extraLen = dv.getUint16(cd + 30, true);
    const commentLen = dv.getUint16(cd + 32, true);
    const lho = dv.getUint32(cd + 42, true);
    const name = new TextDecoder().decode(bytes.subarray(cd + 46, cd + 46 + nameLen));
    const lNameLen = dv.getUint16(lho + 26, true);
    const lExtraLen = dv.getUint16(lho + 28, true);
    const size = dv.getUint32(lho + 18, true);
    const start = lho + 30 + lNameLen + lExtraLen;
    files[name] = bytes.subarray(start, start + size);
    cd += 46 + nameLen + extraLen + commentLen;
  }
  return files;
}

function downloadBlob(name, blob) {
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 0);
}

async function exportJSON() {
  const entries = [];
  const photos = {}; // id -> { mime, fullRes, file }
  const ids = new Set();
  for (const p of Object.values(db.profiles)) photoIdsOf(p.state).forEach(id => ids.add(id));
  for (const id of ids) {
    const rec = await getPhotoRecord(id);
    if (!rec) continue;
    const file = `photos/${id}.${photoExt(rec.mime)}`;
    photos[id] = { mime: rec.mime, fullRes: !!rec.fullRes, file };
    entries.push({ name: file, bytes: new Uint8Array(await rec.blob.arrayBuffer()) });
  }
  const manifest = {
    schema: EXPORT_SCHEMA,
    exportedAt: new Date().toISOString(),
    profiles: db.profiles,
    photos,
  };
  entries.unshift({ name: "inspection.json", bytes: new TextEncoder().encode(JSON.stringify(manifest, null, 2)) });
  downloadBlob("inspection-checklist.zip", buildZip(entries));
}

async function importJSON(e) {
  const file = e.target.files[0];
  e.target.value = "";
  if (!file) return;
  try {
    const isZip = file.name.toLowerCase().endsWith(".zip") || file.type === "application/zip";
    const profiles = isZip ? await importZip(file) : await importJSONFile(file);
    let lastId = db.activeId;
    for (const [id, p] of Object.entries(profiles)) { db.profiles[id] = p; lastId = id; }
    db.activeId = lastId;
    saveDB();
    renderProfiles();
    renderPhase();
    hidePanels();
  } catch (err) {
    console.error(err);
    alert("That file didn't look like an inspection export.");
  }
}

// importZip restores photos from the archive's photos/ entries (raw bytes) and
// returns the profiles to merge.
async function importZip(file) {
  const files = readZip(await file.arrayBuffer());
  const manifestBytes = files["inspection.json"];
  if (!manifestBytes) throw new Error("zip has no inspection.json");
  const manifest = JSON.parse(new TextDecoder().decode(manifestBytes));
  for (const [id, meta] of Object.entries(manifest.photos || {})) {
    const bytes = files[meta.file];
    if (!bytes) continue;
    // Copy out of the shared backing buffer before handing to IndexedDB.
    await putPhoto(id, {
      blob: new Blob([bytes.slice()], { type: meta.mime }),
      mime: meta.mime || "image/jpeg", fullRes: !!meta.fullRes,
      createdAt: new Date().toISOString(),
    });
  }
  return normalizeImport(manifest);
}

// importJSONFile handles the older base64-in-JSON export and legacy plain JSON.
async function importJSONFile(file) {
  const data = JSON.parse(await file.text());
  if (data.photos) {
    for (const [id, ph] of Object.entries(data.photos)) {
      if (!ph.data) continue; // base64-only; zip manifests have .file, not .data
      await putPhoto(id, {
        blob: base64ToBlob(ph.data, ph.mime), mime: ph.mime || "image/jpeg",
        fullRes: !!ph.fullRes, createdAt: new Date().toISOString(),
      });
    }
  }
  return normalizeImport(data);
}

// normalizeImport accepts the v2 snapshot, or wraps a bare/legacy payload into a
// single new profile so old hand-made files still import.
function normalizeImport(data) {
  if (data.schema === EXPORT_SCHEMA && data.profiles) {
    // Re-key onto fresh ids only if they collide with existing ones.
    const out = {};
    for (const [id, p] of Object.entries(data.profiles)) {
      const key = db.profiles[id] && db.profiles[id] !== p ? newId() : id;
      out[key] = Object.assign(freshProfile(p.name), p);
    }
    return out;
  }
  // Legacy: { property, state } or a bare { itemId: {...} } map.
  const state = data.state || data;
  const p = freshProfile(data.property || "Imported inspection");
  p.state = state && typeof state === "object" ? state : {};
  return { [newId()]: p };
}

async function resetActive() {
  const p = activeProfile();
  if (!confirm(`Clear all findings and photos for "${p.name}"?`)) return;
  await deletePhotos(photoIdsOf(p.state));
  p.state = {};
  touchProfile();
  saveDB();
  renderPhase();
  hidePanels();
}
