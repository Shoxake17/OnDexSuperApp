"use client";

import { ArrowLeft, CreditCard, MapPin, UtensilsCrossed } from "lucide-react";
import Link from "next/link";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import {
  DetailedOrderProgress,
  extractStageTimes,
  stageOf,
  statusStyleOf,
} from "@/lib/order-status";
import { discountLineLabel } from "@/lib/promotions";
import { deliveryMapVisible } from "@/lib/delivery-tracking";
import { useOrderTracking } from "@/lib/use-order-tracking";
import DesktopNavbar from "../../desktop-navbar";
import DeliveryMap from "./delivery-map";

// Buyurtma holati — KOMPYUTER ko'rinishi.
//
// ┌─ NEGA IKKI USTUN ──────────────────────────────────────────────────┐
// Telefonda hammasi ketma-ket: holat, xarita, tarkib, summa — mijoz
// pastga suradi. Kompyuterda esa kuzatuv ekranining butun MA'NOSI —
// bir qarashda "buyurtmam qayerda" degan savolga javob berish.
// Shuning uchun chapda holat va xarita (eng muhim), o'ngda tarkib va
// hisob-kitob — skrollsiz, birga ko'rinadi.
// └────────────────────────────────────────────────────────────────────┘
//
// Ma'lumot va jonli yangilanish `lib/use-order-tracking.ts` da — mobil
// ko'rinish bilan AYNAN bir manba.
export default function DesktopOrder({
  id,
  signedIn,
}: {
  id: string;
  signedIn: boolean;
}) {
  const {
    order,
    loading,
    address,
    courierLatLng,
    tracking,
    paying,
    payError,
    payAgain,
  } = useOrderTracking(id);

  if (loading || !order) {
    return (
      <div className="flex h-dvh flex-col bg-[#302F2D] text-white">
        <DesktopNavbar signedIn={signedIn} />
        <div className="flex flex-1 items-center justify-center">
          <p className="text-white/45">
            {loading ? "Yuklanmoqda…" : "Buyurtma topilmadi"}
          </p>
        </div>
      </div>
    );
  }

  // Karta buyurtmasi puli bloklanmaguncha oshxonaga tushmaydi.
  const awaitingPayment =
    order.payment_method === "card" &&
    order.payment_state !== "held" &&
    order.payment_state !== "paid";

  const cancelled = order.status === "cancelled" || order.status === "rejected";
  const dineIn = order.type === "dine_in";
  const stage = stageOf(order.status, dineIn);
  const { label, Icon, color } = statusStyleOf(order.status, dineIn);
  const createdAt = new Date(order.created_at);
  const stageTimes = extractStageTimes(createdAt, order.history);

  // Xarita yetkazib berish buyurtmasida DOIM (koordinata yoki kuzatuv
  // bo'lsa): manzil buyurtma holatining bir qismi. Kuryer biriktirilgach
  // mashina, yo'lda — A→B yo'li va qolgan vaqt (`delivery-map.tsx`).
  const showMap =
    !dineIn &&
    !cancelled &&
    (deliveryMapVisible(order) || Boolean(order.delivery_lat && order.delivery_lng));

  return (
    <div className="flex h-dvh flex-col overflow-hidden bg-[#302F2D] text-white">
      <DesktopNavbar signedIn={signedIn} />

      <div className="mx-auto flex w-full min-h-0 max-w-[1400px] flex-1 gap-4 px-6 py-5 xl:px-10">
        {/* Orqaga — blokdan TASHQARIDA (menyu va akkaunt sahifalari
            bilan bir xil naqsh): u kontentga emas, undan chiqishga
            tegishli va skroll bilan yo'qolmaydi.

            `router.push("/")` emas, `Link` — bu oddiy navigatsiya va
            tarixga to'g'ri yoziladi; `router.back()` esa ATAYLAB
            ishlatilmadi: buyurtma sahifasiga odatda rasmiylashtirish
            oynasidan yoki bildirishnomadan kelinadi, ya'ni "orqaga"
            mijozni o'sha oraliq holatga qaytarardi. */}
        <Link
          href="/"
          aria-label="Bosh sahifaga qaytish"
          className="mt-1 flex h-11 w-11 shrink-0 items-center justify-center self-start rounded-full border border-white/10 bg-[#141414] text-white transition-colors hover:bg-[#1f1f1f]"
        >
          <ArrowLeft size={20} />
        </Link>

        {/* ── Chap: holat va xarita ─────────────────────────────────── */}
        <div className="ondex-scroll min-w-0 flex-1 overflow-y-auto overscroll-contain rounded-3xl border border-white/10 bg-[#141414] p-6">
          <div className="min-w-0">
            <p className="text-[13px] text-white/40">
              Buyurtma № {order.order_number}
            </p>
            <h1 className="mt-1 flex items-center gap-2.5 text-[26px] font-extrabold">
              <Icon size={24} style={{ color }} />
              {label}
            </h1>
            {dineIn && order.table_label && (
              <p className="mt-1.5 text-[14px] text-white/45">
                Stol: {order.table_label}
                {order.party_size ? ` · ${order.party_size} kishi` : ""}
              </p>
            )}
          </div>

          {/* ── Karta to'lovi kutilmoqda ────────────────────────────── */}
          {awaitingPayment && !cancelled && (
            <div className="mt-5 rounded-2xl border border-brand/30 bg-brand/10 p-4">
              <p className="flex items-center gap-2 text-[15px] font-bold">
                <CreditCard size={18} />
                To&apos;lov kutilmoqda
              </p>
              <p className="mt-1.5 text-[13px] leading-relaxed text-white/60">
                Buyurtma restoranga to&apos;lov tasdiqlangandan keyin
                yuboriladi. To&apos;lov oynasi yopilib qolgan bo&apos;lsa,
                quyidagi tugma yangi urinish ochadi.
              </p>
              {payError && (
                <p className="mt-2 text-[13px] text-red-400">{payError}</p>
              )}
              <button
                type="button"
                onClick={payAgain}
                disabled={paying}
                className="mt-3 h-11 rounded-xl bg-brand px-5 text-[14px] font-bold text-white transition-colors hover:bg-brand-light disabled:opacity-50"
              >
                {paying ? "Ochilmoqda…" : "To'lovni davom ettirish"}
              </button>
            </div>
          )}

          {/* ── Bosqichlar ──────────────────────────────────────────── */}
          {!cancelled && (
            <div className="mt-6">
              <DetailedOrderProgress
                stage={stage}
                stageTimes={stageTimes}
                dineIn={dineIn}
              />
            </div>
          )}

          {/* ┌─ MANZIL XARITASI — HOLAT OSTIDA ────────────────────────┐
              Xarita endi FAQAT kuryer yo'lga chiqqanda emas, buyurtma
              qabul qilingan daqiqadan boshlab ko'rinadi: mijoz
              "qayerga yetkaziladi" ni darhol ko'rishi kerak (manzilni
              noto'g'ri tanlagan bo'lsa, aynan shu yerda sezadi).
              Kuryer paydo bo'lgach xaritada uning belgisi ham chiqadi
              va kamera unga suriladi (`courier-map.tsx`).

              Stol buyurtmasida xarita YO'Q — yetkazish umuman
              bo'lmaydi.
              └─────────────────────────────────────────────────────────┘ */}
          {showMap && (
            <div className="mt-6">
              <DeliveryMap
                order={order}
                courier={courierLatLng}
                tracking={tracking}
                variant="dark"
                mapHeightClassName="h-[340px]"
              />
              <p className="mt-3 flex items-start gap-2 text-[14px] text-white/55">
                <MapPin size={16} className="mt-0.5 shrink-0 text-white/35" />
                {address ?? "Manzil aniqlanmoqda…"}
              </p>
            </div>
          )}
        </div>

        {/* ── O'ng: tarkib va hisob ─────────────────────────────────── */}
        <aside className="ondex-scroll hidden w-[360px] shrink-0 flex-col overflow-y-auto overscroll-contain rounded-3xl border border-white/10 bg-[#141414] lg:flex">
          <h2 className="px-5 pb-3 pt-5 text-[18px] font-bold">
            Buyurtma tarkibi
          </h2>

          <div className="px-5">
            {order.items.map((it) => (
              <div
                key={it.product_id}
                className="flex items-center gap-3 border-b border-white/5 py-3"
              >
                <div className="h-12 w-12 shrink-0 overflow-hidden rounded-xl bg-white/5">
                  {it.image_url ? (
                    // eslint-disable-next-line @next/next/no-img-element -- manzil dinamik
                    <img
                      src={fullImageUrl(it.image_url)}
                      alt=""
                      aria-hidden="true"
                      className="h-full w-full object-cover"
                    />
                  ) : (
                    <div className="flex h-full w-full items-center justify-center text-white/25">
                      <UtensilsCrossed size={18} />
                    </div>
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="line-clamp-2 text-[13.5px] leading-snug">
                    {it.name}
                  </p>
                  <p className="mt-0.5 text-[12.5px] text-white/40">×{it.qty}</p>
                </div>
                <p className="shrink-0 text-[13.5px] font-semibold">
                  {formatSum(it.price_tiyin * it.qty)}
                </p>
              </div>
            ))}
          </div>

          <div className="mt-auto p-5">
            <div className="flex items-baseline justify-between text-[14px]">
              <span className="text-white/45">Taomlar</span>
              <span className="font-semibold">
                {formatSum(order.subtotal_tiyin)}
              </span>
            </div>
            {order.discount_tiyin > 0 && (
              <div className="mt-1.5 flex items-baseline justify-between text-[14px]">
                <span className="text-white/45">
                  {discountLineLabel(
                    order.discount_tiyin,
                    order.promotion_discount_tiyin ?? 0,
                    order.promotion_name,
                  )}
                </span>
                <span className="font-semibold text-green-400">
                  −{formatSum(order.discount_tiyin)}
                </span>
              </div>
            )}
            <div className="mt-3 flex items-baseline justify-between border-t border-white/10 pt-3 text-[17px] font-bold">
              <span>Jami</span>
              <span>{formatSum(order.total_tiyin)}</span>
            </div>
          </div>
        </aside>
      </div>
    </div>
  );
}
