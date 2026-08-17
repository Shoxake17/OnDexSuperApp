// Telegram Mini App (TMA) — brauzer tomonidagi yordamchi.
//
// ┌─ NIMA UCHUN ALOHIDA FAYL ─────────────────────────────────────────┐
// `window.Telegram.WebApp` FAQAT Telegram ichida mavjud. Oddiy
// brauzerda (yoki Flutter WebView'da) u `undefined` bo'ladi va unga
// to'g'ridan-to'g'ri murojaat qilish sahifani BUTUNLAY yiqitadi.
// Shuning uchun barcha murojaatlar shu yerda, bitta joyda va
// himoyalangan holda.
// └───────────────────────────────────────────────────────────────────┘

/** Telegram mavzusi (foydalanuvchi tanlagan rang sxemasi). */
export type TelegramThemeParams = {
  bg_color?: string;
  text_color?: string;
  hint_color?: string;
  link_color?: string;
  button_color?: string;
  button_text_color?: string;
  secondary_bg_color?: string;
  header_bg_color?: string;
  section_bg_color?: string;
  section_separator_color?: string;
  subtitle_text_color?: string;
  destructive_text_color?: string;
};

/** Telegram WebApp API'ning BIZ ISHLATADIGAN qismi. */
export type TelegramWebApp = {
  initData: string;
  initDataUnsafe?: {
    user?: { id?: number; first_name?: string };
    /**
     * start_param — QR kod havolasidagi `?startapp=<token>` qiymati.
     *
     * `Unsafe` nomi qo'rqitmasin: bu yerda u faqat QAYSI SAHIFANI
     * ochishni hal qiladi. Haqiqiy tekshiruv serverda — token
     * `restaurant_tables.qr_token` bilan solishtiriladi va u 32
     * baytlik tasodifiy sir, ya'ni taxmin qilib bo'lmaydi.
     */
    start_param?: string;
  };
  version: string;
  platform: string;
  colorScheme?: "light" | "dark";
  themeParams?: TelegramThemeParams;
  ready: () => void;
  expand: () => void;
  setHeaderColor?: (color: string) => void;
  setBackgroundColor?: (color: string) => void;
  /** Bot API 8.0+ — haqiqiy to'liq ekran (yuqori panel yo'qoladi). */
  requestFullscreen?: () => void;
  onEvent?: (event: string, cb: () => void) => void;
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

  /**
   * showScanQrPopup — Bot API 6.4+. Telegram'ning O'Z native kamera
   * skanerini ochadi.
   *
   * ┌─ NEGA SKANER PAKETI KERAK EMAS ───────────────────────────────┐
   * Mijoz ilovasida (Flutter) skaner paketi yo'q va shu sabab QR
   * tugmasi u yerda faqat tushuntirish oynasini ochadi. Mini App'da
   * esa kamera Telegram tomonidan beriladi — hech qanday qo'shimcha
   * bog'liqlik, ruxsat so'rash yoki `?table=` marshruti kerak emas.
   * └───────────────────────────────────────────────────────────────┘
   *
   * `callback` `true` qaytarsa skaner YOPILADI, `false` qaytarsa ochiq
   * qoladi va skanerlash davom etadi. Notanish QR uchun ataylab `false`
   * qaytariladi: mijoz kadrni qayta tutishi kerak, oyna esa yopilib
   * ketmasin.
   */
  showScanQrPopup?: (
    params: { text?: string },
    callback?: (text: string) => boolean | void,
  ) => void;
  closeScanQrPopup?: () => void;
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

/**
 * applyTelegramTheme — Telegram ranglarini sahifaga qo'llaydi.
 *
 * ┌─ FAQAT TELEGRAM ICHIDA ───────────────────────────────────────────┐
 * `<html data-tg="1">` belgisi FAQAT `initData` mavjud bo'lganda
 * qo'yiladi. Flutter WebView'da va oddiy brauzerda u YO'Q, ya'ni
 * mavjud ranglar (`#121212` — Flutter foni bilan aynan mos)
 * o'zgarishsiz qoladi.
 *
 * Bu MUHIM: WebView Flutter'ning `SafeArea` ichida turadi va tepa/
 * pastda native fon ko'rinadi. Ranglar farq qilsa chok (seam) sezilib
 * qolardi — bu allaqachon bir marta tuzatilgan bug (globals.css).
 * └───────────────────────────────────────────────────────────────────┘
 *
 * ┌─ NEGA O'Z NOM MAYDONI (`--ondex-tg-*`) ───────────────────────────┐
 * Telegram SDK'ning o'zi ham `--tg-theme-*` o'zgaruvchilarini
 * qo'shadi. LEKIN skript Flutter WebView'da HAM yuklanadi (u
 * hamma sahifada bor) va u yerda standart qiymatlarni belgilab
 * qo'yishi mumkin. O'shanda `var(--tg-theme-bg-color, #121212)`
 * dagi ZAXIRA QIYMAT ISHLAMAY QOLARDI va native ilovaning rangi
 * ham o'zgarib ketardi — talab esa aynan buning teskarisi.
 *
 * O'z prefiksimizni FAQAT shu funksiya yozadi, u esa faqat
 * `initData` bor bo'lganda chaqiriladi. Demak Telegram tashqarisida
 * o'zgaruvchilar UMUMAN mavjud emas va zaxira qiymat kafolatlangan.
 * └───────────────────────────────────────────────────────────────────┘
 */
/**
 * `themeChanged` obrabotchigi ulanganmi. Modul darajasida — pastdagi
 * izohga qarang (`applyTelegramTheme` o'zini o'zi qayta chaqiradi).
 */
let themeListenerBound = false;

export function applyTelegramTheme(wa: TelegramWebApp): void {
  const root = document.documentElement;
  root.setAttribute("data-tg", "1");
  if (wa.colorScheme) root.setAttribute("data-tg-scheme", wa.colorScheme);

  // ┌─ TELEGRAM RANGLARI QO'LLANMAYDI (2026-08-17) ─────────────────────┐
  // Avval bu yerda `wa.themeParams` dagi ranglar `--ondex-tg-*`
  // o'zgaruvchilariga yozilardi. Ya'ni Mini App foydalanuvchining
  // TELEGRAM mavzusiga ergashardi: Telegram'i qorong'i bo'lgan mijozda
  // ilova ham qorong'i ochilardi.
  //
  // Endi ilova FAQAT yorug' mavzuda (kirish/ro'yxatdan o'tish ekranlari
  // bilan bir xil). Ranglar o'rnatilmaydi — `globals.css` dagi
  // `html[data-tg="1"]` qoidalari o'zining YORUG' zaxira qiymatlaridan
  // foydalanadi.
  //
  // Bu funksiya baribir kerak: u `data-tg` belgisini qo'yadi (Telegram
  // uchun maxsus bo'shliq qoidalari shunga bog'liq) va `themeChanged`
  // tinglovchisini ulaydi.
  //
  // KELAJAKDA qorong'i rejim qaytarilsa, yuqoridagi `set(...)`
  // chaqiruvlarini tiklash yetarli — `tailwind.config.ts` dagi
  // `darkMode` izohiga qarang.
  // └───────────────────────────────────────────────────────────────────┘
  //
  // Telegram'ning O'Z yuqori paneli va foni ham OQ qilinadi — aks holda
  // qorong'i Telegram'da ilova oq, panel qora bo'lib, chok (seam)
  // ko'rinib qolardi.
  wa.setHeaderColor?.("#ffffff");
  wa.setBackgroundColor?.("#ffffff");

  // ┌─ TO'LIQ EKRAN SO'RALMAYDI ────────────────────────────────────────┐
  // Bu yerda AVVAL `wa.requestFullscreen()` (Bot API 8.0+) chaqirilardi
  // va u ILOVANI BUTUNLAY QOTIRIB QO'YARDI — "hech nima bosilmaydi"
  // belgisining ildizi aynan shu edi.
  //
  // Mexanizm (Telegram Desktop'da ekran surati bilan tasdiqlangan):
  //   1. Mini App ~480x740 oynada ochiladi va sahifa SHU o'lchamda
  //      joylashadi hamda chiziladi;
  //   2. bu chaqiruv oynani butun ekranga yoyadi;
  //   3. WebView qayta joylashmaydi va qayta chizilmaydi — eski
  //      480x740 kadr chap yuqori burchakda qotib qoladi;
  //   4. sichqoncha/barmoq bosishi esa BUTUN OYNA koordinatalarida
  //      yetkaziladi. Sahifa o'zini 1920px keng deb hisoblaydi va
  //      bosish hech qanday elementga tushmaydi.
  // Natijada ko'rinish to'g'ri, lekin BIRORTA tugma ham, `<a>` havola
  // ham ishlamaydi. Xato JIMGINA: konsolda ham, serverda ham iz yo'q.
  //
  // Bu `layout.tsx` dagi viewport meta bug'i bilan AYNAN BIR SINF:
  // chizish koordinatalari bilan bosish koordinatalari mos kelmasligi.
  //
  // To'liq ekran FAQAT kosmetik qulaylik edi (Telegram'ning yuqori
  // panelini yashiradi), narxi esa — ilovaning ishlamay qolishi.
  // Shuning uchun umuman so'ralmaydi.
  //
  // Qaytadan qo'shmoqchi bo'lsangiz: `wa.platform` ni tekshirib FAQAT
  // "android"/"android_x"/"ios" uchun chaqiring (Telegram uni aynan
  // mobil uchun mo'ljallagan) va HAQIQIY qurilmada sinab ko'ring —
  // desktop'da u yuqoridagi holatga olib keladi.
  // └───────────────────────────────────────────────────────────────────┘

  // ┌─ FAQAT BIR MARTA ULANADI ─────────────────────────────────────────┐
  // Telegram SDK'sining `onEvent` funksiyasi takrorni FUNKSIYA
  // IDENTIFIKATORI bo'yicha filtrlaydi (`eventHandlers[type].indexOf`).
  // Bu yerda esa har chaqiriqda YANGI arrow-funksiya beriladi, ya'ni
  // filtr hech qachon ishlamasdi va obrabotchiklar to'planib borardi:
  // har `themeChanged` hodisasi ularning sonini IKKI BAROBAR oshirardi
  // (2^n). Bir necha mavzu o'zgarishidan keyin sahifa qotib qolardi.
  //
  // `applyTelegramTheme` o'zini o'zi qayta chaqirgani uchun bayroq
  // modul darajasida — funksiya ichidagi o'zgaruvchi yordam bermasdi.
  // └───────────────────────────────────────────────────────────────────┘
  if (!themeListenerBound) {
    themeListenerBound = true;
    // Foydalanuvchi Telegram mavzusini almashtirsa sahifa ham o'zgarsin.
    wa.onEvent?.("themeChanged", () => applyTelegramTheme(wa));
  }
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
