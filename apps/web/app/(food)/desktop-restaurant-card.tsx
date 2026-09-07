import { Clock, Star, Store } from "lucide-react";
import Link from "next/link";
import { fullImageUrl } from "@/lib/images";
import type { Restaurant } from "@/lib/types";

// Desktop to'ri uchun restoran kartasi.
//
// ┌─ NEGA MOBIL KARTA (restaurant-card.tsx) BU YERDA ISHLAMAYDI ───────┐
// U matnni (nom, teglar, manzil) COVER RASMI USTIGA yozadi. Telefonda
// karta butun ekran kengligida va matn uchun bo'sh joy yetarli edi.
// Desktop to'rida esa ustun ~3 baravar tor — matn rasmning o'rtasiga
// tushadi. Cover rasmlarida esa restoranning O'Z logotipi/nomi
// ALLAQACHON chizilgan (Avigo, Chust Burger, Feel Food — hammasi
// shunday), natijada ikkita nom ustma-ust tushib, ikkalasi ham
// o'qilmay qoldi (2026-09-04 da topilgan).
//
// Yechim namunadagi ("Магазины" bo'limi, image/eats.png) bilan bir xil:
// rasm — TOZA plitka, matn esa uning OSTIDA. Shunda cover'dagi brend
// ham, bizning matn ham buzilmaydi.
// └───────────────────────────────────────────────────────────────────┘
export default function DesktopRestaurantCard({
  restaurant: r,
}: {
  restaurant: Restaurant;
}) {
  const hasRating = r.rating > 0;
  const hasEta = r.eta_min_minutes > 0 && r.eta_max_minutes > 0;
  const tags = r.tags.trim();

  const body = (
    <>
      <div
        className={`relative overflow-hidden rounded-2xl bg-white/5 ${
          r.open ? "" : "opacity-50"
        }`}
      >
        {/* ┌─ QAT'IY NISBAT: 267×133 (= 2:1) ─────────────────────────┐
            Avval balandlikni rasmning O'ZI belgilardi (`h-auto`). U
            kesilishning ham, tepa/pastdagi yo'lakning ham oldini
            olardi, LEKIN balandlik kartaning kengligiga bog'liq bo'lib
            qolardi: chekinish yoki ustun soni o'zgarishi bilan cover
            "cho'kib" ketardi (2026-09-04 da aynan shu sezildi).

            Endi ramka nisbati QAT'IY — `267/133`. Bu cover'larning
            o'z nisbati (banner ~2:1) bilan deyarli bir xil, shuning
            uchun `object-cover` amalda hech narsani kesmaydi: avvalgi
            `16/10` urinishida esa farq katta bo'lgani uchun logotip
            chetlari qirqilardi.

            Yon foydasi: to'rdagi barcha kartalar endi BIR TEKIS
            balandlikda (rasm nisbati har xil bo'lsa ham).
            └─────────────────────────────────────────────────────────┘ */}
        {r.cover_url ? (
          <div className="aspect-[267/133] w-full">
            {/* eslint-disable-next-line @next/next/no-img-element -- manzil muhitga qarab dinamik (R2/lokal disk), `next/image` uchun `remotePatterns` oldindan belgilab bo'lmaydi */}
            <img
              src={fullImageUrl(r.cover_url)}
              alt={r.name}
              className="h-full w-full object-cover"
            />
          </div>
        ) : (
          <div className="flex aspect-[267/133] w-full items-center justify-center text-white/20">
            <Store size={44} />
          </div>
        )}

        {/* Yopiq bo'lsa — ANIQ belgi. Ochiq holat uchun belgi
            QO'YILMAYDI: u standart holat va har kartada takrorlanganda
            faqat shovqin qo'shadi (namunada ham yo'q). */}
        {!r.open && (
          <span className="absolute left-3 top-3 rounded-full bg-black/75 px-2.5 py-1 text-xs font-bold text-white">
            Yopiq
          </span>
        )}
      </div>

      <p className="mt-2.5 truncate text-[15px] font-bold text-white">{r.name}</p>

      <div className="mt-1 flex items-center gap-3 text-[13px] text-white/55">
        {hasEta && (
          <span className="flex items-center gap-1">
            <Clock size={13} className="shrink-0" />
            {r.eta_min_minutes}–{r.eta_max_minutes} daq
          </span>
        )}
        {hasRating && (
          <span className="flex items-center gap-1">
            <Star size={13} className="shrink-0 fill-brand text-brand" />
            {r.rating.toFixed(1)}
          </span>
        )}
      </div>

      {tags && <p className="mt-0.5 truncate text-[13px] text-white/40">{tags}</p>}
    </>
  );

  if (!r.open) return <div className="block">{body}</div>;
  return (
    <Link href={`/restaurants/${r.id}`} className="group block">
      {body}
    </Link>
  );
}
