import { NextRequest, NextResponse } from "next/server";
import { goFetch } from "@/lib/api";

// POST /api/auth/telegram-start {"phone":"+998901234567"} — Go'ning
// `POST /auth/telegram/start`ini shaffof proksi qiladi va bot havolasini
// (`deep_link`) qaytaradi.
//
// ┌─ OQIM (mobil ilova bilan AYNAN bir xil) ───────────────────────────┐
//   1. foydalanuvchi raqamini kiritadi -> shu endpoint;
//   2. qaytgan havola bilan bot ochiladi, u "Raqamni ulashish"ni
//      so'raydi;
//   3. bot Telegram YUBORGAN raqamni kiritilgan raqam bilan
//      SOLISHTIRADI (`internal/telegram/verifier.go`) — mos kelmasa kod
//      UMUMAN yuborilmaydi. Aynan shu tekshiruv oqimni SMS bilan teng
//      kuchga keltiradi: raqamni foydalanuvchi qo'lda "aytib" qo'ya
//      olmaydi, uni Telegramning o'zi tasdiqlaydi;
//   4. mos bo'lsa bot 6 xonali kodni Telegram chatiga yuboradi;
//   5. kod `POST /api/auth/verify` orqali tekshiriladi va sessiya
//      cookie'si o'rnatiladi (akkaunt yo'q bo'lsa — o'sha yerda
//      yaratiladi, ya'ni ro'yxatdan o'tish alohida qadam emas).
// └───────────────────────────────────────────────────────────────────┘
//
// Kod BU YERDA yaratilmaydi va qaytarilmaydi — u faqat Telegram
// chatiga boradi. Tezlik cheklovi ham Go tomonda (`telegramIPLimiter`).
export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => null);
  if (!body?.phone) {
    return NextResponse.json({ error: "telefon raqami kerak" }, { status: 400 });
  }
  const res = await goFetch("/auth/telegram/start", {
    method: "POST",
    body: JSON.stringify({ phone: body.phone }),
  });
  const data = await res.json().catch(() => ({}));
  return NextResponse.json(data, { status: res.status });
}
