// OnDex — buyurtma idempotentligi YUK OSTIDA.
//
// ┌─ NIMA SINALADI ─────────────────────────────────────────────────────┐
// Mijoz ilovasi buyurtma yaratishda bir martalik `idempotency_key`
// yuboradi (customer_app/lib/api.dart — `newIdempotencyKey()`). Tarmoq
// uzilib javob kelmasa, foydalanuvchi qayta bosadi va ilova XUDDI SHU
// kalitni qayta yuboradi. Server yangi buyurtma yaratmasligi, eskisini
// qaytarishi kerak.
//
// Bu qoida bitta so'rovda oson ishlaydi. HAQIQIY xavf — PARALLEL
// so'rovlar: ikkala so'rov ham "bunday kalit bormi?" deb tekshiradi,
// ikkalasi ham "yo'q" javobini oladi va IKKITA buyurtma yaratiladi.
// Mijoz ikki marta to'laydi.
//
// Kodda ikki qatlamli himoya bor:
//   1. `orders.Service.Create()` — tekshiruv;
//   2. migratsiya 0016 — `customer_id`+`idempotency_key` bo'yicha
//      UNIQUE indeks (check-then-insert orasidagi tabiiy poygaga
//      qarshi oxirgi to'siq).
// Bu test aynan o'sha poygani ataylab hosil qiladi.
// └─────────────────────────────────────────────────────────────────────┘
//
// ISHGA TUSHIRISH:
//   $env:ONDEX_TOKEN = "<mijoz JWT>"
//   k6 run tests/load/idempotency.js
//
// Token olish (dev rejimda):
//   POST /auth/request-code {"phone":"+998..."}  -> javobdagi dev_code
//   POST /auth/verify       {"phone","code"}     -> token

import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.ONDEX_API || 'http://localhost:8080';
const TOKEN = __ENV.ONDEX_TOKEN;
const RESTAURANT = __ENV.ONDEX_RESTAURANT || 'f3e2b323cf5c8752';
const PRODUCT = __ENV.ONDEX_PRODUCT || 'c9ee60df41cc5a0a';

// ┌─ KALIT `setup()` DA YARATILADI — MODUL DARAJASIDA EMAS ────────────┐
// k6 da HAR BIR VU modulni O'Z JS muhitida qaytadan bajaradi. Ya'ni
// `const KEY = Date.now()` modul darajasida yozilsa, har bir VU O'Z
// kalitini oladi va test umuman poyga hosil qilmaydi.
//
// Bu xato birinchi urinishda "5 ta dublikat yaratildi" degan YOLG'ON
// natija berdi — aslida 30 ta so'rov 5 ta TURLI kalit bilan ketgan va
// har biri to'g'ri ishlab, bittadan buyurtma yaratgan edi.
//
// `setup()` FAQAT BIR MARTA ishlaydi va qaytargan qiymati barcha VU
// larga uzatiladi — kalit haqiqatan umumiy bo'lishining yagona yo'li.
// └────────────────────────────────────────────────────────────────────┘

export const options = {
    scenarios: {
        // Bir zumda 30 ta parallel so'rov. `constant-vus` emas,
        // `per-vu-iterations`: har bir VU ANIQ bitta marta uradi,
        // shunda kutilgan natija aniq — 1 ta buyurtma.
        burst: {
            executor: 'per-vu-iterations',
            vus: 30,
            iterations: 1,
            maxDuration: '30s',
        },
    },
    // Xato bo'lsa test qizil bo'lsin.
    thresholds: {
        checks: ['rate==1.0'],
    },
};

function auth() {
    return {
        headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${TOKEN}`,
        },
    };
}

// setup — testdan OLDINGI buyurtmalar sonini yozib qo'yamiz.
export function setup() {
    if (!TOKEN) {
        throw new Error('ONDEX_TOKEN berilmagan — yuqoridagi izohga qarang');
    }
    // Buyurtma berish uchun profilda SAQLANGAN manzil bo'lishi shart
    // (aks holda server "avval yetkazib berish manzilini tanlang"
    // deb 400 qaytaradi). Koordinata Chust hududida bo'lishi kerak —
    // `delivery.CheckPoint` uni tekshiradi.
    const a = http.post(`${BASE}/me/address`, JSON.stringify({
        lat: 41.003, lng: 71.236,
        street: 'Navoiy ko\'chasi', house: '12', note: 'k6 test',
    }), auth());
    if (a.status !== 200) {
        throw new Error(`/me/address ${a.status}: ${a.body}`);
    }

    const r = http.get(`${BASE}/me/orders`, auth());
    if (r.status !== 200) {
        throw new Error(`/me/orders ${r.status} qaytardi — token yaroqlimi?`);
    }
    const before = JSON.parse(r.body || '[]').length;
    const key = `k6-idem-${Date.now()}`;
    console.log(`testdan oldin buyurtmalar: ${before}, umumiy kalit: ${key}`);
    return { before, key };
}

export default function (data) {
    const body = JSON.stringify({
        restaurant_id: RESTAURANT,
        items: [{ product_id: PRODUCT, qty: 1 }],
        delivery_lat: 41.003,
        delivery_lng: 71.236,
        idempotency_key: data.key,
    });

    const r = http.post(`${BASE}/orders`, body, auth());

    // 200/201 — yaratildi yoki mavjudi qaytarildi. Ikkalasi ham to'g'ri.
    // 409 ham maqbul: unique indeks poygani ushlab qolgan va server
    // buni ANIQ xato bilan bildirgan (jimgina ikkinchi buyurtma
    // yaratishdan ko'ra shu yaxshi).
    check(r, {
        'javob kutilgan kodda (200/201/409)': (res) =>
            res.status === 200 || res.status === 201 || res.status === 409,
        'server xatosi YO\'Q (5xx)': (res) => res.status < 500,
    });
}

// teardown — ASOSIY tekshiruv: nechta buyurtma qo'shildi.
export function teardown(data) {
    const r = http.get(`${BASE}/me/orders`, auth());
    const after = JSON.parse(r.body || '[]').length;
    const created = after - data.before;

    console.log('');
    console.log('==============================================');
    console.log(`  parallel so'rov      : 30`);
    console.log(`  yaratilgan buyurtma  : ${created}`);
    console.log('==============================================');

    if (created === 1) {
        console.log('  NATIJA: idempotentlik ISHLAYAPTI');
    } else if (created === 0) {
        console.log('  NATIJA: birorta buyurtma yaratilmadi — so\'rov tanasi');
        console.log('          yoki token noto\'g\'ri bo\'lishi mumkin');
    } else {
        console.log(`  NATIJA: XATO — ${created} ta dublikat buyurtma yaratildi.`);
        console.log('          Mijoz bir necha marta to\'lashi mumkin edi.');
    }
    console.log('');
}
