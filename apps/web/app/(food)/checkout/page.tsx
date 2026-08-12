"use client";

import {
  Banknote,
  ChevronRight,
  Home,
  MessageSquare,
  Phone,
  Users,
  Utensils,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { useCart } from "@/lib/cart-context";
import { formatSum } from "@/lib/format";
import { newIdempotencyKey } from "@/lib/idempotency";
import { goBack } from "@/lib/nav";
import {
  clearTableSession,
  readTableSession,
  type TableSession,
} from "@/lib/table-session";
import MobileSheet from "../mobile-sheet";
import { AppButton, BackButton } from "../ui";

type AddressDetails = {
  text?: string;
  lat?: number;
  lng?: number;
  entrance?: string;
  floor?: string;
  apartment?: string;
  intercom?: string;
  comment?: string;
};

type Quote = {
  subtotal_tiyin: number;
  discount_tiyin: number;
  total_tiyin: number;
  promotion_name?: string;
};

// image/rasmiy.png + rasmiy1.png namunalariga mos (Yandex Go uslubi):
// alohida "kartochka" bloklari — Qayerga yetkazamiz / To'lov / Buyurtma
// qiymati, pastda yopishqoq "Jami" + tugma.
//
// NAMUNADAN ATAYLAB OLINMAGAN elementlar (loyihaning "soxta ma'lumot
// ko'rsatmaslik" qoidasi — bularning HECH BIRI backend'da mavjud emas):
//   • "Yetkazish" narxi va "Xizmat yig'imi" — yetkazish narxi logikasi
//     umuman qurilmagan (ROADMAP "Navbatda" 2-band).
//   • Karta orqali to'lov / "Split bilan muddatli to'lov" — Click/Payme
//     integratsiyasi yo'q (ROADMAP 13-band).
//   • Kupon/promokod va "Nujna sdacha?" — bunday tizim yo'q.
// Ular haqiqiy funksiya qurilgach qo'shiladi; hozir soxta qator
// ko'rsatilmaydi.
export default function CheckoutPage() {
  const router = useRouter();
  const cart = useCart();
  const [address, setAddress] = useState<AddressDetails | null>(null);
  const [loadingAddress, setLoadingAddress] = useState(true);
  const [phone, setPhone] = useState<string>("");
  const [quote, setQuote] = useState<Quote | null>(null);
  const [loadingQuote, setLoadingQuote] = useState(true);
  const [placing, setPlacing] = useState(false);
  const [placeError, setPlaceError] = useState<string | null>(null);
  const idempotencyKeyRef = useRef<string | null>(null);

  // ── Stol rejimi (QR kod) ──
  //
  // `null` = odatiy yetkazib berish. Seans `sessionStorage` da, ya'ni
  // Mini App yopilishi bilan yo'qoladi (`lib/table-session.ts`).
  const [table, setTable] = useState<TableSession | null>(null);
  const [partySize, setPartySize] = useState(2);

  const restaurantId = cart.restaurantId;
  const cartItems = cart.items;

  useEffect(() => {
    const t = readTableSession();
    // ┌─ MOSLIK TEKSHIRUVI ──────────────────────────────────────────┐
    // Savat BOSHQA restoranga tegishli bo'lsa, stol seansi
    // e'tiborga OLINMAYDI. Bunday holat haqiqatan uchraydi: mijoz
    // stolda o'tirib QR skanerlaydi, keyin bosh sahifadan boshqa
    // restoran menyusiga o'tib savat yig'adi.
    //
    // Server ham bu holatni rad etadi (`createDineInOrder` dagi
    // restoran mosligi tekshiruvi), lekin u yerda mijoz tushunarsiz
    // xato olardi. Bu yerda esa oddiygina yetkazib berish rejimiga
    // o'tamiz.
    // └───────────────────────────────────────────────────────────────┘
    if (t && restaurantId && t.restaurantId === restaurantId) {
      setTable(t);
    }
  }, [restaurantId]);

  useEffect(() => {
    // Stol rejimida manzil UMUMAN kerak emas — so'ramaymiz ham.
    if (table) {
      setLoadingAddress(false);
      return;
    }
    fetch("/api/proxy/me/address")
      .then((r) => (r.ok ? r.json() : null))
      .then((a: AddressDetails | null) => setAddress(a?.text?.trim() ? a : null))
      .catch(() => setAddress(null))
      .finally(() => setLoadingAddress(false));

    fetch("/api/proxy/me")
      .then((r) => (r.ok ? r.json() : null))
      .then((u) => setPhone(u?.phone ?? ""))
      .catch(() => {});
  }, [table]);

  useEffect(() => {
    if (!restaurantId || !cart.hydrated) return;
    const items = Object.entries(cartItems)
      .filter(([, qty]) => qty > 0)
      .map(([product_id, qty]) => ({ product_id, qty }));
    if (items.length === 0) return;
    setLoadingQuote(true);
    fetch(`/api/proxy/restaurants/${restaurantId}/quote`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ items }),
    })
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((q: Quote) => setQuote(q))
      // MUHIM: narx OLINMAGANDA `null` qoladi va pastda tugma
      // O'CHIRILADI. Avval bu holatda summa 0 so'm bo'lib ko'rinar,
      // lekin tugma baribir bosiladigan edi — foydalanuvchi "0 so'm"
      // deb yozilgan ekranda haqiqiy narxdagi buyurtma berib yuborishi
      // mumkin edi.
      .catch(() => setQuote(null))
      .finally(() => setLoadingQuote(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [restaurantId, JSON.stringify(cartItems), cart.hydrated]);

  async function placeOrder() {
    if (!restaurantId) return;
    // Manzil FAQAT yetkazib berishda majburiy.
    if (!table && (!address?.lat || !address?.lng)) {
      setPlaceError("Avval yetkazib berish manzilini tanlang");
      return;
    }
    const items = Object.entries(cartItems)
      .filter(([, qty]) => qty > 0)
      .map(([product_id, qty]) => ({ product_id, qty }));
    if (!idempotencyKeyRef.current) {
      idempotencyKeyRef.current = newIdempotencyKey();
    }
    setPlacing(true);
    setPlaceError(null);
    try {
      // Yetkazishda: koordinata va manzil tafsilotlari YUBORILMAYDI —
      // server ularni saqlangan manzildan (`/me/address`) o'zi oladi va
      // xizmat hududini tekshiradi. Shu bilan mijoz soxta koordinata
      // yubora olmaydi va podyezd/kvartira/izoh buyurtmaga kafolatli
      // biriktiriladi.
      //
      // Stolda: `table_token` yuboriladi va server o'zi qaysi stol
      // ekanini aniqlaydi. Stol nomini ("5") mijozdan OLMAYMIZ —
      // aks holda istalgan odam boshqa stol nomidan buyurtma bera
      // olardi. Token esa 32 baytlik sir.
      const res = await fetch("/api/proxy/orders", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          items,
          idempotency_key: idempotencyKeyRef.current,
          ...(table
            ? { table_token: table.token, party_size: partySize }
            : {}),
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Xato yuz berdi");
      idempotencyKeyRef.current = null;
      cart.clear();
      // Stol seansi buyurtmadan KEYIN ham saqlanadi: mijoz odatda
      // ovqatlanish davomida yana buyurtma qo'shadi (choy, shirinlik)
      // va har safar QR skanerlashi kerak bo'lmasin.
      router.push(`/orders/${data.id}`);
    } catch (e) {
      setPlaceError(e instanceof Error ? e.message : "Serverga ulanib bo'lmadi");
    } finally {
      setPlacing(false);
    }
  }

  if (!restaurantId || Object.values(cartItems).every((q) => q <= 0)) {
    return (
      <main className="mx-auto flex min-h-screen max-w-lg flex-col items-center justify-center gap-2 px-4 text-center">
        <p className="text-lg font-semibold">Savat bo&apos;sh</p>
        <a href="/" className="text-[#E53935] underline">
          Bosh sahifaga qaytish
        </a>
      </main>
    );
  }

  const subtotal = quote?.subtotal_tiyin ?? 0;
  const discount = quote?.discount_tiyin ?? 0;
  const total = quote?.total_tiyin ?? 0;
  const details = [
    address?.entrance && `Podyezd ${address.entrance}`,
    address?.floor && `Qavat ${address.floor}`,
    address?.apartment && `Kvartira ${address.apartment}`,
    address?.intercom && `Domofon ${address.intercom}`,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <MobileSheet maxWidthClassName="max-w-lg" className="px-3 pb-32 pt-2">
      <div className="relative flex items-center justify-center pb-1">
        <div className="absolute left-0">
          <BackButton onClick={() => goBack(() => router.push("/cart"))} />
        </div>
        <h1 className="text-base font-bold">Buyurtmani rasmiylashtirish</h1>
      </div>

      {/* ── Stol rejimi (QR kod) ─────────────────────────────────────── */}
      {table ? (
        <section className="tg-surface mt-3 rounded-2xl bg-neutral-100 p-4 dark:bg-[#242424]">
          <h2 className="text-xl font-extrabold">Stolda buyurtma</h2>

          <div className="mt-3 flex items-center gap-3 border-b border-neutral-300 pb-3 dark:border-neutral-700">
            <Utensils size={22} className="shrink-0" />
            <span className="min-w-0 flex-1">
              <span className="block font-semibold">
                {table.tableLabel ? `${table.tableLabel}-stol` : "Stol"}
              </span>
              {table.restaurantName && (
                <span className="tg-muted mt-0.5 block truncate text-sm text-neutral-500">
                  {table.restaurantName}
                </span>
              )}
            </span>
            {/* Chiqish yo'li: mijoz stol rejimidan qaytib, oddiy
                yetkazib berish buyurtmasi bera olishi kerak. Busiz
                u Mini App'ni butunlay yopishga majbur bo'lardi. */}
            <button
              type="button"
              onClick={() => {
                clearTableSession();
                setTable(null);
              }}
              className="shrink-0 text-sm text-neutral-500 underline"
            >
              Bekor qilish
            </button>
          </div>

          {/* ── Nechta kishi ──
              Narxga TA'SIR QILMAYDI — restoranga idish-tovoq, non va
              joy tayyorlash uchun kerak. */}
          <div className="flex items-center gap-3 py-3">
            <Users size={22} className="shrink-0" />
            <span className="flex-1">
              <span className="block font-semibold">Nechta kishi</span>
              <span className="tg-muted block text-sm text-neutral-500">
                Restoran joy tayyorlashi uchun
              </span>
            </span>
            <span className="flex items-center gap-3">
              <button
                type="button"
                aria-label="Kamaytirish"
                onClick={() => setPartySize((n) => Math.max(1, n - 1))}
                className="h-9 w-9 rounded-full bg-neutral-200 text-lg font-bold dark:bg-neutral-700"
              >
                −
              </button>
              <span className="w-6 text-center text-lg font-bold">
                {partySize}
              </span>
              <button
                type="button"
                aria-label="Ko'paytirish"
                // 50 — serverdagi `maxPartySize` bilan BIR XIL chegara.
                // Farq qilsa, mijoz kiritgan qiymat serverda rad
                // etilib, tushunarsiz xato chiqardi.
                onClick={() => setPartySize((n) => Math.min(50, n + 1))}
                className="h-9 w-9 rounded-full bg-neutral-200 text-lg font-bold dark:bg-neutral-700"
              >
                +
              </button>
            </span>
          </div>

          <div className="flex items-center gap-3 border-t border-neutral-300 pt-3 dark:border-neutral-700">
            <Phone size={22} className="shrink-0" />
            <span className="min-w-0 flex-1">
              <span className="tg-muted block text-sm text-neutral-500">
                Telefon
              </span>
              <span className="block font-semibold">{phone || "—"}</span>
            </span>
          </div>
        </section>
      ) : (
      <>
      {/* ── Qayerga yetkazamiz ───────────────────────────────────────── */}
      <section className="mt-3 rounded-2xl bg-neutral-100 p-4 dark:bg-[#242424]">
        <h2 className="text-xl font-extrabold">Qayerga yetkazamiz</h2>

        <Link
          href="/address?next=/checkout"
          className="mt-3 flex items-center gap-3 border-b border-neutral-300 pb-3 dark:border-neutral-700"
        >
          <Home size={22} className="shrink-0" />
          <span className="min-w-0 flex-1">
            <span className="block truncate font-semibold">
              {loadingAddress
                ? "Yuklanmoqda..."
                : (address?.text ?? "Manzilni xaritadan tanlang")}
            </span>
            {details && (
              <span className="mt-0.5 block truncate text-sm text-neutral-500">
                {details}
              </span>
            )}
          </span>
          <ChevronRight size={20} className="shrink-0 text-neutral-500" />
        </Link>

        <Link
          href="/address?next=/checkout"
          className="flex items-center gap-3 border-b border-neutral-300 py-3 dark:border-neutral-700"
        >
          <MessageSquare size={22} className="shrink-0" />
          <span className="min-w-0 flex-1 truncate">
            {address?.comment ? (
              address.comment
            ) : (
              <span className="text-neutral-500">Kuryer uchun izoh</span>
            )}
          </span>
          <ChevronRight size={20} className="shrink-0 text-neutral-500" />
        </Link>

        <div className="flex items-center gap-3 pt-3">
          <Phone size={22} className="shrink-0" />
          <span className="min-w-0 flex-1">
            <span className="block text-sm text-neutral-500">Qabul qiluvchi telefoni</span>
            <span className="block font-semibold">{phone || "—"}</span>
          </span>
        </div>
      </section>
      </>
      )}

      {/* ── To'lov ───────────────────────────────────────────────────── */}
      <section className="tg-surface mt-3 rounded-2xl bg-neutral-100 p-4 dark:bg-[#242424]">
        <h2 className="text-xl font-extrabold">To&apos;lov</h2>
        <div className="mt-3 flex items-center gap-3 rounded-xl border-2 border-[#FFD100] bg-white p-3 dark:bg-[#1A1A1A]">
          <Banknote size={22} className="shrink-0 text-green-600" />
          <span className="flex-1">
            <span className="block font-semibold">Naqd pul</span>
            <span className="tg-muted block text-sm text-neutral-500">
              {table
                ? "Ovqatlangach affitsiantga to'lanadi"
                : "Kuryerga qo'lda to'lanadi"}
            </span>
          </span>
        </div>
        <p className="tg-muted mt-2 text-xs text-neutral-500">
          Karta orqali to&apos;lov hozircha mavjud emas
        </p>
      </section>

      {/* ── Buyurtma qiymati ─────────────────────────────────────────── */}
      <section className="mt-3 rounded-2xl bg-neutral-100 p-4 dark:bg-[#242424]">
        <h2 className="text-xl font-extrabold">Buyurtma qiymati</h2>
        {loadingQuote ? (
          <p className="mt-3 text-sm text-neutral-500">Yuklanmoqda...</p>
        ) : !quote ? (
          <p className="mt-3 text-sm text-red-500">
            Narxni hisoblab bo&apos;lmadi. Internet aloqasini tekshirib,
            sahifani yangilang.
          </p>
        ) : (
          <div className="mt-3 space-y-2">
            <div className="flex justify-between">
              <span className="text-neutral-500">Savatdagi tovarlar</span>
              <span className="font-semibold">{formatSum(subtotal)}</span>
            </div>
            {discount > 0 && (
              <div className="flex justify-between text-green-600 dark:text-green-400">
                <span>
                  {quote?.promotion_name
                    ? `Aksiya: ${quote.promotion_name}`
                    : "Aksiya chegirmasi"}
                </span>
                <span className="font-bold">-{formatSum(discount)}</span>
              </div>
            )}
          </div>
        )}
      </section>

      <div className="safe-bottom fixed bottom-0 left-0 right-0 border-t border-neutral-200 bg-white px-4 pb-2 pt-1.5 dark:border-neutral-800 dark:bg-[#1A1A1A] md:dark:bg-[#121212]">
        <div className="mx-auto max-w-lg">
          <div className="mb-1 flex items-baseline justify-between">
            <span className="text-base font-extrabold">Jami</span>
            <span className="flex items-baseline gap-2">
              {discount > 0 && (
                <span className="text-sm text-neutral-500 line-through">
                  {formatSum(subtotal)}
                </span>
              )}
              <span className="text-base font-extrabold text-green-600 dark:text-green-400">
                {formatSum(loadingQuote ? subtotal : total)}
              </span>
            </span>
          </div>
          {placeError && <p className="mb-1 text-sm text-red-500">{placeError}</p>}
          <AppButton
            onClick={placeOrder}
            // `!quote` — narx serverdan olinmagan bo'lsa buyurtma
            // berishga YO'L QO'YILMAYDI (yuqoridagi izohga qarang).
            disabled={placing || loadingAddress || loadingQuote || !quote}
          >
            {placing ? "Yuborilmoqda..." : "Buyurtma berish"}
          </AppButton>
        </div>
      </div>
    </MobileSheet>
  );
}
