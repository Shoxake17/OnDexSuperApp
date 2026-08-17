"use client";

import { QrCode } from "lucide-react";
import { useState } from "react";
import { extractTableToken, openTableFromToken } from "@/lib/open-table";
import { getTelegramWebApp, versionAtLeast } from "@/lib/telegram";

// Pastki menyu markazidagi ko'tarilgan QR tugmasi (maket: image/restarant.png).
//
// ┌─ BU TUGMA QAYERDA KO'RINADI ──────────────────────────────────────┐
// Faqat Telegram Mini App'da va oddiy brauzerda. Flutter mijoz
// ilovasida pastki menyu UMUMAN chizilmaydi (`(food)/layout.tsx` —
// `{!inApp && <BottomNav />}`), chunki u yerda Flutter'ning O'Z
// native menyusi bor va uning QR tugmasi native skanerni ochadi
// (`home_shell.dart` -> `qr_scan_screen.dart`, `mobile_scanner`).
//
// ESLATMA (2026-08-17 da yangilandi): bu izohda avval "Flutter
// ilovasida skaner paketi yo'q" deb yozilgan edi. Bu ENDI TO'G'RI
// EMAS — skaner qo'shildi va tugmaga ulandi.
//
// Mini App'da kamerani Telegram beradi (`showScanQrPopup`, Bot API
// 6.4+): yangi bog'liqlik ham, kamera ruxsatini o'zimiz so'rashimiz
// ham kerak emas. Shuning uchun stol oqimi shu yerda ham to'liq
// ishlaydi.
// └───────────────────────────────────────────────────────────────────┘

// ┌─ KAMERA FAQAT MOBIL KLIENTLARDA ──────────────────────────────────┐
// `showScanQrPopup` metodi desktop klientlarda ham MAVJUD (SDK uni
// versiyaga qarab beradi), lekin u yerda kamera yo'q va chaqiruv
// JIMGINA hech narsa qilmaydi — tugma o'lik bo'lib ko'rinardi.
//
// Bu aynan `requestFullscreen` bilan bo'lgan xatoning takrori bo'lardi:
// metod bor deb tekshirish YETARLI EMAS, platformani ham tekshirish
// kerak. Shuning uchun ro'yxat oq ro'yxat (whitelist): notanish yangi
// platforma paydo bo'lsa, u tushuntirish oynasini oladi — o'lik tugma
// emas.
// └───────────────────────────────────────────────────────────────────┘
const CAMERA_PLATFORMS: ReadonlySet<string> = new Set([
  "android",
  "android_x",
  "ios",
]);

export default function QrScanButton() {
  // `note` — Telegram tashqarisida yoki eski klientda ko'rsatiladigan
  // tushuntirish; `error` — skanerlandi, lekin kod yaroqsiz chiqdi.
  const [note, setNote] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function openTable(token: string) {
    setBusy(true);
    // Butun mantiq `lib/open-table.ts` da — mijoz ilovasi va Telegram
    // `start_param` yo'llari ham AYNAN shu funksiyani chaqiradi.
    const res = await openTableFromToken(token);
    if (!res.ok) setError(res.error);
    setBusy(false);
  }

  function onClick() {
    const wa = getTelegramWebApp();

    // Telegram tashqarisida (oddiy brauzer, Flutter WebView), 6.4 dan
    // eski klientda yoki kamerasi yo'q platformada — o'lik tugma
    // qoldirmasdan hozirgi HAQIQIY oqimni tushuntiramiz.
    if (
      !wa?.showScanQrPopup ||
      !versionAtLeast(wa.version, "6.4") ||
      !CAMERA_PLATFORMS.has((wa.platform || "").toLowerCase())
    ) {
      setNote(true);
      return;
    }

    wa.showScanQrPopup({ text: "Stoldagi QR kodni kameraga tuting" }, (text) => {
      const token = extractTableToken(text);
      // `false` — skaner ochiq qoladi. Begona QR tushib qolganda oyna
      // yopilib ketmasin, mijoz kadrni qayta tutsin.
      if (!token) return false;
      wa.closeScanQrPopup?.();
      void openTable(token);
      return true;
    });
  }

  return (
    <>
      <button
        type="button"
        onClick={onClick}
        disabled={busy}
        aria-label="Stol QR kodini skanerlash"
        // `-mt-7` — tugma menyu chizig'idan yuqoriga ko'tarilib turadi
        // (maketdagi asosiy urg'u). `ring` fon rangida: menyu chizig'i
        // tugmaning ostidan o'tib ketmasin.
        className="-mt-7 flex h-14 w-14 items-center justify-center rounded-full bg-brand text-white shadow-lg ring-4 ring-white transition-transform active:scale-95 disabled:opacity-60 dark:ring-[#1A1A1A]"
      >
        <QrCode size={26} />
      </button>

      {(note || error) && (
        <div
          role="dialog"
          className="fixed inset-0 z-50 flex items-end justify-center bg-black/60 p-4"
          onClick={() => {
            setNote(false);
            setError(null);
          }}
        >
          <div
            className="tg-surface w-full max-w-sm rounded-2xl bg-white p-5 text-center dark:bg-[#1E1E1E]"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-brand/15 text-brand">
              <QrCode size={26} />
            </div>
            <h2 className="text-[17px] font-bold">Stol QR kodi</h2>
            <p className="tg-muted mt-2 text-sm leading-relaxed text-neutral-500">
              {error ??
                "Restoranda o'tirgan bo'lsangiz, stoldagi QR kodni telefoningizning o'z kamerasi bilan skanerlang — menyu shu stolga bog'langan holda ochiladi."}
            </p>
            <button
              onClick={() => {
                setNote(false);
                setError(null);
              }}
              className="mt-5 h-12 w-full rounded-2xl bg-brand text-base font-bold text-white"
            >
              Tushunarli
            </button>
          </div>
        </div>
      )}
    </>
  );
}
