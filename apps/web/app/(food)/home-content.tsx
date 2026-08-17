"use client";

import { ChevronDown, MapPin, Search, X } from "lucide-react";
import { useMemo, useState } from "react";
import Link from "next/link";
import CategoryTile from "./category-tile";
import HeaderActions from "./header-actions";
import RestaurantCard from "./restaurant-card";
import { categoryIconFor } from "@/lib/categoryIcons";
import type { Restaurant } from "@/lib/types";
import MobileSheet from "./mobile-sheet";

export default function HomeContent({
  restaurants,
  categories,
}: {
  restaurants: Restaurant[];
  categories: string[];
}) {
  const [query, setQuery] = useState("");
  const [searchOpen, setSearchOpen] = useState(false);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return restaurants;
    return restaurants.filter((r) => r.name.toLowerCase().includes(q));
  }, [restaurants, query]);

  return (
    // `pb-28` — pastki menyu (`bottom-nav.tsx`) kontentning oxirini
    // bosib qolmasin.
    <MobileSheet className="px-4 pb-28">
      {/* ┌─ SARLAVHA ──────────────────────────────────────────────────┐
          Tartib: logotip -> "Super App" -> manzil. Manzil ATAYLAB eng
          pastda — u eng kam o'zgaradigan va eng kam bosiladigan element
          (platforma faqat Chust uchun), shuning uchun brend yuqorida
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

              Maketda "Toshkent, Chilonzor" turibdi — u shunchaki namuna.
              Platforma Chust uchun, shuning uchun shahar nomi qat'iy. */}
          <Link
            href="/address"
            className="mt-1.5 flex items-center gap-1.5 py-0.5 active:opacity-60"
          >
            <MapPin size={16} className="shrink-0" />
            <span className="truncate text-[14px] font-semibold">Chust</span>
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
            <div className="safe-top flex items-center gap-2 px-4 pb-3">
              <div className="flex flex-1 items-center gap-2 rounded-xl border border-neutral-200 bg-neutral-50 px-3 dark:border-neutral-700 dark:bg-neutral-800">
                <Search size={20} className="shrink-0 text-neutral-400" />
                {/* eslint-disable-next-line jsx-a11y/no-autofocus -- qidiruv oynasi ATAYLAB ochilganda klaviatura darhol tayyor bo'lishi kerak */}
                <input
                  autoFocus
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder="Restoran qidirish..."
                  className="w-full bg-transparent py-2.5 text-base outline-none"
                />
              </div>
              <button
                onClick={() => {
                  setSearchOpen(false);
                  setQuery("");
                }}
                className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-neutral-600 active:bg-neutral-200 dark:text-neutral-300 dark:active:bg-neutral-700"
                aria-label="Yopish"
              >
                <X size={22} />
              </button>
            </div>
            <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-4 py-5">
              {filtered.length === 0 ? (
                <p className="py-10 text-center text-neutral-500">
                  {query ? "Mos restoran topilmadi" : "Restoran nomini yozing"}
                </p>
              ) : (
                filtered.map((r) => <RestaurantCard key={r.id} restaurant={r} />)
              )}
            </div>
          </div>
        </div>
      )}
    </MobileSheet>
  );
}
