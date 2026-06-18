// app.js — frontend glue. Boots the Go/WASM module, renders the authored
// checklist content, captures per-item state in localStorage, and asks Go to
// compute the report / render the printable HTML.
//
// Division of labour: ALL inspection logic (scoring, escalation, report HTML)
// lives in Go. This file only renders content and captures user input.
//
// TODO(next-agent):
//   - polish rendering & theme to match fate4.tools / gasslands.tools
//   - render the computed report into a dedicated view (currently opens raw HTML)
//   - reserve/implement photoRef capture (v2)
//   - per-property profiles (currently a single active inspection)

const STORAGE_KEY = "inspection-checklist:v1";
const DISCLAIMER_KEY = "inspection-checklist:disclaimer-accepted";
const SEVERITIES = ["unchecked", "na", "ok", "minor", "moderate", "concern", "dealbreaker"];

let checklist = null;          // authored content (from Go)
let state = loadState();        // { itemId: { severity, note } }
let activePhase = null;

// ---- boot ---------------------------------------------------------------

async function boot() {
  showDisclaimer();
  const go = new Go(); // from wasm_exec.js
  const resp = await fetch("inspect.wasm");
  const { instance } = await WebAssembly.instantiateStreaming(resp, go.importObject);
  go.run(instance); // registers globalThis.goInspect, then blocks (select{})

  checklist = JSON.parse(goInspect.checklist());
  activePhase = checklist.phases[0].id;
  renderTabs();
  renderPhase();
  wireToolbar();
}

window.addEventListener("DOMContentLoaded", boot);

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

// ---- state --------------------------------------------------------------

function loadState() {
  try { return JSON.parse(localStorage.getItem(STORAGE_KEY)) || {}; }
  catch { return {}; }
}
function saveState() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
}
function setItem(id, patch) {
  state[id] = Object.assign({ severity: "unchecked", note: "" }, state[id], patch);
  saveState();
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

function renderItem(item) {
  const el = document.createElement("div");
  el.className = "item";

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
  const cur = (state[item.id] && state[item.id].severity) || "unchecked";
  for (const s of SEVERITIES) {
    const o = document.createElement("option");
    o.value = s; o.textContent = s;
    if (s === cur) o.selected = true;
    sel.appendChild(o);
  }
  sel.dataset.sev = cur;
  sel.addEventListener("change", () => {
    sel.dataset.sev = sel.value;
    setItem(item.id, { severity: sel.value });
  });
  meta.appendChild(sel);

  // badges
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
  note.value = (state[item.id] && state[item.id].note) || "";
  note.addEventListener("input", () => setItem(item.id, { note: note.value }));
  el.appendChild(note);

  return el;
}

function badge(text, cls) {
  const b = document.createElement("span");
  b.className = "badge " + cls;
  b.textContent = text;
  return b;
}

// ---- toolbar actions ----------------------------------------------------

function wireToolbar() {
  document.getElementById("btn-report").addEventListener("click", openReport);
  document.getElementById("btn-export").addEventListener("click", exportJSON);
  document.getElementById("btn-import").addEventListener("click", () =>
    document.getElementById("import-file").click());
  document.getElementById("import-file").addEventListener("change", importJSON);
  document.getElementById("btn-reset").addEventListener("click", resetAll);
}

function meta() {
  return JSON.stringify({ Property: document.getElementById("property-name").value || "Unlabeled property" });
}

function openReport() {
  const html = goInspect.renderReport(JSON.stringify(state), meta());
  const w = window.open("", "_blank");
  w.document.write(html);
  w.document.close();
}

function exportJSON() {
  const payload = { property: document.getElementById("property-name").value, state };
  download("inspection.json", JSON.stringify(payload, null, 2));
}

function importJSON(e) {
  const file = e.target.files[0];
  if (!file) return;
  const reader = new FileReader();
  reader.onload = () => {
    try {
      const data = JSON.parse(reader.result);
      state = data.state || data;
      document.getElementById("property-name").value = data.property || "";
      saveState();
      renderPhase();
    } catch { alert("Could not parse that JSON."); }
  };
  reader.readAsText(file);
}

function resetAll() {
  if (!confirm("Clear all findings for this inspection?")) return;
  state = {};
  saveState();
  renderPhase();
}

function download(name, text) {
  const a = document.createElement("a");
  a.href = URL.createObjectURL(new Blob([text], { type: "application/json" }));
  a.download = name;
  a.click();
  URL.revokeObjectURL(a.href);
}
