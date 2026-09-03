import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";

// ┌─ NEGA BU FAQAT SERVERDA VA TELEGRAM/ILOVA HOLATINI BILISHNING
//    HOJATI YO'Q ─────────────────────────────────────────────────────┐
// Bu funksiya faqat cookie'dagi sessiyani tekshiradi — Telegram yoki
// Flutter ilovasi ichida ekanini alohida aniqlashning HOJATI yo'q:
//
//   * Telegram Mini App'da sessiya `telegram-auth.tsx` orqali TO'LIQ
//     SAHIFA RELOAD bilan o'rnatiladi (window.location.replace) — ya'ni
//     mijoz keyingi (himoyalangan) sahifaga o'tguncha cookie ALLAQACHON
//     bor. Auth tugamaguncha esa `TelegramAuth` butun ekranni to'suvchi
//     modal ko'rsatadi (root layout.tsx) — foydalanuvchi FIZIK ravishda
//     bu yerga (himoyalangan sahifaga) o'ta olmaydi.
//   * Ilova (Flutter WebView) ichida `/api/bridge` cookie'ni SERVER
//     303 REDIRECT orqali, sahifa birinchi marta render bo'lishidan
//     OLDIN o'rnatadi.
//
// Ya'ni ikkala holatda ham bu yerga `signedIn=false` bilan yetib
// kelish AMALDA MUMKIN EMAS — faqat oddiy brauzerda, hech qanday
// avtomatik kirish yo'lisiz kelgan mehmon shu yo'lga tushadi.
// (`eats.ondex.uz` qo'shilgach, 2026-09-04: shu holat aniqlangan —
// "xarita yuklanmadi" xatosi aslida BU EDI, login sahifasi umuman
// yo'q edi.)
// └───────────────────────────────────────────────────────────────────┘

/**
 * Sessiya bo'lmasa `/login?next=<joriy yo'l>`ga yo'naltiradi.
 *
 * `pathname` — joriy sahifaning yo'li (masalan `/address`).
 * `searchParams` — berilsa, login'dan keyin ANIQ shu so'rov satri bilan
 * qaytariladi (masalan `/address?next=/checkout` — login orqali o'tgach
 * ham checkout'ga borish zanjiri uzilmasin uchun).
 */
export async function requireAuth(
  pathname: string,
  searchParams?: Record<string, string | string[] | undefined>,
): Promise<void> {
  if (await getSessionToken()) return;

  const qs = new URLSearchParams();
  if (searchParams) {
    for (const [key, value] of Object.entries(searchParams)) {
      if (typeof value === "string") qs.set(key, value);
      else if (Array.isArray(value) && value[0] !== undefined) qs.set(key, value[0]);
    }
  }
  const target = qs.size > 0 ? `${pathname}?${qs}` : pathname;
  redirect(`/login?next=${encodeURIComponent(target)}`);
}
