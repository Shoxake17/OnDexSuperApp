"use client";

import { useEffect, useRef, useState } from "react";

// Buyurtmani JONLI kuzatish — mobil va kompyuter ko'rinishlari uchun
// YAGONA manba.
//
// ┌─ NEGA HOOK ────────────────────────────────────────────────────────┐
// Bu mantiq oddiy `fetch` emas: birinchi GET, keyin WebSocket (holat,
// kuryer biriktirilishi, kuryer koordinatasi), uzilganda qayta ulanish
// va 15 soniyalik zaxira polling. Uni ikki komponentda alohida yozish
// — loyihada allaqachon qimmatga tushgan xato (API klienti to'rt marta
// ko'chirilgani sabab `401` ishlash va timeout hech qayerda yo'q edi).
// Bir joyda tuzatilgan nosozlik ikkala ko'rinishda ham tuzalishi kerak.
// └────────────────────────────────────────────────────────────────────┘

export type OrderItem = {
  product_id: string;
  name: string;
  qty: number;
  price_tiyin: number;
  image_url?: string;
};

export type HistoryEntry = { to?: string; at?: string };

export type Order = {
  id: string;
  order_number: string;
  restaurant_id: string;
  courier_id?: string;
  items: OrderItem[];
  subtotal_tiyin: number;
  discount_tiyin: number;
  total_tiyin: number;
  promotion_name?: string;
  promotion_discount_tiyin?: number;
  payment_method?: string;
  payment_state?: string;
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

export function useOrderTracking(id: string) {
  const [order, setOrder] = useState<Order | null>(null);
  const [loading, setLoading] = useState(true);
  const [address, setAddress] = useState<string | null>(null);
  const [courierLatLng, setCourierLatLng] = useState<{
    lat: number;
    lng: number;
  } | null>(null);
  const [paying, setPaying] = useState(false);
  const [payError, setPayError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    let cancelled = false;
    // ┌─ TUZATILGAN NOSOZLIK (bug.md 52-band) ────────────────────────┐
    // Avval qayta ulanish QAT'IY 2 soniyada edi, backoff ham,
    // urinishlar chegarasi ham yo'q. Prod'da WebSocket manzili
    // noto'g'ri bo'lgani uchun (`ws://localhost:8080`) har bir ochiq
    // buyurtma sahifasi CHEKSIZ sikl ochardi: har 2 soniyada
    // `/api/ws-ticket` so'rovi + yangi ulanish urinishi. Telefon
    // batareyasi va server bekorga yeyilardi.
    //
    // Endi eksponensial backoff (1s → 30s) va muvaffaqiyatli
    // ulanishdan keyin nolga qaytish. Zaxira polling (15s) baribir
    // ishlaydi, ya'ni backoff uzayganda ham holat yangilanib turadi.
    // └────────────────────────────────────────────────────────────────┘
    let retry = 0;
    const backoffMs = () => Math.min(1000 * 2 ** retry, 30000);

    async function load() {
      try {
        const res = await fetch(`/api/proxy/orders/${id}`);
        if (!res.ok) return;
        const o: Order = await res.json();
        if (!cancelled) setOrder(o);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void load();
    // Zaxira polling: WebSocket uzilib, qayta ulanish ham ishlamasa
    // holat baribir yangilanib turadi.
    const poll = setInterval(load, 15000);

    async function connect() {
      try {
        const res = await fetch("/api/ws-ticket", { method: "POST" });
        if (!res.ok) throw new Error();
        const { ticket } = await res.json();
        if (cancelled) return;
        const wsUrl = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8080";
        const ws = new WebSocket(`${wsUrl}/ws?ticket=${ticket}&order_id=${id}`);
        wsRef.current = ws;
        // Ulanish o'rnatildi — keyingi uzilishda yana 1 soniyadan
        // boshlanadi (uzoq ishlagan ulanish "muammoli" emas).
        ws.onopen = () => {
          retry = 0;
        };
        ws.onmessage = (evt) => {
          const e = JSON.parse(evt.data);
          if (e.order_id !== id) return;
          if (e.type === "order_status" || e.type === "courier_assigned") {
            void load();
          } else if (
            e.type === "courier_location" &&
            typeof e.lat === "number" &&
            typeof e.lng === "number"
          ) {
            setCourierLatLng({ lat: e.lat, lng: e.lng });
          }
        };
        const reconnect = () => {
          if (cancelled) return;
          // `onerror` va `onclose` ikkalasi ham chaqirilishi mumkin —
          // ikki taymer qo'yilmasin.
          if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
          const delay = backoffMs();
          retry += 1;
          reconnectTimer.current = setTimeout(connect, delay);
        };
        ws.onerror = reconnect;
        ws.onclose = reconnect;
      } catch {
        if (cancelled) return;
        const delay = backoffMs();
        retry += 1;
        reconnectTimer.current = setTimeout(connect, delay);
      }
    }
    void connect();

    return () => {
      cancelled = true;
      clearInterval(poll);
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [id]);

  useEffect(() => {
    if (!order?.delivery_lat || !order?.delivery_lng || address) return;
    fetch(
      `/api/proxy/geocode/reverse?lat=${order.delivery_lat}&lng=${order.delivery_lng}`,
    )
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (d?.address) setAddress(d.address);
      })
      .catch(() => {});
  }, [order?.delivery_lat, order?.delivery_lng, address]);

  /**
   * To'lov sahifasini QAYTA ochish.
   *
   * `retry=1` bilan server eski urinishni yopib YANGI tranzaksiya
   * ochadi — bank tomonda uzilgan urinishning eski havolasini qayta
   * ochish yordam bermaydi (masalan "takroriy SMS xabarlarining
   * maksimal soni").
   */
  async function payAgain() {
    if (paying) return;
    setPaying(true);
    setPayError(null);
    try {
      const res = await fetch(`/api/proxy/orders/${id}/pay?retry=1`, {
        method: "POST",
      });
      const data = await res.json();
      // `pay_url` HAR DOIM tashqi `https://`. Sxema qat'iy tekshiriladi:
      // `javascript:` yoki boshqa sxema `location.href` ga tushsa
      // XSS/qayta yo'naltirish bo'lardi — serverdan kelgan URL'ga ham
      // ko'r-ko'rona ishonilmaydi.
      if (
        !res.ok ||
        typeof data.pay_url !== "string" ||
        !/^https:\/\//i.test(data.pay_url)
      ) {
        throw new Error(data.error || "To'lov sahifasini ochib bo'lmadi");
      }
      window.location.href = data.pay_url;
    } catch (e) {
      setPayError(
        e instanceof Error ? e.message : "To'lov sahifasini ochib bo'lmadi",
      );
      setPaying(false);
    }
  }

  return { order, loading, address, courierLatLng, paying, payError, payAgain };
}
