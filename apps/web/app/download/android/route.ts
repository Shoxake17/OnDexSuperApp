import { NextResponse } from "next/server";

// ┌─ ILOVANI YUKLAB OLISH ─────────────────────────────────────────────┐
// Landing'dagi "Ilovani yuklab olish" shu yerga keladi.
//
// APK saytning o'zida TURMAYDI (~100 MB; git'ga ham, Docker image'ga ham
// kirmaydi): u R2 da, reliz `go run ./cmd/r2upload -android-release ...`
// bilan joylanadi va `releases/android/latest.json` manifesti yangilanadi.
// Bu yo'l manifestni o'qib eng so'nggi APK ga yo'naltiradi — yangi reliz
// saytni qayta deploy qilmasdan chiqadi.
//
// Manifestdagi manzilga KO'R-KO'RONA ishonilmaydi: faqat o'sha media
// domenidagi `releases/android/*.apk` qabul qilinadi. Aks holda bu sahifa
// ochiq qayta yo'naltirish (open redirect) vositasiga aylanardi.
// └────────────────────────────────────────────────────────────────────┘

const MANIFEST_PATH = "/releases/android/latest.json";
const RELEASE_DIR = "/releases/android/";

function mediaOrigin(): string | null {
  const raw = process.env.NEXT_PUBLIC_MEDIA_ORIGIN?.trim();
  if (!raw) return null;
  try {
    const u = new URL(raw);
    return u.protocol === "https:" ? u.origin : null;
  } catch {
    return null;
  }
}

/** Manifestdagi APK manzili — faqat tekshiruvdan o'tsa. */
export function trustedApkUrl(raw: unknown, origin: string): string | null {
  if (typeof raw !== "string") return null;
  try {
    const u = new URL(raw);
    if (u.origin !== origin) return null;
    if (!u.pathname.startsWith(RELEASE_DIR) || !u.pathname.endsWith(".apk")) {
      return null;
    }
    if (u.search || u.hash || u.pathname.includes("..")) return null;
    return u.toString();
  } catch {
    return null;
  }
}

async function latestApkUrl(): Promise<string | null> {
  const origin = mediaOrigin();
  if (!origin) return null;
  try {
    // Manifest R2 da 60 soniya keshlanadi — bu yerda ham shuncha.
    const res = await fetch(origin + MANIFEST_PATH, { next: { revalidate: 60 } });
    if (!res.ok) return null;
    const data: unknown = await res.json();
    const url = data && typeof data === "object" ? (data as { url?: unknown }).url : null;
    return trustedApkUrl(url, origin);
  } catch {
    return null;
  }
}

export async function GET() {
  const url = await latestApkUrl();
  if (url) return NextResponse.redirect(url, 302);
  return new NextResponse(
    "OnDex ilovasi hozircha yuklab olish uchun joylanmagan. Tez orada qo'shiladi.",
    {
      status: 503,
      headers: {
        "Content-Type": "text/plain; charset=utf-8",
        "Cache-Control": "no-store",
        "Retry-After": "3600",
      },
    },
  );
}
