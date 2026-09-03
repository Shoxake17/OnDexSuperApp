import Image from "next/image";

/**
 * Bosh ekrandagi ilova rasmi (`public/landing/ondexapp.png`).
 *
 * ┌─ RASM SHAFFOF ─────────────────────────────────────────────────────┐
 * PNG ning burchaklari to'liq shaffof (alfa = 0) — ko'ruvchi
 * dasturlarda qora ko'rinishi shundan. Sahifada u oq fon ustida
 * turadi, shuning uchun hech qanday ramka yoki fon berilmagan.
 * └────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ NEGA `next/image` ────────────────────────────────────────────────┐
 * Asl fayl 1212x1280 va ~1.1 MB. `next/image` uni ekran kengligiga
 * qarab kichraytirib, zamonaviy formatda (AVIF/WebP) beradi — bu
 * bosh ekranning eng og'ir elementi bo'lgani uchun sezilarli farq.
 *
 * `priority` — rasm sahifaning eng yuqorisida, ya'ni u kechikib
 * chizilsa foydalanuvchi bo'sh joyni ko'radi (LCP).
 * └────────────────────────────────────────────────────────────────────┘
 */
export function PhoneMock() {
  return (
    <div className="relative mx-auto w-full max-w-[520px]">
      {/* Orqadagi yumshoq to'q sariq dog' — maketdagidek. Rasmning
          o'zi shaffof bo'lgani uchun u atrofni yoritib turadi. */}
      <div
        aria-hidden
        className="absolute left-1/2 top-1/2 -z-10 h-[85%] w-[85%] -translate-x-1/2 -translate-y-1/2 rounded-full bg-brand/10 blur-3xl"
      />

      <Image
        src="/landing/ondexapp.png"
        alt="OnDex ilovasi — bosh sahifa va logotip ekrani"
        width={1212}
        height={1280}
        priority
        sizes="(max-width: 1024px) 90vw, 520px"
        className="h-auto w-full"
      />
    </div>
  );
}
