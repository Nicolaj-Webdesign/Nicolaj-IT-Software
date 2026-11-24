/* Nicolaj Fernwartung – Dashboard (ohne Frameworks, ohne Build-Schritt) */
'use strict';

// ---------- Helfer ----------
const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const ESC = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
const esc = (v) => String(v ?? '').replace(/[&<>"']/g, (c) => ESC[c]);

const ICONS = {
  monitor: '<rect x="2.5" y="3.5" width="19" height="13" rx="2"/><path d="M8 20.5h8M12 16.5v4"/>',
  users: '<circle cx="9" cy="8" r="3.5"/><path d="M2.5 20c.8-3.6 3.4-5.5 6.5-5.5s5.7 1.9 6.5 5.5"/><circle cx="17" cy="9" r="2.5"/><path d="M17 14.5c2.2.2 3.8 1.8 4.5 4.5"/>',
  user: '<circle cx="12" cy="8" r="4"/><path d="M4 21c1-4 4.5-6 8-6s7 2 8 6"/>',
  package: '<path d="M3 7.5 12 3l9 4.5v9L12 21l-9-4.5z"/><path d="M3 7.5 12 12l9-4.5M12 12v9"/>',
  terminal: '<rect x="2.5" y="4" width="19" height="16" rx="2"/><path d="m6.5 9 3 3-3 3M12 15h5"/>',
  list: '<path d="M9 6h11M9 12h11M9 18h11"/><circle cx="4.5" cy="6" r="1"/><circle cx="4.5" cy="12" r="1"/><circle cx="4.5" cy="18" r="1"/>',
  sliders: '<path d="M4 6h9M17 6h3M4 12h3M11 12h9M4 18h11M19 18h1"/><circle cx="15" cy="6" r="2"/><circle cx="9" cy="12" r="2"/><circle cx="17" cy="18" r="2"/>',
  history: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.5 2"/>',
  power: '<path d="M12 3v8M6.3 6.8a8 8 0 1 0 11.4 0"/>',
  refresh: '<path d="M3 12a9 9 0 0 1 15.5-6.3L21 8M21 3v5h-5M21 12a9 9 0 0 1-15.5 6.3L3 16M3 21v-5h5"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  x: '<path d="M6 6l12 12M18 6 6 18"/>',
  check: '<path d="m5 12.5 4.5 4.5L19 7"/>',
  trash: '<path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13"/>',
  key: '<circle cx="8" cy="15" r="4"/><path d="m11 12 9-9M16 7l3 3"/>',
  shield: '<path d="M12 3l8 3v6c0 4.5-3.4 8.3-8 9-4.6-.7-8-4.5-8-9V6z"/>',
  lock: '<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>',
  message: '<path d="M4 5h16v11H9l-5 4z"/>',
  send: '<path d="M4 12 20 4l-4 16-4-6.5z"/><path d="m12 13.5 8-9.5"/>',
  folder: '<path d="M3 6.5A1.5 1.5 0 0 1 4.5 5H9l2 2.5h8.5A1.5 1.5 0 0 1 21 9v9.5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18.5z"/>',
  download: '<path d="M12 4v11M7 10l5 5 5-5M5 20h14"/>',
  copy: '<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V5a1 1 0 0 0-1-1H5a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h3"/>',
  logout: '<path d="M15 4h3a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-3M10 8l-4 4 4 4M6 12h10"/>',
  back: '<path d="m15 6-6 6 6 6"/>',
  chevron: '<path d="m9 6 6 6-6 6"/>',
  more: '<circle cx="5" cy="12" r="1.3"/><circle cx="12" cy="12" r="1.3"/><circle cx="19" cy="12" r="1.3"/>',
  expand: '<path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5"/>',
  keyboard: '<rect x="2" y="6" width="20" height="12" rx="2"/><path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M7 14h10"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m16.5 16.5 4.5 4.5"/>',
  alert: '<path d="M12 3 2 20h20z"/><path d="M12 10v4M12 17h.01"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>',
  play: '<path d="M7 5l12 7-12 7z"/>',
  cpu: '<rect x="6" y="6" width="12" height="12" rx="1.5"/><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3"/>',
  disk: '<rect x="3" y="13" width="18" height="7" rx="2"/><path d="M5 13l2.5-8h9L19 13M7 16.5h.01"/>',
  wifi: '<path d="M2 9a15 15 0 0 1 20 0M5.5 12.5a10 10 0 0 1 13 0M9 16a5 5 0 0 1 6 0M12 19.5h.01"/>',
  eye: '<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>',
  dice: '<rect x="4" y="4" width="16" height="16" rx="3"/><path d="M9 9h.01M15 15h.01M12 12h.01M15 9h.01M9 15h.01"/>',
  stop: '<rect x="6" y="6" width="12" height="12" rx="2"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.5 2"/>',
};
const icon = (n, s = 16) => `<svg width="${s}" height="${s}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[n] || ''}</svg>`;

const S = {
  me: null,
  devices: new Map(),
  info: null,
  view: null,
  myCmds: new Set(),
  quietCmds: new Set(),
  es: null,
  onEnroll: null,
};

async function api(method, path, body) {
  const opt = { method, headers: { 'X-NRMM': '1' }, credentials: 'same-origin' };
  if (body !== undefined) {
    opt.headers['Content-Type'] = 'application/json';
    opt.body = JSON.stringify(body);
  }
  const r = await fetch(path, opt);
  const t = await r.text();
  let j = {};
  try { j = t ? JSON.parse(t) : {}; } catch { j = { error: t }; }
  if (!r.ok) {
    if (r.status === 401 && S.me && !path.startsWith('/api/login')) { S.me = null; renderLogin(); }
    const e = new Error(j.error || `Fehler ${r.status}`);
    e.status = r.status; e.data = j;
    throw e;
  }
  return j;
}

function ago(ts) {
  if (!ts || ts.startsWith('0001')) return '–';
  const d = new Date(ts), s = (Date.now() - d) / 1000;
  if (s < 45) return 'gerade eben';
  if (s < 3600) return `vor ${Math.round(s / 60)} Min.`;
  if (s < 86400) return `vor ${Math.round(s / 3600)} Std.`;
  if (s < 86400 * 7) { const n = Math.round(s / 86400); return `vor ${n} ${n === 1 ? 'Tag' : 'Tagen'}`; }
  return d.toLocaleDateString('de-DE');
}
const fmtDate = (ts) => (!ts || ts.startsWith('0001')) ? '–' : new Date(ts).toLocaleString('de-DE', { dateStyle: 'medium', timeStyle: 'short' });
const fmtPCID = (id) => (id && id.length === 9) ? `${id.slice(0, 3)} ${id.slice(3, 6)} ${id.slice(6)}` : (id || '–');
const hhmm = (ts) => new Date(ts).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' });
function pausedUntil(d) {
  const t = d?.protectionPausedUntil;
  return t && new Date(t) > new Date() ? t : null;
}
function verLess(a, b) {
  const pa = String(a || '0').split('.').map(Number), pb = String(b || '0').split('.').map(Number);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) { const x = pa[i] || 0, y = pb[i] || 0; if (x !== y) return x < y; }
  return false;
}
const isOutdated = (d) => S.info?.agentLatest && d.agentVersion && verLess(d.agentVersion, S.info.agentLatest);
const num = (v, d = 1) => (v == null || isNaN(v)) ? '–' : Number(v).toLocaleString('de-DE', { maximumFractionDigits: d });

function toast(title, kind = 'info', detail = '', ms = 4500) {
  const el = document.createElement('div');
  el.className = `toast ${kind}`;
  const ic = { ok: 'check', error: 'alert', info: 'info' }[kind] || 'info';
  el.innerHTML = `<span class="ti">${icon(ic, 18)}</span><div><b>${esc(title)}</b>${detail ? `<span>${esc(detail)}</span>` : ''}</div>`;
  $('#toasts').append(el);
  setTimeout(() => el.remove(), ms);
}

function applyBars(root) {
  $$('[data-w]', root).forEach((el) => { el.style.width = Math.max(0, Math.min(100, +el.dataset.w)) + '%'; });
}

async function copyText(t) {
  try { await navigator.clipboard.writeText(t); toast('Kopiert', 'ok'); }
  catch { toast('Kopieren nicht möglich', 'error', 'Bitte manuell markieren.'); }
}

// Gut merkbare Passwörter (z. B. für Oma): Wort-Wort-Zahl
const WORDS = ['Apfel', 'Birne', 'Sonne', 'Mond', 'Garten', 'Tisch', 'Blume', 'Wolke', 'Insel', 'Kaffee', 'Brezel', 'Katze', 'Hase', 'Vogel', 'Baum', 'Fluss', 'Berg', 'Wald', 'Stern', 'Kerze', 'Rose', 'Tulpe', 'Brot', 'Honig', 'Muschel', 'Schiff', 'Zug', 'Radio', 'Lampe', 'Fenster', 'Sommer', 'Winter', 'Herbst', 'Regen', 'Schnee', 'Wiese', 'Biene', 'Igel', 'Eule', 'Fuchs'];
function genPassword() {
  const r = crypto.getRandomValues(new Uint32Array(3));
  return `${WORDS[r[0] % WORDS.length]}-${WORDS[r[1] % WORDS.length]}-${10 + (r[2] % 90)}`;
}

// ---------- Modal ----------
function openModal({ title, body, submitLabel = 'Speichern', danger = false, wide = false, onSubmit, footer, onOpen }) {
  const wrap = document.createElement('div');
  wrap.className = 'modal-backdrop';
  const foot = footer ?? `<button type="button" class="btn ghost" data-close>Abbrechen</button><button type="submit" class="btn ${danger ? 'danger solid' : 'primary'}">${esc(submitLabel)}</button>`;
  wrap.innerHTML = `<form class="modal ${wide ? 'wide' : ''}" novalidate autocomplete="off">
    <header><h2>${esc(title)}</h2><button type="button" class="icon-btn" data-close aria-label="Schließen">${icon('x', 18)}</button></header>
    <div class="modal-body">${body}</div>
    <div class="modal-error" hidden></div>
    <footer>${foot}</footer></form>`;
  document.body.append(wrap);
  const form = $('form', wrap);
  const err = $('.modal-error', wrap);
  const onKey = (e) => { if (e.key === 'Escape') close(); };
  const close = () => { wrap.remove(); document.removeEventListener('keydown', onKey); };
  document.addEventListener('keydown', onKey);
  wrap.addEventListener('mousedown', (e) => { if (e.target === wrap) close(); });
  wrap.addEventListener('click', (e) => { if (e.target.closest('[data-close]')) close(); });
  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (!onSubmit) return close();
    const btn = $('[type=submit]', form);
    if (btn) btn.disabled = true;
    err.hidden = true;
    try {
      const res = await onSubmit(form);
      if (res !== false) close();
    } catch (ex) {
      err.textContent = ex.message;
      err.hidden = false;
    } finally {
      if (btn) btn.disabled = false;
    }
  });
  bindPasswordTools(form);
  onOpen?.(form, close);
  setTimeout(() => $('input:not([type=hidden]):not([type=checkbox]), textarea, select', form)?.focus(), 30);
  return { form, close, showError: (m) => { err.textContent = m; err.hidden = false; } };
}

function confirmModal(title, text, label = 'Bestätigen', danger = false) {
  return new Promise((resolve) => {
    let ok = false;
    const m = openModal({ title, body: `<p>${text}</p>`, submitLabel: label, danger, onSubmit: () => { ok = true; } });
    const obs = new MutationObserver(() => { if (!m.form.isConnected) { obs.disconnect(); resolve(ok); } });
    obs.observe(document.body, { childList: true });
  });
}

function passwordField(name = 'password', label = 'Neues Passwort') {
  return `<label class="field">${esc(label)}
    <div class="input-group"><input class="input mono" name="${name}" type="text" required maxlength="127" autocomplete="new-password" spellcheck="false">
    <button type="button" class="btn" data-gen="${name}" title="Merkbares Passwort erzeugen">${icon('dice')} Erzeugen</button></div>
    <span class="hint">Wird nur einmal übertragen und nirgends gespeichert. Am besten gleich notieren.</span></label>`;
}
function bindPasswordTools(root) {
  $$('[data-gen]', root).forEach((b) => b.addEventListener('click', () => {
    const inp = $(`[name="${b.dataset.gen}"]`, root);
    inp.value = genPassword();
    inp.focus();
  }));
}

// ---------- Start ----------
async function boot() {
  try {
    const st = await api('GET', '/api/state');
    if (st.setupNeeded) return renderSetup();
    if (!st.user) return renderLogin();
    S.me = st.user;
    await startApp();
  } catch (e) {
    $('#app').className = 'auth';
    $('#app').innerHTML = `<div class="card auth-card"><h1>Server nicht erreichbar</h1><p>${esc(e.message)}</p><button class="btn primary" id="retry">Erneut versuchen</button></div>`;
    $('#retry').onclick = () => location.reload();
  }
}

function authShell(inner) {
  if (S.es) { S.es.close(); S.es = null; }
  const app = $('#app');
  app.className = 'auth';
  app.innerHTML = `<div class="card auth-card"><div class="brand"><span class="wordmark">nicolaj<i class="cursor"></i></span><span class="product">Fernwartung</span></div>${inner}</div>`;
}

function renderSetup() {
  authShell(`<h1>Willkommen 👋</h1><p>Lege dein Administrator-Konto an. Den Einrichtungscode zeigt das Server-Fenster an.</p>
    <form id="f"><div class="auth-err" hidden></div>
      <label class="field">Einrichtungscode<input class="input mono" name="setupCode" required autocomplete="one-time-code" placeholder="XXXX-XXXX"></label>
      <label class="field">Benutzername<input class="input" name="username" required autocomplete="username" value="nicolaj"></label>
      <label class="field">Passwort<input class="input" name="password" type="password" required minlength="10" autocomplete="new-password"><span class="hint">Mindestens 10 Zeichen.</span></label>
      <label class="field">Passwort wiederholen<input class="input" name="password2" type="password" required autocomplete="new-password"></label>
      <button class="btn primary lg">Konto anlegen</button></form>`);
  const f = $('#f');
  f.addEventListener('submit', async (e) => {
    e.preventDefault();
    const err = $('.auth-err', f);
    err.hidden = true;
    if (f.password.value !== f.password2.value) { err.textContent = 'Die Passwörter stimmen nicht überein.'; err.hidden = false; return; }
    try {
      await api('POST', '/api/setup', { setupCode: f.setupCode.value, username: f.username.value, password: f.password.value });
      boot();
    } catch (ex) { err.textContent = ex.message; err.hidden = false; }
  });
}

function renderLogin(needTotp = false) {
  authShell(`<h1>Anmelden</h1><p>Fernwartung für deine Kunden, Familie und Freunde.</p>
    <form id="f"><div class="auth-err" hidden></div>
      <label class="field">Benutzername<input class="input" name="username" required autocomplete="username"></label>
      <label class="field">Passwort<input class="input" name="password" type="password" required autocomplete="current-password"></label>
      <label class="field" id="totpf" ${needTotp ? '' : 'hidden'}>Code aus der Authenticator-App<input class="input mono" name="totp" inputmode="numeric" autocomplete="one-time-code" maxlength="7" placeholder="123456"></label>
      <button class="btn primary lg">Anmelden</button></form>`);
  const f = $('#f');
  f.username.focus();
  f.addEventListener('submit', async (e) => {
    e.preventDefault();
    const err = $('.auth-err', f);
    err.hidden = true;
    try {
      await api('POST', '/api/login', { username: f.username.value, password: f.password.value, totp: f.totp.value });
      boot();
    } catch (ex) {
      if (ex.data?.needTotp) { $('#totpf').hidden = false; f.totp.focus(); }
      err.textContent = ex.message; err.hidden = false;
    }
  });
}

async function startApp() {
  const app = $('#app');
  app.className = '';
  app.innerHTML = `<div class="shell">
    <aside class="side">
      <a class="brand" href="#/"><img src="/logo.svg" alt="" width="34" height="34"><span><span class="wordmark">nicolaj<i class="cursor"></i></span><span class="product">Fernwartung</span></span></a>
      <button type="button" class="side-search" id="psearch">${icon('search', 16)}<span>Suchen</span><kbd>Strg K</kbd></button>
      <nav class="nav">
        <a href="#/" data-nav="devices">${icon('monitor', 18)}<span>Geräte</span><em class="count" id="devcount"></em></a>
        <a href="#/messages" data-nav="messages">${icon('message', 18)}<span>Nachrichten</span><em class="count" id="chatcount"></em></a>
        <a href="#/security" data-nav="security">${icon('shield', 18)}<span>Sicherheit</span><em class="count" id="seccount"></em></a>
        <a href="#/audit" data-nav="audit">${icon('history', 18)}<span>Protokoll</span></a>
        <a href="#/settings" data-nav="settings">${icon('sliders', 18)}<span>Einstellungen</span></a>
      </nav>
      <div class="side-foot"><div class="who"><b>${esc(S.me.username)}</b><span>Administrator</span></div>
        <button class="icon-btn" id="logout" title="Abmelden">${icon('logout', 18)}</button></div>
    </aside>
    <main class="main" id="main"></main></div>
    <div class="conn-banner" id="connbanner" hidden>Verbindung zum Server unterbrochen …</div>`;
  $('#logout').onclick = async () => { await api('POST', '/api/logout').catch(() => {}); S.me = null; renderLogin(); };
  $('#psearch').onclick = () => openPalette();
  if (!S.paletteKey) {
    S.paletteKey = true;
    document.addEventListener('keydown', (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k' && S.me) { e.preventDefault(); openPalette(); }
    });
  }
  await loadDevices();
  await loadChats();
  S.info = await api('GET', '/api/server-info').catch(() => null);
  connectEvents();
  window.onhashchange = route;
  route();
}

async function loadDevices() {
  const list = await api('GET', '/api/devices');
  S.devices = new Map(list.map((d) => [d.id, d]));
  updateCount();
}
function updateCount() {
  const el = $('#devcount');
  if (!el) return;
  const all = [...S.devices.values()];
  const pend = all.filter((d) => d.pending).length;
  const real = all.filter((d) => !d.pending);
  const on = real.filter((d) => d.online).length;
  el.textContent = pend ? `${pend} neu` : real.length ? `${on}/${real.length}` : '';
  el.classList.toggle('fresh', pend > 0);
  const sc = $('#seccount');
  if (sc) {
    const bad = [...S.devices.values()].filter((d) => d.securityEnabled && secState(d).level === 'danger').length;
    sc.textContent = bad ? `${bad} ⚠` : '';
    sc.classList.toggle('alarm', bad > 0);
  }
}

function connectEvents() {
  if (S.es) S.es.close();
  const es = new EventSource('/api/events');
  S.es = es;
  let first = true;
  es.addEventListener('hello', async () => {
    $('#connbanner').hidden = true;
    if (!first) { await loadDevices().catch(() => {}); await loadChats(); S.view?.onReconnect?.(); }
    first = false;
  });
  es.onerror = async () => {
    $('#connbanner').hidden = false;
    try { const st = await api('GET', '/api/state'); if (!st.user) { S.me = null; renderLogin(); } } catch { /* Server weg – EventSource versucht es weiter */ }
  };
  es.addEventListener('device', (e) => {
    const d = JSON.parse(e.data);
    const isNew = !S.devices.has(d.id);
    S.devices.set(d.id, { ...S.devices.get(d.id), ...d });
    updateCount();
    if (isNew) S.onEnroll?.(d);
    S.view?.onDevice?.(d);
  });
  es.addEventListener('chat', (e) => {
    const m = JSON.parse(e.data);
    onChatEvent(m);
  });
  es.addEventListener('chat-read', (e) => {
    const m = JSON.parse(e.data);
    if (m.by === 'admin') { const c = S.chats.get(m.deviceId); if (c) { c.unread = 0; updateChatCount(); } }
    S.view?.onChatRead?.(m);
  });
  es.addEventListener('device-removed', (e) => {
    const { id } = JSON.parse(e.data);
    S.devices.delete(id);
    updateCount();
    S.view?.onDeviceRemoved?.(id);
  });
  es.addEventListener('command', (e) => {
    const c = JSON.parse(e.data);
    if (S.myCmds.has(c.id) && ['done', 'failed', 'timeout', 'expired', 'canceled'].includes(c.status)) {
      S.myCmds.delete(c.id);
      const first = (c.output || '').split('\n').find((l) => l.trim()) || '';
      const quiet = S.quietCmds.delete(c.id);
      if (c.status === 'done') { if (!quiet) toast(c.summary, 'ok', first.slice(0, 160)); }
      else if (c.status !== 'canceled') toast(c.summary, 'error', (c.output || 'Fehlgeschlagen').slice(0, 300), 9000);
    }
    S.view?.onCommand?.(c);
  });
  es.addEventListener('alert', (e) => {
    const a = JSON.parse(e.data);
    toast(a.title, a.level === 'info' ? 'info' : 'error', a.detail, a.level === 'info' ? 9000 : 15000);
    notifyDesktop(a.title, a.detail || '', a.deviceId ? `#/d/${a.deviceId}/overview` : '#/');
    S.view?.onAlert?.(a);
  });
  es.addEventListener('screen', (e) => {
    const m = JSON.parse(e.data);
    const d = S.devices.get(m.deviceId);
    if (d) d.session = m.session;
    S.view?.onScreen?.(m);
  });
}

async function sendCommand(devId, type, args = {}, { quiet = false } = {}) {
  try {
    const c = await api('POST', `/api/devices/${devId}/commands`, { type, args });
    S.myCmds.add(c.id);
    if (quiet) S.quietCmds.add(c.id);
    if (!quiet) toast(c.status === 'queued' ? 'Auftrag wartet' : 'Auftrag gesendet', 'info', c.summary + (c.status === 'queued' ? ' – wird ausgeführt, sobald der PC online ist.' : ''));
    return c;
  } catch (e) {
    toast('Nicht möglich', 'error', e.message, 7000);
    throw e;
  }
}

function route() {
  const h = location.hash.replace(/^#/, '') || '/';
  const parts = h.split('/').filter(Boolean);
  S.view?.destroy?.();
  S.view = null;
  const main = $('#main');
  if (!main) return;
  main.innerHTML = '';
  let nav = 'devices';
  if (parts[0] === 'd' && parts[1]) S.view = new DeviceView(parts[1], parts[2] || 'overview');
  else if (parts[0] === 'audit') { S.view = new AuditView(); nav = 'audit'; }
  else if (parts[0] === 'security') { S.view = new SecurityView(); nav = 'security'; }
  else if (parts[0] === 'settings') { S.view = new SettingsView(); nav = 'settings'; }
  else if (parts[0] === 'messages') { S.view = new MessagesView(parts[1] || ''); nav = 'messages'; }
  else S.view = new DevicesView();
  $$('.nav a').forEach((a) => a.classList.toggle('active', a.dataset.nav === nav));
  S.view.render(main);
  window.scrollTo(0, 0);
}

document.addEventListener('click', (e) => {
  // offene Menüs schließen
  $$('details.menu[open]').forEach((d) => { if (!d.contains(e.target) || e.target.closest('.menu-pop button')) d.open = false; });
});

// ---------- Gemeinsame Darstellung ----------
function inv(d) { return d?.inventory || {}; }
// lowDisk: Laufwerk mit weniger als 10 % (bzw. 5 GB) frei – oder ''.
function lowDisk(d) {
  const k = (inv(d).disks || []).find((x) => x.sizeGB > 0 && (x.freeGB / x.sizeGB < 0.1 || x.freeGB < 5));
  return k ? k.name : '';
}
function osShort(d) { return (inv(d).os || '').replace(/^Microsoft\s+/, '') || 'Windows'; }
function loggedUser(d) {
  const u = inv(d).loggedOnUser;
  if (!d.screenOnline) return d.online ? 'niemand angemeldet' : '–';
  return u ? u.split('\\').pop() : 'angemeldet';
}
function statusPill(d) {
  if (d.session) return `<span class="pill accent live">${d.session.state === 'active' ? 'Bildschirm verbunden' : 'Verbindung wird aufgebaut'}</span>`;
  if (d.online) return '<span class="pill ok">Online</span>';
  return '<span class="pill">Offline</span>';
}

// ---------- Geräteliste ----------
class DevicesView {
  constructor() { this.filter = ''; this.view = 'all'; this.group = ''; this.selecting = false; this.sel = new Set(); }
  render(main) {
    this.main = main;
    main.innerHTML = `<div class="page-head"><div class="grow"><h1>Geräte</h1><p>Alle PCs, die du betreust.</p></div>
      <button class="btn" id="selmode">${icon('check', 16)} Auswählen</button>
      <button class="btn primary" id="add">${icon('plus')} Gerät hinzufügen</button></div>
      <div id="req"></div>
      <div class="stats" id="stats"></div>
      <div class="toolbar" id="tb"><input class="input" id="flt" type="search" placeholder="Suchen nach Name, PC-ID, Benutzer, IP …"><div class="chips" id="groups"></div></div>
      <div id="list"></div>
      <div class="selbar" id="selbar" hidden></div>`;
    $('#add', main).onclick = () => addDeviceModal();
    $('#selmode', main).onclick = () => { this.selecting = !this.selecting; this.sel.clear(); this.draw(); };
    $('#flt', main).oninput = (e) => { this.filter = e.target.value.toLowerCase(); this.draw(); };
    this.draw();
    this.timer = setInterval(() => this.draw(), 30000);
  }
  destroy() { clearInterval(this.timer); }
  attention(d) {
    if (d.securityEnabled && secState(d).level === 'danger') return true;
    if (inv(d).rebootPending || lowDisk(d)) return true;
    return !d.online && d.lastSeen && Date.now() - new Date(d.lastSeen) > 7 * 864e5;
  }
  draw() {
    const list = $('#list', this.main);
    const everything = [...S.devices.values()];
    const pend = everything.filter((d) => d.pending).sort((a, b) => String(b.requestedAt).localeCompare(String(a.requestedAt)));
    const all = everything.filter((d) => !d.pending);
    const reqBox = $('#req', this.main);
    reqBox.innerHTML = pend.length ? requestsCard(pend) : '';
    bindRequests(reqBox);
    $('#selmode', this.main).classList.toggle('active', this.selecting);
    $('#selmode', this.main).hidden = all.length < 2;

    // Übersicht
    const on = all.filter((d) => d.online).length;
    const att = all.filter((d) => this.attention(d)).length;
    const old = all.filter((d) => isOutdated(d)).length;
    const tiles = [
      ['all', 'Alle Geräte', all.length, ''],
      ['online', 'Online', on, 'ok'],
      ['offline', 'Offline', all.length - on, ''],
      ['attention', 'Brauchen Aufmerksamkeit', att, att ? 'warn' : ''],
      ['outdated', 'Agent veraltet', old, old ? 'warn' : ''],
    ];
    $('#stats', this.main).innerHTML = all.length ? tiles.map(([k, l, n, tone]) => `<button type="button" class="stat ${this.view === k ? 'active' : ''}" data-view="${k}">
      <b class="${tone}">${n}</b><span>${l}</span></button>`).join('') : '';
    $$('[data-view]', this.main).forEach((b) => { b.onclick = () => { this.view = this.view === b.dataset.view ? 'all' : b.dataset.view; this.draw(); }; });

    // Gruppen
    const groups = [...new Set(all.map((d) => d.group).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'de'));
    if (this.group && this.group !== '-' && !groups.includes(this.group)) this.group = '';
    $('#groups', this.main).innerHTML = groups.length ? [['', 'Alle Gruppen'], ...groups.map((g) => [g, g]), ['-', 'Ohne Gruppe']]
      .map(([v, l]) => `<button type="button" class="chip ${this.group === v ? 'on' : ''}" data-group="${esc(v)}">${esc(l)}</button>`).join('') : '';
    $$('[data-group]', this.main).forEach((b) => { b.onclick = () => { this.group = b.dataset.group; this.draw(); }; });
    $('#tb', this.main).hidden = all.length < 2;

    if (!all.length) {
      list.innerHTML = pend.length ? '' : `<div class="card empty"><h2>Noch kein Gerät verbunden</h2>
        <p>Füge deinen ersten PC hinzu – oder schick den Link zu deiner Website, dann können Leute selbst Zugang anfragen.</p>
        <button class="btn primary" id="add2">${icon('plus')} Ersten PC hinzufügen</button></div>`;
      const a2 = $('#add2', list);
      if (a2) a2.onclick = () => addDeviceModal();
      this.drawSelbar([]);
      return;
    }
    const f = this.filter, fd = f.replace(/\s+/g, '');
    const devs = all.filter((d) => {
      if (f && !([d.name, d.hostname, d.lastIp, d.group, inv(d).loggedOnUser, ...(inv(d).ips || [])].join(' ').toLowerCase().includes(f) || (fd.length >= 3 && /^\d+$/.test(fd) && (d.pcId || '').includes(fd)))) return false;
      if (this.group === '-' && d.group) return false;
      if (this.group && this.group !== '-' && d.group !== this.group) return false;
      switch (this.view) {
        case 'online': return d.online;
        case 'offline': return !d.online;
        case 'attention': return this.attention(d);
        case 'outdated': return isOutdated(d);
      }
      return true;
    }).sort((a, b) => (b.online - a.online) || a.name.localeCompare(b.name, 'de'));
    if (!devs.length) {
      list.innerHTML = `<div class="card empty"><h2>Keine Treffer</h2><p>Kein Gerät passt zu Suche und Filter.</p><button class="btn" id="reset">Filter zurücksetzen</button></div>`;
      $('#reset', list).onclick = () => { this.view = 'all'; this.group = ''; this.filter = ''; $('#flt', this.main).value = ''; this.draw(); };
      this.drawSelbar(devs);
      return;
    }
    const sections = groups.length && !this.group
      ? [...groups.map((g) => [g, devs.filter((d) => d.group === g)]), ['Ohne Gruppe', devs.filter((d) => !d.group)]].filter(([, l]) => l.length)
      : [['', devs]];
    list.innerHTML = sections.map(([title, ds]) => `${title ? `<h3 class="grp-title">${esc(title)} <span>${ds.length}</span></h3>` : ''}
      <div class="dev-grid">${ds.map((d) => this.card(d)).join('')}</div>`).join('');
    $$('.dev-card', list).forEach((c) => {
      const id = c.dataset.id;
      const go = (tab) => { location.hash = `#/d/${id}/${tab}`; };
      c.onclick = (e) => {
        if (this.selecting) { this.sel.has(id) ? this.sel.delete(id) : this.sel.add(id); this.draw(); return; }
        const b = e.target.closest('[data-go]'); go(b ? b.dataset.go : 'overview');
      };
      c.onkeydown = (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); c.click(); } };
    });
    this.drawSelbar(devs);
  }
  card(d) {
    const picked = this.sel.has(d.id);
    return `<article class="card dev-card ${this.selecting ? 'selecting' : ''} ${picked ? 'picked' : ''}" data-id="${esc(d.id)}" tabindex="0" ${this.selecting ? `role="checkbox" aria-checked="${picked}"` : ''}>
        <div class="top">${this.selecting ? `<span class="pick">${picked ? icon('check', 14) : ''}</span>` : ''}<div class="dev-ico ${d.online ? 'on' : ''}">${icon('monitor', 20)}</div>
          <div class="title"><b>${esc(d.name)}</b><span>${esc(d.hostname || '')} · <span class="mono">ID ${esc(fmtPCID(d.pcId))}</span></span></div>${statusPill(d)}</div>
        <dl class="kv">
          <dt>System</dt><dd>${esc(osShort(d))}</dd>
          <dt>Benutzer</dt><dd>${esc(loggedUser(d))}</dd>
          <dt>${d.online ? 'IP' : 'Zuletzt'}</dt><dd>${d.online ? esc((inv(d).ips || [])[0] || d.lastIp || '–') : esc(ago(d.lastSeen))}</dd>
          ${d.securityEnabled ? `<dt>Schutz</dt><dd>${secBadge(d)}</dd>` : ''}
          ${inv(d).rebootPending || lowDisk(d) ? `<dt>Hinweis</dt><dd>${inv(d).rebootPending ? '<span class="badge warn">Neustart nötig</span> ' : ''}${lowDisk(d) ? `<span class="badge warn">Speicher fast voll (${esc(lowDisk(d))})</span>` : ''}</dd>` : ''}
        </dl>
        ${this.selecting ? '' : `<div class="foot"><button class="btn sm" data-go="overview">Öffnen</button>
          <button class="btn sm ${d.screenOnline ? 'primary' : ''}" data-go="screen" ${d.screenOnline ? '' : 'disabled'}>${icon('eye', 15)} Bildschirm</button></div>`}
      </article>`;
  }
  drawSelbar(visible) {
    const bar = $('#selbar', this.main);
    if (!this.selecting) { bar.hidden = true; return; }
    for (const id of this.sel) if (!S.devices.has(id)) this.sel.delete(id);
    const n = this.sel.size;
    bar.hidden = false;
    bar.innerHTML = `<b>${n ? `${n} ausgewählt` : 'Geräte antippen zum Auswählen'}</b>
      <button class="btn sm ghost" id="sall">${visible.length && visible.every((d) => this.sel.has(d.id)) ? 'Keine' : 'Alle'}</button>
      <span class="spacer"></span>
      <button class="btn sm" data-bulk="message" ${n ? '' : 'disabled'}>${icon('message', 15)} Nachricht</button>
      <button class="btn sm" data-bulk="software" ${n ? '' : 'disabled'}>${icon('package', 15)} Software</button>
      <button class="btn sm" data-bulk="script" ${n ? '' : 'disabled'}>${icon('terminal', 15)} Skript</button>
      <button class="btn sm" data-bulk="group" ${n ? '' : 'disabled'}>${icon('folder', 15)} Gruppe</button>
      <button class="btn sm" data-bulk="restart" ${n ? '' : 'disabled'}>${icon('refresh', 15)} Neu starten</button>
      <button class="btn sm ghost" id="sdone">Fertig</button>`;
    $('#sall', bar).onclick = () => {
      const allPicked = visible.every((d) => this.sel.has(d.id));
      visible.forEach((d) => (allPicked ? this.sel.delete(d.id) : this.sel.add(d.id)));
      this.draw();
    };
    $('#sdone', bar).onclick = () => { this.selecting = false; this.sel.clear(); this.draw(); };
    $$('[data-bulk]', bar).forEach((b) => { b.onclick = () => bulkAction(b.dataset.bulk, [...this.sel], () => this.draw()); });
  }
  onDevice() { this.draw(); }
  onDeviceRemoved() { this.draw(); }
  onScreen() { this.draw(); }
  onReconnect() { this.draw(); }
}

// ---------- Sammelaktionen für mehrere PCs ----------
async function runBulk(ids, label, fn) {
  const res = await Promise.allSettled(ids.map((id) => fn(id)));
  const ok = res.filter((r) => r.status === 'fulfilled').length;
  const bad = res.length - ok;
  const first = res.find((r) => r.status === 'rejected');
  toast(`${label}: ${ok} von ${res.length} PCs`, bad ? (ok ? 'info' : 'error') : 'ok',
    bad ? `${bad} nicht möglich – ${first.reason?.message || 'Fehler'}` : 'Die Ergebnisse siehst du jeweils unter „Aufgaben“.', bad ? 9000 : 5000);
}
const cmdFor = (type, args) => (id) => api('POST', `/api/devices/${id}/commands`, { type, args });

async function bulkAction(kind, ids, done) {
  const names = ids.map((id) => S.devices.get(id)?.name).filter(Boolean);
  const who = `<p class="muted" style="margin:0 0 12px">${esc(names.slice(0, 6).join(', '))}${names.length > 6 ? ` und ${names.length - 6} weitere` : ''}</p>`;
  if (kind === 'message') {
    openModal({ title: `Nachricht an ${ids.length} PCs`, submitLabel: 'Senden', body: `${who}
      <label class="field">Nachricht<textarea class="input" name="text" rows="4" maxlength="1000" required placeholder="z. B. Ich mache heute Abend Updates – bitte die PCs anlassen."></textarea></label>
      <p class="small faint" style="margin:8px 0 0">Erscheint auf allen PCs, an denen gerade jemand angemeldet ist.</p>`,
      onSubmit: async (f) => { if (!f.text.value.trim()) throw new Error('Bitte eine Nachricht eingeben.'); runBulk(ids, 'Nachricht', cmdFor('user.message', { text: f.text.value })); } });
  } else if (kind === 'software') {
    openModal({ title: `Software auf ${ids.length} PCs installieren`, submitLabel: 'Installieren', body: `${who}
      <div class="chips" style="margin-bottom:12px">${CATALOG.map(([n, id]) => `<button type="button" class="chip" data-pkg="${esc(id)}" data-name="${esc(n)}">${esc(n)}</button>`).join('')}</div>
      <label class="field">winget-Paket-ID<input class="input mono" name="id" required placeholder="z. B. Google.Chrome"></label>
      <input type="hidden" name="name">`,
      onOpen: (f) => { $$('[data-pkg]', f).forEach((b) => { b.onclick = () => { f.id.value = b.dataset.pkg; f.name.value = b.dataset.name; $$('[data-pkg]', f).forEach((x) => x.classList.toggle('on', x === b)); }; }); },
      onSubmit: async (f) => { if (!f.id.value.trim()) throw new Error('Bitte ein Paket wählen.'); runBulk(ids, 'Installation', cmdFor('software.install', { id: f.id.value.trim(), name: f.name.value || f.id.value.trim() })); } });
  } else if (kind === 'script') {
    const scripts = await api('GET', '/api/scripts').catch(() => []);
    openModal({ title: `Skript auf ${ids.length} PCs ausführen`, wide: true, submitLabel: 'Ausführen', danger: true, body: `${who}
      ${scripts.length ? `<label class="field">Gespeichertes Skript<select class="input" id="pick"><option value="">– eigenes Skript –</option>${scripts.map((x, i) => `<option value="${i}">${esc(x.name)}</option>`).join('')}</select></label>` : ''}
      <label class="field">PowerShell<textarea class="input console" name="script" rows="7" spellcheck="false" required></textarea></label>
      <p class="small faint" style="margin:8px 0 0">Läuft auf jedem PC als SYSTEM. Offline-PCs führen es aus, sobald sie wieder online sind (bis 24 Std.).</p>`,
      onOpen: (f) => { const p = $('#pick', f); if (p) p.onchange = () => { if (p.value !== '') f.script.value = scripts[+p.value].script; }; },
      onSubmit: async (f) => { if (!f.script.value.trim()) throw new Error('Bitte ein Skript eingeben.'); runBulk(ids, 'Skript', cmdFor('script.run', { script: f.script.value, timeout: 300 })); } });
  } else if (kind === 'group') {
    const groups = [...new Set([...S.devices.values()].map((d) => d.group).filter(Boolean))];
    openModal({ title: `Gruppe für ${ids.length} PCs`, submitLabel: 'Zuweisen', body: `${who}
      <label class="field">Gruppe<input class="input" name="group" maxlength="30" list="grplist" placeholder="z. B. Familie, Kunden"><span class="hint">Leer lassen = keine Gruppe.</span></label>
      <datalist id="grplist">${groups.map((g) => `<option value="${esc(g)}">`).join('')}</datalist>`,
      onSubmit: async (f) => { await runBulk(ids, 'Gruppe', (id) => api('PATCH', `/api/devices/${id}`, { group: f.group.value })); done?.(); } });
  } else if (kind === 'restart') {
    if (!await confirmModal(`${ids.length} PCs neu starten?`, 'Die PCs starten in 1 Minute neu. Wer gerade daran sitzt, bekommt vorher einen Hinweis.', 'Neu starten', true)) return;
    runBulk(ids, 'Neustart', cmdFor('power.restart', { delay: 60, message: 'Dein PC startet in einer Minute neu (Wartung durch Nicolaj Webdesign).' }));
  }
}

// ---------- Schnellsuche (Strg + K) ----------
function openPalette() {
  if ($('.palette-wrap')) return;
  const pages = [['Geräte', '#/', 'monitor'], ['Sicherheit', '#/security', 'shield'], ['Protokoll', '#/audit', 'history'], ['Einstellungen', '#/settings', 'sliders'], ['Gerät hinzufügen', 'add', 'plus']];
  const wrap = document.createElement('div');
  wrap.className = 'palette-wrap';
  wrap.innerHTML = `<div class="palette" role="dialog" aria-label="Schnellsuche"><input class="input" id="pq" placeholder="Gerät, PC-ID oder Seite suchen …" autocomplete="off"><div class="p-list" id="pl" role="listbox"></div>
    <div class="p-foot"><kbd>↑</kbd><kbd>↓</kbd> auswählen <kbd>Enter</kbd> öffnen <kbd>Esc</kbd> schließen</div></div>`;
  document.body.append(wrap);
  const q = $('#pq', wrap), pl = $('#pl', wrap);
  let items = [], idx = 0;
  const close = () => { wrap.remove(); document.removeEventListener('keydown', onKey, true); };
  const go = (it) => { close(); if (it.href === 'add') addDeviceModal(); else location.hash = it.href; };
  const draw = () => {
    const f = q.value.trim().toLowerCase(), fd = f.replace(/\s+/g, '');
    const devs = [...S.devices.values()].filter((d) => !f || [d.name, d.hostname, d.group, inv(d).loggedOnUser].join(' ').toLowerCase().includes(f) || (fd.length >= 2 && (d.pcId || '').includes(fd)))
      .sort((a, b) => (b.online - a.online) || a.name.localeCompare(b.name, 'de')).slice(0, 8)
      .map((d) => ({ label: d.name, sub: `${d.pending ? 'wartet auf Freigabe · ' : d.online ? 'online · ' : ''}ID ${fmtPCID(d.pcId)}${d.group ? ' · ' + d.group : ''}`, href: `#/d/${d.id}/overview`, ic: 'monitor', on: d.online }));
    const pg = pages.filter(([l]) => !f || l.toLowerCase().includes(f)).map(([l, h, ic]) => ({ label: l, sub: 'Seite', href: h, ic }));
    items = [...devs, ...pg];
    idx = Math.min(idx, Math.max(0, items.length - 1));
    pl.innerHTML = items.length ? items.map((it, i) => `<button type="button" class="p-item ${i === idx ? 'on' : ''}" data-i="${i}" role="option" aria-selected="${i === idx}">${icon(it.ic, 16)}<span><b>${esc(it.label)}</b><small>${esc(it.sub)}</small></span>${it.on ? '<i class="dot-on"></i>' : ''}</button>`).join('')
      : '<p class="p-empty">Nichts gefunden.</p>';
    $$('.p-item', pl).forEach((b) => { b.onclick = () => go(items[+b.dataset.i]); });
  };
  const onKey = (e) => {
    if (e.key === 'Escape') { e.preventDefault(); close(); }
    else if (e.key === 'ArrowDown') { e.preventDefault(); idx = Math.min(items.length - 1, idx + 1); draw(); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); idx = Math.max(0, idx - 1); draw(); }
    else if (e.key === 'Enter' && items[idx]) { e.preventDefault(); go(items[idx]); }
  };
  document.addEventListener('keydown', onKey, true);
  wrap.addEventListener('mousedown', (e) => { if (e.target === wrap) close(); });
  q.oninput = () => { idx = 0; draw(); };
  draw();
  q.focus();
}

// ---------- Nachrichten ----------
S.chats = new Map();
async function loadChats() {
  const list = await api('GET', '/api/chats').catch(() => null);
  if (!list) return;
  S.chats = new Map(list.map((c) => [c.deviceId, c]));
  updateChatCount();
}
function updateChatCount() {
  const el = $('#chatcount');
  if (!el) return;
  const n = [...S.chats.values()].reduce((a, c) => a + (c.unread || 0), 0);
  el.textContent = n ? `${n} neu` : '';
  el.classList.toggle('fresh', n > 0);
  document.title = (n ? `(${n}) ` : '') + 'Nicolaj Fernwartung';
}
function onChatEvent(m) {
  const msg = m.message;
  const c = S.chats.get(m.deviceId) || { deviceId: m.deviceId, name: m.deviceName, unread: 0 };
  c.name = m.deviceName || c.name;
  c.last = msg;
  const viewing = S.view?.chatDevice === m.deviceId && !document.hidden;
  if (msg.from === 'pc' && !viewing) {
    c.unread = (c.unread || 0) + 1;
    toast(`Nachricht von ${c.name}`, 'info', msg.text.slice(0, 160), 9000);
    notifyDesktop(`Nachricht von ${c.name}`, msg.text, `#/messages/${m.deviceId}`);
  }
  S.chats.set(m.deviceId, c);
  updateChatCount();
  S.view?.onChat?.(m);
}

// Desktop-Benachrichtigungen (pro Browser, siehe Einstellungen)
function notifyOn() { try { return localStorage.getItem('nrmm-notify') === '1' && 'Notification' in window && Notification.permission === 'granted'; } catch { return false; } }
function notifyDesktop(title, body, href) {
  if (!notifyOn() || !document.hidden) return;
  try {
    const n = new Notification(title, { body: body.slice(0, 200), icon: '/favicon.svg', tag: href });
    n.onclick = () => { window.focus(); location.hash = href; n.close(); };
  } catch { /* nicht verfügbar */ }
}

function chatBubbles(msgs) {
  if (!msgs.length) return `<div class="chat-empty">${icon('message', 22)}<p>Noch keine Nachrichten. Schreib etwas – am PC geht dann ein Fenster mit deiner Nachricht auf.</p></div>`;
  let lastDay = '';
  return msgs.map((m) => {
    const d = new Date(m.at);
    const day = d.toLocaleDateString('de-DE', { weekday: 'long', day: 'numeric', month: 'long' });
    const sep = day !== lastDay ? `<div class="chat-day">${esc(day)}</div>` : '';
    lastDay = day;
    const mine = m.from === 'admin';
    const meta = `${d.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' })}${mine ? (m.read ? ' · gelesen' : ' · zugestellt') : ''}`;
    return `${sep}<div class="msg ${mine ? 'mine' : 'theirs'}"><div class="bubble">${esc(m.text).replace(/\n/g, '<br>')}</div>
      <span class="msg-meta">${mine ? '' : `${esc(m.author || '')} · `}${esc(meta)}</span></div>`;
  }).join('');
}

// Unterhaltung mit einem PC (für Seite „Nachrichten“ und den Geräte-Tab)
class ChatThread {
  constructor(devId, el) { this.id = devId; this.el = el; this.msgs = []; }
  async load() {
    const d = S.devices.get(this.id);
    this.el.innerHTML = `<div class="chat">
      <div class="chat-log" id="clog" aria-live="polite"></div>
      <form class="chat-compose" id="cf"><textarea class="input" name="t" rows="2" maxlength="2000" placeholder="Antwort an ${esc(d ? d.name : 'den PC')} …" title="Enter sendet, Umschalt + Enter = neue Zeile"></textarea>
        <button class="btn primary">${icon('send', 16)} Senden</button></form>
      <p class="small faint chat-hint" id="chint"></p></div>`;
    const f = $('#cf', this.el);
    f.t.onkeydown = (e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); f.requestSubmit(); } };
    f.onsubmit = async (e) => {
      e.preventDefault();
      const t = f.t.value.trim();
      if (!t) return;
      f.t.value = '';
      try {
        const r = await api('POST', `/api/devices/${this.id}/chat`, { text: t });
        $('#chint', this.el).textContent = r.delivered ? '' : 'Gerade ist niemand am PC angemeldet – die Nachricht erscheint, sobald sich jemand anmeldet.';
      } catch (ex) { f.t.value = t; toast('Nicht gesendet', 'error', ex.message); }
    };
    await this.refresh();
    f.t.focus();
  }
  async refresh() {
    const r = await api('GET', `/api/devices/${this.id}/chat`).catch(() => null);
    if (!r) return;
    this.msgs = r.messages;
    const c = S.chats.get(this.id);
    if (c) { c.unread = 0; updateChatCount(); }
    this.draw();
  }
  draw() {
    const log = $('#clog', this.el);
    if (!log) return;
    const atEnd = log.scrollHeight - log.scrollTop - log.clientHeight < 60;
    log.innerHTML = chatBubbles(this.msgs);
    if (atEnd || !this.drawn) log.scrollTop = log.scrollHeight;
    this.drawn = true;
  }
  onChat(m) {
    if (m.deviceId !== this.id) return;
    if (!this.msgs.some((x) => x.id === m.message.id)) this.msgs.push(m.message);
    this.draw();
    if (m.message.from === 'pc') api('GET', `/api/devices/${this.id}/chat`).then((r) => { this.msgs = r.messages; this.draw(); }).catch(() => {}); // als gelesen markieren
  }
  onChatRead(m) { if (m.deviceId === this.id && m.by === 'pc') this.refresh(); }
}

class MessagesView {
  constructor(id) { this.sel = id; }
  get chatDevice() { return this.sel; }
  render(main) {
    this.main = main;
    main.innerHTML = `<div class="page-head"><div class="grow"><h1>Nachrichten</h1><p>Was die Leute an ihren PCs dir schreiben. Am PC: Startmenü › „Nachricht an Nicolaj“ oder das n-Symbol unten rechts.</p></div>
      <button class="btn" id="newmsg">${icon('plus')} Neue Nachricht</button></div>
      <div class="msg-layout ${this.sel ? 'has-sel' : ''}"><aside class="card conv-list" id="conv"></aside><section class="card conv-pane" id="pane"></section></div>`;
    $('#newmsg', main).onclick = () => this.pickDevice();
    this.drawList();
    this.open(this.sel);
  }
  drawList() {
    const box = $('#conv', this.main);
    const list = [...S.chats.values()].filter((c) => S.devices.has(c.deviceId)).sort((a, b) => String(b.last?.at).localeCompare(String(a.last?.at)));
    if (this.sel && !list.some((c) => c.deviceId === this.sel) && S.devices.has(this.sel)) list.unshift({ deviceId: this.sel, name: S.devices.get(this.sel).name, unread: 0 });
    box.innerHTML = list.length ? list.map((c) => {
      const d = S.devices.get(c.deviceId);
      const last = c.last ? `${c.last.from === 'admin' ? 'Du: ' : ''}${c.last.text}` : 'Noch keine Nachrichten';
      return `<a class="conv ${c.deviceId === this.sel ? 'active' : ''} ${c.unread ? 'unread' : ''}" href="#/messages/${esc(c.deviceId)}">
        <div class="dev-ico ${d?.online ? 'on' : ''}">${icon('monitor', 16)}</div>
        <div class="grow"><div class="conv-top"><b>${esc(d?.name || c.name)}</b><span>${c.last ? esc(ago(c.last.at)) : ''}</span></div>
          <span class="conv-last">${esc(last.slice(0, 90))}</span></div>${c.unread ? `<em class="badge-n">${c.unread}</em>` : ''}</a>`;
    }).join('') : `<div class="conv-none"><p>Noch keine Unterhaltungen.</p><p class="small faint">Sobald jemand am PC schreibt, erscheint es hier.</p></div>`;
  }
  async open(id) {
    const pane = $('#pane', this.main);
    if (!id || !S.devices.has(id)) {
      pane.innerHTML = `<div class="conv-placeholder">${icon('message', 26)}<p>Wähle links eine Unterhaltung oder schreib einem PC eine neue Nachricht.</p></div>`;
      return;
    }
    const d = S.devices.get(id);
    pane.innerHTML = `<header class="conv-head"><a class="btn ghost sm back-btn" href="#/messages">${icon('back', 15)}</a>
      <div class="dev-ico ${d.online ? 'on' : ''}">${icon('monitor', 18)}</div><div class="grow"><b>${esc(d.name)}</b>
      <span class="small muted">${d.screenOnline ? 'jemand ist angemeldet' : d.online ? 'online, niemand angemeldet' : 'offline'}</span></div>
      <a class="btn sm" href="#/d/${esc(id)}/overview">Gerät öffnen</a></header><div id="thread"></div>`;
    this.thread = new ChatThread(id, $('#thread', pane));
    await this.thread.load();
  }
  pickDevice() {
    const devs = [...S.devices.values()].filter((d) => !d.pending).sort((a, b) => a.name.localeCompare(b.name, 'de'));
    openModal({ title: 'Neue Nachricht', submitLabel: 'Weiter', body: `<label class="field">An welchen PC?<select class="input" name="d">${devs.map((d) => `<option value="${esc(d.id)}">${esc(d.name)}${d.screenOnline ? ' (angemeldet)' : ''}</option>`).join('')}</select></label>`,
      onSubmit: async (f) => { location.hash = `#/messages/${f.d.value}`; } });
  }
  onChat(m) { this.drawList(); this.thread?.onChat(m); }
  onChatRead(m) { this.drawList(); this.thread?.onChatRead(m); }
  onDevice() { this.drawList(); }
  onReconnect() { this.drawList(); this.thread?.refresh(); }
}

class ChatTab {
  constructor(v) { this.v = v; }
  render(el) {
    this.v.chatDevice = this.v.id;
    this.thread = new ChatThread(this.v.id, el);
    this.thread.load();
  }
  onChat(m) { this.thread.onChat(m); }
  onChatRead(m) { this.thread.onChatRead(m); }
}

// ---------- Zugangsanfragen ----------
function requestsCard(pend) {
  return `<section class="card req-card"><div class="req-head"><h2>Zugangsanfragen</h2><span class="muted">${pend.length} offen</span></div>
    <p class="muted req-intro">Diese PCs haben den allgemeinen Installer benutzt. Sie können nichts, bis du sie annimmst. Abgelehnte PCs entfernen das Programm von selbst.</p>
    ${pend.map((d) => `<div class="req-row">
      <div class="dev-ico ${d.online ? 'on' : ''}">${icon('monitor', 18)}</div>
      <div class="grow"><b>${esc(d.name)}</b>
        <span class="small muted">${esc(d.hostname || '–')} · ${esc(osShort(d))} · ${esc(d.lastIp || '')} · angefragt ${esc(ago(d.requestedAt))}</span></div>
      <button class="btn sm" data-rej="${esc(d.id)}">Ablehnen</button>
      <button class="btn sm primary" data-acc="${esc(d.id)}">${icon('check', 15)} Annehmen</button>
    </div>`).join('')}</section>`;
}
function bindRequests(root) {
  $$('[data-acc]', root).forEach((b) => { b.onclick = (e) => { e.stopPropagation(); approveDevice(b.dataset.acc); }; });
  $$('[data-rej]', root).forEach((b) => { b.onclick = (e) => { e.stopPropagation(); rejectDevice(b.dataset.rej); }; });
}
async function approveDevice(id) {
  const d = S.devices.get(id);
  try {
    await api('POST', `/api/devices/${id}/approve`);
    toast(`${d ? d.name : 'PC'} freigegeben`, 'ok', 'Du kannst den PC jetzt betreuen.');
  } catch (ex) { toast('Freigabe fehlgeschlagen', 'error', ex.message); }
}
async function rejectDevice(id) {
  const d = S.devices.get(id);
  if (!await confirmModal('Anfrage ablehnen?', `„${esc(d ? d.name : 'PC')}“ wird abgelehnt. Das Programm entfernt sich danach auf dem PC von selbst.`, 'Ablehnen', true)) return;
  try {
    await api('POST', `/api/devices/${id}/reject`);
    toast('Anfrage abgelehnt', 'info');
  } catch (ex) { toast('Ablehnen fehlgeschlagen', 'error', ex.message); }
}

// ---------- Sicherheit: gemeinsame Bewertung ----------
function secData(d) {
  const full = d.data?.security?.value;
  const df = inv(d).defender;
  if (full && df) {
    const invNewer = new Date(d.inventoryAt) > new Date(d.data.security.at);
    return invNewer ? { ...full, ...df } : { ...df, ...full };
  }
  return full || df || null;
}
function secState(d) {
  const df = secData(d);
  const others = (inv(d).antivirus || []).filter((p) => p.enabled && !/defender/i.test(p.name));
  if (!df) return { level: 'unknown', text: 'Keine Daten', hint: 'Der PC hat noch keinen Virenschutz-Status gemeldet.' };
  if (df.activeThreats > 0) return { level: 'danger', text: `${df.activeThreats} aktive Bedrohung${df.activeThreats > 1 ? 'en' : ''}`, hint: 'Sofort prüfen und entfernen.' };
  const pu = pausedUntil(d);
  if (pu) return { level: 'warn', text: `Schutz pausiert bis ${hhmm(pu)}`, hint: 'Für Reparaturen.', paused: true };
  if (/passive/i.test(df.mode || '') && others.length) return { level: 'info', text: `${others[0].name} aktiv`, hint: 'Ein anderer Virenscanner schützt den PC, der Defender läuft im Hintergrund.' };
  if (!df.enabled || !df.realtime) return { level: 'danger', text: 'Echtzeitschutz aus', hint: 'Der PC ist ungeschützt.' };
  if (df.signatureAgeDays > 3) return { level: 'warn', text: `Signaturen ${df.signatureAgeDays} Tage alt`, hint: 'Signaturen aktualisieren.' };
  if (df.quickScanAgeDays < 0 || df.quickScanAgeDays > 14) return { level: 'warn', text: 'Lange kein Scan', hint: 'Schnellscan starten.' };
  if (df.pua !== 1) return { level: 'warn', text: 'Härtung empfohlen', hint: 'Schutz vor unerwünschten Programmen (Adware, Toolbars) ist aus.' };
  return { level: 'ok', text: 'Geschützt', hint: 'Echtzeitschutz an, Signaturen aktuell, keine aktiven Funde.' };
}
function secBadge(d) {
  const st = secState(d);
  const cls = { ok: 'ok', warn: 'warn', danger: 'danger', info: 'accent' }[st.level] || '';
  return `<span class="pill ${cls}">${esc(st.text)}</span>`;
}
const SEVERITY = { 1: ['Niedrig', ''], 2: ['Mittel', 'warn'], 4: ['Hoch', 'danger'], 5: ['Schwer', 'danger'] };
const THREAT_STATUS = { 1: 'Erkannt', 2: 'Bereinigt', 3: 'In Quarantäne', 4: 'Entfernt', 5: 'Zugelassen', 6: 'Blockiert', 102: 'Quarantäne fehlgeschlagen', 103: 'Entfernen fehlgeschlagen', 104: 'Zulassen fehlgeschlagen', 105: 'Abgebrochen', 107: 'Blockieren fehlgeschlagen' };

// ---------- Gerät hinzufügen ----------
async function addDeviceModal() {
  if (!S.info) S.info = await api('GET', '/api/server-info').catch(() => null);
  let code = null;
  const m = openModal({
    title: 'Gerät hinzufügen', wide: true, submitLabel: 'Code erstellen',
    body: `<p>Du bekommst einen Installer, der nur für diesen einen PC gilt. Einfach auf dem PC starten – fertig.</p>
      <div class="form-grid two">
        <label class="field">Name im Dashboard<input class="input" name="label" maxlength="64" placeholder="z. B. Omas Laptop"><span class="hint">Leer lassen = PC-Name.</span></label>
        <label class="field">Gültig für<select class="input" name="hours"><option value="1">1 Stunde</option><option value="24" selected>24 Stunden</option><option value="168">7 Tage</option></select></label>
      </div>`,
    onSubmit: async (f) => {
      if (code) return false;
      const ec = await api('POST', '/api/enroll-codes', { label: f.label.value, hours: +f.hours.value });
      code = ec.code;
      showCode(f, ec);
      return false;
    },
  });
  function showCode(f, ec) {
    const addrs = S.info?.addresses || [];
    const cur = S.info?.current || location.origin;
    const isLocal = /\/\/(localhost|127\.)/.test(cur);
    const opts = [...new Set([isLocal ? null : cur, ...addrs].filter(Boolean))];
    const fp = S.info?.fingerprint || '';
    $('.modal-body', f.parentElement).innerHTML = `
      <div class="code-big">${esc(ec.code)}</div>
      <label class="field">Adresse, unter der der PC den Server erreicht
        <select class="input" id="srv">${opts.map((o) => `<option>${esc(o)}</option>`).join('')}</select>
        <span class="hint">${isLocal ? 'Achtung: „localhost“ funktioniert nur auf dem Server selbst – wähle die Netzwerk-Adresse.' : 'Im Heimnetz die 192.168.…-Adresse des Servers.'}</span></label>
      ${S.info && !S.info.agentAvailable ? `<div class="callout warn">${icon('alert')}<div>In diesem Server ist noch kein Agent eingebaut. Baue ihn mit <code>build.sh</code> bzw. <code>build.ps1</code>.</div></div>` : ''}
      <ol class="steps"><li>Installer herunterladen und auf den PC bringen (USB-Stick, E-Mail, Cloud).</li>
        <li>Doppelklick → Windows fragt nach Administratorrechten → <b>Ja</b>.</li>
        <li>Installation bestätigen. Der PC taucht hier automatisch auf.</li></ol>
      <details><summary class="muted small">Alternative: per Kommandozeile installieren</summary>
        <div class="copy-line" style="margin-top:8px"><code id="cli"></code><button type="button" class="btn sm" id="cpcli">${icon('copy')}</button></div></details>
      <div id="enrolled"></div>`;
    $('footer', f).innerHTML = `<button type="button" class="btn ghost" data-close>Schließen</button><a class="btn primary" id="dl">${icon('download')} Installer herunterladen</a>`;
    const upd = () => {
      const srv = $('#srv', f).value;
      $('#dl', f).href = `/api/enroll-codes/${encodeURIComponent(ec.code)}/installer?server=${encodeURIComponent(srv)}`;
      $('#cli', f).textContent = `nicolaj-agent.exe install --server ${srv} --code ${ec.code}${fp ? ' --fp ' + fp : ''}`;
    };
    $('#srv', f).onchange = upd;
    $('#cpcli', f).onclick = () => copyText($('#cli', f).textContent);
    upd();
    S.onEnroll = (d) => {
      if (!f.isConnected) { S.onEnroll = null; return; }
      $('#enrolled', f).innerHTML = `<div class="callout accent">${icon('check')}<div><b>${esc(d.name)}</b> ist verbunden – PC-ID <b class="mono">${esc(fmtPCID(d.pcId))}</b>. <a href="#/d/${esc(d.id)}/overview" data-close>Jetzt öffnen →</a></div></div>`;
    };
  }
  return m;
}

// ---------- Gerät ----------
const TABS = [
  ['overview', 'Übersicht', 'monitor'],
  ['screen', 'Bildschirm', 'eye'],
  ['users', 'Benutzer', 'users'],
  ['software', 'Software', 'package'],
  ['security', 'Sicherheit', 'shield'],
  ['shell', 'PowerShell', 'terminal'],
  ['tasks', 'Aufgaben', 'list'],
  ['chat', 'Nachrichten', 'message'],
  ['settings', 'Einstellungen', 'sliders'],
];

class DeviceView {
  constructor(id, tab) {
    this.id = id;
    this.tab = TABS.some((t) => t[0] === tab) ? tab : 'overview';
    this.dev = null;
    this.cmds = [];
  }
  async render(main) {
    this.main = main;
    main.innerHTML = '<p class="muted">Lade …</p>';
    try {
      [this.dev, this.cmds] = await Promise.all([api('GET', `/api/devices/${this.id}`), api('GET', `/api/devices/${this.id}/commands`)]);
    } catch (e) {
      main.innerHTML = `<div class="card empty"><h2>Gerät nicht gefunden</h2><p>${esc(e.message)}</p><a class="btn" href="#/">Zur Übersicht</a></div>`;
      return;
    }
    if (S.view !== this) return;
    main.innerHTML = `<a class="back" href="#/">${icon('back', 14)} Alle Geräte</a>
      <div class="dev-head" id="head"></div>
      <nav class="tabs">${TABS.map(([k, l, i]) => `<a href="#/d/${esc(this.id)}/${k}" class="${k === this.tab ? 'active' : ''}">${icon(i, 15)} ${l}</a>`).join('')}</nav>
      <div id="offline"></div><div id="tab"></div>`;
    this.drawHead();
    const T = { overview: OverviewTab, screen: ScreenTab, users: UsersTab, software: SoftwareTab, security: SecurityTab, shell: ShellTab, tasks: TasksTab, chat: ChatTab, settings: SettingsTab }[this.tab] || OverviewTab;
    this.tabObj = new T(this);
    this.tabObj.render($('#tab', main));
  }
  drawHead() {
    const d = this.dev;
    const head = $('#head', this.main);
    if (!head) return;
    const live = S.devices.get(d.id);
    if (live) Object.assign(d, { online: live.online, screenOnline: live.screenOnline, session: live.session });
    head.innerHTML = `<div class="dev-ico ${d.online ? 'on' : ''}">${icon('monitor', 24)}</div>
      <div class="grow"><h1>${esc(d.name)} ${statusPill(d)}</h1>
        <p><button type="button" class="pcid" id="cpid" title="PC-ID kopieren">ID ${esc(fmtPCID(d.pcId))} ${icon('copy', 13)}</button> · ${esc(d.hostname || '')} · ${esc(osShort(d))} · Benutzer: ${esc(loggedUser(d))}${d.online ? '' : ' · zuletzt online ' + esc(ago(d.lastSeen))}</p></div>
      ${this.tab !== 'screen' ? `<a class="btn ${d.screenOnline ? 'primary' : ''}" href="#/d/${esc(d.id)}/screen">${icon('eye')} Bildschirm</a>` : ''}`;
    $('#cpid', head).onclick = () => copyText(d.pcId || '');
    const off = $('#offline', this.main);
    if (d.pending) {
      off.innerHTML = `<div class="callout accent offline-note">${icon('info')}<div style="flex:1"><b>Wartet auf Freigabe.</b> „${esc(d.name)}“ hat ${esc(ago(d.requestedAt))} Zugang angefragt. Bis du annimmst, kannst du hier nichts ausführen.</div>
        <button class="btn sm" data-rej="${esc(d.id)}">Ablehnen</button><button class="btn sm primary" data-acc="${esc(d.id)}">${icon('check', 15)} Annehmen</button></div>`;
      bindRequests(off);
      return;
    }
    off.innerHTML = d.online ? '' : `<div class="callout warn offline-note">${icon('alert')}<div>Der PC ist gerade offline. Aufträge werden gesammelt und ausgeführt, sobald er wieder online ist (bis zu 24 Stunden).</div></div>`;
  }
  async refresh() {
    clearTimeout(this.rt);
    this.rt = setTimeout(async () => {
      try {
        const d = await api('GET', `/api/devices/${this.id}`);
        if (S.view !== this) return;
        this.dev = d;
        this.drawHead();
        this.tabObj?.update?.();
      } catch { /* ignorieren */ }
    }, 250);
  }
  onDevice(d) { if (d.id === this.id) this.refresh(); }
  onChat(m) { this.tabObj?.onChat?.(m); }
  onChatRead(m) { this.tabObj?.onChatRead?.(m); }
  onDeviceRemoved(id) { if (id === this.id) { toast('Gerät wurde entfernt', 'info'); location.hash = '#/'; } }
  onCommand(c) {
    if (c.deviceId !== this.id || !this.dev) return;
    const i = this.cmds.findIndex((x) => x.id === c.id);
    if (i >= 0) this.cmds[i] = c; else this.cmds.unshift(c);
    this.tabObj?.onCommand?.(c);
  }
  onScreen(m) {
    if (m.deviceId !== this.id || !this.dev) return;
    this.dev.session = m.session;
    this.drawHead();
    this.tabObj?.onScreen?.(m);
  }
  onReconnect() { this.refresh(); }
  destroy() { this.tabObj?.destroy?.(); clearTimeout(this.rt); }
  cmd(type, args, opts) { return sendCommand(this.id, type, args, opts); }
  running(prefix) { return this.cmds.filter((c) => (c.status === 'sent' || c.status === 'queued') && c.type.startsWith(prefix)); }
}

// ---------- Übersicht ----------
class OverviewTab {
  constructor(v) { this.v = v; }
  render(el) {
    this.el = el;
    el.innerHTML = `<div class="quick">
        <button class="btn" data-a="restart">${icon('refresh')} Neu starten</button>
        <button class="btn" data-a="shutdown">${icon('power')} Herunterfahren</button>
        <button class="btn ghost" data-a="abort">${icon('x')} Neustart abbrechen</button>
        <button class="btn" data-a="lock">${icon('lock')} Sperren</button>
        <button class="btn" data-a="msg">${icon('message')} Nachricht senden</button>
        <span class="spacer"></span>
        <button class="btn ghost" data-a="inv" title="Systeminfo neu laden">${icon('refresh')} Aktualisieren</button>
      </div><div id="ov"></div>`;
    el.querySelector('.quick').onclick = (e) => { const b = e.target.closest('[data-a]'); if (b) this.action(b.dataset.a); };
    this.update();
  }
  action(a) {
    const v = this.v;
    if (a === 'inv') return v.cmd('inventory', {}, { quiet: true }).then(() => toast('Systeminfo wird geladen …'));
    if (a === 'abort') return v.cmd('power.abort');
    if (a === 'lock') return v.cmd('user.lock');
    if (a === 'msg') {
      return openModal({
        title: 'Nachricht an den PC', submitLabel: 'Anzeigen',
        body: `<p>Erscheint als Fenster auf dem Bildschirm der angemeldeten Person.</p>
          <label class="field">Nachricht<textarea class="input" name="text" rows="4" maxlength="1000" required>Hallo! Ich schaue mir deinen PC gleich kurz an. Viele Grüße, Nicolaj</textarea></label>`,
        onSubmit: async (f) => { await v.cmd('user.message', { text: f.text.value }); },
      });
    }
    const restart = a === 'restart';
    openModal({
      title: restart ? 'PC neu starten' : 'PC herunterfahren', submitLabel: restart ? 'Neu starten' : 'Herunterfahren', danger: true,
      body: `<label class="field">Wann?<select class="input" name="delay">
          <option value="0">Sofort</option><option value="60" selected>In 1 Minute</option><option value="300">In 5 Minuten</option><option value="900">In 15 Minuten</option></select></label>
        <label class="field">Hinweis für die Person am PC <span class="hint">optional</span><input class="input" name="message" maxlength="200" placeholder="${restart ? 'Nicolaj Support startet diesen PC neu …' : 'Nicolaj Support fährt diesen PC herunter …'}"></label>
        ${restart ? '' : '<div class="callout warn">' + icon('alert') + '<div>Nach dem Herunterfahren ist der PC nicht mehr erreichbar, bis ihn jemand wieder einschaltet.</div></div>'}`,
      onSubmit: async (f) => { await v.cmd(restart ? 'power.restart' : 'power.shutdown', { delay: +f.delay.value, message: f.message.value }); },
    });
  }
  update() {
    const d = this.v.dev, i = inv(d), box = $('#ov', this.el);
    if (!box) return;
    if (!d.inventory) {
      box.innerHTML = `<div class="card empty"><h2>Noch keine Systeminfos</h2><p>${d.online ? 'Der PC meldet sie in wenigen Sekunden.' : 'Sobald der PC online ist, erscheinen hier die Infos.'}</p></div>`;
      return;
    }
    const ramUsed = i.ramGB && i.ramFreeGB != null ? (i.ramGB - i.ramFreeGB) / i.ramGB * 100 : 0;
    const lvl = (p) => p > 92 ? 'danger' : p > 80 ? 'warn' : '';
    const disks = (i.disks || []).map((k) => {
      const used = k.sizeGB ? (k.sizeGB - k.freeGB) / k.sizeGB * 100 : 0;
      return `<div style="margin-bottom:10px"><div class="row"><b>${esc(k.name)}</b><span class="spacer"></span><span class="muted small">${num(k.freeGB)} GB frei von ${num(k.sizeGB)} GB</span></div>
        <div class="bar ${lvl(used)}"><i data-w="${used.toFixed(1)}"></i></div></div>`;
    }).join('') || '<p class="muted">–</p>';
    const def = i.defender;
    const defTxt = !def ? '<span class="badge">unbekannt</span>' : (def.enabled && def.realtime) ? '<span class="badge accent">aktiv</span>' : '<span class="badge danger">AUS</span>';
    box.innerHTML = `<div class="ov-grid">
      <section class="card ov-card"><h3>${icon('monitor', 14)} System</h3><dl class="kv">
        <dt>Windows</dt><dd title="${esc(i.os)}">${esc(osShort(d))}</dd>
        <dt>Version</dt><dd>${esc(i.osVersion || '–')}</dd>
        <dt>Gerät</dt><dd title="${esc((i.manufacturer || '') + ' ' + (i.model || ''))}">${esc([i.manufacturer, i.model].filter(Boolean).join(' ') || '–')}</dd>
        <dt>Seriennr.</dt><dd class="mono">${esc(i.serial || '–')}</dd>
        <dt>Läuft seit</dt><dd>${i.uptimeHours != null ? (i.uptimeHours < 48 ? num(i.uptimeHours) + ' Std.' : num(i.uptimeHours / 24) + ' Tagen') : '–'}</dd>
        <dt>Netzwerk</dt><dd>${esc(i.partOfDomain ? 'Domäne ' + i.domain : 'Arbeitsgruppe ' + (i.domain || ''))}</dd>
      </dl></section>
      <section class="card ov-card"><h3>${icon('cpu', 14)} Hardware</h3>
        <p class="small muted" style="margin:0 0 10px">${esc(i.cpu || '–')} · ${esc(i.cores || '?')} Kerne</p>
        <div class="row"><span class="big-num">${num(i.ramGB)} GB</span><span class="muted">Arbeitsspeicher</span></div>
        <div class="bar ${lvl(ramUsed)}"><i data-w="${ramUsed.toFixed(1)}"></i></div>
        <span class="muted small">${num(i.ramFreeGB)} GB frei</span></section>
      <section class="card ov-card"><h3>${icon('disk', 14)} Festplatten</h3>${disks}</section>
      <section class="card ov-card"><h3>${icon('shield', 14)} Sicherheit &amp; Updates</h3><dl class="kv">
        <dt>Virenschutz</dt><dd>${defTxt}${def && def.signatureAgeDays > 3 ? ` <span class="badge warn">Signaturen ${def.signatureAgeDays} Tage alt</span>` : ''}</dd>
        <dt>Neustart nötig</dt><dd>${i.rebootPending ? '<span class="badge warn">ja – Updates warten</span>' : 'nein'}</dd>
        <dt>Letztes Update</dt><dd>${esc(i.lastHotfix || '–')}</dd>
      </dl></section>
      <section class="card ov-card"><h3>${icon('wifi', 14)} Verbindung</h3><dl class="kv">
        <dt>PC-ID</dt><dd class="mono">${esc(fmtPCID(d.pcId))}</dd>
        <dt>IP-Adressen</dt><dd>${esc((i.ips || []).join(', ') || '–')}</dd>
        <dt>Gesehen von</dt><dd>${esc(d.lastIp || '–')}</dd>
        <dt>Angemeldet</dt><dd>${esc(loggedUser(d))}</dd>
        <dt>Agent</dt><dd>v${esc(d.agentVersion || '?')}</dd>
        <dt>Stand</dt><dd>${esc(ago(d.inventoryAt))}</dd>
      </dl></section>
    </div>`;
    applyBars(box);
  }
}

// ---------- Bildschirm ----------
const SPECIAL_KEYS = [
  ['Windows-Taste', 'Win', [['MetaLeft', 1], ['MetaLeft', 0]]],
  ['Task-Manager', 'Strg+Umschalt+Esc', [['ControlLeft', 1], ['ShiftLeft', 1], ['Escape', 1], ['Escape', 0], ['ShiftLeft', 0], ['ControlLeft', 0]]],
  ['Fenster wechseln', 'Alt+Tab', [['AltLeft', 1], ['Tab', 1], ['Tab', 0], ['AltLeft', 0]]],
  ['Ausführen', 'Win+R', [['MetaLeft', 1], ['KeyR', 1], ['KeyR', 0], ['MetaLeft', 0]]],
  ['Explorer', 'Win+E', [['MetaLeft', 1], ['KeyE', 1], ['KeyE', 0], ['MetaLeft', 0]]],
  ['Desktop zeigen', 'Win+D', [['MetaLeft', 1], ['KeyD', 1], ['KeyD', 0], ['MetaLeft', 0]]],
  ['Fenster schließen', 'Alt+F4', [['AltLeft', 1], ['F4', 1], ['F4', 0], ['AltLeft', 0]]],
  ['Eingabe', 'Enter', [['Enter', 1], ['Enter', 0]]],
  ['Escape', 'Esc', [['Escape', 1], ['Escape', 0]]],
];

class ScreenTab {
  constructor(v) {
    this.v = v;
    this.session = v.dev.session;
    this.queue = [];
    this.sending = false;
    this.pressed = new Set();
    this.fit = true;
  }
  render(el) {
    this.el = el;
    el.innerHTML = `<div class="screen-bar">
        <button class="btn primary" id="conn">${icon('play')} Verbinden</button>
        <select class="input" id="q" title="Bildqualität"><option value="high">Hohe Qualität</option><option value="medium" selected>Ausgewogen</option><option value="low">Langsame Leitung</option></select>
        <button class="btn" id="fit" title="Größe umschalten">${icon('monitor')} <span>Anpassen</span></button>
        <button class="btn" id="fs" title="Vollbild">${icon('expand')} Vollbild</button>
        <details class="menu"><summary class="btn">${icon('keyboard')} Tasten</summary>
          <div class="menu-pop keys-pop">${SPECIAL_KEYS.map(([l, k], i) => `<button type="button" data-k="${i}">${esc(l)}<kbd>${esc(k)}</kbd></button>`).join('')}
            <hr><button type="button" disabled title="Diese Kombination fängt Windows aus Sicherheitsgründen ab.">Strg+Alt+Entf<kbd>nicht möglich</kbd></button></div></details>
        <span class="spacer"></span>
        <form class="input-group" id="txt" style="min-width:min(320px,100%)"><input class="input" name="t" placeholder="Text an den PC senden (z. B. Passwort)" autocomplete="off"><button class="btn">Tippen</button></form>
      </div>
      <div class="viewer fit" id="viewer" tabindex="0">
        <img id="img" alt="" draggable="false" hidden>
        <div class="overlay" id="ovl"></div>
        <div class="kbd-hint" id="hint" hidden>Tastatur wird an den PC gesendet</div>
      </div>
      <p class="small faint" style="margin-top:10px">Klick ins Bild, um Maus und Tastatur zu steuern. Die Person am PC sieht währenddessen oben eine Hinweisleiste und kann jederzeit trennen.</p>`;
    this.img = $('#img', el);
    this.viewer = $('#viewer', el);
    $('#conn', el).onclick = () => (this.session ? this.stop() : this.start());
    $('#q', el).onchange = () => this.setQuality();
    $('#fit', el).onclick = () => { this.fit = !this.fit; this.viewer.classList.toggle('fit', this.fit); this.viewer.classList.toggle('native', !this.fit); $('#fit span', el).textContent = this.fit ? 'Anpassen' : 'Originalgröße'; };
    $('#fs', el).onclick = () => (document.fullscreenElement ? document.exitFullscreen() : this.viewer.requestFullscreen?.());
    $('.keys-pop', el).onclick = (e) => {
      const b = e.target.closest('[data-k]');
      if (!b || !this.active()) return;
      SPECIAL_KEYS[+b.dataset.k][2].forEach(([c, d]) => this.push({ t: 'k', c, d: !!d }));
    };
    $('#txt', el).onsubmit = (e) => {
      e.preventDefault();
      const t = e.target.t.value;
      if (!t || !this.active()) return;
      this.push({ t: 'text', s: t });
      e.target.t.value = '';
      toast('Text wird getippt', 'info');
    };
    this.bindInput();
    this.draw();
    if (this.session) this.attach(); // laufende Sitzung weiter anzeigen – neue nur per Klick auf „Verbinden“
  }
  active() { return this.session && this.session.state === 'active'; }
  async start() {
    try {
      this.session = await api('POST', `/api/devices/${this.v.id}/screen/start`, { quality: $('#q', this.el).value });
      this.attach();
    } catch (e) { this.message = e.message; }
    this.draw();
  }
  async stop() {
    await api('POST', `/api/devices/${this.v.id}/screen/stop`).catch(() => {});
    this.session = null;
    this.message = 'Verbindung getrennt.';
    this.detach();
    this.draw();
  }
  attach() {
    if (!this.session) return;
    if (this.img.dataset.sid !== this.session.id) {
      this.img.dataset.sid = this.session.id;
      this.img.src = `/api/devices/${this.v.id}/screen/stream?session=${this.session.id}&t=${Date.now()}`;
    }
  }
  detach() {
    this.img.removeAttribute('src');
    delete this.img.dataset.sid;
    this.releaseKeys();
  }
  setQuality() {
    if (!this.session) return;
    api('POST', `/api/devices/${this.v.id}/screen/config`, { sessionId: this.session.id, quality: $('#q', this.el).value }).catch(() => {});
  }
  draw() {
    const ovl = $('#ovl', this.el), btn = $('#conn', this.el), d = this.v.dev;
    btn.innerHTML = this.session ? `${icon('stop')} Trennen` : `${icon('play')} Verbinden`;
    btn.className = this.session ? 'btn danger' : 'btn primary';
    btn.disabled = !this.session && !d.screenOnline;
    const show = this.active();
    this.img.hidden = !show && !this.session;
    if (show) { ovl.hidden = true; return; }
    ovl.hidden = false;
    if (this.session) {
      ovl.innerHTML = `<div><div class="spin" style="margin:0 auto 14px;width:28px;height:28px"></div><h2>Verbindung wird aufgebaut …</h2>
        <p>${d.consentRequired ? 'Am PC erscheint jetzt die Frage, ob du dich verbinden darfst. Die Person muss auf „Erlauben“ klicken.' : 'Einen Moment.'}</p></div>`;
    } else if (!d.screenOnline) {
      ovl.innerHTML = `<div><h2>Bildschirm nicht verfügbar</h2><p>${d.online ? 'Gerade ist niemand am PC angemeldet. Der Bildschirm ist erst nach der Windows-Anmeldung verfügbar.' : 'Der PC ist offline.'}</p></div>`;
    } else {
      const again = !!this.message;
      ovl.innerHTML = `<div class="screen-idle">
        <div class="screen-idle-icon">${icon('monitor', 26)}</div>
        <h2>${again ? esc(this.message) : `Bildschirm von ${esc(d.name)}`}</h2>
        <p>${d.consentRequired
          ? 'Erst wenn du auf „Verbinden“ klickst, erscheint am PC eine Frage. Die Person muss sie mit „Erlauben“ bestätigen.'
          : 'Verbindet sofort ohne Rückfrage – am PC erscheint nur die Hinweisleiste.'}</p>
        <button class="btn primary lg" id="conn2">${icon('play')} ${again ? 'Nochmal verbinden' : 'Verbinden'}</button></div>`;
      $('#conn2', ovl).onclick = () => this.start();
    }
  }
  onScreen(m) {
    if (m.session) {
      this.session = m.session;
      this.attach();
    } else if (this.session) {
      this.session = null;
      this.message = m.reason || 'Verbindung beendet.';
      this.detach();
    }
    this.draw();
  }
  update() { this.draw(); }

  // --- Eingaben ---
  pos(e) {
    const r = this.img.getBoundingClientRect();
    const nw = this.img.naturalWidth || r.width, nh = this.img.naturalHeight || r.height;
    let w = r.width, h = r.height, ox = 0, oy = 0;
    if (this.fit) {
      const s = Math.min(r.width / nw, r.height / nh);
      w = nw * s; h = nh * s; ox = (r.width - w) / 2; oy = (r.height - h) / 2;
    }
    const x = (e.clientX - r.left - ox) / w, y = (e.clientY - r.top - oy) / h;
    return { x: Math.min(1, Math.max(0, x)), y: Math.min(1, Math.max(0, y)), inside: x >= 0 && x <= 1 && y >= 0 && y <= 1 };
  }
  push(ev) {
    if (!this.active()) return;
    const last = this.queue[this.queue.length - 1];
    if (ev.t === 'm' && last && last.t === 'm') this.queue[this.queue.length - 1] = ev;
    else this.queue.push(ev);
    if (!this.flushT) this.flushT = setTimeout(() => { this.flushT = null; this.flush(); }, 25);
  }
  async flush() {
    if (this.sending || !this.queue.length || !this.session) return;
    this.sending = true;
    const events = this.queue.splice(0, 250);
    try { await api('POST', `/api/devices/${this.v.id}/screen/input`, { sessionId: this.session.id, events }); } catch { /* Sitzung evtl. beendet */ }
    this.sending = false;
    if (this.queue.length) this.flush();
  }
  releaseKeys() {
    if (this.session) this.pressed.forEach((c) => this.push({ t: 'k', c, d: false }));
    this.pressed.clear();
  }
  bindInput() {
    const img = this.img, vw = this.viewer;
    let touch = null;
    img.addEventListener('pointermove', (e) => {
      if (e.pointerType !== 'mouse') return;
      const p = this.pos(e);
      if (p.inside) this.push({ t: 'm', x: p.x, y: p.y });
    });
    img.addEventListener('pointerdown', (e) => {
      vw.focus();
      const p = this.pos(e);
      if (e.pointerType !== 'mouse') { touch = { x: e.clientX, y: e.clientY, t: Date.now(), p }; return; }
      e.preventDefault();
      img.setPointerCapture(e.pointerId);
      this.push({ t: 'b', b: e.button === 2 ? 2 : e.button === 1 ? 1 : 0, d: true, x: p.x, y: p.y });
    });
    img.addEventListener('pointerup', (e) => {
      const p = this.pos(e);
      if (e.pointerType !== 'mouse') {
        if (touch && Math.hypot(e.clientX - touch.x, e.clientY - touch.y) < 12) {
          const b = Date.now() - touch.t > 550 ? 2 : 0; // langes Tippen = Rechtsklick
          this.push({ t: 'm', x: p.x, y: p.y });
          this.push({ t: 'b', b, d: true, x: p.x, y: p.y });
          this.push({ t: 'b', b, d: false, x: p.x, y: p.y });
        }
        touch = null;
        return;
      }
      this.push({ t: 'b', b: e.button === 2 ? 2 : e.button === 1 ? 1 : 0, d: false, x: p.x, y: p.y });
    });
    img.addEventListener('contextmenu', (e) => e.preventDefault());
    img.addEventListener('wheel', (e) => {
      if (!this.active()) return;
      e.preventDefault();
      const p = this.pos(e);
      const notches = e.deltaMode === 1 ? e.deltaY / 3 : e.deltaY / 100;
      const dy = Math.max(-1200, Math.min(1200, Math.round(-notches * 120)));
      if (dy) this.push({ t: 'w', dy, x: p.x, y: p.y });
    }, { passive: false });
    vw.addEventListener('focus', () => { vw.classList.add('focused'); $('#hint', this.el).hidden = !this.active(); });
    vw.addEventListener('blur', () => { vw.classList.remove('focused'); $('#hint', this.el).hidden = true; this.releaseKeys(); });
    vw.addEventListener('keydown', (e) => {
      if (!this.active() || !e.code) return;
      e.preventDefault();
      this.pressed.add(e.code);
      this.push({ t: 'k', c: e.code, d: true });
    });
    vw.addEventListener('keyup', (e) => {
      if (!this.active() || !e.code) return;
      e.preventDefault();
      this.pressed.delete(e.code);
      this.push({ t: 'k', c: e.code, d: false });
    });
    this.onUnload = () => {
      if (this.session) fetch(`/api/devices/${this.v.id}/screen/stop`, { method: 'POST', headers: { 'X-NRMM': '1' }, keepalive: true });
    };
    window.addEventListener('pagehide', this.onUnload);
  }
  destroy() {
    window.removeEventListener('pagehide', this.onUnload);
    if (this.session) api('POST', `/api/devices/${this.v.id}/screen/stop`).catch(() => {});
    this.detach();
  }
}

// ---------- Benutzer ----------
class UsersTab {
  constructor(v) { this.v = v; this.filter = ''; }
  render(el) {
    this.el = el;
    el.innerHTML = `<div class="toolbar">
        <button class="btn primary" id="new">${icon('plus')} Benutzer anlegen</button>
        <button class="btn ghost" id="reload">${icon('refresh')} Neu laden</button>
        <span class="spacer"></span><span class="faint small" id="stand"></span></div>
      <div class="card" id="box"></div>
      <p class="small faint" style="margin-top:10px">Lokale Windows-Konten dieses PCs. Bei <b>Microsoft-Konten</b> kann das Passwort nur online unter account.microsoft.com geändert werden.</p>`;
    $('#new', el).onclick = () => this.createModal();
    $('#reload', el).onclick = () => this.v.cmd('users.list', {}, { quiet: true });
    $('#box', el).onclick = (e) => { const b = e.target.closest('[data-act]'); if (b) this.act(b.dataset.act, b.dataset.user); };
    this.update();
    if (!this.v.dev.data?.users && this.v.dev.online) this.v.cmd('users.list', {}, { quiet: true }).catch(() => {});
  }
  users() { return this.v.dev.data?.users?.value || null; }
  update() {
    const box = $('#box', this.el), list = this.users();
    if (!box) return;
    const run = this.v.running('users.');
    $('#stand', this.el).textContent = this.v.dev.data?.users ? `Stand: ${ago(this.v.dev.data.users.at)}` : '';
    if (!list) {
      box.innerHTML = `<div class="empty">${run.length || this.v.dev.online ? '<div class="spin" style="margin:0 auto 10px"></div>Benutzer werden geladen …' : 'Der PC ist offline – noch keine Benutzerliste vorhanden.'}</div>`;
      return;
    }
    box.innerHTML = `<div class="table-wrap"><table class="table"><thead><tr><th>Benutzer</th><th>Typ</th><th>Status</th><th>Letzte Anmeldung</th><th></th></tr></thead><tbody>
      ${list.map((u) => `<tr>
        <td><div class="row" style="gap:10px;flex-wrap:nowrap"><span class="dev-ico" style="width:32px;height:32px;border-radius:50%">${icon('user', 16)}</span>
          <div><div class="name">${esc(u.name)} ${u.admin ? '<span class="badge accent">Admin</span>' : ''}</div>${u.fullName ? `<div class="sub">${esc(u.fullName)}</div>` : ''}</div></div></td>
        <td>${u.source === 'MicrosoftAccount' ? '<span class="badge">Microsoft-Konto</span>' : u.builtin ? '<span class="badge">Eingebaut</span>' : '<span class="badge">Lokal</span>'}</td>
        <td>${u.enabled ? '<span class="pill ok">Aktiv</span>' : '<span class="pill">Deaktiviert</span>'}</td>
        <td class="muted">${u.lastLogon ? esc(fmtDate(u.lastLogon)) : 'nie'}</td>
        <td class="actions"><details class="menu"><summary class="btn sm">Aktionen ${icon('more', 14)}</summary><div class="menu-pop">
          <button type="button" data-act="reset" data-user="${esc(u.name)}" ${u.source === 'MicrosoftAccount' ? 'disabled title="Microsoft-Konto"' : ''}>${icon('key')} Passwort zurücksetzen</button>
          <button type="button" data-act="${u.admin ? 'unadmin' : 'admin'}" data-user="${esc(u.name)}">${icon('shield')} ${u.admin ? 'Admin-Rechte entfernen' : 'Zum Administrator machen'}</button>
          <button type="button" data-act="${u.enabled ? 'disable' : 'enable'}" data-user="${esc(u.name)}">${icon(u.enabled ? 'lock' : 'check')} ${u.enabled ? 'Deaktivieren' : 'Aktivieren'}</button>
          ${u.builtin ? '' : `<hr><button type="button" class="danger" data-act="delete" data-user="${esc(u.name)}">${icon('trash')} Löschen</button>`}
        </div></details></td></tr>`).join('')}
      </tbody></table></div>${run.length ? `<div class="running" style="padding:10px 14px;border-top:1px solid var(--border)">${run.map((c) => `<div><span class="spin"></span>${esc(c.summary)}</div>`).join('')}</div>` : ''}`;
  }
  onCommand(c) { if (c.type.startsWith('users.')) this.update(); }
  createModal() {
    openModal({
      title: 'Benutzer anlegen', submitLabel: 'Anlegen',
      body: `<div class="form-grid two">
          <label class="field">Benutzername<input class="input" name="username" required maxlength="20" placeholder="z. B. oma" autocomplete="off"></label>
          <label class="field">Voller Name <span class="hint">optional</span><input class="input" name="fullName" maxlength="64" placeholder="Helga Muster"></label></div>
        ${passwordField('password', 'Passwort')}
        <label class="check"><input type="checkbox" name="admin"> <span>Administrator<small>Darf Programme installieren und Einstellungen ändern.</small></span></label>
        <label class="check"><input type="checkbox" name="mustChange"> <span>Passwort bei der ersten Anmeldung ändern</span></label>
        <label class="check"><input type="checkbox" name="neverExpires" checked> <span>Passwort läuft nie ab</span></label>`,
      onSubmit: async (f) => {
        await this.v.cmd('users.create', {
          username: f.username.value, fullName: f.fullName.value, password: f.password.value,
          admin: f.admin.checked, mustChange: f.mustChange.checked, neverExpires: f.neverExpires.checked,
        });
      },
    });
  }
  act(a, user) {
    const v = this.v;
    if (a === 'reset') {
      return openModal({
        title: `Passwort von „${user}“ zurücksetzen`, submitLabel: 'Zurücksetzen',
        body: `${passwordField()}<label class="check"><input type="checkbox" name="mustChange"> <span>Muss das Passwort bei der nächsten Anmeldung ändern</span></label>`,
        onSubmit: async (f) => { await v.cmd('users.resetPassword', { username: user, password: f.password.value, mustChange: f.mustChange.checked }); },
      });
    }
    if (a === 'admin' || a === 'unadmin') return v.cmd('users.setAdmin', { username: user, admin: a === 'admin' });
    if (a === 'enable' || a === 'disable') return v.cmd('users.setEnabled', { username: user, enabled: a === 'enable' });
    if (a === 'delete') {
      return openModal({
        title: `„${user}“ löschen?`, submitLabel: 'Endgültig löschen', danger: true,
        body: `<p>Das Benutzerkonto wird von diesem PC entfernt.</p>
          <label class="check"><input type="checkbox" name="deleteProfile"> <span>Auch alle Dateien löschen<small>Dokumente, Bilder, Desktop dieses Benutzers – nicht wiederherstellbar.</small></span></label>`,
        onSubmit: async (f) => { await v.cmd('users.delete', { username: user, deleteProfile: f.deleteProfile.checked }); },
      });
    }
  }
}

// ---------- Software ----------
const CATALOG = [
  ['Google Chrome', 'Google.Chrome'], ['Firefox', 'Mozilla.Firefox.de'], ['Thunderbird', 'Mozilla.Thunderbird.de'],
  ['VLC Player', 'VideoLAN.VLC'], ['7-Zip', '7zip.7zip'], ['Adobe Reader', 'Adobe.Acrobat.Reader.64-bit'],
  ['LibreOffice', 'TheDocumentFoundation.LibreOffice'], ['Zoom', 'Zoom.Zoom'], ['Notepad++', 'Notepad++.Notepad++'],
];

class SoftwareTab {
  constructor(v) { this.v = v; this.filter = ''; }
  render(el) {
    this.el = el;
    el.innerHTML = `<div id="running"></div>
      <div class="two-col">
        <section class="card card-pad"><h2 style="margin-bottom:12px">Schnell installieren</h2><div class="chips" id="cat"></div>
          <form class="input-group" id="sf" style="margin-top:16px"><input class="input" name="q" placeholder="Andere Software suchen, z. B. „Paint.NET“" minlength="2" maxlength="80" required><button class="btn">${icon('search')} Suchen</button></form>
          <div id="search"></div></section>
        <section class="card card-pad"><div class="row" style="margin-bottom:12px"><h2>Updates</h2><span class="spacer"></span>
            <button class="btn sm" id="chk">${icon('refresh', 14)} Prüfen</button><button class="btn sm primary" id="all">Alle aktualisieren</button></div>
          <div id="upd"></div></section>
      </div>
      <section class="card section"><header><h2>Installierte Programme</h2><input class="input" id="flt" type="search" placeholder="Filtern …" style="max-width:220px;height:32px">
          <button class="btn sm ghost" id="rl">${icon('refresh', 14)}</button></header><div id="apps"></div></section>`;
    $('#cat', el).onclick = (e) => { const b = e.target.closest('[data-id]'); if (b) this.install(b.dataset.id, b.dataset.name); };
    $('#sf', el).onsubmit = (e) => { e.preventDefault(); this.v.cmd('software.search', { query: e.target.q.value }, { quiet: true }); this.searching = true; this.drawSearch(); };
    $('#search', el).onclick = (e) => { const b = e.target.closest('[data-id]'); if (b) this.install(b.dataset.id, b.dataset.name); };
    $('#upd', el).onclick = (e) => { const b = e.target.closest('[data-up]'); if (b) this.v.cmd('software.upgrade', { id: b.dataset.up, name: b.dataset.name }); };
    $('#chk', el).onclick = () => this.v.cmd('software.updates', {}, { quiet: true });
    $('#all', el).onclick = async () => { if (await confirmModal('Alle Programme aktualisieren?', 'Alle Programme mit verfügbarem Update werden im Hintergrund aktualisiert. Das kann einige Minuten dauern; offene Programme werden dabei evtl. kurz geschlossen.', 'Aktualisieren')) this.v.cmd('software.upgradeAll'); };
    $('#flt', el).oninput = (e) => { this.filter = e.target.value.toLowerCase(); this.drawApps(); };
    $('#rl', el).onclick = () => this.v.cmd('software.list', {}, { quiet: true });
    $('#apps', el).onclick = async (e) => {
      const b = e.target.closest('[data-un]');
      if (b && await confirmModal('Programm deinstallieren?', `„${esc(b.dataset.un)}“ wird von diesem PC entfernt.`, 'Deinstallieren', true)) this.v.cmd('software.uninstall', { name: b.dataset.un });
    };
    this.update();
    const d = this.v.dev;
    if (d.online && !d.data?.software) this.v.cmd('software.list', {}, { quiet: true }).catch(() => {});
    if (d.online && !d.data?.updates) this.v.cmd('software.updates', {}, { quiet: true }).catch(() => {});
  }
  install(id, name) { this.v.cmd('software.install', { id, name }); }
  apps() { return this.v.dev.data?.software?.value || null; }
  update() { this.drawRunning(); this.drawCatalog(); this.drawSearch(); this.drawUpdates(); this.drawApps(); }
  onCommand(c) {
    if (!c.type.startsWith('software.')) return;
    if (c.type === 'software.search' && c.status !== 'sent' && c.status !== 'queued') this.searching = false;
    this.drawRunning();
    this.drawCatalog();
  }
  drawRunning() {
    const r = this.v.running('software.').filter((c) => !['software.list', 'software.search'].includes(c.type));
    $('#running', this.el).innerHTML = r.length ? `<div class="card card-pad running" style="margin-bottom:14px">${r.map((c) => `<div><span class="spin"></span><b>${esc(c.summary)}</b><span class="faint">${c.status === 'queued' ? 'wartet auf PC' : 'läuft …'}</span></div>`).join('')}</div>` : '';
  }
  installedNames() { return (this.apps() || []).map((a) => a.name.toLowerCase()).join('\n'); }
  drawCatalog() {
    const names = this.installedNames();
    const busy = new Set(this.v.running('software.install').map((c) => JSON.stringify(c.args?.id)));
    $('#cat', this.el).innerHTML = CATALOG.map(([n, id]) => {
      const has = names.includes(n.toLowerCase().split(' ')[0]);
      return `<button type="button" class="chip" data-id="${esc(id)}" data-name="${esc(n)}" ${busy.has(JSON.stringify(id)) ? 'disabled' : ''} title="${esc(id)}">${has ? `<span class="ok">${icon('check', 14)}</span>` : icon('plus', 14)}${esc(n)}</button>`;
    }).join('');
  }
  drawSearch() {
    const box = $('#search', this.el);
    const s = this.v.dev.data?.search?.value;
    if (this.searching) { box.innerHTML = '<p class="muted" style="margin:12px 0 0"><span class="spin" style="display:inline-block;vertical-align:-2px;margin-right:6px"></span>Suche läuft …</p>'; return; }
    if (!s) { box.innerHTML = ''; return; }
    const rows = s.rows || [];
    box.innerHTML = `<p class="small muted" style="margin:14px 0 6px">Treffer für „${esc(s.query)}“</p>` + (rows.length ? `<div class="table-wrap"><table class="table"><tbody>${rows.slice(0, 15).map((r) => `<tr>
        <td><div class="name">${esc(r.name)}</div><div class="sub mono">${esc(r.id)} · ${esc(r.version)}</div></td>
        <td class="actions">${r.truncated ? '<span class="faint small">ID gekürzt</span>' : `<button class="btn sm" data-id="${esc(r.id)}" data-name="${esc(r.name)}">Installieren</button>`}</td></tr>`).join('')}</tbody></table></div>` : '<p class="muted">Nichts gefunden.</p>');
  }
  drawUpdates() {
    const box = $('#upd', this.el);
    const u = this.v.dev.data?.updates;
    if (!u) { box.innerHTML = `<p class="muted">${this.v.dev.online ? 'Wird geprüft …' : 'Noch nicht geprüft.'}</p>`; return; }
    const rows = u.value || [];
    box.innerHTML = (rows.length ? `<div class="table-wrap"><table class="table"><tbody>${rows.map((r) => `<tr>
        <td><div class="name">${esc(r.name)}</div><div class="sub">${esc(r.version)} → <b>${esc(r.available)}</b></div></td>
        <td class="actions"><button class="btn sm" data-up="${esc(r.id)}" data-name="${esc(r.name)}">Aktualisieren</button></td></tr>`).join('')}</tbody></table></div>`
      : `<div class="callout accent">${icon('check')}<div>Alles aktuell.</div></div>`) + `<p class="small faint" style="margin:10px 0 0">Geprüft ${esc(ago(u.at))}</p>`;
  }
  drawApps() {
    const box = $('#apps', this.el), apps = this.apps();
    if (!apps) { box.innerHTML = `<div class="empty">${this.v.dev.online ? 'Wird geladen …' : 'Noch keine Liste vorhanden.'}</div>`; return; }
    const f = this.filter;
    const list = apps.filter((a) => !f || (a.name + ' ' + a.publisher).toLowerCase().includes(f));
    box.innerHTML = `<div class="table-wrap"><table class="table"><thead><tr><th>Programm</th><th>Version</th><th>Hersteller</th><th></th></tr></thead><tbody>
      ${list.map((a) => `<tr><td class="name">${esc(a.name)}</td><td class="muted mono">${esc(a.version || '–')}</td><td class="muted">${esc(a.publisher || '–')}</td>
        <td class="actions"><button class="btn sm ghost" data-un="${esc(a.name)}" title="Deinstallieren">${icon('trash', 14)}</button></td></tr>`).join('')}
      </tbody></table></div><p class="small faint" style="padding:8px 14px;margin:0">${list.length} von ${apps.length} Programmen · Stand ${esc(ago(this.v.dev.data.software.at))}</p>`;
  }
}

// ---------- Sicherheit (Gerät) ----------
class SecurityTab {
  constructor(v) { this.v = v; }
  render(el) {
    this.el = el;
    if (!this.v.dev.securityEnabled) {
      el.innerHTML = `<div class="card card-pad sec-intro">
        <div class="sec-shield unknown">${icon('shield', 34)}</div>
        <div><h2>Sicherheitspaket aktivieren</h2>
        <p class="muted">Überwacht den Virenschutz dieses PCs und macht ihn stärker – ohne Zusatzkosten und ohne zusätzliche Software:</p>
        <ul class="ticks"><li>Nutzt den eingebauten <b>Microsoft Defender</b> (in unabhängigen Tests in der Spitzengruppe)</li>
          <li>Härtung mit einem Klick: Schutz vor Adware und unerwünschten Programmen, Cloud-Schutz, Schutz vor gefährlichen Webseiten, optional Ransomware-Schutz</li>
          <li>Schnell-, Voll- und Offline-Scans, Signaturen aktualisieren, Funde entfernen</li>
          <li>Zweitmeinung mit dem <b>Microsoft Safety Scanner</b></li>
          <li>Sofort-Alarm im Dashboard bei Bedrohungen oder abgeschaltetem Schutz</li></ul>
        <button class="btn primary" id="act">${icon('shield')} Für diesen PC aktivieren</button></div></div>`;
      $('#act', el).onclick = async () => {
        try { await api('PATCH', `/api/devices/${this.v.id}`, { securityEnabled: true }); this.v.dev.securityEnabled = true; this.render(el); toast('Sicherheitspaket aktiviert', 'ok'); }
        catch (e) { toast('Fehler', 'error', e.message); }
      };
      return;
    }
    el.innerHTML = `<div id="hero"></div><div id="running"></div>
      <div class="two-col section"><section class="card card-pad" id="checks"></section><section class="card card-pad" id="details"></section></div>
      <section class="card section" id="threats"></section>
      <section class="section" id="second"></section>`;
    el.onclick = (e) => { const b = e.target.closest('[data-sa]'); if (b) this.action(b.dataset.sa); };
    clearInterval(this.tick);
    this.tick = setInterval(() => { if (pausedUntil(this.v.dev) || this.v.dev.protectionPausedUntil) this.update(); }, 30000);
    this.update();
    if (this.v.dev.online && !this.v.dev.data?.security) this.v.cmd('security.status', {}, { quiet: true }).catch(() => {});
  }
  sec() {
    // Neuester Stand aus Systeminfo (alle 5 Min.) und ausführlichem Status (nach Scans/Abfragen)
    const s = secData(this.v.dev);
    if (s && !this.v.dev.data?.security) s.threats = null;
    return s;
  }
  action(a) {
    const v = this.v;
    if (a === 'quick') return v.cmd('security.scan', { type: 'quick' });
    if (a === 'full') return confirmModal('Vollständigen Scan starten?', 'Prüft alle Dateien auf dem PC. Das kann je nach Festplatte 30 Minuten bis mehrere Stunden dauern; der PC wird dabei etwas langsamer.', 'Scan starten').then((ok) => ok && v.cmd('security.scan', { type: 'full' }));
    if (a === 'update') return v.cmd('security.update');
    if (a === 'remove') return confirmModal('Aktive Bedrohungen entfernen?', 'Microsoft Defender entfernt bzw. isoliert alle aktiven Funde.', 'Entfernen', true).then((ok) => ok && v.cmd('security.removeThreats'));
    if (a === 'refresh') return v.cmd('security.status', {}, { quiet: true });
    if (a === 'msert-quick') return v.cmd('security.msert', { type: 'quick' });
    if (a === 'msert-full') return confirmModal('Gründliche Zweitmeinung?', 'Der Microsoft Safety Scanner wird frisch von Microsoft geladen (ca. 200 MB), geprüft und durchsucht den ganzen PC. Funde werden automatisch entfernt. Dauer: oft über eine Stunde.', 'Starten').then((ok) => ok && v.cmd('security.msert', { type: 'full' }));
    if (a === 'offline') return confirmModal('Offline-Scan mit Neustart?', 'Der PC startet <b>sofort</b> neu und wird vor dem Windows-Start geprüft (ca. 15 Minuten). Hilft gegen hartnäckige Schädlinge, die sich im laufenden Windows verstecken. Vorher sollte die Person am PC ihre Arbeit speichern.', 'Jetzt neu starten', true).then((ok) => ok && v.cmd('security.offlineScan'));
    if (a === 'pause') return this.pauseModal();
    if (a === 'resume') return v.cmd('security.resume');
    if (a === 'opensec') return v.cmd('user.openSecurity');
    if (a === 'harden') return this.hardenModal(false);
    if (a === 'harden-rec') return this.hardenModal(true);
    if (a === 'off') return confirmModal('Sicherheitspaket deaktivieren?', 'Der Virenschutz auf dem PC bleibt unverändert, aber du bekommst keine Alarme mehr und kannst ihn hier nicht mehr steuern.', 'Deaktivieren', true).then(async (ok) => {
      if (!ok) return;
      await api('PATCH', `/api/devices/${v.id}`, { securityEnabled: false });
      v.dev.securityEnabled = false; this.render(this.el);
    });
  }
  pauseModal() {
    openModal({
      title: 'Echtzeitschutz pausieren', submitLabel: 'Pausieren', danger: true,
      body: `<p>Für Reparaturen, bei denen der Virenschutz stört (z. B. ein Tool, das fälschlich blockiert wird). Der Schutz schaltet sich <b>automatisch wieder ein</b> – auch wenn du es vergisst oder der PC neu startet.</p>
        <label class="field">Wie lange?<select class="input" name="minutes"><option value="15">15 Minuten</option><option value="30" selected>30 Minuten</option><option value="60">1 Stunde</option><option value="120">2 Stunden</option></select></label>
        <div class="callout warn">${icon('alert')}<div>Während der Pause werden neue Dateien nicht geprüft. Die Person am PC sollte in der Zeit nichts herunterladen oder öffnen.</div></div>
        <p class="small faint">Ist am PC der Manipulationsschutz an, blockiert Windows das Pausieren aus der Ferne. Dann: Bildschirm verbinden → „Windows-Sicherheit am PC öffnen“.</p>`,
      onSubmit: async (f) => { await this.v.cmd('security.pause', { minutes: +f.minutes.value }); },
    });
  }
  hardenModal(recommended) {
    const s = this.sec() || {};
    const on = (x, val = 1) => (x === val || (val === 2 && x >= 1)) ? 'checked' : '';
    const fresh = recommended || s.pua === undefined || s.pua === -1;
    openModal({
      title: 'Schutz-Einstellungen', submitLabel: 'Anwenden', wide: true,
      body: `<p>${recommended ? 'Die empfohlenen Einstellungen sind vorausgewählt.' : 'Aktueller Stand des PCs.'} Empfohlen ist alles außer Ransomware-Schutz. Wirkt sofort.</p>
        <label class="check"><input type="checkbox" name="pua" ${fresh || s.pua === 1 ? 'checked' : ''}> <span>Schutz vor unerwünschten Programmen (PUA)<small>Blockt Adware, Toolbars, „Gratis-PDF-Konverter“ und ähnlichen Müll – genau das, wofür viele Malwarebytes nutzen.</small></span></label>
        <label class="check"><input type="checkbox" name="cloud" ${fresh || on(s.cloud, 2) ? 'checked' : ''}> <span>Cloud-Schutz<small>Unbekannte Dateien werden in Sekunden mit Microsofts Cloud abgeglichen. Erkennt neue Schädlinge deutlich schneller.</small></span></label>
        <label class="check"><input type="checkbox" name="network" ${fresh || s.network === 1 ? 'checked' : ''}> <span>Netzwerkschutz<small>Sperrt bekannte Betrugs-, Phishing- und Schadsoftware-Seiten in allen Browsern.</small></span></label>
        <label class="check"><input type="checkbox" name="ransomware" ${s.ransomware === 1 ? 'checked' : ''}> <span>Ransomware-Schutz (überwachter Ordnerzugriff)<small>Nur vertrauenswürdige Programme dürfen in Dokumente, Bilder usw. schreiben. Sehr wirksam, kann aber ältere Programme beim Speichern blockieren – erst beobachten.</small></span></label>`,
      onSubmit: async (f) => { await this.v.cmd('security.harden', { pua: f.pua.checked, cloud: f.cloud.checked, network: f.network.checked, ransomware: f.ransomware.checked }); },
    });
  }
  update() {
    if (!this.v.dev.securityEnabled || !$('#hero', this.el)) return;
    const d = this.v.dev, s = this.sec(), st = secState(d), pu = pausedUntil(d);
    const busy = this.v.running('security.');
    const title = { ok: 'Geschützt', warn: 'Achtung', danger: 'Gefahr', info: 'Geschützt durch anderes Programm', unknown: 'Noch kein Status' }[st.level];
    $('#hero', this.el).innerHTML = `<div class="card card-pad sec-hero ${st.level}">
      <div class="sec-shield ${st.level}">${icon(st.level === 'danger' ? 'alert' : 'shield', 34)}</div>
      <div class="grow"><h2>${esc(st.paused ? 'Schutz pausiert' : title)}</h2><p>${(() => {
        const note = pu ? `Echtzeitschutz pausiert – geht in ${Math.max(1, Math.round((new Date(pu) - Date.now()) / 60000))} Min. (${hhmm(pu)} Uhr) automatisch wieder an.` : '';
        if (st.paused) return esc(note);
        return (st.level === 'ok' ? '' : esc(st.text) + ' · ') + esc(st.hint) + (note ? ` <b>${esc(note)}</b>` : '');
      })()}</p></div>
      <div class="row">
        ${s?.activeThreats > 0 ? `<button class="btn danger solid" data-sa="remove">${icon('trash')} Bedrohungen entfernen</button>` : ''}
        ${st.text === 'Härtung empfohlen' ? `<button class="btn" data-sa="harden-rec" ${d.online ? '' : 'disabled'}>${icon('shield')} Empfohlene Härtung</button>` : ''}
        ${pu ? `<button class="btn primary" data-sa="resume" ${d.online ? '' : 'disabled'}>${icon('shield')} Schutz jetzt wieder einschalten</button>` : ''}
        <button class="btn primary" data-sa="quick" ${d.online ? '' : 'disabled'}>${icon('search')} Schnellscan</button>
        <button class="btn" data-sa="update" ${d.online ? '' : 'disabled'}>${icon('refresh')} Signaturen</button>
        <details class="menu"><summary class="btn">${icon('more')}</summary><div class="menu-pop">
          <button type="button" data-sa="full">${icon('search')} Vollständiger Scan</button>
          <button type="button" data-sa="msert-quick">${icon('eye')} Zweitmeinung (schnell, nur melden)</button>
          <button type="button" data-sa="msert-full">${icon('eye')} Zweitmeinung (gründlich, entfernt Funde)</button>
          <button type="button" data-sa="offline">${icon('power')} Offline-Scan mit Neustart</button>
          <hr>${pu ? `<button type="button" data-sa="resume">${icon('shield')} Echtzeitschutz wieder einschalten</button>` : `<button type="button" data-sa="pause" ${s?.realtime === false ? 'disabled' : ''}>${icon('clock')} Schutz pausieren (für Reparaturen)</button>`}
          <button type="button" data-sa="opensec" ${d.screenOnline ? '' : 'disabled title="Niemand angemeldet"'}>${icon('monitor')} Windows-Sicherheit am PC öffnen</button><hr>
          <button type="button" data-sa="harden">${icon('sliders')} Schutz-Einstellungen</button>
          <button type="button" data-sa="refresh">${icon('refresh')} Status neu laden</button>
          <hr><button type="button" class="danger" data-sa="off">${icon('x')} Paket deaktivieren</button></div></details>
      </div></div>`;
    $('#running', this.el).innerHTML = busy.length ? `<div class="card card-pad running section">${busy.map((c) => `<div><span class="spin"></span><b>${esc(c.summary)}</b><span class="faint">${c.status === 'queued' ? 'wartet auf PC' : 'läuft seit ' + esc(ago(c.sentAt || c.createdAt))}</span></div>`).join('')}</div>` : '';

    const chk = (ok, label, sub, warnOnly) => `<div class="chk-row ${ok ? 'ok' : warnOnly ? 'warn' : 'bad'}"><span>${icon(ok ? 'check' : warnOnly ? 'info' : 'x', 16)}</span><div><b>${esc(label)}</b>${sub ? `<small>${esc(sub)}</small>` : ''}</div></div>`;
    if (!s) {
      $('#checks', this.el).innerHTML = '<p class="muted">Noch keine Daten vom PC.</p>';
      $('#details', this.el).innerHTML = '';
    } else {
      $('#checks', this.el).innerHTML = `<div class="row" style="margin-bottom:10px"><h2>Schutzfunktionen</h2><span class="spacer"></span><button class="btn sm" data-sa="harden">${icon('sliders', 14)} Anpassen</button></div>
        ${chk(s.realtime, 'Echtzeitschutz', s.realtime ? 'Dateien werden beim Öffnen geprüft' : pu ? `Pausiert bis ${hhmm(pu)} Uhr – geht automatisch wieder an` : 'AUS – der PC ist ungeschützt', !!pu)}
        ${s.behavior !== undefined ? chk(s.behavior, 'Verhaltensüberwachung', 'Erkennt verdächtiges Verhalten laufender Programme') : ''}
        ${chk(s.tamper, 'Manipulationsschutz', s.tamper ? 'Schädlinge können den Schutz nicht abschalten' : 'In Windows-Sicherheit einschalten (nur am PC möglich)', true)}
        ${chk(s.pua === 1, 'Schutz vor unerwünschten Programmen', 'Adware, Toolbars, Bundle-Installer', true)}
        ${s.cloud !== undefined ? chk(s.cloud >= 1, 'Cloud-Schutz', 'Schnelle Erkennung neuer Schädlinge', true) : ''}
        ${s.network !== undefined ? chk(s.network === 1, 'Netzwerkschutz', 'Sperrt gefährliche Webseiten', true) : ''}
        ${s.ransomware !== undefined ? chk(s.ransomware === 1, 'Ransomware-Schutz', 'Überwachter Ordnerzugriff (optional)', true) : ''}`;
      const age = (n) => n < 0 ? 'nie' : n === 0 ? 'heute' : n === 1 ? 'gestern' : `vor ${n} Tagen`;
      const prods = (s.products || inv(d).antivirus || []);
      $('#details', this.el).innerHTML = `<h2 style="margin-bottom:12px">Details</h2><dl class="kv">
        <dt>Signaturen</dt><dd>${esc(age(s.signatureAgeDays))}${s.signatureVersion ? ' · ' + esc(s.signatureVersion) : ''}</dd>
        <dt>Schnellscan</dt><dd>${esc(age(s.quickScanAgeDays))}</dd>
        ${s.fullScanAgeDays !== undefined ? `<dt>Vollscan</dt><dd>${esc(age(s.fullScanAgeDays))}</dd>` : ''}
        <dt>Modus</dt><dd>${esc(s.mode === 'Normal' ? 'Aktiv (Hauptschutz)' : s.mode || '–')}</dd>
        ${s.engineVersion ? `<dt>Engine</dt><dd class="mono small">${esc(s.engineVersion)}</dd>` : ''}
        <dt>Virenscanner</dt><dd style="white-space:normal">${prods.length ? prods.map((p) => `${esc(p.name)} ${p.enabled ? '<span class="badge accent">an</span>' : '<span class="badge">aus</span>'}`).join('<br>') : '–'}</dd>
        <dt>Stand</dt><dd>${esc(ago(d.data?.security?.at || d.inventoryAt))}</dd></dl>`;
    }

    const threats = s?.threats;
    const tb = $('#threats', this.el);
    if (!threats) {
      tb.innerHTML = `<header><h2>Funde</h2></header><div class="empty">${d.online ? 'Wird geladen …' : 'Noch keine Liste vorhanden.'}</div>`;
    } else if (!threats.length) {
      tb.innerHTML = `<header><h2>Funde</h2></header><div class="empty">${icon('check', 20)}<p>Bisher wurde auf diesem PC nichts gefunden.</p></div>`;
    } else {
      tb.innerHTML = `<header><h2>Funde</h2><span class="faint small">die letzten ${threats.length}</span></header><div class="table-wrap"><table class="table"><thead><tr><th>Bedrohung</th><th>Gefahr</th><th>Status</th><th>Datei</th><th>Gefunden</th></tr></thead><tbody>
        ${threats.map((t) => { const sv = SEVERITY[t.severity] || ['Unbekannt', '']; return `<tr>
          <td><div class="name mono small">${esc(t.name || 'Unbekannt')}</div>${t.process ? `<div class="sub">über ${esc(t.process)}</div>` : ''}</td>
          <td><span class="badge ${sv[1]}">${esc(sv[0])}</span></td>
          <td>${t.active ? '<span class="pill danger">Aktiv</span>' : `<span class="pill ok plain">${esc(THREAT_STATUS[t.statusId] || 'Erledigt')}</span>`}</td>
          <td class="small mono" style="max-width:340px;word-break:break-all">${esc((t.resources || []).join('\n') || '–')}</td>
          <td class="muted nowrap">${esc(fmtDate(t.time))}</td></tr>`; }).join('')}</tbody></table></div>`;
    }

    const ms = this.v.cmds.find((c) => c.type === 'security.msert');
    $('#second', this.el).innerHTML = ms ? `<h3 style="margin-bottom:10px">Letzte Zweitmeinung</h3><div class="card">${taskItem(ms, true)}</div>` : '';
  }
  onCommand(c) { if (c.type.startsWith('security.')) this.update(); }
  destroy() { clearInterval(this.tick); }
}

// ---------- Sicherheit (alle Geräte) ----------
class SecurityView {
  render(main) {
    this.main = main;
    main.innerHTML = `<div class="page-head"><div class="grow"><h1>Sicherheit</h1><p>Virenschutz aller PCs auf einen Blick – mit dem eingebauten Microsoft Defender.</p></div>
      <div class="row"><button class="btn" id="upd">${icon('refresh')} Alle: Signaturen</button><button class="btn primary" id="scan">${icon('search')} Alle: Schnellscan</button></div></div>
      <div class="sec-stats" id="stats"></div><div class="card" id="box"></div>`;
    $('#upd', main).onclick = () => this.all('security.update', {});
    $('#scan', main).onclick = () => this.all('security.scan', { type: 'quick' });
    $('#box', main).onclick = async (e) => {
      const b = e.target.closest('[data-on]');
      if (b) {
        e.stopPropagation();
        await api('PATCH', `/api/devices/${b.dataset.on}`, { securityEnabled: true }).catch((ex) => toast('Fehler', 'error', ex.message));
        return;
      }
      const r = e.target.closest('tr[data-id]');
      if (r) location.hash = `#/d/${r.dataset.id}/security`;
    };
    this.draw();
  }
  targets() { return [...S.devices.values()].filter((d) => d.securityEnabled && d.online); }
  async all(type, args) {
    const t = this.targets();
    if (!t.length) return toast('Keine passenden PCs', 'info', 'Es ist kein PC mit aktivem Sicherheitspaket online.');
    for (const d of t) await sendCommand(d.id, type, args, { quiet: true }).catch(() => {});
    toast(`An ${t.length} PC${t.length > 1 ? 's' : ''} gesendet`, 'ok');
  }
  draw() {
    const all = [...S.devices.values()];
    const on = all.filter((d) => d.securityEnabled);
    const cnt = { ok: 0, warn: 0, danger: 0 };
    on.forEach((d) => { const l = secState(d).level; if (l === 'ok' || l === 'info') cnt.ok++; else if (cnt[l] !== undefined) cnt[l]++; });
    $('#stats', this.main).innerHTML = `
      <div class="card stat ok"><b>${cnt.ok}</b><span>geschützt</span></div>
      <div class="card stat warn"><b>${cnt.warn}</b><span>Achtung</span></div>
      <div class="card stat danger"><b>${cnt.danger}</b><span>Gefahr</span></div>
      <div class="card stat"><b>${all.length - on.length}</b><span>ohne Paket</span></div>`;
    if (!all.length) { $('#box', this.main).innerHTML = '<div class="empty">Noch keine Geräte.</div>'; return; }
    const order = { danger: 0, warn: 1, unknown: 2, info: 3, ok: 4 };
    const rows = all.slice().sort((a, b) => (b.securityEnabled - a.securityEnabled) || (order[secState(a).level] - order[secState(b).level]) || a.name.localeCompare(b.name, 'de'));
    const age = (n) => n == null ? '–' : n < 0 ? 'nie' : n === 0 ? 'heute' : `vor ${n} T.`;
    $('#box', this.main).innerHTML = `<div class="table-wrap"><table class="table"><thead><tr><th>Gerät</th><th>Status</th><th>Bedrohungen</th><th>Signaturen</th><th>Letzter Scan</th><th></th></tr></thead><tbody>
      ${rows.map((d) => { const df = inv(d).defender || {}; return d.securityEnabled ? `<tr data-id="${esc(d.id)}" class="clickable">
        <td><div class="name">${esc(d.name)}</div><div class="sub"><span class="mono">ID ${esc(fmtPCID(d.pcId))}</span> · ${d.online ? 'online' : 'offline · ' + esc(ago(d.lastSeen))}</div></td>
        <td>${secBadge(d)}</td><td>${df.activeThreats > 0 ? `<b style="color:var(--danger)">${df.activeThreats} aktiv</b>` : df.activeThreats === 0 ? '0' : '–'}</td>
        <td class="muted">${esc(age(df.signatureAgeDays))}</td><td class="muted">${esc(age(df.quickScanAgeDays))}</td>
        <td class="actions"><span class="faint">${icon('chevron', 16)}</span></td></tr>` : `<tr data-id="${esc(d.id)}" class="clickable off">
        <td><div class="name">${esc(d.name)}</div><div class="sub"><span class="mono">ID ${esc(fmtPCID(d.pcId))}</span> · Sicherheitspaket aus</div></td><td colspan="4" class="faint">${df.enabled ? 'Defender läuft, wird aber nicht überwacht' : '–'}</td>
        <td class="actions"><button class="btn sm" data-on="${esc(d.id)}">Aktivieren</button></td></tr>`; }).join('')}</tbody></table></div>`;
  }
  onDevice() { this.draw(); }
  onDeviceRemoved() { this.draw(); }
  onAlert() { this.draw(); }
}

// ---------- PowerShell ----------
const SNIPPETS = [
  ['Freier Speicherplatz', "Get-PSDrive -PSProvider FileSystem | Select-Object Name, @{n='Frei (GB)';e={[math]::Round($_.Free/1GB,1)}}, @{n='Belegt (GB)';e={[math]::Round($_.Used/1GB,1)}} | Format-Table -AutoSize"],
  ['Top 10 Programme nach RAM', "Get-Process | Sort-Object WorkingSet64 -Descending | Select-Object -First 10 Name, Id, @{n='RAM (MB)';e={[math]::Round($_.WorkingSet64/1MB)}} | Format-Table -AutoSize"],
  ['Netzwerk-Infos', 'ipconfig /all'],
  ['Letzte Neustarts', "Get-WinEvent -FilterHashtable @{LogName='System'; Id=6005,6006,1074} -MaxEvents 10 | Select-Object TimeCreated, Id, Message | Format-List"],
  ['Autostart-Programme', 'Get-CimInstance Win32_StartupCommand | Select-Object Name, Command, User | Format-Table -AutoSize -Wrap'],
  ['Windows-Updates suchen lassen', 'UsoClient StartScan; "Suche nach Windows-Updates wurde angestoßen."'],
  ['Druckerwarteschlange leeren', "Stop-Service Spooler -Force; Remove-Item \"$env:SystemRoot\\System32\\spool\\PRINTERS\\*\" -Force -ErrorAction SilentlyContinue; Start-Service Spooler; 'Druckerwarteschlange geleert.'"],
  ['Temp-Dateien aufräumen', "Remove-Item \"$env:SystemRoot\\Temp\\*\" -Recurse -Force -ErrorAction SilentlyContinue; 'Windows-Temp aufgeräumt.'"],
];

class ShellTab {
  constructor(v) { this.v = v; }
  render(el) {
    this.el = el;
    el.innerHTML = `<div class="callout warn" style="margin-bottom:14px">${icon('alert')}<div>Befehle laufen auf dem PC als <b>SYSTEM</b> – mit allen Rechten. Prüfe Skripte aus dem Internet, bevor du sie ausführst.</div></div>
      <section class="card card-pad"><form id="sf" class="form-grid">
        <textarea class="input console" name="script" spellcheck="false" placeholder="z. B. Get-Service | Where-Object Status -eq 'Running'" required></textarea>
        <div class="row"><select class="input" id="snip" style="width:auto"><option value="">Vorlage einfügen …</option>${SNIPPETS.map(([n], i) => `<option value="${i}">${esc(n)}</option>`).join('')}</select>
          <select class="input" id="mine" style="width:auto" hidden></select>
          <button type="button" class="btn ghost" id="save">${icon('plus', 15)} Speichern</button>
          <select class="input" name="timeout" style="width:auto" title="Maximale Laufzeit"><option value="60">max. 1 Min.</option><option value="300" selected>max. 5 Min.</option><option value="1800">max. 30 Min.</option></select>
          <span class="spacer"></span><span class="faint small">Strg + Enter</span><button class="btn primary">${icon('play')} Ausführen</button></div></form></section>
      <div id="hist" class="section"></div>`;
    const f = $('#sf', el);
    $('#snip', el).onchange = (e) => { if (e.target.value !== '') { f.script.value = SNIPPETS[+e.target.value][1]; e.target.value = ''; f.script.focus(); } };
    $('#save', el).onclick = () => {
      if (!f.script.value.trim()) { toast('Erst ein Skript eingeben', 'info'); return; }
      openModal({ title: 'Skript speichern', submitLabel: 'Speichern',
        body: `<label class="field">Name<input class="input" name="name" maxlength="40" required placeholder="z. B. Druckerwarteschlange leeren"></label>
          <p class="small faint" style="margin:8px 0 0">Gespeicherte Skripte kannst du auf jedem PC einfügen – und unter „Geräte › Auswählen“ auf vielen PCs gleichzeitig ausführen. Gleicher Name überschreibt.</p>`,
        onSubmit: async (m) => { await api('POST', '/api/scripts', { name: m.name.value, script: f.script.value }); toast('Skript gespeichert', 'ok'); this.loadScripts(); } });
    };
    this.loadScripts();
    f.script.onkeydown = (e) => { if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); f.requestSubmit(); } };
    f.onsubmit = (e) => {
      e.preventDefault();
      if (!f.script.value.trim()) return;
      this.v.cmd('script.run', { script: f.script.value, timeout: +f.timeout.value }, { quiet: true });
    };
    this.update();
  }
  async loadScripts() {
    const sel = $('#mine', this.el);
    if (!sel) return;
    this.scripts = await api('GET', '/api/scripts').catch(() => []);
    sel.hidden = !this.scripts.length;
    sel.innerHTML = `<option value="">Meine Skripte (${this.scripts.length}) …</option>${this.scripts.map((x, i) => `<option value="${i}">${esc(x.name)}</option>`).join('')}<option value="manage">Verwalten …</option>`;
    sel.onchange = () => {
      const v = sel.value;
      sel.value = '';
      if (v === 'manage') return this.manageScripts();
      if (v !== '') { const f = $('#sf', this.el); f.script.value = this.scripts[+v].script; f.script.focus(); }
    };
  }
  manageScripts() {
    const row = (x) => `<div class="req-row"><div class="grow"><b>${esc(x.name)}</b><span class="small muted">${esc(x.createdBy)} · ${esc(fmtDate(x.createdAt))}</span></div>
      <button type="button" class="btn sm danger" data-del="${esc(x.id)}">${icon('trash', 14)} Löschen</button></div>`;
    openModal({ title: 'Meine Skripte', footer: '<button type="button" class="btn primary" data-close>Fertig</button>',
      body: `<div id="sl">${this.scripts.map(row).join('') || '<p class="muted">Noch keine Skripte gespeichert.</p>'}</div>`,
      onOpen: (m) => {
        m.addEventListener('click', async (e) => {
          const b = e.target.closest('[data-del]');
          if (!b) return;
          try { await api('DELETE', `/api/scripts/${b.dataset.del}`); b.closest('.req-row').remove(); toast('Skript gelöscht', 'ok'); this.loadScripts(); }
          catch (ex) { toast('Fehler', 'error', ex.message); }
        });
      } });
  }
  update() {
    const list = this.v.cmds.filter((c) => c.type === 'script.run').slice(0, 20);
    $('#hist', this.el).innerHTML = list.length ? `<h3 style="margin-bottom:10px">Verlauf</h3><div class="card">${list.map((c, i) => taskItem(c, i === 0)).join('')}</div>` : '';
  }
  onCommand(c) { if (c.type === 'script.run') this.update(); }
}

function statusIcon(s) {
  if (s === 'sent') return '<span class="status-ico"><span class="spin"></span></span>';
  const m = { done: 'check', failed: 'x', timeout: 'clock', expired: 'clock', canceled: 'x', queued: 'clock' };
  return `<span class="status-ico ${s}">${icon(m[s] || 'info', 17)}</span>`;
}
const STATUS = { queued: 'wartet', sent: 'läuft', done: 'erledigt', failed: 'fehlgeschlagen', timeout: 'keine Antwort', canceled: 'abgebrochen', expired: 'verfallen' };

function taskItem(c, open = false) {
  const args = c.type === 'script.run' && c.args?.script ? `PS> ${c.args.script}\n\n` : '';
  return `<details class="task" ${open && c.output ? 'open' : ''}><summary>${statusIcon(c.status)}
    <div class="t-main"><b>${esc(c.summary)}</b><span>${esc(STATUS[c.status] || c.status)} · ${esc(c.createdBy)} · ${esc(fmtDate(c.createdAt))}</span></div>
    ${c.status === 'queued' ? `<button type="button" class="btn sm" data-cancel="${esc(c.id)}">Abbrechen</button>` : ''}</summary>
    ${c.output || args ? `<pre>${esc(args + (c.output || ''))}</pre>` : ''}</details>`;
}

// ---------- Aufgaben ----------
class TasksTab {
  constructor(v) { this.v = v; }
  render(el) {
    this.el = el;
    el.innerHTML = '<div class="card" id="list"></div>';
    $('#list', el).onclick = async (e) => {
      const b = e.target.closest('[data-cancel]');
      if (!b) return;
      e.preventDefault();
      try { await api('POST', `/api/commands/${b.dataset.cancel}/cancel`); } catch (ex) { toast('Nicht möglich', 'error', ex.message); }
    };
    this.update();
  }
  update() {
    const box = $('#list', this.el);
    box.innerHTML = this.v.cmds.length ? this.v.cmds.map((c) => taskItem(c)).join('') : '<div class="empty">Noch keine Aufträge für diesen PC.</div>';
  }
  onCommand() { this.update(); }
}

// ---------- Geräte-Einstellungen ----------
class SettingsTab {
  constructor(v) { this.v = v; }
  render(el) {
    const d = this.v.dev;
    el.innerHTML = `<div class="settings-grid">
      <section class="card card-pad"><form id="f" class="form-grid">
        <h2>Allgemein</h2>
        <div class="form-grid two"><label class="field">Name<input class="input" name="name" maxlength="64" value="${esc(d.name)}"></label>
          <label class="field">Gruppe <span class="hint">z. B. Familie, Kunden</span><input class="input" name="group" maxlength="30" list="grplist" value="${esc(d.group || '')}"></label></div>
        <datalist id="grplist">${[...new Set([...S.devices.values()].map((x) => x.group).filter(Boolean))].map((g) => `<option value="${esc(g)}">`).join('')}</datalist>
        <label class="field">Notizen <span class="hint">nur für dich sichtbar</span><textarea class="input" name="notes" rows="3" maxlength="4000" placeholder="z. B. Drucker: HP DeskJet im Flur, WLAN-Passwort liegt …">${esc(d.notes)}</textarea></label>
        <label class="check"><input type="checkbox" name="tester" ${d.updateTester ? 'checked' : ''}> <span>Testgerät für Updates<small>Bekommt neue Agent-Versionen zuerst – ideal ist dein eigener PC.</small></span></label>
        <label class="check"><input type="checkbox" name="security" ${d.securityEnabled ? 'checked' : ''}> <span>Sicherheitspaket<small>Virenschutz überwachen, Scans und Härtung aus dem Dashboard, Alarm bei Bedrohungen. Nutzt den in Windows eingebauten Microsoft Defender.</small></span></label>
        <label class="check"><input type="checkbox" name="consent" ${d.consentRequired ? 'checked' : ''}> <span>Vor dem Fernsteuern um Erlaubnis fragen<small>Empfohlen bei Familie und Kunden. Ohne Häkchen verbindest du dich direkt – die Hinweisleiste am PC erscheint trotzdem immer.</small></span></label>
        <div><button class="btn primary">Speichern</button></div></form></section>
      <section class="card card-pad"><h2 style="margin-bottom:12px">Agent</h2><dl class="kv">
        <dt>Version</dt><dd>${esc(d.agentVersion || '–')} ${isOutdated(d) ? `<span class="badge warn">Update auf ${esc(S.info.agentLatest)}</span> <button type="button" class="btn sm" id="agup" ${d.online ? '' : 'disabled'}>Jetzt aktualisieren</button>` : d.agentVersion ? '<span class="badge accent">aktuell</span>' : ''}</dd><dt>Hinzugefügt</dt><dd>${esc(fmtDate(d.enrolledAt))}</dd>
        <dt>Zuletzt gesehen</dt><dd>${esc(d.online ? 'jetzt' : fmtDate(d.lastSeen))} (${esc(d.lastIp || '–')})</dd><dt>PC-ID</dt><dd class="mono">${esc(fmtPCID(d.pcId))}</dd><dt>Interne ID</dt><dd class="mono faint">${esc(d.id)}</dd></dl>
        <p class="small faint" style="margin:10px 0 0">Die PC-ID sieht die Person am PC im Startmenü unter „Nicolaj Fernwartung“.</p></section>
      <section class="card card-pad"><h2 style="margin-bottom:6px;color:var(--danger)">Gefahrenzone</h2>
        <p class="muted" style="margin:0 0 14px">„Agent entfernen“ deinstalliert die Fernwartung vom PC. „Aus dem Dashboard löschen“ entfernt nur den Eintrag hier – der PC kann sich danach nicht mehr verbinden.</p>
        <div class="row"><button class="btn danger" id="un" ${d.online ? '' : 'disabled title="PC ist offline"'}>${icon('trash')} Agent vom PC entfernen</button>
          <button class="btn danger" id="del">${icon('trash')} Aus dem Dashboard löschen</button></div></section></div>`;
    $('#f', el).onsubmit = async (e) => {
      e.preventDefault();
      const f = e.target;
      try {
        await api('PATCH', `/api/devices/${d.id}`, { name: f.name.value, group: f.group.value, notes: f.notes.value, consentRequired: f.consent.checked, securityEnabled: f.security.checked, updateTester: f.tester.checked });
        toast('Gespeichert', 'ok');
      } catch (ex) { toast('Fehler', 'error', ex.message); }
    };
    const agup = $('#agup', el);
    if (agup) agup.onclick = () => this.v.cmd('agent.update');
    $('#un', el).onclick = async () => {
      if (await confirmModal('Agent vom PC entfernen?', `Die Fernwartung wird auf „${esc(d.name)}“ deinstalliert. Danach ist der PC nicht mehr erreichbar, bis du ihn neu hinzufügst.`, 'Entfernen', true)) this.v.cmd('agent.uninstall');
    };
    $('#del', el).onclick = async () => {
      if (await confirmModal('Aus dem Dashboard löschen?', `„${esc(d.name)}“ wird aus der Liste entfernt. Der Agent bleibt auf dem PC installiert, kann sich aber nicht mehr verbinden.`, 'Löschen', true)) {
        await api('DELETE', `/api/devices/${d.id}`).catch((ex) => toast('Fehler', 'error', ex.message));
      }
    };
  }
}

// ---------- Protokoll ----------
class AuditView {
  render(main) {
    this.main = main;
    main.innerHTML = `<div class="page-head"><div class="grow"><h1>Protokoll</h1><p>Wer hat wann was gemacht – jede Aktion wird festgehalten.</p></div>
      <input class="input" id="flt" type="search" placeholder="Filtern …" style="max-width:260px"></div><div class="card" id="box"><div class="empty">Lade …</div></div>`;
    $('#flt', main).oninput = (e) => { this.f = e.target.value.toLowerCase(); this.draw(); };
    this.load();
  }
  async load() {
    try { this.data = await api('GET', '/api/audit'); this.draw(); } catch (e) { $('#box', this.main).innerHTML = `<div class="empty">${esc(e.message)}</div>`; }
  }
  draw() {
    if (!this.data) return;
    const names = this.data.deviceNames || {};
    const f = this.f || '';
    const rows = this.data.entries.filter((e) => !f || [e.user, e.action, e.detail, names[e.deviceId], e.ip].join(' ').toLowerCase().includes(f));
    $('#box', this.main).innerHTML = rows.length ? `<div class="table-wrap"><table class="table"><thead><tr><th>Zeit</th><th>Wer</th><th>Gerät</th><th>Aktion</th><th>IP</th></tr></thead><tbody>
      ${rows.map((e) => `<tr><td class="nowrap muted">${esc(fmtDate(e.time))}</td><td>${esc(e.user)}</td>
        <td>${e.deviceId ? (names[e.deviceId] ? `<a href="#/d/${esc(e.deviceId)}/overview">${esc(names[e.deviceId])}</a>` : '<span class="faint">gelöscht</span>') : '<span class="faint">–</span>'}</td>
        <td><b style="font-weight:560">${esc(e.action)}</b>${e.detail ? `<div class="sub">${esc(e.detail)}</div>` : ''}</td><td class="muted mono small">${esc(e.ip)}</td></tr>`).join('')}
      </tbody></table></div>` : '<div class="empty">Keine Einträge.</div>';
  }
  onCommand() { clearTimeout(this.t); this.t = setTimeout(() => this.load(), 800); }
  destroy() { clearTimeout(this.t); }
}

// ---------- Einstellungen ----------
class SettingsView {
  render(main) {
    this.main = main;
    main.innerHTML = `<div class="page-head"><div class="grow"><h1>Einstellungen</h1></div></div><div class="settings-grid">
      <section class="card card-pad" id="tfa"></section>
      <section class="card card-pad"><form id="pw" class="form-grid"><h2>Passwort ändern</h2>
        <label class="field">Aktuelles Passwort<input class="input" type="password" name="current" required autocomplete="current-password"></label>
        <label class="field">Neues Passwort<input class="input" type="password" name="new" minlength="10" required autocomplete="new-password"><span class="hint">Mindestens 10 Zeichen.</span></label>
        <div><button class="btn primary">Ändern</button></div></form></section>
      <section class="card card-pad" id="reqs"></section>
      <section class="card card-pad" id="ntf"></section>
      <section class="card card-pad" id="upd"></section>
      <section class="card card-pad" id="gh"></section>
      <section class="card card-pad" id="srv"></section>
      <section class="card" id="codes"></section></div>`;
    $('#pw', main).onsubmit = async (e) => {
      e.preventDefault();
      const f = e.target;
      try { await api('POST', '/api/account/password', { current: f.current.value, new: f.new.value }); f.reset(); toast('Passwort geändert', 'ok'); }
      catch (ex) { toast('Fehler', 'error', ex.message); }
    };
    this.drawTfa();
    this.drawRequests();
    this.drawNotify();
    this.drawUpdates();
    this.drawGitHub();
    this.drawServer();
    this.drawCodes();
  }
  drawNotify() {
    const box = $('#ntf', this.main);
    const supported = 'Notification' in window;
    const perm = supported ? Notification.permission : 'denied';
    box.innerHTML = `<h2 style="margin-bottom:6px">Desktop-Benachrichtigungen</h2>
      <p class="muted" style="margin:0 0 14px">Neue Nachrichten, Zugangsanfragen und Sicherheitsalarme melden sich als Windows- bzw. Handy-Benachrichtigung – auch wenn das Dashboard im Hintergrund offen ist. Gilt nur für diesen Browser.</p>
      <label class="check"><input type="checkbox" id="nten" ${notifyOn() ? 'checked' : ''} ${supported ? '' : 'disabled'}> <span>Benachrichtigungen in diesem Browser<small>${!supported ? 'Dieser Browser unterstützt das nicht.' : perm === 'denied' ? 'Im Browser blockiert – über das Schloss-Symbol in der Adresszeile wieder erlauben.' : 'Der Browser fragt einmal nach der Erlaubnis.'}</small></span></label>`;
    $('#nten', box).onchange = async (e) => {
      try {
        if (e.target.checked) {
          const p = await Notification.requestPermission();
          if (p !== 'granted') { e.target.checked = false; toast('Nicht erlaubt', 'error', 'Der Browser hat Benachrichtigungen abgelehnt.'); return; }
          localStorage.setItem('nrmm-notify', '1');
          new Notification('Benachrichtigungen sind an', { body: 'So sieht es aus, wenn dir jemand schreibt.', icon: '/favicon.svg' });
        } else localStorage.setItem('nrmm-notify', '0');
      } catch (ex) { toast('Fehler', 'error', ex.message); }
      this.drawNotify();
    };
  }
  async drawRequests() {
    const box = $('#reqs', this.main);
    let r;
    try { r = await api('GET', '/api/requests/settings'); } catch (e) { box.innerHTML = `<p class="muted">${esc(e.message)}</p>`; return; }
    box.innerHTML = `<div class="row" style="margin-bottom:6px"><h2>Website &amp; Zugangsanfragen</h2><span class="spacer"></span>
        <span class="pill ${r.enabled ? 'ok' : ''}">${r.enabled ? 'an' : 'aus'}</span></div>
      <p class="muted" style="margin:0 0 14px">Auf deiner Website kann sich jeder den Installer holen, seinen Namen eingeben und Zugang anfragen. Neue Anfragen erscheinen oben in der Geräteliste – erst nach „Annehmen“ kannst du etwas auf dem PC tun.</p>
      <div class="input-group" style="margin-bottom:14px"><input class="input mono" id="rlink" readonly value="${esc(r.link)}"><button class="btn" id="rcopy" type="button">${icon('copy')} Kopieren</button><a class="btn" href="/" target="_blank" rel="noopener">Öffnen</a></div>
      <label class="check"><input type="checkbox" id="ren" ${r.enabled ? 'checked' : ''}> <span>Download und Zugangsanfragen erlauben<small>Aus = die Website zeigt keinen Download, neue Anfragen werden abgelehnt. Installer mit Einrichtungscode funktionieren weiter.</small></span></label>
      ${r.agentAvailable ? '' : '<p class="small" style="color:var(--warn);margin:10px 0 0">Im Server ist noch kein Agent enthalten – der Download funktioniert erst nach einem Release.</p>'}`;
    $('#rcopy', box).onclick = () => copyText(r.link);
    $('#ren', box).onchange = async (e) => {
      try { await api('POST', '/api/requests/settings', { enabled: e.target.checked }); toast(e.target.checked ? 'Zugangsanfragen an' : 'Zugangsanfragen aus', 'ok'); }
      catch (ex) { toast('Fehler', 'error', ex.message); }
      this.drawRequests();
    };
  }
  async drawGitHub() {
    const box = $('#gh', this.main);
    let g;
    try { g = await api('GET', '/api/github'); } catch (e) { box.innerHTML = `<p class="muted">${esc(e.message)}</p>`; return; }
    const st = g.status, cf = g.coolify || {}, box2 = g.container;
    const statusLine = !g.repo ? '<span class="pill">nicht eingerichtet</span>'
      : st.error ? '<span class="pill danger">Fehler</span>'
      : st.tag ? '<span class="pill ok">verbunden</span>' : '<span class="pill">noch nicht geprüft</span>';
    box.innerHTML = `<div class="row" style="margin-bottom:6px"><h2>Neue Versionen per GitHub-Release</h2><span class="spacer"></span>${statusLine}</div>
      <p class="muted" style="margin:0 0 14px">${box2
        ? 'Tag pushen (z. B. <code>git tag v1.0.1 && git push origin v1.0.1</code>) – GitHub baut Image und Release. Der Server prüft stündlich (oder per „Jetzt prüfen“), lässt Coolify das neue Image starten und verteilt den neuen Agent.'
        : 'Tag pushen (z. B. <code>git tag v1.0.1 && git push origin v1.0.1</code>) – GitHub baut alles, der Server holt sich die neue Version stündlich oder sofort per „Jetzt prüfen“.'}</p>
      ${box2 ? (cf.configured
        ? (cf.lastAt && cf.lastAt > '2000' ? `<div class="callout ${cf.lastOk ? 'accent' : 'danger'}" style="margin-bottom:14px">${icon(cf.lastOk ? 'check' : 'alert')}<div style="flex:1">Coolify (${esc(cf.lastVersion)}): ${esc(cf.lastResult)}<div class="small" style="margin-top:3px;opacity:.85">${esc(ago(cf.lastAt))}</div></div></div>` : '')
        : `<div class="callout warn" style="margin-bottom:14px">${icon('info')}<div style="flex:1">Für automatische Server-Updates in Coolify die Umgebungsvariablen <code>NRMM_COOLIFY_WEBHOOK</code> und <code>NRMM_COOLIFY_TOKEN</code> setzen und neu deployen.</div></div>`) : ''}
      ${g.repo && (st.tag || st.error) ? `<div class="callout ${st.error ? 'danger' : 'accent'}" style="margin-bottom:14px">${icon(st.error ? 'alert' : 'check')}<div style="flex:1">
          ${st.error ? esc(st.error) : `Neuestes Release: <b>${esc(st.tag)}</b>${st.url ? ` · <a href="${esc(st.url)}" target="_blank" rel="noopener">ansehen</a>` : ''} · ${st.signed ? 'Signatur geprüft ✓' : 'ohne Release-Signatur'}`}
          <div class="small" style="margin-top:3px;opacity:.85">Zuletzt geprüft ${esc(ago(st.checkedAt))} · Server läuft mit Version ${esc(st.serverVersion)}</div></div></div>` : ''}
      ${st.serverUpdate ? `<div class="callout warn" style="margin-bottom:14px">${icon('download')}<div style="flex:1">Server-Version <b>${esc(st.serverUpdate)}</b> ist verfügbar (du hast ${esc(st.serverVersion)}).</div><button class="btn sm primary" id="srvup" ${box2 && !cf.configured ? 'disabled' : ''}>${box2 ? 'Über Coolify aktualisieren' : 'Server aktualisieren'}</button></div>` : ''}
      <form id="ghf" class="form-grid">
        <div class="form-grid two">
          <label class="field">Repository<input class="input mono" name="repo" value="${esc(g.repo)}" placeholder="nicolaj-webdesign/nicolaj-it-software"></label>
          <label class="field">Zugriffs-Token <span class="hint">nur bei privatem Repo</span><input class="input mono" name="token" type="password" autocomplete="off" placeholder="${g.hasToken ? '•••••••• gespeichert' : 'github_pat_…'}"></label>
        </div>
        <label class="field">Öffentlicher Release-Schlüssel <span class="hint">empfohlen – dann werden nur von dir signierte Releases übernommen</span><input class="input mono" name="pubKey" value="${esc(g.pubKey)}" placeholder="aus: go run ./tools/releasekey new"></label>
        <label class="check"><input type="checkbox" name="auto" ${g.auto ? 'checked' : ''}> <span>${box2 ? 'Neue Versionen automatisch übernehmen' : 'Neue Agent-Versionen automatisch übernehmen'}<small>${box2 ? 'Server: Coolify startet das neue Image. ' : ''}Agent: danach greifen die Regeln oben – erst Testgeräte, dann „Für alle freigeben“.</small></span></label>
        <div class="row"><button class="btn primary">Speichern</button><button type="button" class="btn" id="ghcheck" ${g.repo ? '' : 'disabled'}>${icon('refresh')} Jetzt prüfen</button>
          ${g.hasToken ? '<button type="button" class="btn ghost" id="ghclear">Token löschen</button>' : ''}</div>
      </form>
      <p class="small faint" style="margin:12px 0 0">Token anlegen: GitHub › Settings › Developer settings › Fine-grained tokens · nur dieses Repository · Berechtigung „Contents: Read-only“.</p>`;
    const f = $('#ghf', box);
    f.onsubmit = async (e) => {
      e.preventDefault();
      try {
        await api('POST', '/api/github/settings', { repo: f.repo.value, token: f.token.value, pubKey: f.pubKey.value, auto: f.auto.checked });
        toast('Gespeichert', 'ok'); this.drawGitHub();
      } catch (ex) { toast('Fehler', 'error', ex.message, 7000); }
    };
    const chk = $('#ghcheck', box);
    if (chk) chk.onclick = async () => {
      chk.disabled = true; chk.innerHTML = '<span class="spin"></span> Prüfe …';
      try { const r = await api('POST', '/api/github/check'); toast(r.status.error ? 'Prüfung fehlgeschlagen' : 'Geprüft', r.status.error ? 'error' : 'ok', r.status.error || `Neuestes Release: ${r.status.tag}`); }
      catch (ex) { toast('Fehler', 'error', ex.message); }
      this.drawGitHub(); this.drawUpdates();
    };
    const clr = $('#ghclear', box);
    if (clr) clr.onclick = async () => { await api('POST', '/api/github/settings', { clearToken: true }); this.drawGitHub(); };
    const up = $('#srvup', box);
    if (up) up.onclick = async () => {
      const text = box2
        ? 'Coolify lädt das neue Image und startet den Server neu – das dauert etwa eine Minute. PCs verbinden sich automatisch wieder.'
        : 'Die neue Version wird von GitHub geladen und geprüft. Der Server startet danach neu – das dauert nur ein paar Sekunden. PCs verbinden sich automatisch wieder.';
      if (!await confirmModal(`Server auf ${esc(st.serverUpdate)} aktualisieren?`, text, 'Aktualisieren')) return;
      try {
        const r = await api('POST', '/api/github/update-server');
        toast(`Server wird auf ${r.version} aktualisiert`, 'info', r.coolify ? 'Coolify baut neu, die Seite lädt in einer Minute neu.' : 'Neustart läuft, die Seite lädt gleich neu.', 8000);
        setTimeout(() => location.reload(), r.coolify ? 60000 : 6000);
      } catch (ex) { toast('Server-Update fehlgeschlagen', 'error', ex.message, 9000); this.drawGitHub(); }
    };
  }
  async drawUpdates() {
    const box = $('#upd', this.main);
    let u;
    try { u = await api('GET', '/api/updates'); } catch (e) { box.innerHTML = `<p class="muted">${esc(e.message)}</p>`; return; }
    if (S.info && u.available) S.info.agentLatest = u.available.version;
    const a = u.available, st = u.settings;
    const total = Object.values(u.versions).reduce((x, y) => x + y, 0);
    const current = a ? (u.versions[a.version] || 0) : 0;
    box.innerHTML = `<div class="row" style="margin-bottom:6px"><h2>Agent-Updates</h2><span class="spacer"></span>
        ${a ? `<span class="pill ${u.outdated ? 'warn' : 'ok'}">${u.outdated ? `${u.outdated} veraltet` : 'alle aktuell'}</span>` : ''}</div>
      ${a ? `<p class="muted" style="margin:0 0 14px">Verfügbar: <b>Version ${esc(a.version)}</b> · ${a.source === 'eingebaut' ? 'im Server eingebaut' : esc(a.source) + ' · ' + esc(ago(a.uploadedAt))} · <span class="mono small">SHA-256 ${esc(a.sha256.slice(0, 16))}…</span></p>
        <div class="chips" style="margin-bottom:14px">${Object.entries(u.versions).sort((x, y) => verLess(x[0], y[0]) ? 1 : -1).map(([v, n]) => `<span class="badge ${v === a.version ? 'accent' : 'warn'}">v${esc(v)}: ${n} PC${n > 1 ? 's' : ''}</span>`).join('')}${total ? '' : '<span class="faint small">Noch keine PCs.</span>'}</div>` : '<p class="muted">Im Server ist noch kein Agent enthalten.</p>'}
      <div class="form-grid">
        <label class="check"><input type="checkbox" id="au" ${st.autoUpdate ? 'checked' : ''}> <span>PCs automatisch aktualisieren<small>Jeder PC holt sich neue Versionen selbst, prüft Prüfsumme und Signatur und stellt bei Problemen die alte Version wieder her.</small></span></label>
        <label class="check"><input type="checkbox" id="tf" ${st.testFirst ? 'checked' : ''}> <span>Neue Versionen zuerst nur an Testgeräte<small>${u.testers ? `${u.testers} Testgerät${u.testers > 1 ? 'e' : ''}` : 'Noch kein Testgerät – markiere deinen eigenen PC unter Gerät › Einstellungen.'} Danach gibst du hier für alle frei.</small></span></label>
      </div>
      ${a && st.testFirst && !u.released ? `<div class="callout warn" style="margin-top:14px">${icon('info')}<div style="flex:1">Version ${esc(a.version)} geht bisher nur an Testgeräte.</div><button class="btn sm primary" id="rel">Für alle freigeben</button></div>` : ''}
      <div class="row" style="margin-top:16px">
        <button class="btn" id="upall" ${u.outdated ? '' : 'disabled'}>${icon('refresh')} Alle veralteten jetzt aktualisieren</button>
        <label class="btn" style="cursor:pointer">${icon('download')} Neue Agent-Version hochladen<input type="file" id="upf" accept=".exe" hidden></label>
      </div>
      <p class="small faint" style="margin:12px 0 0">${S.info?.container
        ? 'Neue Version veröffentlichen: <code>git tag v1.2.3</code> und <code>git push origin v1.2.3</code> – GitHub baut alles, Coolify startet den neuen Server.'
        : 'Neue Version veröffentlichen: Git-Tag pushen (GitHub-Release) oder <code>build.sh</code> mit höherer Versionsnummer ausführen und die Server-Datei austauschen – oder hier nur die neue <code>nicolaj-agent.exe</code> hochladen.'}</p>`;
    const save = async (body) => { try { await api('POST', '/api/updates/settings', body); toast('Gespeichert', 'ok'); this.drawUpdates(); } catch (e) { toast('Fehler', 'error', e.message); } };
    $('#au', box).onchange = (e) => save({ autoUpdate: e.target.checked });
    $('#tf', box).onchange = (e) => save({ testFirst: e.target.checked });
    const rel = $('#rel', box);
    if (rel) rel.onclick = async () => {
      if (!await confirmModal(`Version ${esc(a.version)} für alle freigeben?`, 'Alle PCs mit Auto-Update holen sich die neue Version in den nächsten Minuten. Offline-PCs, sobald sie wieder online sind.', 'Freigeben')) return;
      await api('POST', '/api/updates/release').catch((e) => toast('Fehler', 'error', e.message));
      toast('Freigegeben', 'ok'); this.drawUpdates();
    };
    $('#upall', box).onclick = async () => {
      const list = [...S.devices.values()].filter(isOutdated);
      if (!list.length || !await confirmModal('Alle veralteten PCs aktualisieren?', `${list.length} PC${list.length > 1 ? 's' : ''} bekommen sofort Version ${esc(a.version)} – unabhängig von der Testgeräte-Regel. Offline-PCs, sobald sie online sind.`, 'Aktualisieren')) return;
      for (const d of list) await sendCommand(d.id, 'agent.update', {}, { quiet: true }).catch(() => {});
      toast(`Update an ${list.length} PC${list.length > 1 ? 's' : ''} gesendet`, 'ok');
    };
    $('#upf', box).onchange = async (e) => {
      const file = e.target.files[0];
      if (!file) return;
      toast('Wird hochgeladen …', 'info', file.name);
      try {
        const r = await fetch('/api/updates/upload', { method: 'POST', headers: { 'X-NRMM': '1', 'Content-Type': 'application/octet-stream' }, body: file, credentials: 'same-origin' });
        const j = await r.json().catch(() => ({}));
        if (!r.ok) throw new Error(j.error || `Fehler ${r.status}`);
        toast(`Version ${j.available.version} hochgeladen`, 'ok', j.settings.testFirst ? 'Geht zuerst an Testgeräte.' : 'Wird an alle PCs verteilt.');
        this.drawUpdates();
      } catch (ex) { toast('Upload fehlgeschlagen', 'error', ex.message, 8000); }
      e.target.value = '';
    };
  }
  drawTfa() {
    const box = $('#tfa', this.main);
    if (S.me.totpEnabled) {
      box.innerHTML = `<div class="row"><h2>Zwei-Faktor-Anmeldung</h2><span class="pill ok">Aktiv</span></div>
        <p class="muted">Beim Anmelden wird zusätzlich ein Code aus deiner Authenticator-App abgefragt.</p>
        <button class="btn danger" id="off">Deaktivieren</button>`;
      $('#off', box).onclick = () => openModal({
        title: '2FA deaktivieren', submitLabel: 'Deaktivieren', danger: true,
        body: `<label class="field">Passwort<input class="input" type="password" name="password" required></label>
          <label class="field">Aktueller Code<input class="input mono" name="code" inputmode="numeric" maxlength="7" required></label>`,
        onSubmit: async (f) => { await api('POST', '/api/account/totp/disable', { password: f.password.value, code: f.code.value }); S.me.totpEnabled = false; this.drawTfa(); toast('2FA deaktiviert', 'ok'); },
      });
      return;
    }
    box.innerHTML = `<div class="row"><h2>Zwei-Faktor-Anmeldung</h2><span class="pill warn">Aus</span></div>
      <p class="muted">Dringend empfohlen: Wer dein Dashboard öffnet, kann alle PCs steuern. Mit 2FA reicht ein gestohlenes Passwort nicht.</p>
      <button class="btn primary" id="on">${icon('shield')} Einrichten</button>`;
    $('#on', box).onclick = async () => {
      const t = await api('POST', '/api/account/totp/begin');
      openModal({
        title: '2FA einrichten', submitLabel: 'Aktivieren',
        body: `<ol class="steps"><li>Authenticator-App öffnen (z. B. Google Authenticator, Microsoft Authenticator, 2FAS, Bitwarden).</li>
          <li>Neues Konto hinzufügen → „Schlüssel eingeben“ und diesen Schlüssel eintragen:</li></ol>
          <div class="secret">${esc(t.secret.replace(/(.{4})/g, '$1 ').trim())}</div>
          <div class="row"><button type="button" class="btn sm" id="cps">${icon('copy', 14)} Schlüssel kopieren</button><a class="btn sm" href="${esc(t.uri)}">Auf dem Handy direkt öffnen</a></div>
          <label class="field">Angezeigten 6-stelligen Code eingeben<input class="input mono" name="code" inputmode="numeric" maxlength="7" required autocomplete="one-time-code"></label>`,
        onOpen: (f) => { $('#cps', f).onclick = () => copyText(t.secret); },
        onSubmit: async (f) => { await api('POST', '/api/account/totp/enable', { code: f.code.value }); S.me.totpEnabled = true; this.drawTfa(); toast('2FA ist aktiv', 'ok'); },
      });
    };
  }
  async drawServer() {
    const box = $('#srv', this.main);
    const i = S.info = await api('GET', '/api/server-info').catch(() => S.info);
    if (!i) return;
    box.innerHTML = `<h2 style="margin-bottom:12px">Server</h2><dl class="kv">
      <dt>Version</dt><dd>${esc(i.version)}</dd>
      <dt>Adressen</dt><dd style="white-space:normal">${(i.addresses || []).map(esc).join('<br>')}</dd>
      <dt>Verschlüsselung</dt><dd>${i.tls ? 'HTTPS (eigenes Zertifikat)' : i.behindProxy ? 'HTTPS über den Proxy (z. B. Coolify)' : 'HTTP – nur hinter Reverse-Proxy!'}</dd>
      ${i.container ? '<dt>Betrieb</dt><dd>Docker-Container</dd>' : ''}
      ${i.tls ? `<dt>Fingerabdruck</dt><dd class="mono small" style="white-space:normal;word-break:break-all">${esc(i.fingerprintPretty)}</dd>` : ''}
      <dt>Agent</dt><dd>${i.agentAvailable ? `<a href="/download/nicolaj-agent.exe">${icon('download', 14)} nicolaj-agent.exe</a>` : '<span class="badge warn">nicht eingebaut</span>'}</dd></dl>`;
  }
  async drawCodes() {
    const box = $('#codes', this.main);
    const codes = await api('GET', '/api/enroll-codes').catch(() => []);
    const st = (c) => c.usedAt ? '<span class="pill ok">benutzt</span>' : new Date(c.expiresAt) < new Date() ? '<span class="pill">abgelaufen</span>' : '<span class="pill warn">offen</span>';
    box.innerHTML = `<header><h2>Einrichtungscodes</h2><button class="btn sm primary" id="nc">${icon('plus', 14)} Gerät hinzufügen</button></header>
      ${codes.length ? `<div class="table-wrap"><table class="table"><thead><tr><th>Code</th><th>Name</th><th>Status</th><th>Gültig bis</th><th></th></tr></thead><tbody>
      ${codes.map((c) => `<tr><td class="mono">${esc(c.code)}</td><td>${esc(c.label || '–')}</td><td>${st(c)}</td><td class="muted">${esc(fmtDate(c.expiresAt))}</td>
        <td class="actions"><button class="btn sm ghost" data-del="${esc(c.code)}">${icon('trash', 14)}</button></td></tr>`).join('')}</tbody></table></div>` : '<div class="empty">Keine Codes.</div>'}`;
    $('#nc', box).onclick = () => addDeviceModal();
    box.onclick = async (e) => {
      const b = e.target.closest('[data-del]');
      if (!b) return;
      await api('DELETE', `/api/enroll-codes/${b.dataset.del}`).catch(() => {});
      this.drawCodes();
    };
  }
  onDevice() { /* nichts */ }
}

boot();
