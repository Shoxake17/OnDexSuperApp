"use client";

import { Search, X } from "lucide-react";
import { useMemo, useState } from "react";
import Image from "next/image";
import CategoryTile from "./category-tile";
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
    <MobileSheet className="px-4 pb-8">
      <div className="flex items-center pt-1.5">
        <div className="w-8" />
        <div className="flex flex-1 justify-center">
          <Image src="/eltago.png" alt="ChustApp" width={104} height={30} priority />
        </div>
        <button
          onClick={() => setSearchOpen(true)}
          className="flex h-8 w-8 items-center justify-center rounded-full text-neutral-500 hover:bg-neutral-100 dark:hover:bg-neutral-800"
          aria-label="Qidirish"
        >
          <Search size={20} />
        </button>
      </div>

      {categories.length > 0 && (
        <div className="mt-2 flex gap-1.5 overflow-x-auto">
          {categories.map((c) => (
            <CategoryTile key={c} label={c} iconSrc={categoryIconFor(c)} />
          ))}
        </div>
      )}

      <div className="mt-3 flex flex-col gap-4">
        {restaurants.length === 0 ? (
          <p className="py-10 text-center text-neutral-500">Hozircha restoran yo&apos;q</p>
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
            <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-4 py-5">
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
