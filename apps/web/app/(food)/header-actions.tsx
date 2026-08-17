"use client";

import { Bell, Search, Wallet } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

// Bosh sahifa sarlavhasidagi uchta amal (image/restarant.png):
// qidiruv, hamyon, bildirishnoma.
//
// ┌─ NEGA ALOHIDA KOMPONENT ───────────────────────────────────────────┐
// O'qilmagan bildirishnomalar soni SERVERDAN olinadi, ya'ni bu blok
// holatga ega bo'lishi kerak. `home-content.tsx` allaqachon qidiruv
// oynasi holatini ushlab turibdi — sarlavha ham qo'shilsa, bitta
// komponent ikkita bog'liqmas ishni bajarardi.
// └────────────────────────────────────────────────────────────────────┘

/** Badge'da ko'rsatiladigan eng katta son — undan yuqorisi "9+". */
const MAX_BADGE = 9;

export default function HeaderActions({
  onOpenSearch,
}: {
  onOpenSearch: () => void;
}) {
  const unread = useUnreadCount();

  return (
    <div className="mt-1 flex shrink-0 items-center gap-2">
      <IconButton label="Qidirish" onClick={onOpenSearch}>
        <Search size={20} />
      </IconButton>

      {/* Hamyon hali qurilmagan — sahifa buni ochiq aytadi
          (`/wallet`). Ikon maketda bor, shuning uchun joyi band
          qilinadi; mijoz bosganda bo'sh ekran emas, tushuntirish
          ko'radi. */}
      <IconButton label="Hamyon" href="/wallet">
        <Wallet size={20} />
      </IconButton>

      <IconButton label="Bildirishnomalar" href="/notifications">
        <Bell size={20} />
        {unread > 0 && (
          // Maketdagi qizil nuqta — lekin raqam bilan: "nechta?" degan
          // savol nuqtadan javob olmaydi va mijoz ilovani ochishga
          // qiziqmaydi.
          <span
            aria-hidden="true"
            className="absolute -right-0.5 -top-0.5 flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold leading-none text-white"
          >
            {unread > MAX_BADGE ? `${MAX_BADGE}+` : unread}
          </span>
        )}
      </IconButton>
    </div>
  );
}

/**
 * O'qilmagan bildirishnomalar soni.
 *
 * ┌─ XATO JIMGINA YUTILADI ──────────────────────────────────────────┐
 * Kirmagan foydalanuvchida bu endpoint 401 qaytaradi — bu NORMAL
 * holat, xato emas. Bosh sahifa anonim ham ochiladi va unda
 * shunchaki badge chizilmaydi.
 * └──────────────────────────────────────────────────────────────────┘
 */
function useUnreadCount(): number {
  const [count, setCount] = useState(0);

  useEffect(() => {
    // Komponent yo'q qilingandan keyin `setState` chaqirilmasin —
    // sahifa tez almashtirilganda React ogohlantirish berardi.
    let cancelled = false;

    async function load() {
      try {
        const res = await fetch("/api/proxy/notifications/unread-count");
        if (!res.ok) return;
        const d = (await res.json()) as { count?: number };
        if (!cancelled && typeof d.count === "number") setCount(d.count);
      } catch {
        // Tarmoq yo'q — badge eski qiymatda qoladi.
      }
    }

    void load();

    // Sahifa qayta ko'rinadigan bo'lganda yangilanadi. Interval
    // ATAYLAB ishlatilmadi: bosh sahifa uzoq ochiq turadi va har
    // necha soniyada so'rov yuborish batareyani bekorga yeydi.
    // Mijoz boshqa ilovadan qaytganda esa son yangi bo'lishi kerak.
    function onVisible() {
      if (document.visibilityState === "visible") void load();
    }
    document.addEventListener("visibilitychange", onVisible);

    return () => {
      cancelled = true;
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, []);

  return count;
}

/** Dumaloq ikon tugmasi — `href` berilsa havola, aks holda tugma. */
function IconButton({
  label,
  href,
  onClick,
  children,
}: {
  label: string;
  href?: string;
  onClick?: () => void;
  children: React.ReactNode;
}) {
  const className =
    "relative flex h-10 w-10 items-center justify-center rounded-full border border-neutral-200 bg-white text-neutral-600 active:bg-neutral-100";

  if (href) {
    return (
      <Link href={href} aria-label={label} className={className}>
        {children}
      </Link>
    );
  }
  return (
    <button type="button" onClick={onClick} aria-label={label} className={className}>
      {children}
    </button>
  );
}
