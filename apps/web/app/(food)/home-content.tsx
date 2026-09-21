"use client";

import { ChevronDown, ChevronRight, MapPin, Search, UtensilsCrossed, X } from "lucide-react";
import { useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import CategoryTile from "./category-tile";
import DesktopHome from "./desktop-home";
import HeaderActions from "./header-actions";
import RestaurantCard from "./restaurant-card";
import { categoryIconFor } from "@/lib/categoryIcons";
import type { Restaurant } from "@/lib/types";
import { useAddressLabel } from "@/lib/use-address-label";
import MobileSheet from "./mobile-sheet";

export default function HomeContent({
  restaurants,
  categories,
  signedIn,
}: {
  restaurants: Restaurant[];
  categories: string[];
  signedIn: boolean;
}) {
  const [query, setQuery] = useState("");
  const [searchOpen, setSearchOpen] = useState(false);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return restaurants;
    return restaurants.filter((r) => r.name.toLowerCase().includes(q));
  }, [restaurants, query]);

  return (
    <>
      {/* Kompyuter brauzeri — alohida, to'q fonli ko'rinish
          (`desktop-home.tsx`). Mobil/WebView/Telegram'da HECH QACHON
          render bo'lmaydi (`hidden` — CSS bilan yashirilgan bo'lsa ham
          DOM'da bor, lekin ichidagi effektlar/so'rovlar shu tufayli
          foydasiz ishlamasin desa React'ning shart operatoriga qarab
          quyida bo'lingan: ikkalasi ham DOM'da, faqat CSS ko'rsatadi —
          soddaligi uchun shunday, ikkalasi ham YENGIL). */}
      <div className="hidden md:block">
        <DesktopHome restaurants={restaurants} categories={categories} signedIn={signedIn} />
      </div>

      <div className="md:hidden">
        <MobileHome
          restaurants={restaurants}
          categories={categories}
          signedIn={signedIn}
          query={query}
          setQuery={setQuery}
          searchOpen={searchOpen}
          setSearchOpen={setSearchOpen}
          filtered={filtered}
        />
      </div>
    </>
  );
}

function MobileHome({
  restaurants,
  categories,
  signedIn,
  query,
  setQuery,
  searchOpen,
  setSearchOpen,
  filtered,
}: {
  restaurants: Restaurant[];
  categories: string[];
  signedIn: boolean;
  query: string;
  setQuery: (v: string) => void;
  searchOpen: boolean;
  setSearchOpen: (v: boolean) => void;
  filtered: Restaurant[];
}) {
  const router = useRouter();
  const [addressLabel] = useAddressLabel(signedIn);

  // ┌─ TAOMLAR — "desktop-navbar.tsx"dagi `submitSearch` bilan bir xil ─┐
  // Yuqoridagi `filtered` — LOKAL, tarmoqqa chiqmaydi (allaqachon
  // kelgan restoranlar ro'yxatini filtrlaydi). Taom nomi/turkumi
  // bo'yicha qidiruv esa backend'ga (`GET /products/search`, endi
  // MeiliSearch orqali xato-kechiruvchan) borishi kerak — `/search`
  // sahifasi buni allaqachon qiladi, faqat mobil ko'rinishda unga
  // o'tish tugmasi yo'q edi (faqat kompyuter navbar'ida bor edi).
  // └────────────────────────────────────────────────────────────────────┘
  function searchProducts() {
    const q = query.trim();
    if (!q) return;
    router.push(`/search?category=${encodeURIComponent(q)}`);
  }

  return (
    // `pb-28` — pastki menyu (`bottom-nav.tsx`) kontentning oxirini
    // bosib qolmasin.
    <MobileSheet className="px-4 pb-28">
      {/* ┌─ SARLAVHA ──────────────────────────────────────────────────┐
          Tartib: logotip -> "Super App" -> manzil. Manzil ATAYLAB eng
          pastda — u eng kam o'zgaradigan va eng kam bosiladigan element,
          shuning uchun brend yuqorida
          turadi va sahifa nomi bilan boshlanadi.

          Qidiruv chapdagi ustunning YONIDA, o'ngda: u bitta ikon, ya'ni
          butun qatorni egallamaydi va sarlavha balandligini oshirmaydi.
          └─────────────────────────────────────────────────────────────┘ */}
      <div className="flex items-start justify-between gap-3 pt-2">
        <div className="min-w-0">
          {/* Logotip matn bilan chizilgan — public/ ichida OnDex wordmark
              fayli yo'q. SVG/PNG berilsa shu blok bitta <Image> ga
              almashtiriladi. */}
          <p className="text-[34px] font-extrabold leading-none tracking-tight">
            On<span className="text-brand">Dex</span>
          </p>

          {/* "Super App" matni OLIB TASHLANDI — u hech qanday ma'lumot
              bermasdi va logotip ostidagi eng ko'zga tashlanadigan
              joyni egallab turardi. O'sha joyni manzil oldi: u
              bosiladigan va haqiqatan foydali element.

              Yorliq — saqlangan manzil (kompyuter navbar'i bilan bir xil
              manba, `lib/use-address-label.ts`). Avval bu yerda "Chust"
              qattiq yozilgan edi; Toshkent qo'shilgach u noto'g'ri
              bo'lib qolardi. */}
          <Link
            href="/address"
            className="mt-1.5 flex items-center gap-1.5 py-0.5 active:opacity-60"
          >
            <MapPin size={16} className="shrink-0" />
            <span className="truncate text-[14px] font-semibold">{addressLabel}</span>
            <ChevronDown size={16} className="shrink-0 text-neutral-500" />
          </Link>
        </div>

        {/* Qidiruv / hamyon / bildirishnoma — `header-actions.tsx`.
            Qidiruv IKONI (soxta "input" emas): bosilganda pastdagi
            to'liq ekranli oyna ochiladi, u yerda haqiqiy `input` va
            klaviatura darhol tayyor bo'ladi. Bosh sahifada bosib
            bo'lmaydigan input ko'rinishi joy egallardi va ikkita
            alohida qidiruv holati taassurotini berardi. */}
        <HeaderActions onOpenSearch={() => setSearchOpen(true)} />
      </div>

      {categories.length > 0 && (
        // `-mx-4 px-4` — gorizontal skroll ekran chetigacha borsin,
        // lekin birinchi element chekinishni saqlasin.
        <div className="no-scrollbar -mx-4 mt-4 flex gap-2.5 overflow-x-auto px-4 pb-1">
          {categories.map((c) => (
            <CategoryTile key={c} label={c} iconSrc={categoryIconFor(c)} />
          ))}
        </div>
      )}

      <div className="mt-5 flex flex-col gap-4">
        {restaurants.length === 0 ? (
          <p className="py-10 text-center text-neutral-500">
            Hozircha restoran yo&apos;q
          </p>
        ) : (
          restaurants.map((r) => <RestaurantCard key={r.id} restaurant={r} />)
        )}
      </div>

      {searchOpen && (
        <div className="fixed inset-0 z-50 flex flex-col bg-neutral-200 pt-1 dark:bg-[#121212]">
          {/* Menyu sahifasidagi qidiruv oynasi bilan AYNAN bir xil. */}
          <div className="mx-auto flex min-h-0 w-full max-w-2xl flex-1 flex-col overflow-hidden rounded-t-[20px] bg-white dark:bg-[#1A1A1A]">
            <form
              onSubmit={(e) => {
                e.preventDefault();
                searchProducts();
              }}
              className="safe-top flex items-center gap-2 px-4 pb-3"
            >
              <div className="flex flex-1 items-center gap-2 rounded-xl border border-neutral-200 bg-neutral-50 px-3 dark:border-neutral-700 dark:bg-neutral-800">
                <Search size={20} className="shrink-0 text-neutral-400" />
                {/* eslint-disable-next-line jsx-a11y/no-autofocus -- qidiruv oynasi ATAYLAB ochilganda klaviatura darhol tayyor bo'lishi kerak */}
                <input
                  autoFocus
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder="Restoran yoki taom qidirish..."
                  className="w-full bg-transparent py-2.5 text-base outline-none"
                />
              </div>
              <button
                type="button"
                onClick={() => {
                  setSearchOpen(false);
                  setQuery("");
                }}
                className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-neutral-600 active:bg-neutral-200 dark:text-neutral-300 dark:active:bg-neutral-700"
                aria-label="Yopish"
              >
                <X size={22} />
              </button>
            </form>
            <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-4 py-5">
              {query.trim() ? (
                <>
                  {/* Restoran nomi mos kelmasa ham, xuddi shu matn taom
                      nomi bo'lishi mumkin ("mohito" — restoran emas,
                      ichimlik) — shuning uchun qator har doim ko'rinadi. */}
                  <button
                    type="button"
                    onClick={searchProducts}
                    className="flex items-center gap-3 rounded-2xl bg-neutral-100 px-3.5 py-3.5 text-left dark:bg-neutral-800"
                  >
                    <UtensilsCrossed
                      size={18}
                      className="shrink-0 text-brand"
                    />
                    <span className="min-w-0 flex-1 truncate text-sm">
                      <span className="font-bold">
                        &quot;{query.trim()}&quot;
                      </span>{" "}
                      bo&apos;yicha taomlarni qidirish
                    </span>
                    <ChevronRight
                      size={18}
                      className="shrink-0 text-neutral-400"
                    />
                  </button>
                  {filtered.length === 0 ? (
                    <p className="py-6 text-center text-neutral-500">
                      Mos restoran topilmadi
                    </p>
                  ) : (
                    filtered.map((r) => (
                      <RestaurantCard key={r.id} restaurant={r} />
                    ))
                  )}
                </>
              ) : (
                <p className="py-10 text-center text-neutral-500">
                  Restoran nomini yozing
                </p>
              )}
            </div>
          </div>
        </div>
      )}
    </MobileSheet>
  );
}
