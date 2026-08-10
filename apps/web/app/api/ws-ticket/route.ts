import { NextResponse } from "next/server";
import { getSessionToken } from "@/lib/session";
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
  const res = await goFetch("/ws/ticket", { method: "POST" }, token);
  const data = await res.json().catch(() => ({}));
  return NextResponse.json(data, { status: res.status });
}
