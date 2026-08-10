"use client";

import { Banknote, ChevronRight, Home, MessageSquare, Phone } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { useCart } from "@/lib/cart-context";
import { formatSum } from "@/lib/format";
import { newIdempotencyKey } from "@/lib/idempotency";
import { goBack } from "@/lib/nav";
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

  const restaurantId = cart.restaurantId;
  const cartItems = cart.items;

  useEffect(() => {
    fetch("/api/proxy/me/address")
      .then((r) => (r.ok ? r.json() : null))
      .then((a: AddressDetails | null) => setAddress(a?.text?.trim() ? a : null))
      .catch(() => setAddress(null))
      .finally(() => setLoadingAddress(false));

    fetch("/api/proxy/me")
      .then((r) => (r.ok ? r.json() : null))
      .then((u) => setPhone(u?.phone ?? ""))
      .catch(() => {});
  }, []);

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
    if (!address?.lat || !address?.lng || !restaurantId) {
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
      // Koordinata va manzil tafsilotlari YUBORILMAYDI — server ularni
      // saqlangan manzildan (`/me/address`) o'zi oladi va xizmat hududini
      // tekshiradi. Shu bilan mijoz soxta koordinata yubora olmaydi va
      // podyezd/kvartira/izoh buyurtmaga kafolatli biriktiriladi.
      const res = await fetch("/api/proxy/orders", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          items,
          idempotency_key: idempotencyKeyRef.current,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Xato yuz berdi");
      idempotencyKeyRef.current = null;
      cart.clear();
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

      {/* ── To'lov ───────────────────────────────────────────────────── */}
      <section className="mt-3 rounded-2xl bg-neutral-100 p-4 dark:bg-[#242424]">
        <h2 className="text-xl font-extrabold">To&apos;lov</h2>
        <div className="mt-3 flex items-center gap-3 rounded-xl border-2 border-[#FFD100] bg-white p-3 dark:bg-[#1A1A1A]">
          <Banknote size={22} className="shrink-0 text-green-600" />
          <span className="flex-1">
            <span className="block font-semibold">Naqd pul</span>
            <span className="block text-sm text-neutral-500">
              Kuryerga qo&apos;lda to&apos;lanadi
            </span>
          </span>
        </div>
        <p className="mt-2 text-xs text-neutral-500">
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
