// OnDex — YOZISH YO'LI yuklama testi: checkout (quote + buyurtma yaratish).
//
// Buyurtma yaratish o'qishdan ancha qimmat (Postgres'ga yozish, bildirishnomalar,
// aksiya hisobi) va sig'im testiga (`capacity.js`) kirmagan edi. Bu skript
// har iteratsiyada haqiqiy checkout'ni takrorlaydi: avval `quote`, keyin
// `POST /orders` (naqd, noyob idempotency_key).
//
// Mijozlar `cmd/benchseed` bilan oldindan yaratiladi (har biri alohida token) —
// bitta foydalanuvchi minglab buyurtma bersa, real hayotda bo'lmaydigan
// foydalanuvchi-darajasidagi cheklovlar o'lchovni buzardi.
//
// ⚠️ FAQAT izolyatsiyalangan benchmark muhitida (`scripts\bench.ps1 -Script orders -Seed 2000`):
// u minglab buyurtma yozadi. Dev yoki production bazasiga YUBORMANG.

import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';

const BASE = __ENV.ONDEX_API || 'http://localhost:18081';
const STEPS = (__ENV.STEPS || '25,50,100,200,400').split(',').map(Number); // buyurtma/s
const STAGE_S = Number(__ENV.STAGE_S || 30);
const RAMP_S = 10;
const MAX_VUS = Number(__ENV.MAX_VUS || 2000);

const tokens = new SharedArray('tokens', () => JSON.parse(open(__ENV.TOKENS_FILE)));

const stages = [];
for (const r of STEPS) {
  stages.push({ target: r, duration: `${RAMP_S}s` });
  stages.push({ target: r, duration: `${STAGE_S}s` });
}
const EPS = ['quote', 'order_create'];
const thresholds = {
  http_req_failed: [{ threshold: 'rate<0.5', abortOnFail: true, delayAbortEval: '20s' }],
};
for (const e of EPS) {
  thresholds[`http_req_duration{ep:${e}}`] = ['max>=0'];
  thresholds[`http_reqs{ep:${e}}`] = ['count>=0'];
  thresholds[`http_req_failed{ep:${e}}`] = ['rate>=0'];
}
for (const r of STEPS) {
  thresholds[`http_req_duration{step:r${r}}`] = ['max>=0'];
  thresholds[`http_req_failed{step:r${r}}`] = ['rate>=0'];
  thresholds[`http_reqs{step:r${r}}`] = ['count>=0'];
}

export const options = {
  scenarios: {
    checkout: {
      executor: 'ramping-arrival-rate',
      startRate: 5, timeUnit: '1s',
      preAllocatedVUs: Math.min(200, MAX_VUS), maxVUs: MAX_VUS, stages,
    },
  },
  thresholds,
  summaryTrendStats: ['avg', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

function json(res) { try { return res.json(); } catch (_) { return null; } }

export function setup() {
  const list = json(http.get(`${BASE}/restaurants`));
  if (!Array.isArray(list) || list.length === 0) throw new Error('restoranlar yo\'q');
  let restaurant = null;
  let product = null;
  for (const r of list) {
    if (r.open_now === false) continue;
    const m = json(http.get(`${BASE}/restaurants/${r.id}/menu`));
    const p = Array.isArray(m) ? m.find((x) => x.price_tiyin > 0 && x.available !== false) : null;
    if (p) { restaurant = r.id; product = p.id; break; }
  }
  if (!restaurant) throw new Error('ochiq restoran va taom topilmadi');
  return { restaurant, product, startMs: Date.now() };
}

export default function (d) {
  const elapsed = (Date.now() - d.startMs) / 1000;
  const idx = Math.min(Math.floor(elapsed / (RAMP_S + STAGE_S)), STEPS.length - 1);
  const within = elapsed - idx * (RAMP_S + STAGE_S);
  const step = within >= RAMP_S ? `r${STEPS[idx]}` : 'ramp';

  // Har VU/iteratsiya alohida mijoz: foydalanuvchi bo'yicha cheklov o'lchovni buzmasin.
  const token = tokens[(__VU * 7919 + __ITER) % tokens.length];
  const headers = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
  const items = [{ product_id: d.product, qty: 1 + Math.floor(Math.random() * 3) }];

  const q = http.post(`${BASE}/restaurants/${d.restaurant}/quote`, JSON.stringify({ items }),
    { headers, tags: { step, ep: 'quote' } });
  const qb = json(q);
  const okQ = check(q, { 'quote 200': (r) => r.status === 200 });
  if (!okQ || !qb) return;

  const res = http.post(`${BASE}/orders`, JSON.stringify({
    items, payment_method: 'cash',
    idempotency_key: `k6-${__VU}-${__ITER}-${Date.now()}`,
    expected_total_tiyin: qb.total_tiyin,
  }), { headers, tags: { step, ep: 'order_create' } });
  check(res, { 'order 201': (r) => r.status === 201 }, { step });
}

export function handleSummary(data) {
  const m = data.metrics;
  const rows = [];
  for (const r of STEPS) {
    const k = `step:r${r}`;
    const dur = m[`http_req_duration{${k}}`];
    const cnt = m[`http_reqs{${k}}`];
    const fail = m[`http_req_failed{${k}}`];
    if (!dur || !cnt) continue;
    rows.push({
      target_rps: r, // bu yerda: checkout/s (har biri 2 so'rov)
      achieved_rps: Number((cnt.values.count / 2 / STAGE_S).toFixed(1)),
      p50: Number(dur.values.med.toFixed(1)), p95: Number(dur.values['p(95)'].toFixed(1)),
      p99: Number(dur.values['p(99)'].toFixed(1)), max: Number(dur.values.max.toFixed(1)),
      error_pct: Number((((fail && fail.values.rate) || 0) * 100).toFixed(2)),
    });
  }
  const endpoints = [];
  for (const e of EPS) {
    const dur = m[`http_req_duration{ep:${e}}`];
    const cnt = m[`http_reqs{ep:${e}}`];
    const fail = m[`http_req_failed{ep:${e}}`];
    if (!dur || !cnt) continue;
    endpoints.push({
      endpoint: e, requests: cnt.values.count,
      p50: Number(dur.values.med.toFixed(1)), p95: Number(dur.values['p(95)'].toFixed(1)),
      p99: Number(dur.values['p(99)'].toFixed(1)), max: Number(dur.values.max.toFixed(1)),
      error_pct: Number((((fail && fail.values.rate) || 0) * 100).toFixed(2)),
    });
  }
  const dropped = (m.dropped_iterations && m.dropped_iterations.values.count) || 0;
  const out = { base: BASE, steps: STEPS, stage_seconds: STAGE_S, rows, endpoints, dropped_iterations: dropped,
    total_requests: (m.http_reqs && m.http_reqs.values.count) || 0 };
  let txt = `\nYOZISH YO'LI (checkout) — ${BASE}\n`;
  txt += 'maqsad checkout/s | erishilgan | p50 ms | p95 ms | p99 ms | max ms | xato %\n';
  for (const r of rows) {
    txt += `${String(r.target_rps).padStart(17)} | ${String(r.achieved_rps).padStart(10)} | ${String(r.p50).padStart(7)} | ${String(r.p95).padStart(6)} | ${String(r.p99).padStart(6)} | ${String(r.max).padStart(6)} | ${r.error_pct}\n`;
  }
  txt += "\nendpoint      | so'rovlar | p50 ms | p95 ms | p99 ms | max ms | xato %\n";
  for (const e of endpoints) {
    txt += `${e.endpoint.padEnd(13)} | ${String(e.requests).padStart(9)} | ${String(e.p50).padStart(6)} | ${String(e.p95).padStart(6)} | ${String(e.p99).padStart(6)} | ${String(e.max).padStart(6)} | ${e.error_pct}\n`;
  }
  txt += `tashlab yuborilgan iteratsiyalar (server ulgurmadi): ${dropped}\n`;
  return { stdout: txt, [__ENV.SUMMARY_OUT || 'logs/load/orders-summary.json']: JSON.stringify(out, null, 2) };
}
