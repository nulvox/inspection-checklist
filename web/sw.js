// sw.js — offline-shell service worker (adr/0004). A caching shim, nothing more:
// no push, no background sync. It precaches the static shell so the app launches
// at a property with no signal. User data lives in localStorage + IndexedDB,
// which already work offline, so it is NOT cached here.
//
// Cache-busting: __BUILD_VERSION__ is stamped by `make build` (the Makefile owns
// the version, never a human). Each deploy ships a byte-different SW body, so the
// browser runs `activate` and purges the old cache — inspect.wasm can't go stale.

const VERSION = "__BUILD_VERSION__";
const CACHE = "inspection-shell-" + VERSION;

// Relative URLs so the scope is correct under the GitHub Pages subpath.
const SHELL = [
  "./",
  "index.html",
  "css/style.css",
  "js/app.js",
  "js/wasm_exec.js",
  "inspect.wasm",
  "manifest.webmanifest",
  "icon.svg",
];

self.addEventListener("install", (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (e) => {
  e.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

// Cache-first for the static shell; fall back to the network for anything else.
self.addEventListener("fetch", (e) => {
  if (e.request.method !== "GET") return;
  e.respondWith(
    caches.match(e.request).then((hit) => hit || fetch(e.request))
  );
});
