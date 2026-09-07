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

// isSameSiteRequest — so'rov BEGONA saytdan kelmaganini tekshiradi.
//
// ┌─ TUZATILGAN NOSOZLIK (bug.md 51-band) ─────────────────────────────┐
// Bu endpoint tokenning HAQIQIYligini tekshirardi, lekin uni KIM
// yuborganini tekshirmasdi. `multipart/form-data` bilan POST — brauzer
// uchun "oddiy" so'rov (preflight yo'q), ya'ni ISTALGAN sayt avtomatik
// forma yuborishi mumkin edi:
//
//	<form action="https://<domen>/api/bridge" method="POST"
//	      enctype="multipart/form-data">
//	  <input name="token" value="<HUJUMCHINING TOKENI>">
//	</form><script>document.forms[0].submit()</script>
//
// Qurbonning brauzeri hujumchining tokenini httpOnly sessiya
// cookie'si sifatida SAQLARDI. `SameSite=lax` bu yerda yordam
// bermaydi: u cookie YUBORISHNI boshqaradi, O'RNATISHNI emas.
//
// Oqibati — login CSRF / sessiya fiksatsiyasi: qurbon o'zining
// akkauntida ishlayapman deb o'ylab, aslida HUJUMCHINING akkauntida
// ishlaydi. Uning yetkazish manzili, telefon raqami va buyurtmalari
// hujumchiga ko'rinadi.
//
// Qabul qilinadigan holatlar:
//   * `Sec-Fetch-Site: same-origin` / `same-site` — o'z saytimiz;
//   * `Sec-Fetch-Site: none` — foydalanuvchi/ilova BOSHLAGAN yuqori
//     darajali navigatsiya (WebView `loadRequest` bilan POST qilgan
//     holat aynan shu);
//   * ikkala sarlavha ham yo'q — eski native WebView'lar
//     `Sec-Fetch-*` yubormaydi va `Origin` ham qo'ymaydi. Begona
//     saytdan kelgan forma esa `Origin` ni HAR DOIM yuboradi, ya'ni
//     bu shox hujum yo'lini ochmaydi.
//
// Rad etiladi: `Sec-Fetch-Site: cross-site` yoki `Origin` bor va u
// so'rov host'iga mos kelmasa.
// └────────────────────────────────────────────────────────────────────┘
function isSameSiteRequest(req: NextRequest, origin: string): boolean {
  const site = req.headers.get("sec-fetch-site");
  if (site === "cross-site") return false;

  const reqOrigin = req.headers.get("origin");
  if (reqOrigin && reqOrigin !== origin) return false;

  return true;
}

export async function POST(req: NextRequest) {
  const origin = originFrom(req);

  if (!isSameSiteRequest(req, origin)) {
    // Sessiya O'RNATILMAYDI. Redirect ham begona saytga qaytmaydi —
    // faqat o'z bosh sahifamizga.
    return NextResponse.redirect(new URL("/?auth_error=1", origin), 303);
  }

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
