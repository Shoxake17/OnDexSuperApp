import { NextResponse } from "next/server";
import { cookies } from "next/headers";
import { goFetch } from "@/lib/api";

// GET /api/auth/telegram-login/status — "kutilmoqda..." holatini
// ko'rsatish uchun YENGIL polling (kosmetik). Haqiqiy kirish
// `/api/auth/telegram-return` orqali (botning "qaytish" tugmasi
// bosilganda) yakunlanadi — bu marshrut faqat "hali botda tasdiqlanmadi
// / muddati tugadi" holatini bildiradi, hech qanday sessiya cookie'sini
// O'ZGARTIRMAYDI.
export async function GET() {
  const raw = (await cookies()).get("chust_tg_login")?.value;
  if (!raw) return NextResponse.json({ pending: false, expired: true });

  let token: string | undefined;
  try {
    token = JSON.parse(raw)?.token;
  } catch {
    // ignore
  }
  if (!token) return NextResponse.json({ pending: false, expired: true });

  const res = await goFetch(`/auth/telegram/login/status?token=${encodeURIComponent(token)}`);
  if (res.status === 404) return NextResponse.json({ pending: false, expired: true });
  const data = await res.json().catch(() => ({}));
  return NextResponse.json({ pending: Boolean(data?.pending) });
}
