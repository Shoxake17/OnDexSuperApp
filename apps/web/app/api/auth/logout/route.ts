import { NextResponse } from "next/server";
import { goFetch } from "@/lib/api";
import { clearSessionToken, getSessionToken } from "@/lib/session";

// POST /api/auth/logout — chiqish.
//
// Cookie'ni o'chirish YETARLI EMAS: cookie shunchaki brauzerdagi nusxa,
// tokenning o'zi esa Go backend uchun 30 kun amal qilaveradi. Token
// boshqa yo'l bilan qo'lga kiritilgan bo'lsa (masalan qurilma
// yo'qolgan), chiqish uni to'xtatmasdi. Endi avval SERVERDA bekor
// qilamiz (`POST /auth/logout`), keyin cookie'ni tozalaymiz.
export async function POST() {
  const token = await getSessionToken();

  if (token) {
    try {
      await goFetch("/auth/logout", { method: "POST" }, token);
    } catch {
      // Backend javob bermasa ham chiqish davom etadi — cookie
      // baribir o'chiriladi. Bu qo'shimcha himoya, chiqishning sharti
      // emas.
    }
  }

  await clearSessionToken();
  return NextResponse.json({ ok: true });
}
