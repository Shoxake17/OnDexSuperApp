"use client";

import { MapPin, UtensilsCrossed } from "lucide-react";
import { useRouter } from "next/navigation";
import { use } from "react";
import { useOrderTracking } from "@/lib/use-order-tracking";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import { goBack } from "@/lib/nav";
import { discountLineLabel } from "@/lib/promotions";
import { DetailedOrderProgress, stageOf, statusStyleOf, extractStageTimes } from "@/lib/order-status";
import CourierMap from "./courier-map";
import MobileSheet from "../../mobile-sheet";
import { AppButtonLink, BackButton } from "../../ui";


// tracking_screen.dart bilan parity: avval GET bilan hozirgi holat, keyin
// WebSocket orqali jonli yangilanish (order_status/courier_assigned/
// courier_location), 15s zaxira polling. Xarita FAQAT "picked_up"
// bosqichida (kuryer yo'lga chiqqanda) ko'rsatiladi.
export default function OrderTrackingPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const router = useRouter();
  // Jonli kuzatuv mantiqi (GET + WebSocket + zaxira polling + karta
  // to'lovini qayta ochish) `lib/use-order-tracking.ts` da — kompyuter
  // ko'rinishi ham AYNAN shu hook'dan foydalanadi, ya'ni bir joyda
  // tuzatilgan nosozlik ikkalasida ham tuzaladi.
  const { order, loading, address, courierLatLng, paying, payError, payAgain } =
    useOrderTracking(id);

  if (loading) {
    return (
      <main className="flex min-h-screen items-center justify-center">
        <p className="text-neutral-500">Yuklanmoqda...</p>
      </main>
    );
  }
  if (!order) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center gap-2">
        <p className="text-lg font-semibold">Buyurtma topilmadi</p>
        <a href="/" className="text-[#E53935] underline">
          Bosh sahifaga qaytish
        </a>
      </main>
    );
  }

  // Karta buyurtmasi puli bloklanmaguncha oshxonaga tushmaydi.
  const awaitingPayment =
    order.payment_method === "card" &&
    order.payment_state !== "held" &&
    order.payment_state !== "paid";

  const cancelled = order.status === "cancelled" || order.status === "rejected";
  const dineIn = order.type === "dine_in";
  const done = order.status === "delivered" || order.status === "served";
  const stage = stageOf(order.status, dineIn);
  const { label, Icon, color } = statusStyleOf(order.status, dineIn);
  const createdAt = new Date(order.created_at);
  const stageTimes = extractStageTimes(createdAt, order.history);
  const totalItems = order.items.reduce((a, i) => a + i.qty, 0);
  const destination =
    order.delivery_lat && order.delivery_lng
      ? { lat: order.delivery_lat, lng: order.delivery_lng }
      : null;

  return (
    <MobileSheet maxWidthClassName="max-w-lg" className="px-4 pb-10 pt-3">
      <div className="flex items-center gap-2">
        <BackButton onClick={() => goBack(() => router.push("/"))} />
        <h1 className="text-xl font-bold">Buyurtma holati</h1>
      </div>

      <p className="mt-3 text-sm text-neutral-500">
        {order.order_number}
        {"  •  "}
        {createdAt.getHours().toString().padStart(2, "0")}:
        {createdAt.getMinutes().toString().padStart(2, "0")} da joylandi
      </p>

      {/* To'lov kutayotgan buyurtma — eng tepada, chunki bu yerda
          mijozdan HARAKAT talab qilinadi (tracking_screen.dart dagi
          bilan bir xil qoida). */}
      {awaitingPayment && !cancelled && (
        <div className="mt-3 rounded-2xl border border-amber-300 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-950/40">
          <p className="font-bold text-amber-800 dark:text-amber-300">
            To&apos;lov kutilmoqda
          </p>
          <p className="mt-1 text-sm leading-relaxed text-amber-900/80 dark:text-amber-200/80">
            Buyurtma restoranga <b>yuborilmagan</b>. To&apos;lovni
            yakunlaganingizdan keyin u avtomatik oshxonaga tushadi. Pul darhol
            yechilmaydi — vaqtincha bloklanadi va restoran buyurtmani qabul
            qilgandagina yechiladi.
          </p>
          {payError && (
            <p className="mt-2 text-sm text-[#E53935]">{payError}</p>
          )}
          <button
            type="button"
            disabled={paying}
            onClick={payAgain}
            className="mt-3 w-full rounded-xl bg-[#E53935] px-4 py-3 font-semibold text-white disabled:opacity-60"
          >
            {paying ? "Ochilmoqda…" : "To'lovni yakunlash"}
          </button>
        </div>
      )}

      <div
        className="mt-3 rounded-2xl p-5 text-center"
        style={{ backgroundColor: `${color}1F` }}
      >
        <div className="flex justify-center" style={{ color }}>
          <Icon size={40} />
        </div>
        <p className="mt-2 text-lg font-bold" style={{ color }}>
          {label}
        </p>
        {cancelled && (
          <p className="mt-1 text-sm text-neutral-500">
            {order.status === "rejected"
              ? "Restoran buyurtmani rad etdi"
              : "Buyurtma bekor qilindi"}
          </p>
        )}
        {/* Kuryer ID — stol buyurtmasida ma'nosiz (kuryer yo'q). */}
        {!dineIn && order.courier_id && (
          <p className="mt-1 text-sm text-neutral-500">Kuryer: {order.courier_id}</p>
        )}
        {dineIn && order.table_label && (
          <p className="mt-1 text-sm text-neutral-500">
            {order.table_label}-stol
            {order.party_size ? ` · ${order.party_size} kishi` : ""}
          </p>
        )}
        {stage >= 0 && (
          <div className="mt-5">
            <DetailedOrderProgress
              stage={stage}
              stageTimes={stageTimes}
              dineIn={dineIn}
            />
          </div>
        )}
      </div>

      {/* Kuryer xaritasi FAQAT yetkazib berishda. Stol buyurtmasida
          `picked_up` holati umuman uchramaydi, lekin shart ANIQ
          yozilgan — kelajakda holat qo'shilsa xarita tasodifan
          chiqib qolmasin. */}
      {!dineIn && order.status === "picked_up" && courierLatLng && (
        <div className="mt-5">
          <CourierMap courier={courierLatLng} destination={destination} />
        </div>
      )}

      <div className="mt-6 flex items-center justify-between">
        <h2 className="text-lg font-bold">Buyurtma tarkibi</h2>
        <span
          className="rounded-full px-2.5 py-1 text-xs font-semibold"
          style={{ backgroundColor: `${color}29`, color }}
        >
          {totalItems} ta mahsulot
        </span>
      </div>
      <div className="mt-3 divide-y divide-neutral-200 overflow-hidden rounded-2xl border border-neutral-200 dark:divide-neutral-800 dark:border-neutral-800">
        {order.items.map((item, i) => (
          <div key={i} className="flex items-center gap-3 p-3">
            {/* Rasm o'lchami QAT'IY 56x56 — avval `h-13 w-13` yozilgan edi,
                lekin Tailwind'ning standart shkalasida 13 YO'Q, shuning
                uchun klass umuman qo'llanmasdi va rasmlar o'z tabiiy
                o'lchamida, har xil kattalikda chiqardi (foydalanuvchi
                "katta kichik bo'lib ketgan" deb aynan shuni ko'rsatgan).
                `aspect-square` + qat'iy o'lcham bilan endi bir tekis. */}
            <div className="h-14 w-14 shrink-0 overflow-hidden rounded-xl bg-neutral-100 dark:bg-neutral-800">
              {item.image_url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={fullImageUrl(item.image_url)}
                  alt={item.name}
                  className="h-full w-full object-cover"
                />
              ) : (
                <div className="flex h-full w-full items-center justify-center text-neutral-400">
                  <UtensilsCrossed size={22} />
                </div>
              )}
            </div>
            <div className="min-w-0 flex-1">
              <p className="line-clamp-2 text-sm font-medium">{item.name}</p>
              <p className="mt-0.5 text-xs text-neutral-500">
                {item.qty} x {formatSum(item.price_tiyin)}
              </p>
            </div>
            <span className="shrink-0 text-sm font-bold">
              {formatSum(item.price_tiyin * item.qty)}
            </span>
          </div>
        ))}
        {order.discount_tiyin > 0 && (
          <div className="flex justify-between p-3 text-sm text-green-600 dark:text-green-400">
            <span>
              {discountLineLabel(
                order.discount_tiyin,
                order.promotion_discount_tiyin ?? 0,
                order.promotion_name,
              )}
            </span>
            <span className="font-bold">-{formatSum(order.discount_tiyin)}</span>
          </div>
        )}
        <div className="flex items-center justify-between p-3.5">
          <span className="font-bold">Jami</span>
          <span className="text-base font-bold">{formatSum(order.total_tiyin)}</span>
        </div>
      </div>

      {address && (
        <>
          <h2 className="mt-6 text-lg font-bold">Yetkazib berish manzili</h2>
          <div className="mt-3 flex items-center gap-2.5 rounded-2xl border border-neutral-200 p-3.5 dark:border-neutral-800">
            <MapPin size={18} className="shrink-0 text-neutral-500" />
            <span className="text-sm">{address}</span>
          </div>
        </>
      )}

      {(done || cancelled) && (
        <div className="mt-7">
          <AppButtonLink href="/">Bosh sahifaga qaytish</AppButtonLink>
        </div>
      )}
    </MobileSheet>
  );
}
