import { NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";
import { goFetch } from "@/lib/api";
import { safeRedirect } from "@/lib/safe-redirect";
import { setClientKind, setSessionToken } from "@/lib/session";

// GET /api/auth/telegram-return?c=... — botning "OnDex'ga qaytish"
// tugmasi ANIQ SHU manzilga ishora qiladi (`WEB_PUBLIC_BASE_URL` +
// `internal/telegram/verifier.go`dagi `webReturnPath`). Foydalanuvchi
// buni HECH QACHON o'zi ochmaydi — faqat Telegram ichidagi tugma orqali.
//
// ┌─ NEGA `token` SO'ROV QATORIDA YO'Q ─────────────────────────────────┐
// Bot xabaridagi havolada FAQAT `c` (maxfiy kalit) bor — `token`
// `/api/auth/telegram-login/start` o'rnatgan `chust_tg_login`
// cookie'sidan olinadi. Ikkalasi ham SHU BRAUZERDA (Telegram
// ochilgan qurilmada) turgani — aynan fishingga qarshi himoyaning
// o'zi (`Pending.ConfirmSecret` izohiga qarang, `internal/telegram`).
// Token URL'da bo'lsa, uni ko'rgan har KIM (masalan bot suhbatini
// ulashib qo'yilsa) kirishni yakunlashga urinishi mumkin edi.
// └───────────────────────────────────────────────────────────────────┘
const COOKIE_NAME = "chust_tg_login";

function originFrom(req: NextRequest): string {
  const host = req.headers.get("host") ?? req.nextUrl.host;
  const proto = req.headers.get("x-forwarded-proto") ?? req.nextUrl.protocol.replace(":", "");
  return `${proto}://${host}`;
}

export async function GET(req: NextRequest) {
  const origin = originFrom(req);
  const secret = req.nextUrl.searchParams.get("c") ?? "";
  const store = await cookies();
  const raw = store.get(COOKIE_NAME)?.value;
  store.delete(COOKIE_NAME);

  let token = "";
  let next = "/";
  if (raw) {
    try {
      const parsed = JSON.parse(raw);
      if (typeof parsed?.token === "string") token = parsed.token;
      if (typeof parsed?.next === "string") next = safeRedirect(parsed.next);
    } catch {
      // ignore — pastda token bo'sh bo'lsa xato holatiga tushadi
    }
  }

  if (!token || !secret) {
    return NextResponse.redirect(new URL("/login?error=telegram", origin), 303);
  }

  const res = await goFetch(
    `/auth/telegram/login/status?token=${encodeURIComponent(token)}&c=${encodeURIComponent(secret)}`,
  );
  const data = await res.json().catch(() => ({}));
  if (!res.ok || typeof data?.token !== "string") {
    return NextResponse.redirect(new URL("/login?error=telegram", origin), 303);
  }

  await setSessionToken(data.token);
  await setClientKind("web");
  return NextResponse.redirect(new URL(next, origin), 303);
}
