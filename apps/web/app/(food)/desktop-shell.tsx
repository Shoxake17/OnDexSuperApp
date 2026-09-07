"use client";

import { ArrowLeft } from "lucide-react";
import { useRouter } from "next/navigation";
import DesktopNavbar from "./desktop-navbar";

// Akkaunt sahifalarining (buyurtmalar, sevimlilar, bildirishnomalar)
// KOMPYUTER qobig'i.
//
// ┌─ NEGA UMUMIY QOBIQ ────────────────────────────────────────────────┐
// Uchala sahifa ham bir xil tuzilishga ega: navbar, orqaga tugmasi,
// sarlavha va bitta chegaralangan blok. Har birida qaytadan yozilsa,
// keyingi har bir tuzatish (masalan navbar balandligi yoki blok
// radiusi) uch joyda qilinishi kerak bo'lardi — restoran menyusi bilan
// navbar orasida bu xato allaqachon bir marta yuz bergan.
// └────────────────────────────────────────────────────────────────────┘
//
// Sahifaning O'ZI skroll bo'lmaydi — skroll blok ichida
// (`.ondex-scroll`, `globals.css`). Shu tufayli navbar doim joyida
// qoladi va brauzerning qo'pol standart polosasi chetda chiqmaydi.
export default function DesktopShell({
  title,
  subtitle,
  signedIn,
  /** Blok kengligi — ro'yxatlar tor, to'rlar kengroq bo'ladi. */
  maxWidthClassName = "max-w-[900px]",
  children,
}: {
  title: string;
  subtitle?: string;
  signedIn: boolean;
  maxWidthClassName?: string;
  children: React.ReactNode;
}) {
  const router = useRouter();

  return (
    <div className="flex h-dvh flex-col overflow-hidden bg-[#302F2D] text-white">
      <DesktopNavbar signedIn={signedIn} />

      <div className="mx-auto flex w-full min-h-0 max-w-[1800px] flex-1 gap-4 px-6 py-5 xl:px-10 2xl:px-14">
        {/* Orqaga — blokdan TASHQARIDA (restoran menyusidagi bilan bir
            xil naqsh): u kontentga emas, undan chiqishga tegishli va
            skroll bilan yo'qolmaydi.

            `router.push("/")` — `router.back()` EMAS: tarixdagi oldingi
            yozuv begona sayt bo'lishi mumkin (havola orqali kirilgan
            bo'lsa) va "orqaga" mijozni OnDex'dan chiqarib yuborardi. */}
        <button
          type="button"
          onClick={() => router.push("/")}
          aria-label="Bosh sahifaga qaytish"
          className="mt-1 flex h-11 w-11 shrink-0 items-center justify-center self-start rounded-full border border-white/10 bg-[#141414] text-white transition-colors hover:bg-[#1f1f1f]"
        >
          <ArrowLeft size={20} />
        </button>

        <div className={`mx-auto flex min-h-0 w-full flex-col ${maxWidthClassName}`}>
          <div className="shrink-0 rounded-3xl border border-white/10 bg-[#141414] px-6 py-5">
            <h1 className="text-[26px] font-extrabold leading-none">{title}</h1>
            {subtitle && (
              <p className="mt-2 text-[14px] text-white/45">{subtitle}</p>
            )}
          </div>

          <div className="ondex-scroll mt-4 min-h-0 flex-1 overflow-y-auto overscroll-contain rounded-3xl border border-white/10 bg-[#141414]">
            {children}
          </div>
        </div>
      </div>
    </div>
  );
}

/** Ro'yxat bo'sh yoki yuklanmagan holatlar uchun umumiy ko'rinish. */
export function DesktopEmpty({
  image,
  title,
  text,
  action,
}: {
  /** `public/` ichidagi illyustratsiya (ixtiyoriy). */
  image?: string;
  title: string;
  text?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex h-full flex-col items-center justify-center px-8 py-16 text-center">
      {image && (
        // eslint-disable-next-line @next/next/no-img-element -- statik fayl
        <img src={image} alt="" aria-hidden="true" className="w-[180px] object-contain" />
      )}
      <p className="mt-5 text-[18px] font-bold">{title}</p>
      {text && (
        <p className="mt-1.5 max-w-[340px] text-[14px] leading-relaxed text-white/45">
          {text}
        </p>
      )}
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}
