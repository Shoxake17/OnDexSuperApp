import type { ReactNode } from "react";

// Yandex Eats uslubidagi "karta" (sheet): MOBILDA karta ekranga QOTIRILGAN
// (h-dvh + flex) — skroll qilinganda kartaning o'zi va uning 20px dumaloq
// yuqori burchagi JOYIDA QOLADI, faqat ICHIDAGI kontent siljiydi. Avval
// butun sahifa skroll bo'lardi va karta tepasi ekrandan chiqib ketardi.
//
// Ranglar Flutter'ning `scaffoldBackgroundColor` (#121212) bilan atayin
// bir xil — WebView SafeArea ichida, tepasida/pastida Flutter foni
// ko'rinadi, chok sezilmasligi kerak.
//
// `md:` (768px+) — desktop/tablet uchun HAMMASI bekor qilinadi: oddiy
// sahifa skrolli, dumaloq burchaksiz, mustaqil veb-sayt ko'rinishi.
export default function MobileSheet({
  children,
  maxWidthClassName = "max-w-2xl",
  className = "",
}: {
  children: ReactNode;
  maxWidthClassName?: string;
  className?: string;
}) {
  return (
    <div className="sheet-top flex h-dvh flex-col bg-neutral-200 dark:bg-[#121212] md:block md:h-auto md:min-h-screen md:bg-white md:pt-0 md:dark:bg-[#121212]">
      <main
        className={`mx-auto flex w-full min-h-0 flex-1 flex-col overflow-hidden rounded-t-[20px] bg-white dark:bg-[#1A1A1A] md:block md:rounded-none md:dark:bg-[#121212] ${maxWidthClassName}`}
      >
        <div className={`min-h-0 flex-1 overflow-y-auto md:overflow-visible ${className}`}>
          {children}
        </div>
      </main>
    </div>
  );
}
