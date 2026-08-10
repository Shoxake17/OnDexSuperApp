import { UtensilsCrossed } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { publicFetch } from "@/lib/api";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import type { ProductSearchResult } from "@/lib/types";
import MobileSheet from "../mobile-sheet";
import SearchBackButton from "./back-button";

// customer_app/lib/screens/category_products_screen.dart bilan bir xil:
// bitta turkum bo'yicha BARCHA restoranlardagi mos taomlarni bitta
// ro'yxatda ko'rsatadi (GET /products/search — ochiq, restoran paneli
// "taom qo'shish"da ishlatadigan bir xil turkum ro'yxatidan keladi).
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

export default async function SearchPage({
  searchParams,
}: {
  searchParams: Promise<{ category?: string }>;
}) {
  const { category } = await searchParams;
  const q = (category ?? "").trim();

  let results: ProductSearchResult[] = [];
  if (q) {
    const res = await publicFetch(
      `/products/search?q=${encodeURIComponent(q)}`,
      30,
    );
    if (res.ok) results = (await res.json()) ?? [];
  }

  return (
    <MobileSheet className="px-4 pb-10 pt-2">
      <div className="flex items-center gap-2">
        <SearchBackButton />
        <h1 className="truncate text-xl font-bold">{q || "Qidiruv"}</h1>
      </div>

      {results.length === 0 ? (
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
  );
}
