// OnDex — PRODUCTION real yuklama testi (egasi ruxsati 2026-11-12: real biznes/mijoz hali yo'q).
//
// Model: ~10 000 bir vaqtdagi foydalanuvchi har 5-10 soniyada bitta so'rov
// yuboradi => ~1 000-2 000 so'rov/s. Bosqichlar shundan yuqoriga (chegarani
// topish uchun) ochiq modelda (`ramping-arrival-rate`, coordinated omission yo'q).
//
// Aralash (mijoz ilovasi): ko'rish 60%, savat 15%, buyurtma 5%,
// buyurtmalarim 10%, o'qilmagan bildirishnomalar 10%.
// YOZISH faqat `cmd/loadtest` yaratgan test restoraniga va test mijozlariga
// (RESTAURANT_ID `loadtest-` bilan boshlanmasa test to'xtaydi).
//
//   k6 run -e ONDEX_API=http://localhost:8080 -e RESTAURANT_ID=loadtest-<run>-rest \
//          -e TOKENS_FILE=/data/tokens-<run>.json -e STEPS=250,500,1000,1500,2000,3000 \
//          -e STAGE_S=60 -e SUMMARY_OUT=/data/k6-<run>.json tests/load/prod_load.js
import http from 'k6/http';
import { check, fail } from 'k6';
import { SharedArray } from 'k6/data';

const BASE = __ENV.ONDEX_API || 'http://localhost:8080';
const RID = __ENV.RESTAURANT_ID || '';
const STEPS = (__ENV.STEPS || '250,500,1000,1500,2000,3000').split(',').map(Number);
const STAGE_S = Number(__ENV.STAGE_S || 60);
const RAMP_S = 10;
const MAX_VUS = Number(__ENV.MAX_VUS || 4000);
const tokens = new SharedArray('tokens', () => JSON.parse(open(__ENV.TOKENS_FILE)));

const stages = [];
for (const r of STEPS) {
  stages.push({ target: r, duration: `${RAMP_S}s` });
  stages.push({ target: r, duration: `${STAGE_S}s` });
}
const EPS = ['restaurants', 'restaurant', 'menu', 'categories', 'quote', 'order_create', 'me_orders', 'unread'];
const thresholds = {
  // Falokatda (xatolar 30% dan oshsa) test o'zi to'xtaydi — prod'ni bekorga bosmaslik uchun.
  http_req_failed: [{ threshold: 'rate<0.3', abortOnFail: true, delayAbortEval: '20s' }],
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
    mix: {
      executor: 'ramping-arrival-rate', startRate: 50, timeUnit: '1s',
      preAllocatedVUs: Math.min(800, MAX_VUS), maxVUs: MAX_VUS, stages,
    },
  },
  thresholds,
  summaryTrendStats: ['avg', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

function json(r) { try { return r.json(); } catch (_) { return null; } }

export function setup() {
  if (!RID.startsWith('loadtest-')) fail('RESTAURANT_ID loadtest- bilan boshlanmaydi — haqiqiy restoranga yozish RAD');
  const menu = json(http.get(`${BASE}/restaurants/${RID}/menu`));
  const products = Array.isArray(menu) ? menu.filter((p) => p.price_tiyin > 0 && p.available !== false).map((p) => p.id) : [];
  if (products.length === 0) fail('test restorani menyusi bo\'sh');
  return { products, startMs: Date.now() };
}

export default function (d) {
  const elapsed = (Date.now() - d.startMs) / 1000;
  const idx = Math.min(Math.floor(elapsed / (RAMP_S + STAGE_S)), STEPS.length - 1);
  const within = elapsed - idx * (RAMP_S + STAGE_S);
  const step = within >= RAMP_S ? `r${STEPS[idx]}` : 'ramp';
  const token = tokens[(__VU * 7919 + __ITER) % tokens.length];
  const headers = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
  const t = (ep) => ({ tags: { step, ep } });
  const a = (ep) => ({ headers, tags: { step, ep } });
  const items = () => [{ product_id: d.products[Math.floor(Math.random() * d.products.length)], qty: 1 + Math.floor(Math.random() * 3) }];

  const x = Math.random();
  let res; let ep; let ok = (r) => r.status >= 200 && r.status < 300;
  if (x < 0.20) { ep = 'restaurants'; res = http.get(`${BASE}/restaurants`, t(ep)); }
  else if (x < 0.30) { ep = 'restaurant'; res = http.get(`${BASE}/restaurants/${RID}`, t(ep)); }
  else if (x < 0.55) { ep = 'menu'; res = http.get(`${BASE}/restaurants/${RID}/menu`, t(ep)); }
  else if (x < 0.60) { ep = 'categories'; res = http.get(`${BASE}/categories`, t(ep)); }
  else if (x < 0.75) {
    ep = 'quote';
    res = http.post(`${BASE}/restaurants/${RID}/quote`, JSON.stringify({ items: items() }), a(ep));
  } else if (x < 0.80) {
    // To'liq checkout: quote -> buyurtma (naqd, noyob idempotency kaliti).
    const its = items();
    const q = http.post(`${BASE}/restaurants/${RID}/quote`, JSON.stringify({ items: its }), a('quote'));
    const qb = json(q);
    if (q.status !== 200 || !qb) { check(q, { 'quote 2xx': ok }, { step }); return; }
    ep = 'order_create';
    res = http.post(`${BASE}/orders`, JSON.stringify({
      items: its, payment_method: 'cash',
      idempotency_key: `k6-${__VU}-${__ITER}-${Date.now()}`, expected_total_tiyin: qb.total_tiyin,
    }), a(ep));
  } else if (x < 0.90) { ep = 'me_orders'; res = http.get(`${BASE}/me/orders`, a(ep)); }
  else { ep = 'unread'; res = http.get(`${BASE}/notifications/unread-count`, a(ep)); }
  check(res, { [`${ep} 2xx`]: ok }, { step });
}

export function handleSummary(data) {
  const m = data.metrics;
  const rows = [];
  for (const r of STEPS) {
    const k = `step:r${r}`;
    const dur = m[`http_req_duration{${k}}`]; const cnt = m[`http_reqs{${k}}`]; const f = m[`http_req_failed{${k}}`];
    if (!dur || !cnt) continue;
    rows.push({
      target_rps: r, achieved_rps: Number((cnt.values.count / STAGE_S).toFixed(1)),
      p50: Number(dur.values.med.toFixed(1)), p95: Number(dur.values['p(95)'].toFixed(1)),
      p99: Number(dur.values['p(99)'].toFixed(1)), max: Number(dur.values.max.toFixed(1)),
      error_pct: Number((((f && f.values.rate) || 0) * 100).toFixed(2)),
    });
  }
  const endpoints = [];
  for (const e of EPS) {
    const dur = m[`http_req_duration{ep:${e}}`]; const cnt = m[`http_reqs{ep:${e}}`]; const f = m[`http_req_failed{ep:${e}}`];
    if (!dur || !cnt) continue;
    endpoints.push({
      endpoint: e, requests: cnt.values.count, p50: Number(dur.values.med.toFixed(1)),
      p95: Number(dur.values['p(95)'].toFixed(1)), p99: Number(dur.values['p(99)'].toFixed(1)),
      max: Number(dur.values.max.toFixed(1)), error_pct: Number((((f && f.values.rate) || 0) * 100).toFixed(2)),
    });
  }
  const dropped = (m.dropped_iterations && m.dropped_iterations.values.count) || 0;
  const out = { base: BASE, steps: STEPS, stage_seconds: STAGE_S, rows, endpoints, dropped_iterations: dropped,
    vus_max: (m.vus_max && m.vus_max.values.max) || 0, total_requests: (m.http_reqs && m.http_reqs.values.count) || 0 };
  let txt = `\nPROD YUKLAMA TESTI — ${BASE}\nmaqsad rps | erishilgan | p50 | p95 | p99 | max | xato %\n`;
  for (const r of rows) txt += `${r.target_rps} | ${r.achieved_rps} | ${r.p50} | ${r.p95} | ${r.p99} | ${r.max} | ${r.error_pct}\n`;
  txt += '\nendpoint | so\'rovlar | p50 | p95 | p99 | max | xato %\n';
  for (const e of endpoints) txt += `${e.endpoint} | ${e.requests} | ${e.p50} | ${e.p95} | ${e.p99} | ${e.max} | ${e.error_pct}\n`;
  txt += `tashlab yuborilgan (server ulgurmadi): ${dropped}\n`;
  return { stdout: txt, [__ENV.SUMMARY_OUT || '/data/k6-summary.json']: JSON.stringify(out, null, 2) };
}
