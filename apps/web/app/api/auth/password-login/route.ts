import { NextRequest, NextResponse } from "next/server";
import { goFetch } from "@/lib/api";
import { setClientKind, setSessionToken } from "@/lib/session";

// POST /api/auth/password-login {"login":"+998...","password":"..."} — Go'ning
// POST /auth/login'ini chaqiradi va muvaffaqiyatda JWT'ni httpOnly cookie'ga
// yozadi (xuddi /api/auth/verify kabi: token brauzerga JSON bo'lib chiqmaydi).
//
// Go xatosi (noto'g'ri parol / bloklangan / limit) o'sha holat kodi bilan
// qaytariladi; xabar bir xil umumiy matn, ya'ni raqam mavjudligi sezilmaydi.
export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => null);
  if (typeof body?.login !== "string" || typeof body?.password !== "string" ||
      !body.login || !body.password) {
    return NextResponse.json({ error: "telefon va parol kerak" }, { status: 400 });
  }
  const res = await goFetch("/auth/login", {
    method: "POST",
    body: JSON.stringify({ login: body.login, password: body.password }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok || typeof data?.token !== "string") {
    return NextResponse.json(
      { error: data?.error ?? "Telefon yoki parol noto'g'ri." },
      { status: res.status || 400 },
    );
  }
  await setSessionToken(data.token);
  await setClientKind("web");
  return NextResponse.json({ ok: true });
}
