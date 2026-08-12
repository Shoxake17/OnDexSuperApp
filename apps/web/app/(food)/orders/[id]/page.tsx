"use client";

import { MapPin, UtensilsCrossed } from "lucide-react";
import { useRouter } from "next/navigation";
import { use, useEffect, useRef, useState } from "react";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import { goBack } from "@/lib/nav";
import { DetailedOrderProgress, stageOf, statusStyleOf, extractStageTimes } from "@/lib/order-status";
import CourierMap from "./courier-map";
import MobileSheet from "../../mobile-sheet";
import { AppButtonLink, BackButton } from "../../ui";

type OrderItem = {
  product_id: string;
  name: string;
  qty: number;
  price_tiyin: number;
  image_url?: string;
};

type HistoryEntry = { to?: string; at?: string };

type Order = {
  id: string;
  order_number: string;
  restaurant_id: string;
  courier_id?: string;
  items: OrderItem[];
  subtotal_tiyin: number;
  discount_tiyin: number;
  total_tiyin: number;
  promotion_name?: string;
  status: string;
  history?: HistoryEntry[];
  delivery_lat?: number;
  delivery_lng?: number;
  created_at: string;
  /** "dine_in" — stol buyurtmasi. Bo'sh/yo'q = yetkazib berish. */
  type?: string;
  table_label?: string;
  party_size?: number;
};

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
  const [order, setOrder] = useState<Order | null>(null);
  const [loading, setLoading] = useState(true);
  const [address, setAddress] = useState<string | null>(null);
  const [courierLatLng, setCourierLatLng] = useState<{ lat: number; lng: number } | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  async function load() {
    try {
      const res = await fetch(`/api/proxy/orders/${id}`);
      if (!res.ok) return;
      const o: Order = await res.json();
      setOrder(o);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const poll = setInterval(load, 15000);

    let cancelled = false;
    async function connect() {
      try {
        const res = await fetch("/api/ws-ticket", { method: "POST" });
        if (!res.ok) throw new Error();
        const { ticket } = await res.json();
        if (cancelled) return;
        const wsUrl = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8080";
        const ws = new WebSocket(`${wsUrl}/ws?ticket=${ticket}&order_id=${id}`);
        wsRef.current = ws;
        ws.onmessage = (evt) => {
          const e = JSON.parse(evt.data);
          if (e.order_id !== id) return;
          if (e.type === "order_status" || e.type === "courier_assigned") {
            load();
          } else if (e.type === "courier_location" && typeof e.lat === "number" && typeof e.lng === "number") {
            setCourierLatLng({ lat: e.lat, lng: e.lng });
          }
        };
        const reconnect = () => {
          if (cancelled) return;
          reconnectTimer.current = setTimeout(connect, 2000);
        };
        ws.onerror = reconnect;
        ws.onclose = reconnect;
      } catch {
        if (!cancelled) reconnectTimer.current = setTimeout(connect, 2000);
      }
    }
    connect();

    return () => {
      cancelled = true;
      clearInterval(poll);
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  useEffect(() => {
    if (!order?.delivery_lat || !order?.delivery_lng || address) return;
    fetch(`/api/proxy/geocode/reverse?lat=${order.delivery_lat}&lng=${order.delivery_lng}`)
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (d?.address) setAddress(d.address);
      })
      .catch(() => {});
  }, [order?.delivery_lat, order?.delivery_lng, address]);

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
              {order.promotion_name ? `Aksiya: ${order.promotion_name}` : "Aksiya chegirmasi"}
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
