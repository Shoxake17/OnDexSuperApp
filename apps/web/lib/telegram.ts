// Telegram Mini App (TMA) — brauzer tomonidagi yordamchi.
//
// ┌─ NIMA UCHUN ALOHIDA FAYL ─────────────────────────────────────────┐
// `window.Telegram.WebApp` FAQAT Telegram ichida mavjud. Oddiy
// brauzerda (yoki Flutter WebView'da) u `undefined` bo'ladi va unga
// to'g'ridan-to'g'ri murojaat qilish sahifani BUTUNLAY yiqitadi.
// Shuning uchun barcha murojaatlar shu yerda, bitta joyda va
// himoyalangan holda.
// └───────────────────────────────────────────────────────────────────┘

/** Telegram WebApp API'ning BIZ ISHLATADIGAN qismi. */
export type TelegramWebApp = {
  initData: string;
  initDataUnsafe?: { user?: { id?: number; first_name?: string } };
  version: string;
  platform: string;
  ready: () => void;
  expand: () => void;
  openTelegramLink?: (url: string) => void;
  /**
   * requestContact — Bot API 6.9+ da paydo bo'lgan.
   *
   * Foydalanuvchiga native oyna ko'rsatadi; rozi bo'lsa Telegram
   * kontaktni BOTGA yuboradi (Mini App'ning o'ziga EMAS). Server esa
   * uni `internal/telegram` hook'i orqali qabul qilib bog'laydi.
   *
   * Ixtiyoriy: eski Telegram versiyalarida yo'q, shuning uchun
   * chaqirishdan oldin mavjudligi tekshiriladi.
   */
  requestContact?: (cb: (ok: boolean) => void) => void;
};

declare global {
  interface Window {
    Telegram?: { WebApp?: TelegramWebApp };
  }
}

/** Telegram ichida ochilganmi. */
export function getTelegramWebApp(): TelegramWebApp | null {
  if (typeof window === "undefined") return null;
  const wa = window.Telegram?.WebApp;
  // `initData` bo'sh bo'lsa — bu Telegram emas (yoki sahifa to'g'ridan
  // brauzerda ochilgan). Imzosiz kirishga urinishning ma'nosi yo'q.
  if (!wa || typeof wa.initData !== "string" || wa.initData.length === 0) {
    return null;
  }
  return wa;
}

/**
 * versionAtLeast — Telegram klient versiyasini solishtiradi.
 *
 * `requestContact` 6.9 dan boshlab bor. Eski klientda uni chaqirish
 * jimgina hech narsa qilmaydi va foydalanuvchi nima bo'layotganini
 * tushunmay qoladi — shuning uchun oldindan tekshiramiz.
 */
export function versionAtLeast(version: string, min: string): boolean {
  const a = version.split(".").map((n) => parseInt(n, 10) || 0);
  const b = min.split(".").map((n) => parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    const x = a[i] ?? 0;
    const y = b[i] ?? 0;
    if (x !== y) return x > y;
  }
  return true;
}

export type MiniAppAuthResult =
  | { status: "ok" }
  | { status: "need_contact"; botLink: string; firstName: string }
  | { status: "not_telegram" }
  | { status: "error"; message: string };

/**
 * authenticate — `initData` ni serverga yuboradi.
 *
 * Token BRAUZERGA QAYTMAYDI: server uni httpOnly cookie'ga yozadi
 * (`app/api/auth/telegram-miniapp/route.ts`). Shuning uchun bu
 * funksiya faqat holatni qaytaradi.
 */
export async function authenticateWithTelegram(): Promise<MiniAppAuthResult> {
  const wa = getTelegramWebApp();
  if (!wa) return { status: "not_telegram" };

  try {
    const res = await fetch("/api/auth/telegram-miniapp", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ initData: wa.initData }),
    });

    if (res.ok) return { status: "ok" };

    const data = await res.json().catch(() => ({}));
    if (res.status === 409 && data?.need === "share_contact") {
      return {
        status: "need_contact",
        botLink: typeof data.botLink === "string" ? data.botLink : "",
        firstName: typeof data.firstName === "string" ? data.firstName : "",
      };
    }
    return {
      status: "error",
      message: typeof data?.error === "string" ? data.error : "Kirish amalga oshmadi",
    };
  } catch {
    // Tarmoq xatosi — sahifa yiqilmasligi kerak.
    return { status: "error", message: "Serverga ulanib bo'lmadi" };
  }
}
