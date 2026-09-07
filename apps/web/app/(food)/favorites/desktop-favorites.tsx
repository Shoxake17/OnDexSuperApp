"use client";

import { UtensilsCrossed } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import type { ProductSearchResult } from "@/lib/types";
import DesktopShell, { DesktopEmpty } from "../desktop-shell";

// "Sevimlilar" — kompyuter ko'rinishi.
//
// Mobil variant ikki ustunli to'r (telefon kengligi shuncha imkon
// beradi); kompyuterda to'rt ustun sig'adi va kartalar kattaroq
// bo'lishi mumkin. Ma'lumot bir xil: `GET /favorites`.
export default function DesktopFavorites({ signedIn }: { signedIn: boolean }) {
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
    <DesktopShell
      title="Sevimlilar"
      subtitle={items && items.length > 0 ? `${items.length} ta taom` : undefined}
      signedIn={signedIn}
      maxWidthClassName="max-w-[1100px]"
    >
      {failed ? (
        <DesktopEmpty
          title="Sevimlilarni yuklab bo'lmadi"
          text="Internet aloqasini tekshirib, sahifani yangilang."
          action={
            <button
              type="button"
              onClick={() => window.location.reload()}
              className="rounded-xl bg-white/10 px-4 py-2.5 text-sm font-semibold hover:bg-white/[0.16]"
            >
              Qaytadan urinish
            </button>
          }
        />
      ) : items === null ? (
        <div className="grid grid-cols-2 gap-5 p-5 lg:grid-cols-3 2xl:grid-cols-4">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="aspect-square animate-pulse rounded-2xl bg-white/5" />
          ))}
        </div>
      ) : items.length === 0 ? (
        <DesktopEmpty
          title="Sevimlilar bo'sh"
          text="Menyudagi taom kartasidagi yurakni bosing — u shu yerga tushadi."
          action={
            <Link
              href="/"
              className="inline-flex rounded-xl bg-brand px-5 py-2.5 text-sm font-bold text-white hover:bg-brand-light"
            >
              Restoranlarni ko'rish
            </Link>
          }
        />
      ) : (
        <div className="grid grid-cols-2 gap-x-5 gap-y-7 p-5 lg:grid-cols-3 2xl:grid-cols-4">
          {items.map((p) => (
            <Link
              key={p.id}
              href={`/restaurants/${p.restaurant_id}`}
              // Yopiq restoran taomi ochilmaydi: menyuga kirib ham
              // buyurtma berib bo'lmaydi, ya'ni bosish bekor urinish
              // bo'lardi (mobil ro'yxatdagi bilan bir xil qoida).
              className={p.restaurant_open ? "group" : "pointer-events-none opacity-40"}
            >
              <div className="aspect-square w-full overflow-hidden rounded-2xl bg-white/5">
                {p.image_url ? (
                  // eslint-disable-next-line @next/next/no-img-element -- manzil dinamik
                  <img
                    src={fullImageUrl(p.image_url)}
                    alt={p.name}
                    className="h-full w-full object-cover transition-transform duration-200 group-hover:scale-[1.03]"
                  />
                ) : (
                  <div className="flex h-full w-full items-center justify-center text-white/20">
                    <UtensilsCrossed size={34} />
                  </div>
                )}
              </div>
              <p className="mt-2.5 text-[15px] font-bold">
                {formatSum(p.price_tiyin)}
              </p>
              <p className="mt-0.5 line-clamp-2 text-[13px] leading-snug text-white/60">
                {p.name}
              </p>
              <p className="mt-1 truncate text-[12px] text-white/35">
                {p.restaurant_open
                  ? p.restaurant_name
                  : `${p.restaurant_name} · yopiq`}
              </p>
            </Link>
          ))}
        </div>
      )}
    </DesktopShell>
  );
}
