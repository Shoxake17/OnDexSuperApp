"use client";

import { Banknote, CreditCard, MapPin, UtensilsCrossed, X } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { useCart } from "@/lib/cart-context";
import { fetchRestaurantMenu } from "@/lib/client-fetch";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import { newIdempotencyKey } from "@/lib/idempotency";
import { computeProductDiscount } from "@/lib/promotions";
import { useTableSession } from "@/lib/table-session";
import type { ActivePromotion, Product } from "@/lib/types";
import DesktopAddressDialog from "./desktop-address-dialog";

// Buyurtmani rasmiylashtirish — KOMPYUTER uchun modal oyna.
//
// ┌─ NEGA SAHIFA EMAS ─────────────────────────────────────────────────┐
// `/checkout` sahifasi mobil ko'rinishda qurilgan (`MobileSheet`,
// pastda yopishgan tugma). Kompyuterda unga o'tish ikki muammo
// tug'dirardi: (1) telefonga mo'ljallangan tor ustun keng ekranda
// bo'sh va g'alati ko'rinardi, (2) mijoz menyudan uzoqlashib,
// savatiga qaytish uchun orqaga bosishga majbur bo'lardi.
//
// Sahifaning O'ZI qoladi — mobil ko'rinish va Telegram Mini App
// undan foydalanadi.
// └────────────────────────────────────────────────────────────────────┘
//
// ┌─ SERVER QOIDALARI O'ZGARMAYDI ─────────────────────────────────────┐
// Yetkazishda koordinata va manzil tafsilotlari YUBORILMAYDI — server
// ularni saqlangan manzildan (`/me/address`) oladi va xizmat hududini
// tekshiradi, ya'ni mijoz soxta koordinata yubora olmaydi.
// Stol rejimida esa faqat `table_token` ketadi (32 baytlik sir) —
// stol nomi mijozdan OLINMAYDI, aks holda istalgan odam boshqa stol
// nomidan buyurtma bera olardi.
// `idempotency_key` — takroriy bosishda ikkinchi buyurtma
// yaratilmasligi uchun (`lib/idempotency.ts`).
// └────────────────────────────────────────────────────────────────────┘

type AddressDetails = {
  text?: string;
  lat?: number;
  lng?: number;
};

type Quote = {
  subtotal_tiyin: number;
  discount_tiyin: number;
  total_tiyin: number;
  promotion_name?: string;
  promotion_discount_tiyin?: number;
};

export default function DesktopCheckoutDialog({
  restaurantId,
  onClose,
}: {
  restaurantId: string;
  onClose: () => void;
}) {
  const router = useRouter();
  const cart = useCart();

  const [menu, setMenu] = useState<Product[]>([]);
  const [promotions, setPromotions] = useState<ActivePromotion[]>([]);
  const [restaurantName, setRestaurantName] = useState("");

  const [address, setAddress] = useState<AddressDetails | null>(null);
  const [loadingAddress, setLoadingAddress] = useState(true);
  const [addressOpen, setAddressOpen] = useState(false);

  const [quote, setQuote] = useState<Quote | null>(null);
  const [loadingQuote, setLoadingQuote] = useState(true);
  const [payMethod, setPayMethod] = useState<"cash" | "card">("cash");
  const [placing, setPlacing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const idempotencyKeyRef = useRef<string | null>(null);

  // Stol rejimi (QR) — savat sahifasi bilan AYNAN bir manbadan, aks
  // holda bir ekranda manzil so'ralib, boshqasida "stol buyurtmasi"
  // ko'rinardi.
  const { table } = useTableSession(restaurantId);
  const [partySize, setPartySize] = useState(2);

  const items = cart.itemsFor(restaurantId);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      // Buyurtma yuborilayotganda yopilmasin — mijoz natijani
      // ko'rmasdan qolib, ikkinchi marta bosishi mumkin edi.
      if (e.key === "Escape" && !placing) onClose();
    }
    document.addEventListener("keydown", onKey);
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prev;
    };
  }, [onClose, placing]);

  useEffect(() => {
    void fetchRestaurantMenu(restaurantId).then((d) => {
      setMenu(d.menu);
      setPromotions(d.promotions);
      setRestaurantName(d.restaurant?.name ?? "");
    });
  }, [restaurantId]);

  useEffect(() => {
    if (table) {
      setLoadingAddress(false);
      return;
    }
    fetch("/api/proxy/me/address")
      .then((r) => (r.ok ? r.json() : null))
      .then((a: AddressDetails | null) => setAddress(a?.text?.trim() ? a : null))
      .catch(() => setAddress(null))
      .finally(() => setLoadingAddress(false));
  }, [table]);

  useEffect(() => {
    const list = Object.entries(items)
      .filter(([, qty]) => qty > 0)
      .map(([product_id, qty]) => ({ product_id, qty }));
    if (list.length === 0) return;
    setLoadingQuote(true);
    fetch(`/api/proxy/restaurants/${restaurantId}/quote`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ items: list }),
    })
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((q: Quote) => setQuote(q))
      // Narx OLINMASA `null` qoladi va tugma O'CHIRILADI: aks holda
      // ekranda "0 so'm" ko'rinib, mijoz haqiqiy narxdagi buyurtmani
      // bilmasdan yuborishi mumkin edi.
      .catch(() => setQuote(null))
      .finally(() => setLoadingQuote(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [restaurantId, JSON.stringify(items)]);

  const lines = Object.entries(items)
    .filter(([, qty]) => qty > 0)
    .map(([id, qty]) => ({ id, qty, product: menu.find((p) => p.id === id) }))
    .filter((l): l is { id: string; qty: number; product: Product } =>
      Boolean(l.product),
    );

  const subtotal = quote?.subtotal_tiyin ?? 0;
  const discount = quote?.discount_tiyin ?? 0;
  const total = quote?.total_tiyin ?? 0;
  const needsAddress = !table && !address?.lat;
  const canPlace = !placing && quote !== null && !needsAddress && lines.length > 0;

  async function placeOrder() {
    if (!canPlace) return;
    const list = Object.entries(items)
      .filter(([, qty]) => qty > 0)
      .map(([product_id, qty]) => ({ product_id, qty }));
    if (!idempotencyKeyRef.current) {
      idempotencyKeyRef.current = newIdempotencyKey();
    }
    setPlacing(true);
    setError(null);
    try {
      const res = await fetch("/api/proxy/orders", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          items: list,
          idempotency_key: idempotencyKeyRef.current,
          payment_method: payMethod,
          ...(table ? { table_token: table.token, party_size: partySize } : {}),
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Xato yuz berdi");

      if (payMethod === "card") {
        const payRes = await fetch(`/api/proxy/orders/${data.id}/pay`, {
          method: "POST",
        });
        const pay = await payRes.json();
        // `pay_url` — HAR DOIM tashqi `https://`. Sxema qat'iy
        // tekshiriladi: `javascript:` yoki boshqa sxema
        // `location.href` ga tushsa XSS bo'lardi. Tekshiruv savat
        // tozalashdan OLDIN — noto'g'ri havolada buyurtma yo'qolmasin.
        if (
          !payRes.ok ||
          typeof pay.pay_url !== "string" ||
          !/^https:\/\//i.test(pay.pay_url)
        ) {
          throw new Error(
            pay.error ||
              "To'lov sahifasini ochib bo'lmadi. Buyurtma to'lanmagan holda " +
                "kutmoqda — «Buyurtmalarim» bo'limidan qayta urinib ko'ring.",
          );
        }
        idempotencyKeyRef.current = null;
        cart.removeCart(restaurantId);
        window.location.href = pay.pay_url;
        return;
      }

      idempotencyKeyRef.current = null;
      cart.removeCart(restaurantId);
      onClose();
      router.push(`/orders/${data.id}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Serverga ulanib bo'lmadi");
      setPlacing(false);
    }
  }

  return (
    <>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Buyurtmani rasmiylashtirish"
        className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-6"
        onMouseDown={(e) => {
          if (e.target === e.currentTarget && !placing) onClose();
        }}
      >
        <div className="flex max-h-[88vh] w-full max-w-[520px] flex-col overflow-hidden rounded-3xl border border-white/10 bg-[#1e1e1e] text-white shadow-2xl">
          <div className="flex items-start justify-between gap-4 px-6 pb-4 pt-5">
            <div className="min-w-0">
              <h2 className="text-[22px] font-extrabold">
                Buyurtmani rasmiylashtirish
              </h2>
              {restaurantName && (
                <p className="mt-1 truncate text-[14px] text-white/45">
                  {restaurantName}
                  {table && ` · ${table.tableLabel || "stol"}`}
                </p>
              )}
            </div>
            <button
              type="button"
              onClick={onClose}
              disabled={placing}
              aria-label="Yopish"
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-white/60 transition-colors hover:bg-white/10 hover:text-white disabled:opacity-40"
            >
              <X size={20} />
            </button>
          </div>

          <div className="ondex-scroll min-h-0 flex-1 overflow-y-auto px-6 pb-2">
            {/* ── Manzil (stol rejimida umuman kerak emas) ─────────── */}
            {!table && (
              <section className="rounded-2xl bg-white/[0.05] p-4">
                <div className="flex items-start gap-3">
                  <MapPin size={18} className="mt-0.5 shrink-0 text-white/50" />
                  <div className="min-w-0 flex-1">
                    <p className="text-[12px] text-white/40">Yetkazish manzili</p>
                    <p className="mt-0.5 truncate text-[15px] font-semibold">
                      {loadingAddress
                        ? "Yuklanmoqda…"
                        : (address?.text ?? "Tanlanmagan")}
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => setAddressOpen(true)}
                    className="shrink-0 rounded-xl bg-white/10 px-3.5 py-2 text-[13px] font-semibold transition-colors hover:bg-white/[0.16]"
                  >
                    {address ? "O'zgartirish" : "Tanlash"}
                  </button>
                </div>
              </section>
            )}

            {/* ── Stol rejimi: mehmonlar soni ──────────────────────── */}
            {table && (
              <section className="rounded-2xl bg-white/[0.05] p-4">
                <p className="text-[12px] text-white/40">Necha kishisiz?</p>
                <div className="mt-2 flex gap-2">
                  {[1, 2, 3, 4, 5, 6].map((n) => (
                    <button
                      key={n}
                      type="button"
                      onClick={() => setPartySize(n)}
                      className={`h-10 w-10 rounded-xl text-sm font-bold transition-colors ${
                        partySize === n
                          ? "bg-brand text-white"
                          : "bg-white/10 text-white/70 hover:bg-white/[0.16]"
                      }`}
                    >
                      {n}
                    </button>
                  ))}
                </div>
              </section>
            )}

            {/* ── Savat tarkibi ─────────────────────────────────────── */}
            <section className="mt-3 rounded-2xl bg-white/[0.05] p-4">
              <p className="text-[12px] text-white/40">Buyurtma tarkibi</p>
              <div className="mt-2.5 space-y-2.5">
                {lines.map(({ id, product, qty }) => {
                  const d = computeProductDiscount(product, promotions);
                  const price = d?.discountedPriceTiyin ?? product.price_tiyin ?? 0;
                  return (
                    <div key={id} className="flex items-center gap-3">
                      <div className="h-10 w-10 shrink-0 overflow-hidden rounded-lg bg-white/5">
                        {product.image_url ? (
                          // eslint-disable-next-line @next/next/no-img-element -- manzil dinamik
                          <img
                            src={fullImageUrl(product.image_url)}
                            alt=""
                            aria-hidden="true"
                            className="h-full w-full object-cover"
                          />
                        ) : (
                          <div className="flex h-full w-full items-center justify-center text-white/25">
                            <UtensilsCrossed size={16} />
                          </div>
                        )}
                      </div>
                      <p className="min-w-0 flex-1 truncate text-[14px]">
                        {product.name}
                      </p>
                      <p className="shrink-0 text-[13px] text-white/45">×{qty}</p>
                      <p className="w-[92px] shrink-0 text-right text-[14px] font-semibold">
                        {formatSum(price * qty)}
                      </p>
                    </div>
                  );
                })}
              </div>
            </section>

            {/* ── To'lov usuli ──────────────────────────────────────── */}
            <section className="mt-3 rounded-2xl bg-white/[0.05] p-4">
              <p className="text-[12px] text-white/40">To&apos;lov usuli</p>
              <div className="mt-2.5 grid grid-cols-2 gap-2.5">
                <PayOption
                  active={payMethod === "cash"}
                  onClick={() => setPayMethod("cash")}
                  Icon={Banknote}
                  label="Naqd"
                />
                <PayOption
                  active={payMethod === "card"}
                  onClick={() => setPayMethod("card")}
                  Icon={CreditCard}
                  label="Karta"
                />
              </div>
              {payMethod === "card" && (
                <p className="mt-2.5 text-[12px] leading-relaxed text-white/40">
                  Pul darhol yechilmaydi — vaqtincha bloklanadi va restoran
                  buyurtmani qabul qilgandagina yechiladi.
                </p>
              )}
            </section>
          </div>

          {/* ── Yakuniy summa va tugma ─────────────────────────────── */}
          <div className="shrink-0 border-t border-white/10 px-6 py-4">
            {loadingQuote ? (
              <p className="text-[13px] text-white/40">Narx hisoblanmoqda…</p>
            ) : !quote ? (
              <p className="text-[13px] text-red-400">
                Narxni hisoblab bo&apos;lmadi. Internet aloqasini tekshirib,
                qaytadan oching.
              </p>
            ) : (
              <div className="space-y-1.5 text-[14px]">
                <Row label="Taomlar" value={formatSum(subtotal)} />
                {discount > 0 && (
                  <Row
                    label={quote.promotion_name ? `Aksiya: ${quote.promotion_name}` : "Chegirma"}
                    value={`−${formatSum(discount)}`}
                    green
                  />
                )}
                <div className="flex items-baseline justify-between pt-1.5 text-[17px] font-bold">
                  <span>Jami</span>
                  <span>{formatSum(total)}</span>
                </div>
              </div>
            )}

            {needsAddress && !loadingAddress && (
              <p className="mt-3 text-[13px] text-white/50">
                Buyurtma berish uchun avval yetkazish manzilini tanlang.
              </p>
            )}
            {error && <p className="mt-3 text-[13px] text-red-400">{error}</p>}

            <button
              type="button"
              onClick={placeOrder}
              disabled={!canPlace}
              className="mt-4 h-12 w-full rounded-2xl bg-brand text-[15px] font-bold text-white transition-colors hover:bg-brand-light disabled:cursor-not-allowed disabled:bg-white/10 disabled:text-white/40"
            >
              {placing ? "Yuborilmoqda…" : "Buyurtma berish"}
            </button>
          </div>
        </div>
      </div>

      {addressOpen && (
        <DesktopAddressDialog
          onClose={() => setAddressOpen(false)}
          onSaved={(text) => {
            // Saqlangan manzilni QAYTA so'raymiz: koordinata ham kerak
            // (`needsAddress` shunga qaraydi), matnning o'zi yetarli emas.
            setAddress((prev) => ({ ...prev, text }));
            fetch("/api/proxy/me/address")
              .then((r) => (r.ok ? r.json() : null))
              .then((a: AddressDetails | null) => a && setAddress(a))
              .catch(() => {});
          }}
        />
      )}
    </>
  );
}

function Row({
  label,
  value,
  green,
}: {
  label: string;
  value: string;
  green?: boolean;
}) {
  return (
    <div className="flex items-baseline justify-between">
      <span className="text-white/45">{label}</span>
      <span className={green ? "font-semibold text-green-400" : "font-semibold"}>
        {value}
      </span>
    </div>
  );
}

function PayOption({
  active,
  onClick,
  Icon,
  label,
}: {
  active: boolean;
  onClick: () => void;
  Icon: typeof Banknote;
  label: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex h-12 items-center justify-center gap-2 rounded-xl text-[14px] font-semibold transition-colors ${
        active
          ? "bg-brand text-white"
          : "bg-white/10 text-white/70 hover:bg-white/[0.16]"
      }`}
    >
      <Icon size={17} />
      {label}
    </button>
  );
}
