const RPC = '/api/rpc';
const WS_URL = `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/api/ws`;

let blockCount = 0;       // getblockcount result (number of blocks)
const BLOCKS_PER_PAGE = 15;
const TXS_PER_PAGE = 25;
const BLOCK_TXS_PER_PAGE = 20;
const CONTRACTS_PER_PAGE = 20;
const MEMPOOL_PER_PAGE = 25;
let currentView = 'blocks';
let deployedContractsCache = { height: -1, list: [] };
let renderGen = 0;
const viewRefreshTimers = new Map();
const appLogCache = new Map();
const activityBatch = { blocks: 0, txs: 0, timer: null };
let lastBlockToastAt = 0;
let ws = null;
let wsReqId = 1;
let wsPending = new Map();
const activityLog = [];
const MAX_ACTIVITY = 30;
const contractNameCache = new Map();
const contractManifestCache = new Map();
let gasTokenHash = null;

// ── RPC helpers ──────────────────────────────────────────────

async function rpc(method, params = []) {
  let res;
  try {
    res = await fetch(RPC, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
    });
  } catch (_) {
    throw new Error('Cannot reach nanvil RPC — is the node running?');
  }
  if (!res.ok) {
    throw new Error(`RPC request failed (HTTP ${res.status})`);
  }
  let data;
  try {
    data = await res.json();
  } catch (_) {
    throw new Error('Invalid RPC response from node');
  }
  if (data.error) throw new Error(data.error.message || JSON.stringify(data.error));
  return data.result;
}

function beginRender() {
  return ++renderGen;
}

function isStale(gen) {
  return gen !== renderGen;
}

function scheduleViewRefresh(view, fn) {
  clearTimeout(viewRefreshTimers.get(view));
  viewRefreshTimers.set(view, setTimeout(() => {
    viewRefreshTimers.delete(view);
    if (currentView === view) fn();
  }, 500));
}

async function mapPool(items, limit, fn) {
  const results = new Array(items.length);
  let next = 0;
  async function worker() {
    while (next < items.length) {
      const idx = next++;
      results[idx] = await fn(items[idx], idx);
    }
  }
  const workers = Math.max(1, Math.min(limit, items.length));
  await Promise.all(Array.from({ length: workers }, worker));
  return results;
}

async function getApplicationLogCached(hash) {
  const key = String(hash);
  if (appLogCache.has(key)) return appLogCache.get(key);
  const log = await rpc('getapplicationlog', [hash]);
  if (appLogCache.size >= 500) {
    const oldest = appLogCache.keys().next().value;
    appLogCache.delete(oldest);
  }
  appLogCache.set(key, log);
  return log;
}

function latestIndex() {
  return blockCount > 0 ? blockCount - 1 : 0;
}

function routeParts() {
  const hash = location.hash.slice(1) || '/blocks';
  return hash.split('/').filter(Boolean);
}

function parseListPage(parts, pageIdx = 1) {
  if (parts[pageIdx] === 'p' && /^\d+$/.test(parts[pageIdx + 1])) {
    return parseInt(parts[pageIdx + 1], 10);
  }
  return 0;
}

function listHref(view, page) {
  return page > 0 ? `#/${view}/p/${page}` : `#/${view}`;
}

function blockHref(id, txPage) {
  return txPage > 0 ? `#/block/${id}/p/${txPage}` : `#/block/${id}`;
}

function pagerHtml({ page, pageCount, range, hasOlder, hasNewer, olderHref, newerHref }) {
  const meta = [
    `Page ${page + 1}`,
    pageCount ? `of ${pageCount}` : '',
    range ? `· ${range}` : '',
  ].filter(Boolean).join(' ');
  return `
    <div class="pager">
      <button class="pager-older" ${!hasOlder ? 'disabled' : ''} data-href="${esc(olderHref || '')}">← Older</button>
      <span class="pager-meta">${meta}</span>
      <button class="pager-newer" ${!hasNewer ? 'disabled' : ''} data-href="${esc(newerHref || '')}">Newer →</button>
    </div>`;
}

function bindPager(root) {
  if (!root) return;
  root.querySelector('.pager-older')?.addEventListener('click', e => {
    const href = e.currentTarget.dataset.href;
    if (href) location.hash = href;
  });
  root.querySelector('.pager-newer')?.addEventListener('click', e => {
    const href = e.currentTarget.dataset.href;
    if (href) location.hash = href;
  });
}

function paginateSlice(items, page, perPage) {
  const total = items.length;
  const pageCount = Math.max(1, Math.ceil(total / perPage));
  const safePage = Math.min(Math.max(0, page), pageCount - 1);
  const start = safePage * perPage;
  const slice = items.slice(start, start + perPage);
  const range = total ? `${start + 1}–${start + slice.length} of ${total}` : '';
  return { slice, page: safePage, pageCount, range, hasOlder: safePage < pageCount - 1, hasNewer: safePage > 0 };
}

// ── Formatting ─────────────────────────────────────────────

function esc(s) {
  if (s == null) return '';
  const d = document.createElement('div');
  d.textContent = String(s);
  return d.innerHTML;
}

function shortHash(h, n = 8) {
  if (!h) return '—';
  const s = String(h);
  if (s.length <= n * 2 + 2) return s;
  return s.slice(0, n + 2) + '…' + s.slice(-n);
}

function formatTime(ms) {
  if (!ms) return '—';
  return new Date(Number(ms)).toLocaleString();
}

function formatGAS(amount) {
  if (amount == null || amount === '') return '—';
  const n = BigInt(amount);
  const whole = n / 100000000n;
  const frac = n % 100000000n;
  if (frac === 0n) return whole.toString() + ' GAS';
  return (Number(n) / 1e8).toFixed(8) + ' GAS';
}

function b64ToHex(b64) {
  if (!b64) return '—';
  try {
    return [...atob(b64)].map(c => c.charCodeAt(0).toString(16).padStart(2, '0')).join('');
  } catch {
    return String(b64);
  }
}

function b64ToBytes(b64) {
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

function hexToBytes(hex) {
  const s = hex.replace(/^0x/i, '');
  const out = new Uint8Array(s.length / 2);
  for (let i = 0; i < out.length; i++) {
    out[i] = parseInt(s.slice(i * 2, i * 2 + 2), 16);
  }
  return out;
}

const BASE58_ALPHABET = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
const addressCache = new Map();

async function sha256Bytes(data) {
  return new Uint8Array(await crypto.subtle.digest('SHA-256', data));
}

function base58Encode(bytes) {
  let num = 0n;
  for (const b of bytes) num = (num << 8n) + BigInt(b);
  let encoded = '';
  while (num > 0n) {
    const rem = Number(num % 58n);
    num /= 58n;
    encoded = BASE58_ALPHABET[rem] + encoded;
  }
  for (const b of bytes) {
    if (b === 0) encoded = '1' + encoded;
    else break;
  }
  return encoded || '1';
}

async function uint160LeToAddress(leBytes) {
  if (!leBytes || leBytes.length !== 20) return null;
  if (leBytes.every(b => b === 0)) return 'null';
  const be = leBytes.slice().reverse();
  const payload = new Uint8Array(21);
  payload[0] = 0x35; // NEO3 address prefix
  payload.set(be, 1);
  const h1 = await sha256Bytes(payload);
  const h2 = await sha256Bytes(h1);
  const full = new Uint8Array(25);
  full.set(payload);
  full.set(h2.slice(0, 4), 21);
  return base58Encode(full);
}

async function decodeStackHash160(item) {
  if (!item) return '—';
  const raw = item.value ?? item;
  const cacheKey = typeof raw === 'string' ? raw : JSON.stringify(raw);
  if (addressCache.has(cacheKey)) return addressCache.get(cacheKey);

  let leBytes;
  if (typeof raw === 'string') {
    if (/^[0-9a-fA-F]{40}$/.test(raw)) {
      leBytes = hexToBytes(raw);
    } else if (/^0x[0-9a-fA-F]{40}$/i.test(raw)) {
      leBytes = hexToBytes(raw);
    } else {
      try { leBytes = b64ToBytes(raw); } catch { leBytes = null; }
    }
  }
  if (!leBytes || leBytes.length !== 20) {
    const fallback = shortHash(String(raw), 4);
    addressCache.set(cacheKey, fallback);
    return fallback;
  }
  const addr = await uint160LeToAddress(leBytes);
  const result = addr || shortHash(b64ToHex(typeof raw === 'string' ? raw : ''), 4);
  addressCache.set(cacheKey, result);
  return result;
}

function decodeStackInteger(item) {
  if (!item) return null;
  const v = item.value ?? item;
  if (typeof v === 'number') return String(v);
  if (typeof v === 'string' && /^-?\d+$/.test(v)) return v;
  if (item.type === 'Integer' && v != null) return String(v);
  return null;
}

function contractKey(hash) {
  return String(hash || '').toLowerCase().replace(/^0x/i, '');
}

function contractHashForRPC(hash) {
  const key = contractKey(hash);
  return key ? '0x' + key : '';
}

function invocationContractHash(inv) {
  return inv?.contract || inv?.hash || '';
}

function invocationArgsArray(inv) {
  const args = inv?.arguments;
  if (!args) return null;
  if (args.type === 'Array' && Array.isArray(args.value)) return args.value;
  if (Array.isArray(args)) return args;
  return null;
}

function findFullInvocation(executions, inv) {
  const key = contractKey(invocationContractHash(inv));
  const method = inv.method;
  for (const ex of executions || []) {
    for (const full of ex.invocations || []) {
      if (contractKey(invocationContractHash(full)) === key && full.method === method) {
        return full;
      }
    }
  }
  return null;
}

function normalizeParamType(type) {
  if (!type) return 'Any';
  if (typeof type === 'string') return type;
  return String(type);
}

function findAbiMethod(manifest, methodName, argc) {
  const methods = manifest?.abi?.methods || [];
  const exact = methods.find(m => m.name === methodName && (m.parameters || []).length === argc);
  if (exact) return exact;
  const matches = methods.filter(m => m.name === methodName);
  if (matches.length === 1) return matches[0];
  return matches.find(m => m.name === methodName) || null;
}

function decodeStackString(item) {
  if (!item) return '—';
  const v = item.value;
  if (typeof v === 'string' && !/[=+/]/.test(v) && v.length < 64) return v;
  try {
    const bytes = b64ToBytes(String(v));
    const text = new TextDecoder().decode(bytes);
    if (text && /^[\x20-\x7E\u00A0-\uFFFF]*$/.test(text)) return text;
    return '0x' + [...bytes].map(b => b.toString(16).padStart(2, '0')).join('');
  } catch {
    return String(v ?? '—');
  }
}

function isEmptyStackItem(item) {
  if (!item) return true;
  if (item.type === 'Any' && (item.value == null || item.value === '')) return true;
  const v = item.value;
  if ((item.type === 'ByteString' || item.type === 'Buffer') && typeof v === 'string') {
    try { return b64ToBytes(v).length === 0; } catch { return !v; }
  }
  return false;
}

async function formatHash256(item) {
  if (!item?.value) return '—';
  try {
    const bytes = typeof item.value === 'string' ? b64ToBytes(item.value) : item.value;
    if (bytes.length !== 32) return shortHash(b64ToHex(item.value), 8);
    const hex = '0x' + [...bytes].map(b => b.toString(16).padStart(2, '0')).join('');
    return hex;
  } catch {
    return shortHash(String(item.value), 8);
  }
}

async function formatTypedStackItem(item, param, contractHash) {
  const type = normalizeParamType(param?.type);
  if (isEmptyStackItem(item)) return 'null';

  switch (type) {
    case 'Hash160':
      return await decodeStackHash160(item);
    case 'Hash256':
      return await formatHash256(item);
    case 'Integer': {
      const n = decodeStackInteger(item);
      if (param?.name === 'amount' || param?.name === 'quantity') {
        return formatTokenAmount(n, contractHash);
      }
      return n ?? '—';
    }
    case 'Boolean':
      return String(item.value ?? false);
    case 'String':
      return decodeStackString(item);
    case 'ByteArray':
      return '0x' + b64ToHex(item.value);
    case 'PublicKey':
      return '0x' + b64ToHex(item.value);
    case 'Signature':
      return shortHash(b64ToHex(item.value), 8);
    case 'Array':
    case 'Map':
    case 'InteropInterface':
      return await stackItemPreview(item);
    case 'Any':
    default:
      if (param?.name === 'amount') {
        return formatTokenAmount(decodeStackInteger(item), contractHash);
      }
      if (item.type === 'ByteString' || item.type === 'Buffer') {
        return await decodeStackHash160(item);
      }
      return await stackItemPreview(item);
  }
}

async function getContractManifest(hash) {
  const key = contractKey(hash);
  if (!key) return null;
  if (contractManifestCache.has(key)) return contractManifestCache.get(key);
  try {
    const cs = await rpc('getcontractstate', [contractHashForRPC(hash)]);
    if (cs?.manifest) {
      contractManifestCache.set(key, cs.manifest);
      if (cs.manifest.name) contractNameCache.set(key, cs.manifest.name);
    }
    return cs?.manifest || null;
  } catch {
    return null;
  }
}

function blockTime(block) {
  return block?.time ?? block?.timestamp ?? 0;
}

function txType(tx) {
  if (!tx.script || tx.script.length === 0) return 'Miner';
  return 'Invocation';
}

function formatScopes(scopes) {
  if (scopes == null || scopes === '') return '—';
  if (Array.isArray(scopes)) return scopes.join(', ');
  return String(scopes);
}

function normalizeHash(input) {
  if (!input) return '';
  const s = String(input).trim().replace(/^0x/i, '');
  if (!/^[0-9a-fA-F]{40}$/.test(s)) return '';
  return '0x' + s.toLowerCase();
}

function normalizeTxHash(input) {
  if (!input) return '';
  const s = String(input).trim().replace(/^0x/i, '');
  if (!/^[0-9a-fA-F]{64}$/.test(s)) return '';
  return '0x' + s.toLowerCase();
}

async function loadContractCache() {
  try {
    const natives = await rpc('getnativecontracts');
    for (const c of natives || []) {
      const h = contractKey(c.hash);
      if (h) {
        contractNameCache.set(h, c.manifest?.name || 'Native');
        const lower = (c.manifest?.name || '').toLowerCase();
        if (lower === 'gas' || lower === 'gastoken') {
          gasTokenHash = h;
        }
      }
    }
  } catch (_) {}
}

async function getContractName(hash) {
  const key = contractKey(hash);
  if (!key) return 'Unknown';
  if (contractNameCache.has(key)) return contractNameCache.get(key);
  try {
    const cs = await rpc('getcontractstate', [contractHashForRPC(hash)]);
    const name = cs.manifest?.name || shortHash('0x' + key);
    contractNameCache.set(key, name);
    const lower = name.toLowerCase();
    if (lower === 'gas' || lower === 'gastoken') gasTokenHash = key;
    return name;
  } catch {
    return shortHash('0x' + key);
  }
}

function isGasToken(assetHash) {
  const key = contractKey(assetHash);
  if (!key) return false;
  if (gasTokenHash && key === gasTokenHash) return true;
  const name = contractNameCache.get(key);
  return name && (name.toLowerCase() === 'gastoken' || name.toLowerCase() === 'gas');
}

function formatTokenAmount(amount, assetHash) {
  if (amount == null || amount === '') return '—';
  if (isGasToken(assetHash)) return formatGAS(amount);
  const n = BigInt(amount);
  if (n < 100000000n) return n.toString();
  return (Number(n) / 1e8).toFixed(8);
}

async function stackItemPreview(item) {
  if (!item) return '—';
  if (item.type === 'ByteString' || item.type === 'Buffer') {
    const v = item.value;
    if (typeof v === 'string') {
      if (/^[0-9a-fA-F]{40}$/.test(v) || /^0x[0-9a-fA-F]{40}$/i.test(v)) {
        return await decodeStackHash160(item);
      }
      if (v.length <= 24 && !/[=+/]/.test(v)) return v;
      return await decodeStackHash160(item);
    }
    return shortHash(v, 6);
  }
  if (item.type === 'Integer') return String(item.value ?? '');
  if (item.type === 'Boolean') return String(item.value);
  if (item.type === 'Array' && item.value) {
    const parts = await Promise.all(item.value.map(stackItemPreview));
    return '[' + parts.join(', ') + ']';
  }
  return `${item.type}: ${item.value ?? ''}`;
}

async function formatInvocationArgs(method, args, assetHash) {
  if (!args?.length) return '—';
  if (method === 'transfer' && args.length >= 3) {
    const from = await decodeStackHash160(args[0]);
    const to = await decodeStackHash160(args[1]);
    const amount = formatTokenAmount(decodeStackInteger(args[2]), assetHash);
    return `${from} → ${to}, ${amount}`;
  }
  const parts = await Promise.all(args.map(stackItemPreview));
  return parts.join(', ');
}

function summarizeInvocations(executions) {
  const methods = [];
  for (const ex of executions || []) {
    for (const inv of ex.invocations || []) {
      methods.push({
        contract: invocationContractHash(inv),
        method: inv.method || '—',
        argc: inv.argumentscount ?? 0,
        truncated: !!inv.truncated,
        raw: inv,
      });
    }
  }
  return methods;
}

function summarizeTransfers(executions) {
  const transfers = [];
  for (const ex of executions || []) {
    for (const n of ex.notifications || []) {
      if (n.eventname === 'Transfer' && n.state?.value?.length >= 3) {
        transfers.push({
          fromItem: n.state.value[0],
          toItem: n.state.value[1],
          amount: decodeStackInteger(n.state.value[2]),
          asset: n.contract,
        });
      }
    }
  }
  return transfers;
}

async function resolveSearch(query) {
  const q = query.trim();
  if (!q) return { error: 'Enter a block number, hash, or contract name' };

  if (/^\d+$/.test(q)) {
    const idx = parseInt(q, 10);
    if (idx >= 0 && idx <= latestIndex()) {
      return { type: 'block', id: String(idx) };
    }
    return { error: `Block #${idx} not found (chain height ${latestIndex()})` };
  }

  const txHash = normalizeTxHash(q);
  if (txHash) {
    try {
      await rpc('getrawtransaction', [txHash, true]);
      return { type: 'tx', id: txHash };
    } catch (_) {}
  }

  const contractHash = normalizeHash(q);
  if (contractHash) {
    try {
      await rpc('getblock', [contractHash, 0]);
      return { type: 'block', id: contractHash };
    } catch (_) {}
    try {
      await rpc('getrawtransaction', [contractHash, true]);
      return { type: 'tx', id: contractHash };
    } catch (_) {}
    try {
      await rpc('getcontractstate', [contractHash]);
      return { type: 'contract', id: contractHash };
    } catch (_) {}
    return { error: 'Hash not found as block, transaction, or contract' };
  }

  const needle = q.toLowerCase();
  try {
    const natives = await rpc('getnativecontracts');
    for (const c of natives || []) {
      const name = (c.manifest?.name || '').toLowerCase();
      if (name === needle || name.includes(needle)) {
        return { type: 'contract', id: c.hash };
      }
    }
    const deployed = await findDeployedContracts();
    for (const c of deployed) {
      const name = (c.name || '').toLowerCase();
      if (name === needle || name.includes(needle)) {
        return { type: 'contract', id: c.hash };
      }
    }
  } catch (_) {}

  return { error: `No match for "${q}"` };
}

async function handleSearch(ev) {
  ev.preventDefault();
  const input = document.getElementById('search-input');
  const q = input?.value?.trim();
  if (!q) return;
  const btn = document.querySelector('.search-btn');
  if (btn) btn.disabled = true;
  try {
    await refreshStats(true);
    const hit = await resolveSearch(q);
    if (hit.error) {
      showToast(hit.error, 'info');
      return;
    }
    location.hash = `#/${hit.type}/${hit.id}`;
  } finally {
    if (btn) btn.disabled = false;
  }
}

function hashLink(hash, type) {
  let h = String(hash || '');
  if (type === 'contract') {
    const key = contractKey(h);
    h = key ? '0x' + key : h;
  }
  const safe = esc(h);
  if (type === 'tx') return `<a class="hash-link mono" href="#/tx/${safe}">${shortHash(safe)}</a>`;
  if (type === 'block') return `<a class="hash-link mono" href="#/block/${safe}">${shortHash(safe)}</a>`;
  if (type === 'contract') return `<a class="hash-link mono" href="#/contract/${safe}">${shortHash(safe)}</a>`;
  return `<span class="mono">${shortHash(safe)}</span>`;
}

function contractCell(hash, name) {
  const displayName = name && name !== 'Unknown' ? name : shortHash(contractHashForRPC(hash));
  return `<span class="contract-name">${esc(displayName)}</span> ${hashLink(hash, 'contract')}`;
}

function bumpStat(id) {
  const el = document.getElementById(id);
  if (!el) return;
  el.classList.remove('bump');
  void el.offsetWidth;
  el.classList.add('bump');
}

function showToast(msg, kind = 'info') {
  const el = document.getElementById('activity-toast');
  if (!el) return;
  el.hidden = false;
  el.className = `activity-toast toast-${kind} toast-show`;
  el.textContent = msg;
  clearTimeout(showToast._t);
  showToast._t = setTimeout(() => {
    el.classList.remove('toast-show');
    setTimeout(() => { el.hidden = true; }, 300);
  }, 2800);
}

function addActivity(kind, message) {
  activityLog.unshift({ time: new Date(), kind, message });
  if (activityLog.length > MAX_ACTIVITY) activityLog.pop();
  const feed = document.getElementById('activity-feed');
  if (feed) renderActivityFeed(feed);
}

function flushActivityBatch() {
  if (activityBatch.blocks > 0) {
    const n = activityBatch.blocks;
    addActivity('BLOCK', n === 1 ? '1 new block' : `${n} new blocks`);
    activityBatch.blocks = 0;
  }
  if (activityBatch.txs > 0) {
    const n = activityBatch.txs;
    addActivity('TX', n === 1 ? '1 transaction' : `${n} transactions`);
    activityBatch.txs = 0;
  }
  activityBatch.timer = null;
}

function addActivityBatched(kind) {
  if (kind === 'BLOCK') activityBatch.blocks++;
  else if (kind === 'TX') activityBatch.txs++;
  else return;
  if (!activityBatch.timer) {
    activityBatch.timer = setTimeout(flushActivityBatch, 600);
  }
}

function showBlockToast(idx) {
  const now = Date.now();
  if (now - lastBlockToastAt < 2500) return;
  lastBlockToastAt = now;
  showToast(`New block #${idx}`, 'block');
}

// ── WebSocket live feed ────────────────────────────────────

function setLiveStatus(connected) {
  const el = document.getElementById('stat-live');
  if (!el) return;
  el.classList.toggle('connected', connected);
  el.classList.toggle('disconnected', !connected);
  el.title = connected ? 'Connected — receiving live events' : 'Disconnected — retrying…';
}

function wsSend(method, params) {
  return new Promise((resolve, reject) => {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      reject(new Error('websocket not connected'));
      return;
    }
    const id = wsReqId++;
    wsPending.set(id, { resolve, reject });
    ws.send(JSON.stringify({ jsonrpc: '2.0', id, method, params }));
    setTimeout(() => {
      if (wsPending.has(id)) {
        wsPending.delete(id);
        reject(new Error('ws timeout'));
      }
    }, 10000);
  });
}

function connectWS() {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return;
  ws = new WebSocket(WS_URL);

  ws.onopen = async () => {
    setLiveStatus(true);
    try {
      await wsSend('subscribe', ['block_added', null]);
      await wsSend('subscribe', ['transaction_added', null]);
      await wsSend('subscribe', ['mempool_event', null]);
      await wsSend('subscribe', ['transaction_executed', null]);
    } catch (e) {
      console.warn('subscribe failed', e);
    }
  };

  ws.onclose = () => {
    setLiveStatus(false);
    setTimeout(connectWS, 3000);
  };

  ws.onerror = () => setLiveStatus(false);

  ws.onmessage = (ev) => {
    let msg;
    try { msg = JSON.parse(ev.data); } catch { return; }

    if (msg.id != null && wsPending.has(msg.id)) {
      const { resolve, reject } = wsPending.get(msg.id);
      wsPending.delete(msg.id);
      if (msg.error) reject(new Error(msg.error.message));
      else resolve(msg.result);
      return;
    }

    handleWSNotification(msg);
  };
}

function handleWSNotification(msg) {
  const event = msg.method;
  const payload = msg.params?.[0];
  if (!event || !payload) return;

  switch (event) {
    case 'block_added': {
      const idx = payload.index;
      refreshStats(true);
      addActivityBatched('BLOCK');
      showBlockToast(idx);
      if (currentView === 'blocks' && parseListPage(routeParts()) === 0) {
        scheduleViewRefresh('blocks', () => renderBlocks(true));
      }
      deployedContractsCache.height = -1;
      break;
    }
    case 'transaction_added': {
      addActivityBatched('TX');
      if (currentView === 'mempool') {
        scheduleViewRefresh('mempool', () => renderMempool(false));
      }
      if (currentView === 'transactions' && parseListPage(routeParts()) === 0) {
        scheduleViewRefresh('transactions', () => renderTransactions(false));
      }
      break;
    }
    case 'mempool_event': {
      const tx = payload.transaction;
      const h = tx?.hash;
      const removed = payload.type === 'removed';
      addActivity('MEMPOOL', `${removed ? '−' : '+'} ${shortHash(h)}`);
      refreshStats(true);
      if (currentView === 'mempool') {
        scheduleViewRefresh('mempool', () => renderMempool(false));
      }
      break;
    }
    case 'transaction_executed': {
      const state = payload.vmstate || payload.VMState || '—';
      const id = payload.container || payload.txid || payload.hash || '';
      addActivity('EXEC', `${shortHash(id)} → ${state}`);
      break;
    }
  }
}

// ── Stats ──────────────────────────────────────────────────

async function refreshStats(quiet = false) {
  try {
    const prev = blockCount;
    blockCount = await rpc('getblockcount');
    const mempool = await rpc('getrawmempool');
    document.getElementById('stat-height').textContent = latestIndex();
    document.getElementById('stat-mempool').textContent = Array.isArray(mempool) ? mempool.length : 0;
    if (!quiet && blockCount !== prev) bumpStat('stat-height');
  } catch (_) {
    document.getElementById('stat-height').textContent = 'offline';
  }
}

async function loadNetworkInfo() {
  try {
    const ver = await rpc('getversion');
    const magic = ver.protocol?.network ?? ver.Protocol?.network;
    if (magic != null) {
      document.getElementById('stat-network').textContent = `0x${Number(magic).toString(16)}`;
    }
  } catch (_) {
    document.getElementById('stat-network').textContent = '—';
  }
}

// ── Blocks ─────────────────────────────────────────────────

async function renderBlocks(animateNew = false) {
  const gen = beginRender();
  const app = document.getElementById('app');
  if (!animateNew) app.innerHTML = '<div class="loading">Loading blocks…</div>';
  try {
    await refreshStats(true);
    if (isStale(gen)) return;
    const page = parseListPage(routeParts());
    const top = latestIndex();
    const totalBlocks = top + 1;
    const pageCount = Math.max(1, Math.ceil(totalBlocks / BLOCKS_PER_PAGE));
    const safePage = Math.min(Math.max(0, page), pageCount - 1);
    const start = Math.max(0, top - safePage * BLOCKS_PER_PAGE);
    const end = Math.max(0, start - BLOCKS_PER_PAGE + 1);
    const rangeStart = top - start + 1;
    const rangeEnd = top - end + 1;
    const rows = [];

    for (let i = start; i >= end; i--) {
      const hash = await rpc('getblockhash', [i]);
      if (isStale(gen)) return;
      const block = await rpc('getblock', [hash, 1]);
      if (isStale(gen)) return;
      const isNew = animateNew && i === top;
      rows.push(`
        <tr class="${isNew ? 'row-new' : ''}">
          <td><a href="#/block/${i}">${block.index}</a></td>
          <td>${hashLink(block.hash, 'block')}</td>
          <td>${formatTime(blockTime(block))}</td>
          <td>${(block.tx || []).length}</td>
          <td>${formatGAS(block.sysfee || 0)}</td>
        </tr>
      `);
    }

    const pager = pagerHtml({
      page: safePage,
      pageCount,
      range: `#${rangeStart}–#${rangeEnd} of ${totalBlocks}`,
      hasOlder: safePage < pageCount - 1,
      hasNewer: safePage > 0,
      olderHref: listHref('blocks', safePage + 1),
      newerHref: listHref('blocks', safePage - 1),
    });
    app.innerHTML = `
      <div class="panel fade-in">
        <div class="panel-header">
          <h2>Recent Blocks</h2>
          ${pager}
        </div>
        <table>
          <thead>
            <tr><th>Index</th><th>Hash</th><th>Time</th><th>Txs</th><th>Sys Fee</th></tr>
          </thead>
          <tbody id="blocks-tbody">${rows.join('') || '<tr><td colspan="5" class="empty">No blocks</td></tr>'}</tbody>
        </table>
      </div>
      <div class="panel activity-panel fade-in">
        <div class="panel-header"><h2>Live Activity</h2></div>
        <ul id="activity-feed" class="activity-feed"></ul>
      </div>
    `;
    renderActivityFeed(document.getElementById('activity-feed'));
    bindPager(app.querySelector('.panel-header'));
  } catch (e) {
    if (!isStale(gen)) {
      app.innerHTML = `<div class="error-box">${esc(e.message)}</div>`;
    }
  }
}

function renderActivityFeed(el) {
  if (!el) return;
  if (!activityLog.length) {
    el.innerHTML = '<li class="empty">Waiting for chain events…</li>';
    return;
  }
  el.innerHTML = activityLog.map(e => {
    const cls = `act-${e.kind.toLowerCase()}`;
    return `<li class="${cls}"><span class="act-time">${e.time.toLocaleTimeString()}</span> <span class="act-kind">${e.kind}</span> ${esc(e.message)}</li>`;
  }).join('');
}

// ── Block detail ───────────────────────────────────────────

async function renderBlock(id, txPage = 0) {
  const app = document.getElementById('app');
  app.innerHTML = '<div class="loading">Loading block…</div>';
  try {
    let hash = id;
    if (/^\d+$/.test(id)) {
      hash = await rpc('getblockhash', [parseInt(id, 10)]);
    }
    const block = await rpc('getblock', [hash, 1]);
    const blockId = String(block.index ?? id);
    const allTxs = (block.tx || []).map(tx => (typeof tx === 'object' ? tx : { hash: tx }));
    const { slice, page, pageCount, range, hasOlder, hasNewer } = paginateSlice(allTxs, txPage, BLOCK_TXS_PER_PAGE);
    const txRows = slice.map(txObj => `
        <tr>
          <td>${hashLink(txObj.hash, 'tx')}</td>
          <td><span class="badge badge-type">${txType(txObj)}</span></td>
          <td class="mono">${esc(txObj.sender || '—')}</td>
          <td>${formatGAS(txObj.sysfee)}</td>
          <td>${formatGAS(txObj.netfee)}</td>
          <td>${txObj.size || '—'}</td>
        </tr>
      `).join('');
    const txPager = allTxs.length > BLOCK_TXS_PER_PAGE ? pagerHtml({
      page,
      pageCount,
      range,
      hasOlder,
      hasNewer,
      olderHref: blockHref(blockId, page + 1),
      newerHref: blockHref(blockId, page - 1),
    }) : '';

    app.innerHTML = `
      <div class="panel fade-in">
        <div class="panel-header">
          <a class="back-link" href="#/blocks">← Blocks</a>
          <h2>Block #${block.index}</h2>
        </div>
        <dl class="detail-grid">
          <dt>Hash</dt><dd>${esc(block.hash)}</dd>
          <dt>Previous</dt><dd>${block.previousblockhash ? hashLink(block.previousblockhash, 'block') : '—'}</dd>
          <dt>Index</dt><dd>${block.index}</dd>
          <dt>Timestamp</dt><dd>${formatTime(blockTime(block))} <span class="dim">(${blockTime(block)} ms)</span></dd>
          <dt>Size</dt><dd>${block.size || '—'} bytes</dd>
          <dt>Confirmations</dt><dd>${block.confirmations ?? '—'}</dd>
          <dt>Next Consensus</dt><dd class="mono">${esc(block.nextconsensus || '—')}</dd>
          <dt>Merkle Root</dt><dd class="mono">${esc(block.merkleroot || '—')}</dd>
          <dt>Nonce</dt><dd>${block.nonce ?? '—'}</dd>
        </dl>
        <div class="section">
          <div class="section-header">
            <h3>Transactions (${allTxs.length})</h3>
            ${txPager}
          </div>
          <table>
            <thead><tr><th>Hash</th><th>Type</th><th>Sender</th><th>Sys Fee</th><th>Net Fee</th><th>Size</th></tr></thead>
            <tbody>${txRows || '<tr><td colspan="6" class="empty">No transactions</td></tr>'}</tbody>
          </table>
        </div>
      </div>
    `;
    bindPager(app.querySelector('.section-header'));
  } catch (e) {
    app.innerHTML = `<div class="error-box">${esc(e.message)}</div><a href="#/blocks">← Back</a>`;
  }
}

// ── Transactions list ──────────────────────────────────────

async function collectRecentTxs(skip, limit, gen) {
  const txs = [];
  let skipped = 0;
  const top = latestIndex();
  for (let i = top; i >= 0; i--) {
    if (gen != null && isStale(gen)) return txs;
    const hash = await rpc('getblockhash', [i]);
    if (gen != null && isStale(gen)) return txs;
    const block = await rpc('getblock', [hash, 1]);
    const blockTxs = [...(block.tx || [])].reverse();
    for (const tx of blockTxs) {
      if (skipped < skip) {
        skipped++;
        continue;
      }
      const txObj = typeof tx === 'object' ? tx : { hash: tx };
      txs.push({
        ...txObj,
        blockindex: block.index,
        blockhash: block.hash,
        blocktime: blockTime(block),
      });
      if (txs.length >= limit) return txs;
    }
  }
  return txs;
}

async function renderTransactions(showLoading = true) {
  const gen = beginRender();
  const app = document.getElementById('app');
  if (showLoading) app.innerHTML = '<div class="loading">Loading transactions…</div>';
  try {
    await refreshStats(true);
    if (isStale(gen)) return;
    await loadContractCache();
    if (isStale(gen)) return;
    const page = parseListPage(routeParts());
    const skip = page * TXS_PER_PAGE;
    const fetched = await collectRecentTxs(skip, TXS_PER_PAGE + 1, gen);
    if (isStale(gen)) return;
    const hasOlder = fetched.length > TXS_PER_PAGE;
    const pageTxs = fetched.slice(0, TXS_PER_PAGE);
    const range = pageTxs.length ? `${skip + 1}–${skip + pageTxs.length}` : '';

    const rows = await mapPool(pageTxs, 4, async tx => {
      if (isStale(gen)) return '';
      let methods = '—';
      let transferHint = '';
      if (tx.script?.length) {
        try {
          const log = await getApplicationLogCached(tx.hash);
          if (isStale(gen)) return '';
          const invs = summarizeInvocations(log.executions);
          if (invs.length) {
            const parts = await Promise.all(invs.map(async i => {
              const name = await getContractName(i.contract);
              return `${name}.${i.method}()`;
            }));
            methods = parts.join(', ');
          }
          const xfers = summarizeTransfers(log.executions);
          if (xfers.length === 1) {
            transferHint = formatTokenAmount(xfers[0].amount, xfers[0].asset);
          } else if (xfers.length > 1) {
            transferHint = `${xfers.length} transfers`;
          }
        } catch (_) {
          methods = 'Invocation';
        }
      }
      return `
        <tr>
          <td>${hashLink(tx.hash, 'tx')}</td>
          <td><a href="#/block/${tx.blockindex}">#${tx.blockindex}</a></td>
          <td>${formatTime(tx.blocktime)}</td>
          <td><span class="badge badge-type">${txType(tx)}</span></td>
          <td class="mono tx-methods">${esc(methods)}</td>
          <td>${transferHint ? esc(transferHint) : '—'}</td>
          <td class="mono">${esc(tx.sender || '—')}</td>
        </tr>
      `;
    });
    if (isStale(gen)) return;

    const pager = pagerHtml({
      page,
      range,
      hasOlder,
      hasNewer: page > 0,
      olderHref: listHref('transactions', page + 1),
      newerHref: listHref('transactions', page - 1),
    });
    app.innerHTML = `
      <div class="panel fade-in">
        <div class="panel-header">
          <h2>Transactions</h2>
          ${pager}
        </div>
        <table>
          <thead>
            <tr><th>Hash</th><th>Block</th><th>Time</th><th>Type</th><th>Methods</th><th>Transfer</th><th>Sender</th></tr>
          </thead>
          <tbody>${rows.join('') || '<tr><td colspan="7" class="empty">No transactions</td></tr>'}</tbody>
        </table>
      </div>
    `;
    bindPager(app.querySelector('.panel-header'));
  } catch (e) {
    if (!isStale(gen)) {
      app.innerHTML = `<div class="error-box">${esc(e.message)}</div>`;
    }
  }
}

// ── Transaction detail ─────────────────────────────────────

function renderStackItem(item, depth = 0) {
  if (!item) return 'null';
  const pad = '  '.repeat(depth);
  if (item.type === 'Array' && item.value) {
    return item.value.map(v => pad + renderStackItem(v, depth + 1)).join('\n');
  }
  if (item.type === 'Struct' && item.value) {
    return item.value.map(v => pad + renderStackItem(v, depth + 1)).join('\n');
  }
  if (item.type === 'Map' && item.value) {
    return JSON.stringify(item.value, null, 2);
  }
  return `${item.type}: ${item.value ?? ''}`;
}

async function parseTransferFromNotif(n) {
  if (n.eventname !== 'Transfer' || !n.state?.value) return null;
  const vals = n.state.value;
  if (vals.length < 3) return null;
  const from = await decodeStackHash160(vals[0]);
  const to = await decodeStackHash160(vals[1]);
  const amount = decodeStackInteger(vals[2]);
  return { from, to, amount, asset: n.contract };
}

async function renderInvocationRows(executions) {
  const invs = summarizeInvocations(executions);
  if (!invs.length) return '';
  const rows = await Promise.all(invs.map(async inv => {
    const contractHash = inv.contract;
    const name = await getContractName(contractHash);
    const isTransfer = inv.method === 'transfer';
    let argsHtml = '—';
    if (!inv.truncated) {
      const full = findFullInvocation(executions, inv.raw || inv);
      const args = invocationArgsArray(full);
      if (args) {
        const manifest = await getContractManifest(contractHash);
        const abiMethod = findAbiMethod(manifest, inv.method, args.length);
        if (abiMethod?.parameters?.length) {
          const parts = await Promise.all(args.map((arg, idx) =>
            formatTypedStackItem(arg, abiMethod.parameters[idx], contractHash)));
          argsHtml = parts.join(', ');
        } else {
          argsHtml = await formatInvocationArgs(inv.method, args, contractHash);
        }
      }
    } else {
      argsHtml = `${inv.argc} args (truncated)`;
    }
    return `<tr class="${isTransfer ? 'row-transfer' : ''}">
      <td>${contractCell(contractHash, name)}</td>
      <td><span class="method-name">${esc(inv.method)}</span></td>
      <td class="mono">${esc(argsHtml)}</td>
    </tr>`;
  }));
  return `
    <div class="section">
      <h3>Methods Called</h3>
      <table>
        <thead><tr><th>Contract</th><th>Method</th><th>Arguments</th></tr></thead>
        <tbody>${rows.join('')}</tbody>
      </table>
    </div>`;
}

async function renderTx(hash) {
  const app = document.getElementById('app');
  app.innerHTML = '<div class="loading">Loading transaction…</div>';
  try {
    const tx = await rpc('getrawtransaction', [hash, true]);
    let appLog = null;
    try { appLog = await rpc('getapplicationlog', [hash]); } catch (_) {}
    await loadContractCache();

    const signers = (tx.signers || []).map(s => `
      <li>
        <span class="mono">${esc(s.account)}</span>
        <span class="dim">scopes: ${esc(formatScopes(s.scopes ?? s.scope))}</span>
        ${s.allowedcontracts?.length ? `<br><span class="dim">allowed: ${s.allowedcontracts.join(', ')}</span>` : ''}
      </li>
    `).join('');

    const witnesses = (tx.witnesses || []).map((w, i) => `
      <div class="witness-card">
        <h4>Witness #${i}</h4>
        <dl class="detail-grid compact">
          <dt>Invocation</dt><dd><pre class="code">${esc(b64ToHex(w.invocation))}</pre></dd>
          <dt>Verification</dt><dd><pre class="code">${esc(b64ToHex(w.verification))}</pre></dd>
        </dl>
      </div>
    `).join('');

    const attributes = (tx.attributes || []).map(a =>
      `<li><span class="badge badge-type">${esc(a.type)}</span> ${esc(JSON.stringify(a))}</li>`
    ).join('');

    let transfers = [];
    let execHtml = '';
    if (appLog?.executions) {
      for (const ex of appLog.executions) {
        for (const n of ex.notifications || []) {
          const t = await parseTransferFromNotif(n);
          if (t) transfers.push(t);
        }
      }
      execHtml = (await Promise.all(appLog.executions.map(async (ex, i) => {
        const state = ex.vmstate || 'UNKNOWN';
        const badge = state === 'HALT' ? 'badge-halt' : 'badge-fault';
        const notifs = (await Promise.all((ex.notifications || []).map(async n => {
          const t = await parseTransferFromNotif(n);
          let detail = '';
          if (t) {
            detail = `<div class="transfer-detail">
              <span class="mono">${esc(t.from)}</span> → <span class="mono">${esc(t.to)}</span>
              <strong>${esc(formatTokenAmount(t.amount, t.asset))}</strong>
            </div>`;
          }
          return `<li class="notif-item">
            <span class="event-name">${esc(n.eventname)}</span>
            <span class="dim">@ ${esc(n.contract)}</span>
            ${detail}
            <pre class="code">${esc(JSON.stringify(n.state, null, 2))}</pre>
          </li>`;
        }))).join('');
        const stack = (ex.stack || []).map((s, j) =>
          `<pre class="code stack-item">[${j}] ${esc(renderStackItem(s))}</pre>`
        ).join('');
        return `
          <div class="section exec-section">
            <h3>Execution #${i + 1} <span class="badge ${badge}">${esc(state)}</span></h3>
            <dl class="detail-grid compact">
              <dt>Trigger</dt><dd>${esc(ex.trigger || '—')}</dd>
              <dt>Gas consumed</dt><dd>${esc(ex.gasconsumed ?? '—')}</dd>
              <dt>Exception</dt><dd>${esc(ex.exception || ex.faultexception || '—')}</dd>
            </dl>
            ${notifs ? `<h4 class="sub-heading">Notifications</h4><ul class="notif-list">${notifs}</ul>` : ''}
            ${stack ? `<h4 class="sub-heading">Stack</h4>${stack}` : ''}
          </div>
        `;
      }))).join('');
    }

    const methodsHtml = await renderInvocationRows(appLog?.executions);

    const transferRows = await Promise.all(transfers.map(async t => {
      const assetName = await getContractName(t.asset);
      return `<tr>
        <td>${hashLink(t.asset, 'contract')} <span class="dim">${esc(assetName)}</span></td>
        <td class="mono">${esc(t.from)}</td>
        <td class="mono">${esc(t.to)}</td>
        <td><strong>${esc(formatTokenAmount(t.amount, t.asset))}</strong></td>
      </tr>`;
    }));

    const transferTable = transferRows.length ? `
      <div class="section highlight-section">
        <h3>NEP-17 Transfers</h3>
        <table>
          <thead><tr><th>Token</th><th>From</th><th>To</th><th>Amount</th></tr></thead>
          <tbody>${transferRows.join('')}</tbody>
        </table>
      </div>` : '';

    const scriptHex = b64ToHex(tx.script);

    app.innerHTML = `
      <div class="panel fade-in">
        <div class="panel-header">
          <a class="back-link" href="#/transactions">← Transactions</a>
          <h2>Transaction</h2>
        </div>
        <dl class="detail-grid">
          <dt>Hash</dt><dd>${esc(tx.hash)}</dd>
          <dt>Type</dt><dd><span class="badge badge-type">${txType(tx)}</span></dd>
          <dt>Size</dt><dd>${tx.size || '—'} bytes</dd>
          <dt>Version</dt><dd>${tx.version ?? '—'}</dd>
          <dt>Nonce</dt><dd>${tx.nonce ?? '—'}</dd>
          <dt>Block</dt><dd>${tx.blockhash ? hashLink(tx.blockhash, 'block') + ` <span class="dim">(#${tx.blockindex ?? '—'})</span>` : '<span class="badge badge-type">Unconfirmed</span>'}</dd>
          <dt>Time</dt><dd>${formatTime(tx.blocktime)}</dd>
          <dt>Sender</dt><dd class="mono">${esc(tx.sender || '—')}</dd>
          <dt>System Fee</dt><dd>${formatGAS(tx.sysfee)}</dd>
          <dt>Network Fee</dt><dd>${formatGAS(tx.netfee)}</dd>
          <dt>Valid Until</dt><dd>block ${tx.validuntilblock}</dd>
          <dt>Confirmations</dt><dd>${tx.confirmations ?? '—'}</dd>
          <dt>VM State</dt><dd>${tx.vmstate ? `<span class="badge ${tx.vmstate === 'HALT' ? 'badge-halt' : 'badge-fault'}">${esc(tx.vmstate)}</span>` : '—'}</dd>
        </dl>
        ${transferTable}
        ${methodsHtml}
        ${signers ? `<div class="section"><h3>Signers (${tx.signers.length})</h3><ul class="notif-list">${signers}</ul></div>` : ''}
        ${attributes ? `<div class="section"><h3>Attributes</h3><ul class="notif-list">${attributes}</ul></div>` : ''}
        ${witnesses ? `<div class="section"><h3>Witnesses</h3>${witnesses}</div>` : ''}
        <div class="section"><h3>Script</h3><pre class="code">${esc(scriptHex)}</pre></div>
        ${execHtml || '<div class="section"><p class="empty">No execution log (miner tx or unconfirmed)</p></div>'}
      </div>
    `;
  } catch (e) {
    app.innerHTML = `<div class="error-box">${esc(e.message)}</div><a href="#/transactions">← Back</a>`;
  }
}

// ── Contracts ──────────────────────────────────────────────

async function getDeployedContracts() {
  const top = latestIndex();
  if (deployedContractsCache.height === top) return deployedContractsCache.list;
  const list = await findDeployedContracts();
  deployedContractsCache = { height: top, list };
  return list;
}

async function renderContracts() {
  const app = document.getElementById('app');
  app.innerHTML = '<div class="loading">Loading contracts…</div>';
  try {
    await refreshStats(true);
    const page = parseListPage(routeParts());
    const natives = await rpc('getnativecontracts');
    const deployed = await getDeployedContracts();
    for (const c of natives || []) {
      if (c.hash) contractNameCache.set(contractKey(c.hash), c.manifest?.name || 'Native');
    }
    for (const c of deployed) {
      if (c.hash) contractNameCache.set(contractKey(c.hash), c.name);
    }
    const nativeRows = (natives || []).map(c => `
      <tr>
        <td><span class="badge badge-native">Native</span></td>
        <td>${esc(c.manifest?.name || 'Unknown')}</td>
        <td>${hashLink(c.hash, 'contract')}</td>
        <td>${c.id ?? '—'}</td>
      </tr>
    `).join('');
    const { slice, page: safePage, pageCount, range, hasOlder, hasNewer } = paginateSlice(deployed, page, CONTRACTS_PER_PAGE);
    const deployedRows = slice.map(c => `
      <tr>
        <td><span class="badge badge-type">Deployed</span></td>
        <td>${esc(c.name)}</td>
        <td>${hashLink(c.hash, 'contract')}</td>
        <td>${c.id ?? '—'}</td>
      </tr>
    `).join('');
    const deployedPager = deployed.length ? pagerHtml({
      page: safePage,
      pageCount,
      range,
      hasOlder,
      hasNewer,
      olderHref: listHref('contracts', safePage + 1),
      newerHref: listHref('contracts', safePage - 1),
    }) : '';

    app.innerHTML = `
      <div class="panel fade-in">
        <div class="panel-header"><h2>Native Contracts</h2></div>
        <table>
          <thead><tr><th>Kind</th><th>Name</th><th>Hash</th><th>ID</th></tr></thead>
          <tbody>${nativeRows || '<tr><td colspan="4" class="empty">No native contracts</td></tr>'}</tbody>
        </table>
      </div>
      <div class="panel fade-in">
        <div class="panel-header">
          <h2>Deployed Contracts (${deployed.length})</h2>
          ${deployedPager}
        </div>
        <table>
          <thead><tr><th>Kind</th><th>Name</th><th>Hash</th><th>ID</th></tr></thead>
          <tbody>${deployedRows || '<tr><td colspan="4" class="empty">No deployed contracts</td></tr>'}</tbody>
        </table>
      </div>
    `;
    bindPager(app.querySelectorAll('.panel')[1]?.querySelector('.panel-header'));
  } catch (e) {
    app.innerHTML = `<div class="error-box">${esc(e.message)}</div>`;
  }
}

async function findDeployedContracts() {
  const found = new Map();
  const natives = await rpc('getnativecontracts');
  const nativeHashes = new Set((natives || []).map(c => contractKey(c.hash)));
  const top = latestIndex();
  for (let i = 0; i <= top; i++) {
    const hash = await rpc('getblockhash', [i]);
    const block = await rpc('getblock', [hash, 1]);
    for (const tx of block.tx || []) {
      if (!tx.script?.length) continue;
      try {
        const log = await rpc('getapplicationlog', [tx.hash]);
        for (const ex of log.executions || []) {
          for (const n of ex.notifications || []) {
            if (n.eventname === 'Deploy' && n.contract) {
              const h = contractKey(n.contract);
              if (!nativeHashes.has(h) && !found.has(h)) {
                let name = 'Contract';
                try {
                  const cs = await rpc('getcontractstate', [n.contract]);
                  name = cs.manifest?.name || name;
                  found.set(h, { hash: n.contract, name, id: cs.id });
                } catch {
                  found.set(h, { hash: n.contract, name });
                }
              }
            }
          }
        }
      } catch (_) {}
    }
  }
  return [...found.values()];
}

async function renderContract(hash) {
  const app = document.getElementById('app');
  app.innerHTML = '<div class="loading">Loading contract…</div>';
  try {
    const cs = await rpc('getcontractstate', [hash]);
    const manifest = cs.manifest || {};
    const methods = (manifest.abi?.methods || []).map(m =>
      `<tr><td class="mono">${esc(m.name)}</td><td>${esc((m.parameters || []).map(p => p.name + ':' + p.type).join(', '))}</td><td>${esc(m.returntype || 'Void')}</td><td>${m.safe ? 'yes' : 'no'}</td></tr>`
    ).join('');

    app.innerHTML = `
      <div class="panel fade-in">
        <div class="panel-header">
          <a class="back-link" href="#/contracts">← Contracts</a>
          <h2>${esc(manifest.name || 'Contract')}</h2>
        </div>
        <dl class="detail-grid">
          <dt>Hash</dt><dd>${esc(cs.hash || hash)}</dd>
          <dt>ID</dt><dd>${cs.id ?? '—'}</dd>
          <dt>Update counter</dt><dd>${cs.updatecounter ?? '—'}</dd>
          <dt>NEF checksum</dt><dd>${cs.nef?.checksum ?? '—'}</dd>
        </dl>
        ${methods ? `<div class="section"><h3>ABI Methods</h3><table>
          <thead><tr><th>Name</th><th>Parameters</th><th>Returns</th><th>Safe</th></tr></thead>
          <tbody>${methods}</tbody></table></div>` : ''}
        <div class="section"><h3>Manifest</h3><pre class="code">${esc(JSON.stringify(manifest, null, 2))}</pre></div>
      </div>
    `;
  } catch (e) {
    app.innerHTML = `<div class="error-box">${esc(e.message)}</div><a href="#/contracts">← Back</a>`;
  }
}

// ── Mempool ────────────────────────────────────────────────

async function renderMempool(showLoading = true) {
  const gen = beginRender();
  const app = document.getElementById('app');
  if (showLoading) app.innerHTML = '<div class="loading">Loading mempool…</div>';
  try {
    const page = parseListPage(routeParts());
    const hashes = await rpc('getrawmempool');
    if (isStale(gen)) return;
    if (!hashes?.length) {
      app.innerHTML = '<div class="panel fade-in"><p class="empty">Mempool is empty</p></div>';
      return;
    }
    const { slice, page: safePage, pageCount, range, hasOlder, hasNewer } = paginateSlice(hashes, page, MEMPOOL_PER_PAGE);
    const rows = [];
    for (const h of slice) {
      try {
        const tx = await rpc('getrawtransaction', [h, true]);
        rows.push(`<tr>
          <td>${hashLink(h, 'tx')}</td>
          <td><span class="badge badge-type">${txType(tx)}</span></td>
          <td class="mono">${esc(tx.sender || '—')}</td>
          <td>${formatGAS(tx.sysfee)}</td>
          <td>${formatGAS(tx.netfee)}</td>
          <td>${tx.validuntilblock}</td>
        </tr>`);
      } catch {
        rows.push(`<tr><td class="mono">${shortHash(h)}</td><td colspan="5">—</td></tr>`);
      }
    }
    const pager = pagerHtml({
      page: safePage,
      pageCount,
      range,
      hasOlder,
      hasNewer,
      olderHref: listHref('mempool', safePage + 1),
      newerHref: listHref('mempool', safePage - 1),
    });
    app.innerHTML = `
      <div class="panel fade-in">
        <div class="panel-header">
          <h2>Mempool (${hashes.length})</h2>
          ${pager}
        </div>
        <table>
          <thead><tr><th>Hash</th><th>Type</th><th>Sender</th><th>Sys Fee</th><th>Net Fee</th><th>VUB</th></tr></thead>
          <tbody>${rows.join('')}</tbody>
        </table>
      </div>
    `;
    bindPager(app.querySelector('.panel-header'));
  } catch (e) {
    if (!isStale(gen)) {
      app.innerHTML = `<div class="error-box">${esc(e.message)}</div>`;
    }
  }
}

// ── Routing ────────────────────────────────────────────────

function setActiveTab(view) {
  document.querySelectorAll('.tab').forEach(t => {
    t.classList.toggle('active', t.dataset.view === view);
  });
}

function route() {
  const parts = routeParts();
  const view = parts[0] || 'blocks';
  currentView = view;

  if (view === 'blocks' || view === '') {
    setActiveTab('blocks');
    renderBlocks();
  } else if (view === 'transactions') {
    setActiveTab('transactions');
    renderTransactions();
  } else if (view === 'block' && parts[1]) {
    setActiveTab('blocks');
    const txPage = parts[2] === 'p' ? parseListPage(parts, 2) : 0;
    renderBlock(parts[1], txPage);
  } else if (view === 'tx' && parts[1]) {
    setActiveTab('transactions');
    renderTx(parts[1]);
  } else if (view === 'contracts') {
    setActiveTab('contracts');
    renderContracts();
  } else if (view === 'contract' && parts[1]) {
    setActiveTab('contracts');
    renderContract(parts[1]);
  } else if (view === 'mempool') {
    setActiveTab('mempool');
    renderMempool();
  } else {
    location.hash = '#/blocks';
  }
}

document.querySelectorAll('.tab').forEach(btn => {
  btn.addEventListener('click', () => { location.hash = '#/' + btn.dataset.view; });
});

document.getElementById('search-form')?.addEventListener('submit', handleSearch);

window.addEventListener('hashchange', route);
route();
connectWS();
loadNetworkInfo();
loadContractCache();
setInterval(() => refreshStats(), 5000);
