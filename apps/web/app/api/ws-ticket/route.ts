import { NextResponse } from "next/server";
import { getClientKind, getSessionToken } from "@/lib/session";
import { goFetch } from "@/lib/api";

// POST /api/ws-ticket — cookie'dagi sessiyani Go'ning qisqa muddatli
// (30s, bir martalik) WS biletiga almashtiradi. Qaytgan bilet keyin
// brauzerdan TO'G'RIDAN-TO'G'RI wss://.../ws?ticket=...ga ulanish uchun
// ishlatiladi (WebSocket ulanishlar CORS'ga bog'liq emas, shuning uchun
// bu qadamdan keyin BFF proksisi kerak emas).
export async function POST() {
  const token = await getSessionToken();
  if (!token) {
    return NextResponse.json({ error: "tizimga kirilmagan" }, { status: 401 });
  }
  // Bilet so'rovi ham `X-Ondex-Client` bilan ketadi: mini-app'ning
  // ko'p sahifasi serverda render qilinadi va proksiga umuman
  // tegmasligi mumkin — u holda "qaysi dasturdan kirgani" hech qachon
  // yozilmasdi. Jonli holat kanali esa deyarli har sessiyada ochiladi.
  const res = await goFetch(
    "/ws/ticket",
    { method: "POST" },
    token,
    await getClientKind(),
  );
  const data = await res.json().catch(() => ({}));
  return NextResponse.json(data, { status: res.status });
}
