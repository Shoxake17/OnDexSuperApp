import { NextRequest, NextResponse } from "next/server";
import { goFetch } from "@/lib/api";
import { getSessionToken } from "@/lib/session";

// POST /api/sso/authorize — tasdiqlash sahifasidagi "Ruxsat berish" tugmasi.
//
// Go'dan bir martalik kod bilan to'ldirilgan qaytish manzilini oladi va JSON
// qilib qaytaradi: `{ "redirect_to": "https://..." }`. O'tishni brauzerdagi
// sahifa kodi bajaradi (`ConsentButton`).
//
// ┌─ NEGA 303 EMAS ──────────────────────────────────────────────────────┐
// Avval bu yerda `NextResponse.redirect(target, 303)` edi. Lekin manzil
// ATAYLAB boshqa domenda (POS paneli), sahifaning CSP'si esa
// `form-action 'self'`: Chrome bu qoidani forma yuborilgach KELGAN
// yo'naltirishga ham qo'llaydi va o'tishni to'sadi ("Sending form data ...
// violates form-action"). `form-action` ni kengaytirish yomon yechim:
// qaytish domenlari ro'yxati dinamik, qoidani ochish esa har qanday
// formani begona saytga yuborishga yo'l ochardi. JS orqali navigatsiyaga
// `form-action` tegishli emas.
// └──────────────────────────────────────────────────────────────────────┘
//
// `Location` mijoz kiritgan qiymat EMAS: Go uni ro'yxatdan o'tgan
// manzillarga ANIQ solishtirgach o'zi quradi. Bu yerda faqat Go qaytargan
// manzil ishlatiladi (u http(s) bo'lishi qo'shimcha tekshiriladi).
//
// CSRF: sessiya cookie'si `SameSite=Lax` — begona saytdan yuborilgan
// POST'ga cookie qo'shilmaydi, ya'ni so'rov kirmagan hisoblanadi. Manba
// tekshiruvi — qo'shimcha qatlam (cookie sozlamasi o'zgarib qolsa ham
// himoya yo'qolmasin).

function originFrom(req: NextRequest): string {
  const host = req.headers.get("x-forwarded-host") ?? req.headers.get("host") ?? req.nextUrl.host;
  const proto = req.headers.get("x-forwarded-proto") ?? req.nextUrl.protocol.replace(":", "");
  return `${proto}://${host}`;
}

const noStore = { "Cache-Control": "no-store" };

export async function POST(req: NextRequest) {
  const origin = originFrom(req);

  // ┌─ MANBA TEKSHIRUVI ────────────────────────────────────────────────┐
  // 1) `Sec-Fetch-Site` — brauzer qo'yadi, sahifa kodi (JS) o'zgartira
  //    OLMAYDI. Faqat `same-origin` (yoki `none`) o'tadi. Bu `Origin` ga
  //    qaraganda ishonchli: `Origin` boshqa domendan qaytib kelgan
  //    zanjirdan keyin "null" bo'lib, yoki proksi ortida `Host` boshqacha
  //    bo'lib, to'g'ri so'rovni ham rad etardi.
  // 2) Brauzer `Sec-Fetch-Site` yubormasa (eski) — `Origin` solishtiriladi.
  // └───────────────────────────────────────────────────────────────────┘
  const sent = req.headers.get("origin");
  const site = req.headers.get("sec-fetch-site");
  const allowed = site
    ? site === "same-origin" || site === "none"
    : !sent || sent === origin;
  if (!allowed) {
    const body: Record<string, string> = { error: "so'rov manbasi noto'g'ri" };
    if (process.env.NODE_ENV !== "production") {
      // Faqat dev: qaysi qiymat mos kelmaganini ko'rsatadi.
      body.detail = `origin=${sent} kutilgan=${origin} sec-fetch-site=${site}`;
    }
    return NextResponse.json(body, { status: 403, headers: noStore });
  }

  const input = await req.json().catch(() => null);
  const str = (v: unknown) => (typeof v === "string" ? v : "");
  const clientId = str(input?.client_id);
  const redirectUri = str(input?.redirect_uri);
  const state = str(input?.state);

  // Sessiya yo'q: sahifa qayta yuklanadi va `/login` ga o'tkazadi.
  const token = await getSessionToken();
  if (!token) {
    return NextResponse.json({ error: "login" }, { status: 401, headers: noStore });
  }

  let location = "";
  try {
    const res = await goFetch(
      "/sso/authorize",
      {
        method: "POST",
        body: JSON.stringify({ client_id: clientId, redirect_uri: redirectUri, state }),
      },
      token,
    );
    const data = await res.json().catch(() => ({}));
    if (!res.ok || typeof data?.redirect_to !== "string") {
      return NextResponse.json({ error: "failed" }, { status: 400, headers: noStore });
    }
    location = data.redirect_to;
  } catch {
    return NextResponse.json({ error: "failed" }, { status: 502, headers: noStore });
  }

  // Go qaytargan manzil http(s) bo'lishi shart (`javascript:` kabilarga
  // qarshi qo'shimcha tekshiruv — Go ro'yxati bunga yo'l qo'ymaydi).
  let target: URL;
  try {
    target = new URL(location);
  } catch {
    return NextResponse.json({ error: "failed" }, { status: 400, headers: noStore });
  }
  if (target.protocol !== "https:" && target.protocol !== "http:") {
    return NextResponse.json({ error: "failed" }, { status: 400, headers: noStore });
  }

  return NextResponse.json({ redirect_to: target.toString() }, { headers: noStore });
}
