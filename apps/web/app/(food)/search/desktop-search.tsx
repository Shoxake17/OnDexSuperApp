"use client";

import { UtensilsCrossed } from "lucide-react";
import Link from "next/link";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import type { ProductSearchResult, Restaurant } from "@/lib/types";
import DesktopRestaurantCard from "../desktop-restaurant-card";
import DesktopShell, { DesktopEmpty } from "../desktop-shell";

// "Qidiruv" — kompyuter ko'rinishi.
//
// ┌─ NEGA (2026-09-15) ────────────────────────────────────────────────┐
// Navbar'dagi qidiruv `/search` ga olib borardi, sahifaning esa faqat
// MOBIL ko'rinishi bor edi: kompyuterda navbar yo'qolib, ekranda telefon
// kengligidagi oddiy ro'yxat qolardi va yangi so'z qidirish uchun bosh
// sahifaga qaytish kerak edi. Endi boshqa desktop sahifalar kabi
// `DesktopShell` ichida: navbar (so'rov maydonda turadi), mos restoranlar
// va taomlar to'ri.
// └────────────────────────────────────────────────────────────────────┘
//
// Ma'lumotni server sahifa oladi (`page.tsx`, SSR) — bu yer faqat chizadi.
export default function DesktopSearch({
  query,
  restaurants,
  products,
  failed,
  signedIn,
}: {
  query: string;
  restaurants: Restaurant[];
  products: ProductSearchResult[];
  /** Taomlarni yuklab bo'lmadi (restoranlar ro'yxati bunga bog'liq emas). */
  failed: boolean;
  signedIn: boolean;
}) {
  const counts = [
    restaurants.length > 0 ? `${restaurants.length} ta restoran` : null,
    products.length > 0 ? `${products.length} ta taom` : null,
  ].filter(Boolean);

  return (
    <DesktopShell
      title={query ? `«${query}»` : "Qidiruv"}
      subtitle={counts.length > 0 ? counts.join(" · ") : undefined}
      signedIn={signedIn}
      initialQuery={query}
      maxWidthClassName="max-w-[1400px]"
    >
      {!query ? (
        <DesktopEmpty
          title="Nima qidiramiz?"
          text="Yuqoridagi maydonga taom, turkum yoki restoran nomini yozing."
        />
      ) : failed && restaurants.length === 0 ? (
        // "Topilmadi" va "yuklab bo'lmadi" ATAYLAB ajratiladi (`page.tsx`
        // dagi izoh): vaqtinchalik nosozlik "bunday taom yo'q" deb
        // ko'rinmasligi kerak.
        <DesktopEmpty
          title="Natijalarni yuklab bo'lmadi"
          text="Internet aloqasini tekshirib, sahifani yangilang."
          action={<ReloadButton />}
        />
      ) : restaurants.length === 0 && products.length === 0 ? (
        <DesktopEmpty
          title="Hech narsa topilmadi"
          text={`«${query}» bo'yicha taom ham, restoran ham yo'q. Boshqa so'z bilan urinib ko'ring.`}
          action={
            <Link
              href="/"
              className="inline-flex rounded-xl bg-brand px-5 py-2.5 text-sm font-bold text-white hover:bg-brand-light"
            >
              Restoranlarni ko&apos;rish
            </Link>
          }
        />
      ) : (
        <div className="space-y-9 p-5">
          {restaurants.length > 0 && (
            <section>
              <SectionTitle count={restaurants.length}>Restoranlar</SectionTitle>
              <div className="grid grid-cols-2 gap-x-5 gap-y-7 lg:grid-cols-3 2xl:grid-cols-4">
                {restaurants.map((r) => (
                  <DesktopRestaurantCard key={r.id} restaurant={r} />
                ))}
              </div>
            </section>
          )}

          {failed ? (
            <div className="flex items-center justify-between gap-4 rounded-2xl bg-white/5 px-5 py-4">
              <p className="text-[14px] text-white/60">
                Taomlarni yuklab bo&apos;lmadi — internet aloqasini tekshiring.
              </p>
              <ReloadButton />
            </div>
          ) : (
            products.length > 0 && (
              <section>
                <SectionTitle count={products.length}>Taomlar</SectionTitle>
                <div className="grid grid-cols-2 gap-x-5 gap-y-7 lg:grid-cols-4 2xl:grid-cols-5">
                  {products.map((p) => (
                    <ProductCard key={p.id} product={p} />
                  ))}
                </div>
              </section>
            )
          )}
        </div>
      )}
    </DesktopShell>
  );
}

function SectionTitle({ count, children }: { count: number; children: React.ReactNode }) {
  return (
    <h2 className="mb-4 flex items-baseline gap-2 text-xl font-bold">
      {children}
      <span className="text-[14px] font-medium text-white/40">{count}</span>
    </h2>
  );
}

function ProductCard({ product: p }: { product: ProductSearchResult }) {
  return (
    <Link
      href={`/restaurants/${p.restaurant_id}`}
      // Yopiq restoran taomi ochilmaydi: menyuga kirib ham buyurtma berib
      // bo'lmaydi (mobil ro'yxat va sevimlilardagi bilan bir xil qoida).
      className={p.restaurant_open ? "group" : "pointer-events-none opacity-40"}
    >
      <div className="aspect-square w-full overflow-hidden rounded-2xl bg-white/5">
        {p.image_url ? (
          // eslint-disable-next-line @next/next/no-img-element -- manzil dinamik
          <img
            src={fullImageUrl(p.image_url)}
            alt={p.name}
            loading="lazy"
            className="h-full w-full object-cover transition-transform duration-200 group-hover:scale-[1.03]"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-white/20">
            <UtensilsCrossed size={34} />
          </div>
        )}
      </div>
      <p className="mt-2.5 text-[15px] font-bold">{formatSum(p.price_tiyin)}</p>
      <p className="mt-0.5 line-clamp-2 text-[13px] leading-snug text-white/60">{p.name}</p>
      <p className="mt-1 truncate text-[12px] text-white/35">
        {p.restaurant_open ? p.restaurant_name : `${p.restaurant_name} · yopiq`}
      </p>
    </Link>
  );
}

function ReloadButton() {
  return (
    <button
      type="button"
      onClick={() => window.location.reload()}
      className="shrink-0 rounded-xl bg-white/10 px-4 py-2.5 text-sm font-semibold hover:bg-white/[0.16]"
    >
      Qaytadan urinish
    </button>
  );
}
