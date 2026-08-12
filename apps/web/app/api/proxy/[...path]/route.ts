import { NextRequest, NextResponse } from "next/server";
import { getSessionToken } from "@/lib/session";
import { goFetch } from "@/lib/api";

// Generic BFF proksi: /api/proxy/restaurants -> Go GET /restaurants,
// /api/proxy/orders -> Go POST /orders va h.k. Brauzer/WebView hech qachon
// Go backend'ga to'g'ridan-to'g'ri murojaat qilmaydi — faqat shu qatlam
// orqali (cookie'dagi token serverda Bearer'ga aylantiriladi). Kelajakdagi
// har qanday yangi mini-app ham shu bitta proksidan foydalana oladi,
// alohida route yozish shart emas.
// Mijoz ilovasi HAQIQATAN ishlatadigan endpointlar. Har bir yozuv —
// metod + yo'l shabloni; `*` bitta segmentga mos keladi.
//
// NEGA KERAK: avval proksi kelgan HAR QANDAY yo'lni Go'ga uzatardi va
// sessiya tokenini biriktirardi — ya'ni sahifadagi istalgan skript
// `/api/proxy/admin/...` yoki `/api/proxy/couriers/.../location` ni
// chaqira olardi. Himoya butunlay Go tomonidagi rol tekshiruvlariga
// bog'liq edi; bu qatlam esa hech qanday qo'shimcha to'siq bermasdi.
// Endi ruxsat etilgan ro'yxat (allowlist) — chuqurlashtirilgan himoya.
const ALLOWED: ReadonlyArray<{ method: string; pattern: string }> = [
  { method: "GET", pattern: "me" },
  { method: "GET", pattern: "me/address" },
  { method: "POST", pattern: "me/address" },
  { method: "GET", pattern: "me/orders" },
  { method: "GET", pattern: "restaurants" },
  { method: "GET", pattern: "restaurants/*" },
  { method: "GET", pattern: "restaurants/*/menu" },
  { method: "GET", pattern: "restaurants/*/active-promotions" },
  { method: "POST", pattern: "restaurants/*/quote" },
  { method: "GET", pattern: "categories" },
  { method: "GET", pattern: "products/search" },
  { method: "POST", pattern: "orders" },
  { method: "GET", pattern: "orders/*" },
  { method: "GET", pattern: "favorites" },
  { method: "GET", pattern: "favorites/ids" },
  { method: "POST", pattern: "favorites/*" },
  { method: "DELETE", pattern: "favorites/*" },
  { method: "GET", pattern: "geocode/reverse" },
  { method: "GET", pattern: "config/maps" },
  // Stol QR kodi: token → qaysi restoran va qaysi stol.
  //
  // FAQAT `resolve` ochiladi. Stol yaratish/o'chirish/token yangilash
  // (`POST /restaurants/*/tables`, `POST /tables/*/regenerate`)
  // ATAYLAB yo'q: ular restoran paneliga tegishli va mijoz sahifasidagi
  // skript ularga umuman yeta olmasligi kerak.
  { method: "GET", pattern: "tables/resolve" },
];

function isAllowed(method: string, segments: string[]): boolean {
  return ALLOWED.some((rule) => {
    if (rule.method !== method) return false;
    const parts = rule.pattern.split("/");
    if (parts.length !== segments.length) return false;
    return parts.every((p, i) => p === "*" || p === segments[i]);
  });
}

async function handle(req: NextRequest, path: string[]): Promise<NextResponse> {
  // Yo'l segmentlarini tozalaymiz: bo'sh, `.` va `..` — traversal
  // urinishlarini boshidanoq rad etamiz.
  if (path.some((s) => s === "" || s === "." || s === ".." || s.includes("\\"))) {
    return NextResponse.json({ error: "yo'l noto'g'ri" }, { status: 400 });
  }
  if (!isAllowed(req.method, path)) {
    return NextResponse.json({ error: "ruxsat berilmagan" }, { status: 403 });
  }

  const token = await getSessionToken();
  const targetPath = "/" + path.join("/") + req.nextUrl.search;

  const init: RequestInit = { method: req.method };
  if (req.method !== "GET" && req.method !== "HEAD") {
    const body = await req.text();
    if (body) init.body = body;
  }

  const res = await goFetch(targetPath, init, token);
  const text = await res.text();
  return new NextResponse(text, {
    status: res.status,
    headers: {
      "Content-Type": res.headers.get("Content-Type") ?? "application/json",
    },
  });
}

type RouteContext = { params: Promise<{ path: string[] }> };

export async function GET(req: NextRequest, ctx: RouteContext) {
  return handle(req, (await ctx.params).path);
}
export async function POST(req: NextRequest, ctx: RouteContext) {
  return handle(req, (await ctx.params).path);
}
export async function PUT(req: NextRequest, ctx: RouteContext) {
  return handle(req, (await ctx.params).path);
}
export async function DELETE(req: NextRequest, ctx: RouteContext) {
  return handle(req, (await ctx.params).path);
}
