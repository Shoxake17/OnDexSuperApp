import { NextRequest, NextResponse } from "next/server";
import { goFetch } from "@/lib/api";
import { setSessionToken } from "@/lib/session";

// POST /api/auth/verify {"phone":"...","code":"..."} — Go'ning
// POST /auth/verify'ini chaqiradi va muvaffaqiyatda qaytgan JWT'ni
// httpOnly cookie'ga yozadi. Token o'zi hech qachon brauzerga JSON
// sifatida qaytarilmaydi — faqat {"ok": true}.
export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => null);
  if (!body?.phone || !body?.code) {
    return NextResponse.json({ error: "telefon va kod kerak" }, { status: 400 });
  }
  const res = await goFetch("/auth/verify", {
    method: "POST",
    body: JSON.stringify({ phone: body.phone, code: body.code }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok || typeof data?.token !== "string") {
    return NextResponse.json(data, { status: res.status || 400 });
  }
  await setSessionToken(data.token);
  return NextResponse.json({ ok: true, user: data.user });
}
