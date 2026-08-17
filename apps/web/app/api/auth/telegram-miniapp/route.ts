import { NextRequest, NextResponse } from "next/server";
import { goFetch } from "@/lib/api";
import { setClientKind, setSessionToken } from "@/lib/session";

// POST /api/auth/telegram-miniapp  {"initData": "..."}
//
// ┌─ TELEGRAM MINI APP KIRISHI ───────────────────────────────────────┐
// Telegram Mini App ochilganda sahifaga `initData` beradi. Bu marshrut
// uni Go backend'ga uzatadi, Go esa imzoni bot tokeni bilan
// HMAC-SHA256 orqali tekshiradi (`internal/telegram/initdata.go`).
//
// ── NEGA TEKSHIRUV SERVERDA ──
// `initData` KLIENT tomonida turadi va uni istalgan odam tahrirlashi
// mumkin. Brauzerda tekshirish ma'nosiz: bot tokeni brauzerga
// tushishi kerak bo'lardi, u esa butun botni egallash demak.
//
// ── NEGA TOKEN BRAUZERGA QAYTARILMAYDI ──
// Go bergan JWT javobda EMAS, httpOnly cookie'ga yoziladi (`verify`
// marshruti bilan bir xil naqsh). Klient JS unga umuman kira olmaydi,
// ya'ni XSS orqali sessiyani o'g'irlab bo'lmaydi.
// └───────────────────────────────────────────────────────────────────┘

// Haqiqiy `initData` ~500-1500 belgi. Chegara — xotira sarfiga qarshi
// oddiy himoya; Go tomonda ham xuddi shunday tekshiruv bor.
const MAX_INIT_DATA = 4096;

export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => null);
  const initData = typeof body?.initData === "string" ? body.initData : "";

  if (!initData) {
    return NextResponse.json({ error: "initData kerak" }, { status: 400 });
  }
  if (initData.length > MAX_INIT_DATA) {
    return NextResponse.json({ error: "initData juda uzun" }, { status: 400 });
  }

  const res = await goFetch("/auth/telegram/miniapp", {
    method: "POST",
    body: JSON.stringify({ init_data: initData }),
  });
  const data = await res.json().catch(() => ({}));

  // 409 — "kirish rad etildi" EMAS: imzo TO'G'RI, lekin foydalanuvchi
  // hali botda kontaktini ulashmagan. Klient buni ko'rib uni botga
  // yo'naltiradi. Telefon raqami `initData` da HECH QACHON bo'lmaydi,
  // shuning uchun bu qadamni chetlab o'tib bo'lmaydi.
  if (res.status === 409) {
    return NextResponse.json(
      {
        need: "share_contact",
        botLink: typeof data?.bot_link === "string" ? data.bot_link : "",
        firstName: typeof data?.first_name === "string" ? data.first_name : "",
      },
      { status: 409 },
    );
  }

  if (!res.ok || typeof data?.token !== "string") {
    // Sabab OSHKOR QILINMAYDI: Go tomonda ham "imzo noto'g'ri" va
    // "muddati o'tgan" farqlanmaydi — farqning o'zi hujumchiga imzo
    // tanlashda ma'lumot berardi.
    return NextResponse.json(
      { error: "Telegram ma'lumoti tasdiqlanmadi" },
      { status: res.status === 401 ? 401 : 400 },
    );
  }

  await setSessionToken(data.token);
  // Bu yo'lga FAQAT Telegram Mini App keladi (initData imzosi Go
  // tomonda tekshirilgan) — sessiya aynan shunday belgilanadi.
  await setClientKind("tma");
  return NextResponse.json({ ok: true, user: data.user });
}
