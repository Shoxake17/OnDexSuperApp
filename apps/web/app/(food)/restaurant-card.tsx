import { Store } from "lucide-react";
import Link from "next/link";
import { fullImageUrl } from "@/lib/images";
import type { Restaurant } from "@/lib/types";

// Flutter'dagi _RestaurantCard bilan bir xil: 16:9 cover, nom, ochiq/yopiq
// belgi, tags, manzil. Yopiq restoran xiralashtiriladi va bosilmaydi.
export default function RestaurantCard({
  restaurant: r,
}: {
  restaurant: Restaurant;
}) {
  const cover = r.cover_url;
  const tags = r.tags.trim();

  const card = (
    <div className={r.open ? "" : "pointer-events-none opacity-50"}>
      <div className="relative aspect-video w-full overflow-hidden rounded-2xl bg-neutral-100 dark:bg-neutral-800">
        {cover ? (
          // eslint-disable-next-line @next/next/no-img-element -- rasm manzili
          // muhitga qarab dinamik (R2/lokal disk), next/image uchun oldindan
          // domen ro'yxati (remotePatterns) belgilab bo'lmaydi.
          <img
            src={fullImageUrl(cover)}
            alt={r.name}
            className="h-full w-full object-cover"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-neutral-400">
            <Store size={36} />
          </div>
        )}
      </div>
      <div className="mt-2.5 flex items-start justify-between gap-2">
        <h3 className="text-lg font-bold">{r.name}</h3>
        <span
          className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold ${
            r.open
              ? "bg-green-500/15 text-green-600 dark:text-green-400"
              : "bg-red-500/15 text-red-600 dark:text-red-400"
          }`}
        >
          {r.open ? "Ochiq" : "Yopiq"}
        </span>
      </div>
      {tags && <p className="mt-1 text-sm text-neutral-500">{tags}</p>}
      {r.address && (
        <p className="mt-0.5 line-clamp-1 text-xs text-neutral-500">
          {r.address}
        </p>
      )}
    </div>
  );

  if (!r.open) return card;
  return <Link href={`/restaurants/${r.id}`}>{card}</Link>;
}
