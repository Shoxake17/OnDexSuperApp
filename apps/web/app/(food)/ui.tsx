"use client";

import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

// ─────────────────────────────────────────────────────────────────────
// Umumiy UI komponentlari — takrorlanadigan elementlar FAQAT shu yerda
// yoziladi, qolgan sahifalar shu yerdan import qiladi. Sabab: avval
// har bir sahifa o'z tugmasini alohida yozgani uchun balandliklar
// har xil bo'lib ketgan edi (h-11 / h-12 / py-3.5 / py-3 ...).
// ─────────────────────────────────────────────────────────────────────

/// Barcha asosiy tugmalar uchun YAGONA balandlik. Qiymat mahsulot
/// tafsiloti oynasidagi "Qo'shish" tugmasidan olingan (py-3.5 + 24px
/// matn = 52px) — foydalanuvchi aynan shuni etalon qilib ko'rsatgan.
const BUTTON_HEIGHT = "h-[52px]";

type Variant = "primary" | "outline" | "danger" | "ghost";

const VARIANTS: Record<Variant, string> = {
  primary: "bg-[#FFD100] text-black",
  outline:
    "border border-neutral-300 text-current dark:border-neutral-700",
  danger: "bg-red-600 text-white",
  ghost: "text-current",
};

function buttonClass(variant: Variant, spread: boolean, extra: string) {
  return [
    BUTTON_HEIGHT,
    "flex w-full items-center rounded-2xl px-4 text-base font-bold",
    "transition-transform active:scale-[0.99] disabled:opacity-50",
    spread ? "justify-between" : "justify-center",
    VARIANTS[variant],
    extra,
  ].join(" ");
}

type CommonProps = {
  children: ReactNode;
  variant?: Variant;
  /// true — kontent chetlarga taqsimlanadi (masalan "Buyurtma berish" +
  /// o'ng tomonda narx). Standart: markazda.
  spread?: boolean;
  className?: string;
};

export function AppButton({
  children,
  variant = "primary",
  spread = false,
  className = "",
  ...rest
}: CommonProps & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button className={buttonClass(variant, spread, className)} {...rest}>
      {children}
    </button>
  );
}

export function AppButtonLink({
  children,
  href,
  variant = "primary",
  spread = false,
  className = "",
}: CommonProps & { href: string }) {
  return (
    <Link href={href} className={buttonClass(variant, spread, className)}>
      {children}
    </Link>
  );
}

/// Barcha sahifalardagi "orqaga" strelkasi — bir xil o'lcham va
/// teginish maydoni. Har bir sahifa o'zi chizmaydi.
export function BackButton({
  onClick,
  className = "",
}: {
  onClick: () => void;
  className?: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label="Orqaga"
      className={`-ml-1.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-neutral-600 active:bg-neutral-200 dark:text-neutral-300 dark:active:bg-neutral-700 ${className}`}
    >
      <ArrowLeft size={26} />
    </button>
  );
}

/// Pastdan chiquvchi oynalarning yuqorisidagi "tortish" chizig'i
/// (Yandex Eats naqshi) — X tugmasi o'rniga.
///
/// `overlay` — chiziq alohida qatorda emas, ostidagi kontent (masalan
/// mahsulot rasmi) USTIDA suzib turadi. Shunda tepada ortiqcha qora
/// yo'lak paydo bo'lmaydi.
export function SheetHandle({ overlay = false }: { overlay?: boolean }) {
  if (overlay) {
    return (
      <div className="pointer-events-none absolute inset-x-0 top-0 z-10 flex justify-center pt-2.5">
        <div className="h-1 w-10 rounded-full bg-black/30 shadow-sm" />
      </div>
    );
  }
  return (
    <div className="flex justify-center py-2.5">
      <div className="h-1 w-10 rounded-full bg-neutral-400/70 dark:bg-neutral-500/70" />
    </div>
  );
}
