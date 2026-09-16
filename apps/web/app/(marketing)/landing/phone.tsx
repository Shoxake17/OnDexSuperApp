import Image from "next/image";

/**
 * Bosh ekrandagi ilova rasmi (`public/landing/ondex-app.png`).
 *
 * ┌─ MANBA VA TOZALASH (2026-09-16) ───────────────────────────────────┐
 * Asl fayl — `image/app.png` (1875×1970). Unda o'ng telefon ORQASIDA
 * ramkadan tashqariga chiqib turgan to'q sariq to'rtburchak fon bor edi.
 * Sahifadagi nusxada u olib tashlangan: rasm chetidan ramkagacha bo'lgan
 * to'q sariq piksellar shaffof qilingan (telefon ekrani ramka bilan
 * o'ralgani uchun unga tegilmagan), ramka chetidagi aralash rangli
 * piksellar kulranglashtirilgan. Burchaklar shaffof — oq fonda turadi.
 * └────────────────────────────────────────────────────────────────────┘
 *
 * `next/image` rasmni ekran kengligiga mos kichraytirib zamonaviy
 * formatda beradi; `priority` — u bosh ekranning eng katta elementi (LCP).
 */
export function PhoneMock() {
  return (
    <div className="relative mx-auto w-full max-w-[520px]">
      {/* Orqadagi yumshoq to'q sariq dog' — rasmning o'zi shaffof. */}
      <div
        aria-hidden
        className="absolute left-1/2 top-1/2 -z-10 h-[85%] w-[85%] -translate-x-1/2 -translate-y-1/2 rounded-full bg-brand/10 blur-3xl"
      />

      <Image
        src="/landing/ondex-app.png"
        alt="OnDex ilovasi — bosh sahifa va logotip ekrani"
        width={1875}
        height={1970}
        priority
        sizes="(max-width: 1024px) 90vw, 520px"
        className="h-auto w-full"
      />
    </div>
  );
}
