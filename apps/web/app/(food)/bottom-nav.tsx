"use client";

import { Heart, House, ShoppingBag, User } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import QrScanButton from "./qr-scan-button";

// Maketdagi (image/restarant.png) pastki menyu: to'rt bo'lim va
// markazda ko'tarilgan QR tugmasi. Flutter tomonidagi `home_shell.dart`
// bilan bir xil tuzilma — mijoz ikkala ilovada bir xil joyni bosadi.

const TABS = [
  { href: "/", label: "Bosh sahifa", Icon: House },
  { href: "/orders", label: "Buyurtmalar", Icon: ShoppingBag },
  { href: "/favorites", label: "Sevimlilar", Icon: Heart },
  { href: "/profile", label: "Profil", Icon: User },
] as const;

// ┌─ MENYU FAQAT SHU TO'RT SAHIFADA ──────────────────────────────────┐
// Savat, checkout, menyu va buyurtma kuzatuvi sahifalarida pastda
// YOPISHIB TURADIGAN amal paneli bor ("Buyurtma berish", "Savatga",
// "Tayyor"). Menyu global bo'lsa, u aynan o'sha tugmalar ustiga
// tushib, ularni bosib bo'lmay qolardi.
//
// Shuning uchun ro'yxat ANIQ sanaladi, `startsWith` ishlatilmaydi:
// `/orders` menyuni ko'rsatadi, `/orders/<id>` (kuzatuv, o'z amal
// paneli bilan) esa ko'rsatmaydi.
// └───────────────────────────────────────────────────────────────────┘
const ROOTS: ReadonlySet<string> = new Set(TABS.map((t) => t.href));

export default function BottomNav() {
  const pathname = usePathname();
  if (!ROOTS.has(pathname)) return null;

  // ┌─ KATTA EKRANDA UMUMAN CHIZILMAYDI ────────────────────────────────┐
  // Bu menyu — MOBIL navigatsiya (o'rtasida QR skaner tugmasi bilan).
  // Kompyuterda uning o'rnini navbar bosadi: qidiruv, manzil, savat va
  // foydalanuvchi menyusi (buyurtmalar, sevimlilar, bildirishnomalar —
  // hammasi panel bo'lib ochiladi, `desktop-navbar.tsx`).
  //
  // QR skaner esa kompyuterda MA'NOSIZ: u stol ustidagi kodni telefon
  // kamerasi bilan o'qish uchun.
  //
  // Ilgari shart faqat bosh sahifaga tegishli edi (`pathname === "/"`),
  // chunki qolgan sahifalarda desktop navigatsiya yo'q edi. Endi
  // navbar hamma joyda mavjud.
  // └───────────────────────────────────────────────────────────────────┘
  return (
    <nav className="tg-surface safe-bottom fixed inset-x-0 bottom-0 z-40 border-t border-neutral-200 bg-white dark:border-neutral-800 dark:bg-[#1A1A1A] md:hidden">
      {/* `grid-cols-5` — o'rtadagi katak QR tugmasi uchun. Tab'lar
          tartibi maketdagidek: Bosh sahifa, Buyurtmalar, [QR],
          Sevimlilar, Profil. */}
      <div className="mx-auto grid max-w-2xl grid-cols-5 items-center px-1 pt-1">
        {TABS.slice(0, 2).map((t) => (
          <NavItem key={t.href} {...t} active={pathname === t.href} />
        ))}

        <div className="flex justify-center">
          <QrScanButton />
        </div>

        {TABS.slice(2).map((t) => (
          <NavItem key={t.href} {...t} active={pathname === t.href} />
        ))}
      </div>
    </nav>
  );
}

function NavItem({
  href,
  label,
  Icon,
  active,
}: {
  href: string;
  label: string;
  Icon: typeof House;
  active: boolean;
}) {
  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      className="flex flex-col items-center gap-0.5 py-1.5"
    >
      <Icon
        size={21}
        className={active ? "text-brand" : "text-neutral-400"}
        // Faol bo'limning ikoni to'ldirilgan — maketda ham shunday va
        // rang ko'rmaydiganlar uchun rangdan boshqa belgi qoladi.
        fill={active ? "currentColor" : "none"}
        strokeWidth={active ? 1.5 : 2}
      />
      <span
        className={`text-[10px] font-medium leading-none ${
          active ? "text-brand" : "text-neutral-500"
        }`}
      >
        {label}
      </span>
    </Link>
  );
}
