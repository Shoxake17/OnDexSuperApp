// OnDex — PRODUCTION yuklama testidan OLDIN bitta to'liq oqim smoke'i:
// test restorani -> menyu -> quote -> buyurtma (naqd) -> buyurtmalarim.
//
// Faqat `cmd/loadtest` yaratgan test restorani va mijozlari bilan ishlaydi
// (RESTAURANT_ID `loadtest-` bilan boshlanmasa — darhol to'xtaydi).
//
//   k6 run -e ONDEX_API=http://localhost:8080 -e RESTAURANT_ID=loadtest-<run>-rest \
//          -e TOKENS_FILE=/data/tokens-<run>.json tests/load/prod_smoke.js
import http from 'k6/http';
import { check, fail } from 'k6';

const BASE = __ENV.ONDEX_API || 'http://localhost:8080';
const RID = __ENV.RESTAURANT_ID || '';
const tokens = JSON.parse(open(__ENV.TOKENS_FILE));

export const options = { vus: 1, iterations: 1 };

function json(r) { try { return r.json(); } catch (_) { return null; } }

export default function () {
  if (!RID.startsWith('loadtest-')) fail('RESTAURANT_ID loadtest- bilan boshlanmaydi — haqiqiy restoranga yozish RAD');
  const headers = { Authorization: `Bearer ${tokens[0]}`, 'Content-Type': 'application/json' };

  const r = http.get(`${BASE}/restaurants/${RID}`);
  check(r, { 'restoran 200': (x) => x.status === 200 }) || console.log(`restoran: ${r.status} ${r.body}`);
  const menu = json(http.get(`${BASE}/restaurants/${RID}/menu`));
  const p = Array.isArray(menu) ? menu.find((x) => x.price_tiyin > 0) : null;
  if (!p) fail('menyu bo\'sh');
  const items = [{ product_id: p.id, qty: 2 }];

  const q = http.post(`${BASE}/restaurants/${RID}/quote`, JSON.stringify({ items }), { headers });
  const qb = json(q);
  if (!check(q, { 'quote 200': (x) => x.status === 200 })) { console.log(`quote: ${q.status} ${q.body}`); return; }

  const o = http.post(`${BASE}/orders`, JSON.stringify({
    items, payment_method: 'cash', idempotency_key: `smoke-${Date.now()}`, expected_total_tiyin: qb.total_tiyin,
  }), { headers });
  if (!check(o, { 'buyurtma 201': (x) => x.status === 201 })) { console.log(`order: ${o.status} ${o.body}`); return; }
  const ob = json(o) || {};
  console.log(`BUYURTMA OK: id=${ob.id} restoran=${ob.restaurant_id} jami=${ob.total_tiyin}`);

  const mine = http.get(`${BASE}/me/orders`, { headers });
  check(mine, { 'buyurtmalarim 200': (x) => x.status === 200 });
  const u = http.get(`${BASE}/notifications/unread-count`, { headers });
  check(u, { 'unread 200': (x) => x.status === 200 });
}
