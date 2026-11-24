// Service Worker des Dashboards: macht es installierbar, zeigt Push-Benachrichtigungen
// und eine Hinweisseite ohne Verbindung. API und Live-Daten werden nie zwischengespeichert.
const SHELL = 'nrmm-shell-v1';
const ASSETS = ['/admin/', '/app.js', '/app.css', '/logo.svg', '/favicon.svg', '/icons/icon-192.png'];

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(SHELL).then((c) => c.addAll(ASSETS)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (e) => {
  e.waitUntil(caches.keys().then((keys) => Promise.all(keys.filter((k) => k !== SHELL).map((k) => caches.delete(k))))
    .then(() => self.clients.claim()));
});

// Immer zuerst das Netz (damit Updates sofort da sind) – nur ohne Verbindung aus dem Speicher.
self.addEventListener('fetch', (e) => {
  const u = new URL(e.request.url);
  if (e.request.method !== 'GET' || u.origin !== location.origin || u.pathname.startsWith('/api/')) return;
  if (!ASSETS.includes(u.pathname) && u.pathname !== '/admin') return;
  e.respondWith(fetch(e.request).then((r) => {
    if (r.ok) { const copy = r.clone(); caches.open(SHELL).then((c) => c.put(e.request, copy)); }
    return r;
  }).catch(() => caches.match(e.request).then((r) => r || caches.match('/admin/'))));
});

self.addEventListener('push', (e) => {
  let d = {};
  try { d = e.data ? e.data.json() : {}; } catch { d = { title: 'Nicolaj Fernwartung', body: e.data ? e.data.text() : '' }; }
  e.waitUntil((async () => {
    const wins = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
    if (d.tag !== 'test' && wins.some((w) => w.visibilityState === 'visible' && w.focused)) return; // das offene Dashboard zeigt es selbst
    if (navigator.setAppBadge && d.tag && !d.tag.startsWith('sec')) navigator.setAppBadge().catch(() => {});
    await self.registration.showNotification(d.title || 'Nicolaj Fernwartung', {
      body: d.body || '', tag: d.tag || undefined, renotify: !!d.tag, data: { href: d.href || '/admin/' },
      icon: '/icons/icon-192.png', badge: '/icons/badge-96.png', lang: 'de',
    });
  })());
});

self.addEventListener('notificationclick', (e) => {
  e.notification.close();
  const href = new URL((e.notification.data && e.notification.data.href) || '/admin/', location.origin).href;
  e.waitUntil((async () => {
    const wins = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
    for (const w of wins) {
      if (new URL(w.url).pathname.startsWith('/admin')) { w.postMessage({ type: 'open', href }); return w.focus(); }
    }
    return self.clients.openWindow(href);
  })());
});

// Eigene Benachrichtigungen aus der Seite (Android erlaubt nur diesen Weg)
self.addEventListener('message', (e) => {
  const d = e.data || {};
  if (d.type === 'notify') {
    self.registration.showNotification(d.title, { body: d.body, tag: d.tag, data: { href: d.href }, icon: '/icons/icon-192.png', badge: '/icons/badge-96.png' });
  }
});
