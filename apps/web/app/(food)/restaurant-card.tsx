import { Clock, MapPin, Star, Store } from "lucide-react";
import Link from "next/link";
import { fullImageUrl } from "@/lib/images";
import type { Restaurant } from "@/lib/types";

// Mijoz super-app bosh sahifasidagi restoran kartasi (image/restarant.png).
//
// ┌─ FON NEGA COVER RASMI ─────────────────────────────────────────────┐
// Maketda har kartaning o'z brend foni bor (ko'k / oq / qora). Backendda
// bunday maydon YO'Q va uni har restoran uchun qo'lda kiritish kerak
// bo'lardi. Cover rasmi esa allaqachon bor va amalda o'sha brend
// bannerining o'zi — shuning uchun u fon sifatida ishlatiladi.
//
// Rasm ustidan CHAPDAN o'ngga qorayadigan gradient tushadi: maketdagi
// "chapda rang, o'ngda taom surati" tuzilishi shundan chiqadi va matn
// rasm qanday bo'lishidan qat'i nazar o'qiladi (oq matn har doim to'q
// yuzada). Busiz och rangli cover'da oq matn ko'rinmay qolardi.
// └────────────────────────────────────────────────────────────────────┘
export default function RestaurantCard({
  restaurant: r,
}: {
  restaurant: Restaurant;
}) {
  const cover = r.cover_url;
  const tags = r.tags.trim();

  // 0 = kiritilmagan. Soxta qiymat ko'rsatmaymiz — chip butunlay
  // chizilmaydi (backend izohiga qarang: catalog.Restaurant).
  const hasRating = r.rating > 0;
  const hasEta = r.eta_min_minutes > 0 && r.eta_max_minutes > 0;

  const card = (
    <div
      className={`relative isolate overflow-hidden rounded-3xl ${
        // Cover yo'q bo'lsa fon TO'Q bo'lishi shart: oq matn och kulrang
        // ustida o'qilmasdi.
        cover ? "bg-neutral-800" : "bg-neutral-800 dark:bg-neutral-700"
      } ${r.open ? "" : "pointer-events-none opacity-60"}`}
    >
      {cover ? (
        // eslint-disable-next-line @next/next/no-img-element -- rasm manzili
        // muhitga qarab dinamik (R2/lokal disk), next/image uchun oldindan
        // domen ro'yxati (remotePatterns) belgilab bo'lmaydi.
        <img
          src={fullImageUrl(cover)}
          alt=""
          aria-hidden="true"
          className="absolute inset-0 -z-10 h-full w-full object-cover"
        />
      ) : (
        <div className="absolute inset-0 -z-10 flex items-center justify-center text-white/25">
          <Store size={64} />
        </div>
      )}

      {/* ┌─ COVER RASMI O'ZGARTIRILMAYDI ──────────────────────────────┐
          Avval bu yerda chapdan o'ngga qorayadigan gradient turardi va
          u brend suratini bosib, kartani xira ko'rsatardi. Endi cover
          ASL holida chiziladi.

          Matn o'qilishi gradient bilan emas, HARF SOYASI bilan
          ta'minlanadi (`drop-shadow`): soya faqat harflar atrofida
          bo'ladi, rasmning o'ziga tegmaydi.
          └─────────────────────────────────────────────────────────────┘ */}

      <span
        className={`absolute right-4 top-4 rounded-full bg-white px-3 py-1.5 text-xs font-bold shadow-sm ${
          r.open ? "text-green-600" : "text-red-600"
        }`}
      >
        {r.open ? "Ochiq" : "Yopiq"}
      </span>

      {/* Logotip ATAYLAB chizilmaydi. Cover rasmining o'zida brend
          logotipi allaqachon bor (u amalda brend bannerining o'zi), va
          ustiga yana bitta logotip qo'yilganda ikkitasi bir-birini
          takrorlab, kartani qalashtirib yuborardi — haqiqiy qurilmada
          aynan shu sezilgan. `r.logo_url` ma'lumotda qoladi va boshqa
          joylarda (menyu sahifasi sarlavhasi) ishlatiladi. */}
      <div className="flex min-h-[200px] flex-col justify-end gap-1 p-4 pr-24 [text-shadow:0_1px_4px_rgba(0,0,0,0.75)]">
        <h3 className="text-xl font-bold leading-tight text-white">{r.name}</h3>

        {tags && <p className="text-sm text-white/90">{tags}</p>}

        {r.address && (
          <p className="flex items-center gap-1 text-xs text-white/80">
            <MapPin size={13} className="shrink-0" />
            <span className="line-clamp-1">{r.address}</span>
          </p>
        )}

        {(hasEta || hasRating) && (
          <div className="mt-2 flex flex-wrap gap-2">
            {hasEta && (
              <Chip>
                <Clock size={14} className="text-neutral-700" />
                {r.eta_min_minutes}–{r.eta_max_minutes} daqiqa
              </Chip>
            )}
            {hasRating && (
              <Chip>
                <Star size={14} className="fill-brand text-brand" />
                {/* `toFixed(1)` — 5 emas, "5.0" ko'rinsin (maketdagidek). */}
                {r.rating.toFixed(1)}
                {r.rating_count > 0 && ` (${r.rating_count}+)`}
              </Chip>
            )}
          </div>
        )}
      </div>
    </div>
  );

  if (!r.open) return card;
  return (
    <Link href={`/restaurants/${r.id}`} className="block">
      {card}
    </Link>
  );
}

function Chip({ children }: { children: React.ReactNode }) {
  return (
    <span className="flex items-center gap-1.5 rounded-full bg-white px-3 py-1.5 text-xs font-semibold text-neutral-800 shadow-sm">
      {children}
    </span>
  );
}
