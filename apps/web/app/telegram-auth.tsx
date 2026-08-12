"use client";

import { useEffect, useRef, useState } from "react";
import Script from "next/script";
import {
  applyTelegramTheme,
  authenticateWithTelegram,
  getTelegramWebApp,
  versionAtLeast,
  type MiniAppAuthResult,
} from "@/lib/telegram";

// Telegram Mini App avtomatik kirishi.
//
// ┌─ QANDAY ISHLAYDI ─────────────────────────────────────────────────┐
// 1. Telegram SDK yuklanadi (faqat Telegram ichida ma'noga ega);
// 2. `initData` serverga yuboriladi, u imzoni tekshiradi;
// 3. Muvaffaqiyatda httpOnly cookie o'rnatiladi — sahifa yangilanadi
//    va foydalanuvchi kirgan holatda bo'ladi;
// 4. Raqam hali bog'lanmagan bo'lsa (409) — kontakt so'raladi.
//
// Telegram TASHQARISIDA bu komponent HECH NARSA QILMAYDI: brauzerda
// va Flutter WebView'da `initData` bo'lmaydi, shuning uchun u jimgina
// chetga chiqadi va mavjud kirish oqimlariga xalaqit bermaydi.
// └───────────────────────────────────────────────────────────────────┘

export default function TelegramAuth({ signedIn }: { signedIn: boolean }) {
  const [state, setState] = useState<MiniAppAuthResult | null>(null);
  const [busy, setBusy] = useState(false);
  // Bir martalik: React `useEffect` ni development'da IKKI MARTA
  // chaqiradi (StrictMode) va busiz server ikki marta so'rov olardi.
  const started = useRef(false);

  // ┌─ NEGA `onLoad`, `useEffect` EMAS ─────────────────────────────────┐
  // `window.Telegram.WebApp` FAQAT SDK yuklangandan keyin paydo
  // bo'ladi. `useEffect` esa skript yuklanishini KUTMAYDI — u
  // birinchi renderdan keyin darhol ishlaydi va o'sha paytda
  // `window.Telegram` hali `undefined` bo'lishi mumkin. Natijada
  // Telegram ichida bo'lsak ham "Telegram emas" degan xulosaga
  // kelinardi va avtomatik kirish JIMGINA ishlamasdi.
  //
  // `beforeInteractive` strategiyasi buni hal qilardi, LEKIN u
  // Next.js App Router'da faqat root `layout.tsx` da (server
  // komponentda) qo'llanadi — client komponentda e'tiborsiz qoladi.
  // Shuning uchun `afterInteractive` + aniq `onLoad`.
  // └───────────────────────────────────────────────────────────────────┘
  function onSdkReady() {
    const wa = getTelegramWebApp();
    if (!wa) return; // Telegram emas — aralashmaymiz

    // ┌─ MAVZU HAR DOIM ──────────────────────────────────────────────┐
    // Ranglar kirish holatiga BOG'LIQ EMAS. Ilgari bu yerda birinchi
    // qator `if (signedIn) return` edi — mavzu shu sababli aynan eng
    // ko'p uchraydigan holatda (foydalanuvchi allaqachon kirgan)
    // qo'llanmasdan qolardi.
    //
    // `applyTelegramTheme` idempotent: bir xil qiymatlarni qayta
    // yozadi, xolos.
    // └───────────────────────────────────────────────────────────────┘
    wa.ready();
    wa.expand();
    applyTelegramTheme(wa);

    if (signedIn || started.current) return;
    started.current = true;

    void (async () => {
      const r = await authenticateWithTelegram();
      setState(r);
      if (r.status === "ok") {
        // Cookie o'rnatildi — server komponentlari uni ko'rishi uchun
        // sahifa qayta yuklanadi. `replace` ATAYLAB: orqaga tugmasi
        // foydalanuvchini kirmagan holatga qaytarmasin.
        window.location.replace(window.location.pathname + window.location.search);
      }
    })();
  }

  // SDK sahifadan OLDIN yuklangan bo'lishi ham mumkin (masalan
  // Telegram uni o'zi kiritganda) — o'shanda `onLoad` chaqirilmaydi.
  useEffect(() => {
    if (getTelegramWebApp()) onSdkReady();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signedIn]);

  if (signedIn || !state || state.status === "ok" || state.status === "not_telegram") {
    return (
      <Script
        src="https://telegram.org/js/telegram-web-app.js"
        strategy="afterInteractive"
        onLoad={onSdkReady}
      />
    );
  }

  // `state` — `useState` qiymati, shuning uchun TypeScript uni ichki
  // funksiyalarda toraytira olmaydi (u istalgan payt o'zgarishi
  // mumkin deb hisoblaydi). Yuqoridagi tekshiruvdan keyingi ANIQ
  // qiymatni mahalliy o'zgaruvchiga olamiz.
  const current: Extract<MiniAppAuthResult, { status: "need_contact" | "error" }> = state;

  async function shareContact() {
    const wa = getTelegramWebApp();
    if (!wa) return;
    setBusy(true);

    // ┌─ IKKI YO'L ───────────────────────────────────────────────────┐
    // `requestContact` Bot API 6.9+ da bor va Mini App'dan CHIQMASDAN
    // raqam so'raydi — eng qulay yo'l.
    //
    // Eski klientlarda u yo'q, o'shanda botga yo'naltiriladi: u yerda
    // "Raqamni ulashish" tugmasi allaqachon ishlaydi
    // (`internal/telegram` — tokensiz `/start`).
    // └───────────────────────────────────────────────────────────────┘
    if (wa.requestContact && versionAtLeast(wa.version, "6.9")) {
      wa.requestContact((ok) => {
        if (!ok) {
          setBusy(false);
          return;
        }
        // Telegram kontaktni BOTGA yuboradi, server esa uni qabul
        // qilib bog'laydi. Bu bir necha yuz millisekund oladi,
        // shuning uchun bir necha marta qayta urinamiz.
        retryAuth();
      });
      return;
    }

    if (current.status === "need_contact" && current.botLink) {
      if (wa.openTelegramLink) wa.openTelegramLink(current.botLink);
      else window.open(current.botLink, "_blank");
    }
    setBusy(false);
  }

  async function retryAuth() {
    for (let i = 0; i < 6; i++) {
      await new Promise((r) => setTimeout(r, 700));
      const r = await authenticateWithTelegram();
      if (r.status === "ok") {
        window.location.replace(window.location.pathname + window.location.search);
        return;
      }
    }
    setBusy(false);
    setState({
      status: "error",
      message: "Raqam hali tasdiqlanmadi. Bir ozdan keyin qaytadan urinib ko'ring.",
    });
  }

  return (
    <>
      <Script
        src="https://telegram.org/js/telegram-web-app.js"
        strategy="afterInteractive"
        onLoad={onSdkReady}
      />
      <div
        role="dialog"
        aria-live="polite"
        style={{
          position: "fixed",
          inset: 0,
          zIndex: 50,
          display: "grid",
          placeItems: "center",
          background: "rgba(0,0,0,.75)",
          padding: 24,
        }}
      >
        <div
          style={{
            maxWidth: 340,
            width: "100%",
            // Zaxira qiymatlar — Telegram mavzu bermagan holat uchun.
            background: "var(--ondex-tg-bg, #1e1e1e)",
            color: "var(--ondex-tg-text, #fff)",
            borderRadius: 16,
            padding: 24,
            textAlign: "center",
          }}
        >
          {current.status === "need_contact" ? (
            <>
              <h2 style={{ fontSize: 18, fontWeight: 600, marginBottom: 8 }}>
                {current.firstName ? `Salom, ${current.firstName}!` : "Salom!"}
              </h2>
              <p
                style={{
                  fontSize: 14,
                  color: "var(--ondex-tg-hint, #bdbdbd)",
                  marginBottom: 20,
                  lineHeight: 1.5,
                }}
              >
                Buyurtma berish uchun raqamingizni tasdiqlang. Telegram uni
                o&apos;zi yuboradi — qo&apos;lda yozish shart emas.
              </p>
              <button
                onClick={shareContact}
                disabled={busy}
                style={{
                  width: "100%",
                  padding: "12px 16px",
                  borderRadius: 10,
                  border: "none",
                  background: busy ? "#3a5a45" : "var(--ondex-tg-button, #1B873F)",
                  color: "var(--ondex-tg-button-text, #fff)",
                  fontSize: 15,
                  opacity: busy ? 0.7 : 1,
                  fontWeight: 600,
                  cursor: busy ? "default" : "pointer",
                }}
              >
                {busy ? "Kutilmoqda…" : "Raqamni tasdiqlash"}
              </button>
            </>
          ) : (
            <>
              <p style={{ fontSize: 14, marginBottom: 16 }}>
                {current.status === "error" ? current.message : ""}
              </p>
              <button
                onClick={() => window.location.reload()}
                style={{
                  width: "100%",
                  padding: "12px 16px",
                  borderRadius: 10,
                  border: "1px solid var(--ondex-tg-hint, #444)",
                  background: "transparent",
                  color: "var(--ondex-tg-text, #fff)",
                  fontSize: 15,
                  cursor: "pointer",
                }}
              >
                Qaytadan urinish
              </button>
            </>
          )}
        </div>
      </div>
    </>
  );
}
