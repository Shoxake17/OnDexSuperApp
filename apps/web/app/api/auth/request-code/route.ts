import { NextRequest, NextResponse } from "next/server";
import { goFetch } from "@/lib/api";

// POST /api/auth/request-code {"phone":"+998901234567"} — Go'ning
// POST /auth/request-code'ini shaffof proksi qiladi (auth talab qilmaydi,
// cookie bilan ishi yo'q).
export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => null);
  if (!body?.phone) {
    return NextResponse.json({ error: "telefon raqami kerak" }, { status: 400 });
  }
  const res = await goFetch("/auth/request-code", {
    method: "POST",
    body: JSON.stringify({ phone: body.phone }),
  });
  const data = await res.json().catch(() => ({}));
  return NextResponse.json(data, { status: res.status });
}
