import { NextResponse } from "next/server";
import { goFetch } from "@/lib/api";
import { clearSessionToken, getRefreshToken, getSessionToken } from "@/lib/session";

// POST /api/auth/logout — chiqish.
//
// Cookie'ni o'chirish YETARLI EMAS: avval SERVERDA bekor qilinadi, keyin
// cookie'lar tozalanadi.
//   * markaziy sessiya (refresh cookie bor, 137) — `POST /v2/auth/logout`:
//     faqat SHU qurilma sessiyasi yopiladi (boshqa qurilmalar qoladi);
//   * eski token — `POST /auth/logout` (akkauntning hamma eski tokenlari).
export async function POST() {
  const token = await getSessionToken();
  const central = Boolean(await getRefreshToken());

  if (token) {
    try {
      await goFetch(central ? "/v2/auth/logout" : "/auth/logout", { method: "POST" }, token);
    } catch {
      // Backend javob bermasa ham chiqish davom etadi — cookie baribir
      // o'chiriladi. Bu qo'shimcha himoya, chiqishning sharti emas.
    }
  }

  await clearSessionToken();
  return NextResponse.json({ ok: true });
}
