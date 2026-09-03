"use client";

import { UtensilsCrossed } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import type { ProductSearchResult } from "@/lib/types";
import MobileSheet from "../mobile-sheet";
import { AppButtonLink } from "../ui";

// "Sevimlilar" — pastki menyudagi to'rtinchi bo'lim.
//
// `GET /favorites` `/products/search` bilan AYNAN bir xil shaklda
// (`ProductSearchResult`) qaytaradi — shuning uchun bu yerdagi to'r
// qidiruv sahifasidagi bilan bir xil ko'rinadi va mijoz ikkala joyda
// bir xil kartochkani ko'radi.

export default function FavoritesPage() {
  const [items, setItems] = useState<ProductSearchResult[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const res = await fetch("/api/proxy/favorites");
        if (!res.ok) {
          setFailed(true);
          return;
        }
        setItems((await res.json()) ?? []);
      } catch {
        setFailed(true);
      }
    })();
  }, []);

  return (
    <MobileSheet className="px-4 pb-28 pt-3">
      <h1 className="text-2xl font-bold">Sevimlilar</h1>

      {failed ? (
        <div className="py-14 text-center">
          <p className="text-neutral-500">
            Sevimlilarni yuklab bo&apos;lmadi.
          </p>
          <button
            onClick={() => window.location.reload()}
            className="mt-3 text-sm font-semibold text-brand"
          >
            Qaytadan urinish
          </button>
        </div>
      ) : items === null ? (
        <div className="mt-4 grid grid-cols-2 gap-x-3.5 gap-y-5">
          {[0, 1, 2, 3].map((i) => (
            <div
              key={i}
              className="aspect-square animate-pulse rounded-2xl bg-neutral-100 dark:bg-neutral-800"
            />
          ))}
        </div>
      ) : items.length === 0 ? (
        <div className="py-14 text-center">
          <p className="text-neutral-500">
            Hozircha sevimli taom yo&apos;q
          </p>
          <p className="tg-muted mt-1 text-sm text-neutral-500">
            Taom sahifasidagi ♡ belgisini bosib qo&apos;shing.
          </p>
          <div className="mx-auto mt-5 max-w-xs">
            <AppButtonLink href="/">Restoranlarni ko&apos;rish</AppButtonLink>
          </div>
        </div>
      ) : (
        <div className="mt-4 grid grid-cols-2 gap-x-3.5 gap-y-5">
          {items.map((p) => (
            <Link
              key={p.id}
              href={`/restaurants/${p.restaurant_id}`}
              className={
                p.restaurant_open ? "" : "pointer-events-none opacity-40"
              }
            >
              <div className="aspect-square w-full overflow-hidden rounded-2xl bg-neutral-100 dark:bg-neutral-800">
                {p.image_url ? (
                  // eslint-disable-next-line @next/next/no-img-element -- rasm
                  // manzili muhitga qarab dinamik (R2/lokal disk).
                  <img
                    src={fullImageUrl(p.image_url)}
                    alt={p.name}
                    className="h-full w-full object-cover"
                  />
                ) : (
                  <div className="flex h-full items-center justify-center text-neutral-400">
                    <UtensilsCrossed size={28} />
                  </div>
                )}
              </div>
              <p className="mt-2 font-bold">{formatSum(p.price_tiyin)}</p>
              <p className="mt-0.5 line-clamp-2 text-sm">{p.name}</p>
              <p className="tg-muted mt-0.5 line-clamp-1 text-xs text-neutral-500">
                {p.restaurant_open
                  ? p.restaurant_name
                  : `${p.restaurant_name} (yopiq)`}
              </p>
            </Link>
          ))}
        </div>
      )}
    </MobileSheet>
  );
}
