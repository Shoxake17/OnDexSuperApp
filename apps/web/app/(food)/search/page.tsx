import { UtensilsCrossed } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { publicFetch } from "@/lib/api";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import { getSessionToken } from "@/lib/session";
import type { ProductSearchResult, Restaurant } from "@/lib/types";
import MobileSheet from "../mobile-sheet";
import SearchBackButton from "./back-button";
import DesktopSearch from "./desktop-search";

// customer_app/lib/screens/category_products_screen.dart bilan bir xil:
// bitta turkum bo'yicha BARCHA restoranlardagi mos taomlarni bitta
// ro'yxatda ko'rsatadi (GET /products/search — ochiq, restoran paneli
// "taom qo'shish"da ishlatadigan bir xil turkum ro'yxatidan keladi).
//
// Kompyuterda (`md:` va undan katta — bosh sahifa bilan bir xil chegara,
// chunki qidiruvga o'sha sahifaning navbar'idan kiriladi) alohida
// ko'rinish: `desktop-search.tsx`.
export async function generateMetadata({
  searchParams,
}: {
  searchParams: Promise<{ category?: string }>;
}): Promise<Metadata> {
  const { category } = await searchParams;
  return {
    title: category ? `${category} — ChustApp` : "Qidiruv — ChustApp",
  };
}

// ┌─ NEGA `try/catch` ─────────────────────────────────────────────────┐
// `fetch` tarmoq darajasida yiqilsa (API o'chgan, konteyner qayta
// ishga tushmoqda, DNS javob bermadi) u XATO TASHLAYDI — `res.ok`
// gacha yetib ham bormaydi. Bu yerda ushlanmagani uchun butun
// server komponenti qulab, foydalanuvchi Next.js'ning xato ekranini
// ko'rardi (2026-09-04 da aynan shu holat topildi).
//
// "Topilmadi" va "yuklab bo'lmadi" ATAYLAB ajratiladi: birinchisi
// normal natija, ikkinchisi vaqtinchalik nosozlik va foydalanuvchi
// qaytadan urinib ko'rishi kerak. Ikkalasini bir xil ko'rsatish
// mijozni "bunday taom yo'q ekan" degan noto'g'ri xulosaga olib
// kelardi.
// └────────────────────────────────────────────────────────────────────┘

/** Mos taomlar; `null` — yuklab bo'lmadi. */
async function searchProducts(q: string): Promise<ProductSearchResult[] | null> {
  try {
    const res = await publicFetch(`/products/search?q=${encodeURIComponent(q)}`, 30);
    if (!res.ok) return null;
    return (await res.json()) ?? [];
  } catch {
    return null;
  }
}

/**
 * Nomi yoki turkumi mos restoranlar — FAQAT kompyuter ko'rinishi uchun.
 * Ro'yxat bosh sahifadagi bilan bir xil so'rov va kesh (30 s), ya'ni
 * qo'shimcha yuklama yo'q. Nosozlik qidiruvni to'xtatmaydi: taomlar
 * baribir ko'rsatiladi.
 */
async function matchingRestaurants(q: string): Promise<Restaurant[]> {
  try {
    const res = await publicFetch("/restaurants", 30);
    if (!res.ok) return [];
    const all: Restaurant[] = (await res.json()) ?? [];
    const needle = q.toLowerCase();
    return all.filter(
      (r) =>
        r.name.toLowerCase().includes(needle) ||
        (r.tags || "").toLowerCase().includes(needle),
    );
  } catch {
    return [];
  }
}

export default async function SearchPage({
  searchParams,
}: {
  searchParams: Promise<{ category?: string }>;
}) {
  const { category } = await searchParams;
  const q = (category ?? "").trim();

  const [products, restaurants, signedIn] = await Promise.all([
    q ? searchProducts(q) : Promise.resolve<ProductSearchResult[]>([]),
    q ? matchingRestaurants(q) : Promise.resolve<Restaurant[]>([]),
    getSessionToken().then(Boolean),
  ]);
  const failed = products === null;
  const results = products ?? [];

  return (
    <>
      <div className="hidden md:block">
        <DesktopSearch
          query={q}
          restaurants={restaurants}
          products={results}
          failed={failed}
          signedIn={signedIn}
        />
      </div>

      <div className="md:hidden">
        <MobileSheet className="px-4 pb-10 pt-2">
          <div className="flex items-center gap-2">
            <SearchBackButton />
            <h1 className="truncate text-xl font-bold">{q || "Qidiruv"}</h1>
          </div>

          {failed ? (
            <div className="py-10 text-center">
              <p className="text-neutral-500">
                Hozir ro&apos;yxatni yuklab bo&apos;lmadi.
              </p>
              <p className="mt-1 text-sm text-neutral-400">
                Internet aloqasini tekshirib, sahifani yangilang.
              </p>
            </div>
          ) : results.length === 0 ? (
            <p className="py-10 text-center text-neutral-500">
              Bu turkumda taom topilmadi
            </p>
          ) : (
            <div className="mt-4 grid grid-cols-2 gap-x-3.5 gap-y-5">
              {results.map((p) => (
                <Link
                  key={p.id}
                  href={`/restaurants/${p.restaurant_id}`}
                  className={
                    p.restaurant_open ? "" : "pointer-events-none opacity-40"
                  }
                >
                  <div className="aspect-square w-full overflow-hidden rounded-2xl bg-neutral-100 dark:bg-neutral-800">
                    {p.image_url ? (
                      // eslint-disable-next-line @next/next/no-img-element
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
                  <p className="mt-0.5 line-clamp-1 text-xs text-neutral-500">
                    {p.restaurant_open
                      ? p.restaurant_name
                      : `${p.restaurant_name} (yopiq)`}
                  </p>
                </Link>
              ))}
            </div>
          )}
        </MobileSheet>
      </div>
    </>
  );
}
