// OnDex — SIG'IM (capacity) yuklama testi: bosqichma-bosqich oshib boradigan
// so'rovlar tezligi (open model), aralash trafik.
//
// ┌─ NEGA `ramping-arrival-rate` (OCHIQ MODEL) ───────────────────────────┐
// "N ta foydalanuvchi" yopiq modelda (VU) server sekinlashsa so'rovlar ham
// kamayadi va muammo YASHIRINADI (coordinated omission). Haqiqiy dunyoda
// foydalanuvchilar server sekinlashganini kutib o'tirmaydi — so'rovlar
// o'z tezligida kelaveradi. Shuning uchun bu yerda TEZLIK (so'rov/s) beriladi
// va server ulgurmasa `dropped_iterations` hamda kechikish o'sishi ko'rinadi.
//
// 100 000 "faol foydalanuvchi" ≠ 100 000 bir vaqtdagi ulanish: odatda ular
// bir necha minutda bitta so'rov yuboradi (~1–5 000 so'rov/s).
// └───────────────────────────────────────────────────────────────────────┘
//
// ⚠️ FAQAT LOKAL YOKI STAGING. Production'ga YUBORMANG: VPS bir nechta
// loyihaga umumiy (`scripts/load.ps1` begona manzilni rad etadi).
//
// Ishga tushirish: `scripts\load.ps1` (resurs o'lchash va hisobot bilan) yoki
//   k6 run -e STEPS=200,500,1000 -e STAGE_S=30 tests/load/capacity.js

import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.ONDEX_API || 'http://localhost:8080';
const STEPS = (__ENV.STEPS || '200,500,1000,2000,3000').split(',').map(Number);
const STAGE_S = Number(__ENV.STAGE_S || 30); // har bosqichning barqaror qismi
const RAMP_S = 10;                            // bosqichga ko'tarilish (o'lchanmaydi)
const MAX_VUS = Number(__ENV.MAX_VUS || 6000);

const stages = [];
for (const r of STEPS) {
  stages.push({ target: r, duration: `${RAMP_S}s` });
  stages.push({ target: r, duration: `${STAGE_S}s` });
}

// Har bosqich uchun alohida (teglangan) metrikalar hisobotda ko'rinishi uchun
// ularga bo'sh-shartli threshold beriladi (k6 faqat shunda sub-metrika chiqaradi).
const thresholds = {
  // Umumiy qabul chegaralari: bosqich muvaffaqiyatsiz bo'lsa test baribir
  // tugaydi (abort faqat falokatda — pastda).
  http_req_failed: [{ threshold: 'rate<0.5', abortOnFail: true, delayAbortEval: '20s' }],
};
const EPS = ['menu', 'restaurants', 'restaurant', 'categories', 'quote', 'me_orders', 'unread'];
for (const e of EPS) {
  thresholds[`http_req_duration{ep:${e}}`] = ['max>=0'];
  thresholds[`http_reqs{ep:${e}}`] = ['count>=0'];
}
for (const r of STEPS) {
  thresholds[`http_req_duration{step:r${r}}`] = ['max>=0'];
  thresholds[`http_req_failed{step:r${r}}`] = ['rate>=0'];
  thresholds[`http_reqs{step:r${r}}`] = ['count>=0'];
}

export const options = {
  scenarios: {
    load: {
      executor: 'ramping-arrival-rate',
      startRate: 20,
      timeUnit: '1s',
      preAllocatedVUs: Math.min(500, MAX_VUS),
      maxVUs: MAX_VUS,
      stages,
    },
  },
  thresholds,
  summaryTrendStats: ['avg', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
  discardResponseBodies: false,
};

function json(res) {
  try { return res.json(); } catch (_) { return null; }
}

export function setup() {
  // 1) Restoran va taom (katalogdan).
  const rs = http.get(`${BASE}/restaurants`);
  const list = json(rs);
  if (rs.status !== 200 || !Array.isArray(list) || list.length === 0) {
    throw new Error(`GET /restaurants ishlamadi (${rs.status}) — server ishlayaptimi?`);
  }
  let restaurant = null;
  let product = null;
  for (const r of list) {
    const m = json(http.get(`${BASE}/restaurants/${r.id}/menu`));
    const first = Array.isArray(m) ? m.find((p) => p.price_tiyin > 0 && p.available !== false) : null;
    if (first) { restaurant = r.id; product = first.id; break; }
  }
  if (!restaurant) throw new Error('menyusi bor restoran topilmadi');

  // 2) Token: berilgan bo'lsa shuni, bo'lmasa dev OTP bilan (bir marta).
  let token = __ENV.ONDEX_TOKEN || '';
  // PUBLIC_ONLY=1: tokensiz rejim (production: OTP orqali kirish) - faqat ochiq endpointlar.
  if (!token && !__ENV.PUBLIC_ONLY) {
    const phone = __ENV.ONDEX_PHONE || '+998900009991';
    const rc = http.post(`${BASE}/auth/request-code`, JSON.stringify({ phone }),
      { headers: { 'Content-Type': 'application/json' } });
    const code = (json(rc) || {}).dev_code;
    if (!code) throw new Error(`dev_code yo'q (${rc.status}) — APP_ENV=development emasmi? ONDEX_TOKEN bering`);
    const v = http.post(`${BASE}/auth/verify`, JSON.stringify({ phone, code }),
      { headers: { 'Content-Type': 'application/json' } });
    token = (json(v) || {}).token;
    if (!token) throw new Error(`token olinmadi (${v.status})`);
  }
  return { restaurant, product, token, startMs: Date.now() };
}

// Savat tarkibi har so'rovda biroz farq qiladi (kesh "yolg'on yaxshi" natija bermasin).
function qty() { return 1 + Math.floor(Math.random() * 3); }

export default function (d) {
  const elapsed = (Date.now() - d.startMs) / 1000;
  const idx = Math.min(Math.floor(elapsed / (RAMP_S + STAGE_S)), STEPS.length - 1);
  const within = elapsed - idx * (RAMP_S + STAGE_S);
  const tags = { step: within >= RAMP_S ? `r${STEPS[idx]}` : 'ramp' };
  const auth = { headers: { Authorization: `Bearer ${d.token}`, 'Content-Type': 'application/json' }, tags };
  // Har so'rov o'z `ep` tegi bilan: qaysi endpoint sekinlashayotgani ko'rinadi.
  const t = (n) => ({ tags: Object.assign({}, tags, { ep: n }) });
  const a = (n) => ({ headers: auth.headers, tags: Object.assign({}, tags, { ep: n }) });

  // Aralash trafik (tipik mijoz ilovasi): ko'p ko'rish, kam hisoblash.
  const x = Math.random();
  let res;
  let name;
  if (x < 0.30) {
    name = 'menu';
    res = http.get(`${BASE}/restaurants/${d.restaurant}/menu`, t(name));
  } else if (x < 0.50) {
    name = 'restaurants';
    res = http.get(`${BASE}/restaurants`, t(name));
  } else if (x < 0.65) {
    name = 'restaurant';
    res = http.get(`${BASE}/restaurants/${d.restaurant}`, t(name));
  } else if (x < 0.75) {
    name = 'categories';
    res = http.get(`${BASE}/categories`, t(name));
  } else if (!d.token) {
    name = 'menu';
    res = http.get(`${BASE}/restaurants/${d.restaurant}/menu`, t(name));
  } else if (x < 0.90) {
    name = 'quote';
    res = http.post(`${BASE}/restaurants/${d.restaurant}/quote`,
      JSON.stringify({ items: [{ product_id: d.product, qty: qty() }] }), a(name));
  } else if (x < 0.96) {
    name = 'me_orders';
    res = http.get(`${BASE}/me/orders`, a(name));
  } else {
    name = 'unread';
    res = http.get(`${BASE}/notifications/unread-count`, a(name));
  }
  check(res, { [`${name} 2xx`]: (r) => r.status >= 200 && r.status < 300 }, tags);
}

// Hisobot: har bosqich bo'yicha jadval + JSON.
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
      target_rps: r,
      achieved_rps: Number((cnt.values.count / STAGE_S).toFixed(1)),
      p50: Number(dur.values.med.toFixed(1)),
      p95: Number(dur.values['p(95)'].toFixed(1)),
      p99: Number(dur.values['p(99)'].toFixed(1)),
      max: Number(dur.values.max.toFixed(1)),
      error_pct: Number((((fail && fail.values.rate) || 0) * 100).toFixed(2)),
    });
  }
  const endpoints = [];
  for (const e of EPS) {
    const dur = m[`http_req_duration{ep:${e}}`];
    const cnt = m[`http_reqs{ep:${e}}`];
    if (!dur || !cnt) continue;
    endpoints.push({
      endpoint: e, requests: cnt.values.count,
      p50: Number(dur.values.med.toFixed(1)), p95: Number(dur.values['p(95)'].toFixed(1)),
      p99: Number(dur.values['p(99)'].toFixed(1)), max: Number(dur.values.max.toFixed(1)),
    });
  }
  const dropped = (m.dropped_iterations && m.dropped_iterations.values.count) || 0;
  const out = {
    base: BASE, steps: STEPS, stage_seconds: STAGE_S, max_vus: MAX_VUS,
    rows, endpoints, dropped_iterations: dropped,
    vus_max: (m.vus_max && m.vus_max.values.max) || 0,
    total_requests: (m.http_reqs && m.http_reqs.values.count) || 0,
    checks_failed_pct: m.checks ? Number(((1 - m.checks.values.rate) * 100).toFixed(2)) : 0,
  };
  let txt = `\nSIG'IM TESTI — ${BASE}\n`;
  txt += 'maqsad rps | erishilgan | p50 ms | p95 ms | p99 ms | max ms | xato %\n';
  for (const r of rows) {
    txt += `${String(r.target_rps).padStart(10)} | ${String(r.achieved_rps).padStart(10)} | ${String(r.p50).padStart(7)} | ${String(r.p95).padStart(6)} | ${String(r.p99).padStart(6)} | ${String(r.max).padStart(6)} | ${r.error_pct}\n`;
  }
  txt += "\nendpoint      | so'rovlar | p50 ms | p95 ms | p99 ms | max ms\n";
  for (const e of endpoints) {
    txt += `${e.endpoint.padEnd(13)} | ${String(e.requests).padStart(9)} | ${String(e.p50).padStart(6)} | ${String(e.p95).padStart(6)} | ${String(e.p99).padStart(6)} | ${e.max}\n`;
  }
  txt += `tashlab yuborilgan iteratsiyalar (server ulgurmadi): ${dropped}\n`;
  return {
    stdout: txt,
    [__ENV.SUMMARY_OUT || 'logs/load/k6-summary.json']: JSON.stringify(out, null, 2),
  };
}
