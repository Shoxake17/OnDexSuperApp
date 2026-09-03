import { NextRequest, NextResponse } from "next/server";
import { goFetch } from "@/lib/api";
import { safeRedirect } from "@/lib/safe-redirect";
import { setAppShell, setSessionToken } from "@/lib/session";

// POST /api/bridge  (tana: token=...&redirect=/...)
//
// Flutter native login'dan keyin WebView shu manzilga BIR MARTA POST
// qiladi: Go tokenini httpOnly cookie'ga o'tkazadi, shundan keyin WebView
// ham veb-sayt/Telegram bilan bir xil cookie-based sessiyadan foydalanadi
// (qayta ishlatilishi mumkin — kelajakdagi boshqa mini-app'lar ham shu
// bridge'dan foydalanadi). Tokenni ko'r-ko'rona ishonib cookie'ga
// yozmaymiz — Go backend'dan haqiqatan yaroqli ekanini GET /me orqali
// tasdiqlaymiz.
//
// XAVFSIZLIK (tuzatilgan zaiflik): avval token URL so'rov qatorida
// (`?token=eyJ...`) GET bilan yuborilardi. Bu — maxfiy ma'lumot uchun
// eng yomon joy:
//   * reverse-proxy va Next.js kirish loglariga TO'LIQ yoziladi;
//   * WebView/brauzer tarixida qoladi;
//   * sahifadan tashqi resurs so'ralganda `Referer`da chiqib ketishi mumkin.
// Endi token faqat POST TANASIDA qabul qilinadi. GET varianti ataylab
// tokenni UMUMAN o'qimaydi (pastga qarang) — eski manzil "jimgina
// ishlab ketmasin", aks holda zaiflik amalda saqlanib qolardi.

function originFrom(req: NextRequest): string {
  // MUHIM (haqiqiy Android qurilmada topilgan bug): `req.url`/`req.nextUrl`
  // ba'zi holatlarda (masalan Next.js dev server'da LAN IP orqali
  // kirilganda) so'rov qaysi host'ga yuborilgani emas, serverning o'zi
  // haqidagi ichki taxminni qaytaradi — natijada redirect har doim
  // "localhost"ga ketib, WebView'da host mos kelmasligi xatosiga olib
  // keldi. Manzil doim so'rovning HAQIQIY `Host` header'idan quriladi.
  const host = req.headers.get("host") ?? req.nextUrl.host;
  const proto = req.headers.get("x-forwarded-proto") ?? req.nextUrl.protocol.replace(":", "");
  return `${proto}://${host}`;
}

export async function POST(req: NextRequest) {
  const origin = originFrom(req);

  let form: FormData;
  try {
    form = await req.formData();
  } catch {
    return NextResponse.redirect(new URL("/?auth_error=1", origin), 303);
  }

  const token = String(form.get("token") ?? "");
  const redirectTo = safeRedirect(String(form.get("redirect") ?? "/"));

  if (!token) {
    return NextResponse.redirect(new URL("/", origin), 303);
  }

  const check = await goFetch("/me", {}, token);
  if (!check.ok) {
    return NextResponse.redirect(new URL("/?auth_error=1", origin), 303);
  }

  await setSessionToken(token);
  // Bu manzilga FAQAT mijoz ilovasining WebView'i murojaat qiladi —
  // ya'ni sahifa ilova ichida ochilgani ANIQ ma'lum. Belgi qo'yiladi
  // va sahifa o'zining pastki menyusini chizmaydi (ilovada Flutter'ning
  // O'Z menyusi bor; `lib/session.ts` dagi izoh).
  await setAppShell();
  // 303 — POST'dan keyin brauzer/WebView redirect'ni GET sifatida
  // kuzatadi (POST tanasi qayta yuborilmaydi).
  return NextResponse.redirect(new URL(redirectTo, origin), 303);
}

// GET — ATAYLAB tokensiz. Eski `?token=...` manzili bilan kelingan
// bo'lsa, token E'TIBORGA OLINMAYDI va foydalanuvchi bosh sahifaga
// yuboriladi (mavjud cookie bo'lsa, u yerda sessiya baribir ishlaydi).
export async function GET(req: NextRequest) {
  return NextResponse.redirect(new URL("/", originFrom(req)));
}
