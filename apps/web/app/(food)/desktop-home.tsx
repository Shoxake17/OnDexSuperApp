"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import DesktopNavbar from "./desktop-navbar";
import DesktopRestaurantCard from "./desktop-restaurant-card";
import type { Restaurant } from "@/lib/types";

// Desktop uchun alohida bosh sahifa — image/eats.png (Yandex Eats
// desktop varianti) uslubida: to'q fonli navbar (qidiruv + manzil +
// Kirish), turkum yorliqlari, restoranlar to'ri.
//
// ┌─ NEGA MOBIL BILAN BIR XIL FAYLDA EMAS ─────────────────────────────┐
// Mobil ko'rinish (`home-content.tsx`) — Flutter WebView va Telegram
// Mini App ICHIDA ham ishlatiladi, ular esa `tailwind.config.ts`dagi
// "faqat yorug' mavzu" qaroriga BOG'LIQ (WebView'dagi chok — seam —
// muammosi sabab, `globals.css`dagi izohga qarang). Bu sahifa FAQAT
// `md:` va undan katta ekranda ko'rinadi (`hidden md:block`
// chaqiruvchida) — ya'ni WebView/TMA'da HECH QACHON render bo'lmaydi,
// shuning uchun bu yerdagi to'q fon o'sha qarorga zid emas: u shunchaki
// mustaqil veb-sayt (kompyuter brauzeri) uchun alohida ko'rinish.
// └───────────────────────────────────────────────────────────────────┘
//
// ┌─ NEGA PROMO BANNER/DO'KONLAR YO'Q ─────────────────────────────────┐
// Maketda bor, lekin ularga mos HAQIQIY ma'lumot yo'q (chegirma
// kampaniyasi, alohida "do'kon" katalogi — hozircha mavjud emas).
// Yo'q narsani ko'rsatish yolg'on va'da berardi, shuning uchun uslub
// (to'q fon, dumaloq elementlar, kartalar to'ri) olinadi, KONTENT esa
// haqiqatan bor narsalar bilan — turkumlar va restoranlar.
// └───────────────────────────────────────────────────────────────────┘
//
// Kartalar — `desktop-restaurant-card.tsx` (mobil kartadan boshqacha
// tuzilish: matn rasm USTIDA emas, OSTIDA; sababi o'sha fayldagi
// izohda).
export default function DesktopHome({
  restaurants,
  categories,
  signedIn,
}: {
  restaurants: Restaurant[];
  categories: string[];
  signedIn: boolean;
}) {
  const [activeCategory, setActiveCategory] = useState<string | null>(null);

  const shown = activeCategory
    ? restaurants.filter((r) =>
        r.tags.toLowerCase().includes(activeCategory.toLowerCase()),
      )
    : restaurants;

  return (
    <div className="min-h-dvh bg-[#141414] text-white">
      {/* Navbar ikkala desktop sahifa uchun UMUMIY —
          `desktop-navbar.tsx` (menyu sahifasi ham shuni ishlatadi). */}
      <DesktopNavbar signedIn={signedIn} />

      {/* Turkumlar va restoranlar (body) — chetlardan kengroq chekinish
          bilan: `px-6` da kartalar ekran qirg'og'iga yopishib turardi.
          Ekran kattalashgani sayin chekinish ham o'sadi, lekin kontent
          `max-w` bilan cheklangani uchun cheksiz yoyilib ketmaydi.
          Navbar chekinishi ataylab o'zgarishsiz (yuqoridagi izoh) —
          namunadagidek logotip chetga yaqinroq turadi. */}
      <div className="mx-auto max-w-[1600px] px-12 py-8 xl:px-20 2xl:px-24">
        {/* ── Turkumlar ─────────────────────────────────────────────── */}
        {categories.length > 0 && (
          <CategoryRow
            categories={categories}
            active={activeCategory}
            onSelect={setActiveCategory}
          />
        )}

        {/* ── Restoranlar ───────────────────────────────────────────── */}
        <h2 className="mb-4 mt-6 text-xl font-bold">Restoranlar</h2>
        {shown.length === 0 ? (
          <p className="py-16 text-center text-white/50">
            {activeCategory ? "Bu turkumda restoran topilmadi" : "Hozircha restoran yo'q"}
          </p>
        ) : (
          // ┌─ KARTA BALANDLIGI = KARTA KENGLIGI ─────────────────────┐
          // Cover'lar banner shaklida (~2:1 yoki kengroq), ya'ni
          // balandlik kenglikdan KELIB CHIQADI. Uni sun'iy oshirib
          // bo'lmaydi: `min-h` — tepa/pastda yo'lak beradi,
          // `object-cover` — rasmni kesadi (ikkalasi ham sinab
          // ko'rilgan va rad etilgan, `desktop-restaurant-card.tsx`).
          //
          // Shuning uchun balandlik IKKI richag bilan boshqariladi:
          // ustunlar soni va konteyner kengligi. Bir qatorda 4 ta
          // karta QOLDI (shu ko'rinish ma'qul topilgan), balandlik
          // esa konteynerni kengaytirish hisobiga oshdi:
          //   1280px / 4 ustun -> karta ~293px
          //   1600px / 4 ustun -> karta ~373px  (~27% balandroq)
          // └─────────────────────────────────────────────────────────┘
          <div className="grid grid-cols-2 gap-x-5 gap-y-7 lg:grid-cols-3 xl:grid-cols-4">
            {shown.map((r) => (
              <DesktopRestaurantCard key={r.id} restaurant={r} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

/**
 * Turkumlar qatori — surish tugmalari bilan.
 *
 * ┌─ NEGA TUGMA KERAK (topilgan nosozlik) ─────────────────────────────┐
 * Qator `overflow-x-auto` edi, lekin `.no-scrollbar` polosani
 * YASHIRADI (u mobil uchun qo'yilgan: WebView'da doimiy kulrang chiziq
 * chiziladi). Kompyuterda esa natija — turkumlar o'ngga chiqib ketadi,
 * lekin ularni surishning HECH QANDAY yo'li ko'rinmaydi: polosa yo'q,
 * barmoq bilan surish yo'q, sichqoncha g'ildiragi esa vertikal
 * skrollni boshqaradi. Ya'ni ro'yxatning yarmi amalda YETIB
 * BO'LMAYDIGAN bo'lib qolgan edi.
 *
 * Yechim — namunadagi (image/eats.png) kabi aylana tugmalar. Ular
 * FAQAT kerak bo'lganda ko'rinadi: chapga surish mumkin bo'lsa — chap
 * tugma, o'ngga mumkin bo'lsa — o'ng tugma. Chetlarda esa fon rangiga
 * tutashib ketuvchi yumshoq soya: qator "kesilgan" emas, DAVOM etgan
 * ko'rinadi.
 * └────────────────────────────────────────────────────────────────────┘
 */
function CategoryRow({
  categories,
  active,
  onSelect,
}: {
  categories: string[];
  active: string | null;
  onSelect: (c: string | null) => void;
}) {
  const trackRef = useRef<HTMLDivElement>(null);
  const [canLeft, setCanLeft] = useState(false);
  const [canRight, setCanRight] = useState(false);

  const sync = useCallback(() => {
    const el = trackRef.current;
    if (!el) return;
    // 2px — yaxlitlash xatosiga bardosh (`scrollWidth` kasr bo'lishi
    // mumkin), busiz o'ng tugma oxirigacha surilganda ham "yoniq"
    // qolardi.
    setCanLeft(el.scrollLeft > 2);
    setCanRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 2);
  }, []);

  useEffect(() => {
    const el = trackRef.current;
    if (!el) return;
    sync();
    el.addEventListener("scroll", sync, { passive: true });
    window.addEventListener("resize", sync);
    return () => {
      el.removeEventListener("scroll", sync);
      window.removeEventListener("resize", sync);
    };
  }, [sync, categories.length]);

  function nudge(direction: 1 | -1) {
    const el = trackRef.current;
    if (!el) return;
    // Ko'rinib turgan kenglikning ~80% i — foydalanuvchi qayerda
    // qolganini yo'qotmasligi uchun bir necha element ustma-ust qoladi.
    el.scrollBy({ left: direction * el.clientWidth * 0.8, behavior: "smooth" });
  }

  return (
    <div className="relative">
      <div ref={trackRef} className="no-scrollbar flex gap-2.5 overflow-x-auto pb-1">
        <Pill active={active === null} onClick={() => onSelect(null)}>
          Hammasi
        </Pill>
        {categories.map((c) => (
          <Pill key={c} active={active === c} onClick={() => onSelect(c)}>
            {c}
          </Pill>
        ))}
      </div>

      {canLeft && (
        <>
          <div className="pointer-events-none absolute inset-y-0 left-0 w-16 bg-gradient-to-r from-[#141414] to-transparent" />
          <ScrollButton side="left" onClick={() => nudge(-1)} />
        </>
      )}
      {canRight && (
        <>
          <div className="pointer-events-none absolute inset-y-0 right-0 w-16 bg-gradient-to-l from-[#141414] to-transparent" />
          <ScrollButton side="right" onClick={() => nudge(1)} />
        </>
      )}
    </div>
  );
}

function ScrollButton({
  side,
  onClick,
}: {
  side: "left" | "right";
  onClick: () => void;
}) {
  const Icon = side === "left" ? ChevronLeft : ChevronRight;
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={side === "left" ? "Chapga surish" : "O'ngga surish"}
      className={`absolute top-1/2 flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-full bg-[#302F2D] text-white shadow-lg ring-1 ring-white/10 transition-colors hover:bg-[#3b3a37] ${
        side === "left" ? "left-0" : "right-0"
      }`}
    >
      <Icon size={18} />
    </button>
  );
}

function Pill({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`shrink-0 rounded-full px-4 py-2 text-sm font-semibold transition-colors ${
        active ? "bg-brand text-white" : "bg-white/10 text-white/80 hover:bg-white/[0.16]"
      }`}
    >
      {children}
    </button>
  );
}
