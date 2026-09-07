"use client";

import { ShoppingBag, Trash2, UtensilsCrossed } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";
import { useCart } from "@/lib/cart-context";
import { fetchRestaurantMenu } from "@/lib/client-fetch";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import { computeProductDiscount } from "@/lib/promotions";
import type { ActivePromotion, Product, Restaurant } from "@/lib/types";
import { useDesktopCheckout } from "./desktop-checkout-context";

// Navbardagi savat — image/savat.png namunasi.
//
// ┌─ NEGA SAHIFA EMAS, OCHILADIGAN BLOK ───────────────────────────────┐
// Mobil ko'rinishda savat alohida sahifa (`/cart`) — u yerda ekran tor
// va boshqa iloji yo'q. Kompyuterda esa sahifaga o'tish menyudan
// UZOQLASHTIRADI: mijoz taom qo'shadi, savatni ko'rish uchun sahifa
// almashadi, keyin orqaga qaytadi. Namunadagi yechim shu sababdan —
// savat navbardan pastga ochiladi, sahifa o'zgarmaydi.
// └───────────────────────────────────────────────────────────────────┘
//
// ┌─ BO'SH SAVATDA UMUMAN CHIZILMAYDI ─────────────────────────────────┐
// Doim turadigan, ichi bo'sh savat tugmasi joy egallaydi va hech qanday
// ma'lumot bermaydi. `hydrated` kutiladi — busiz `localStorage`
// o'qilgunicha tugma bir zumga "yo'q"dan "bor"ga sakrardi.
// └───────────────────────────────────────────────────────────────────┘

/** Bir qatorga sig'adigan rasm soni — qolgani "+N" ga yig'iladi. */
const THUMBS_PER_ROW = 4;

type MenuData = {
  restaurant: Restaurant | null;
  menu: Product[];
  promotions: ActivePromotion[];
};

export default function DesktopCartMenu() {
  const cart = useCart();
  const [open, setOpen] = useState(false);
  const [data, setData] = useState<Record<string, MenuData>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const rootRef = useRef<HTMLDivElement>(null);

  const cartIds = useMemo(() => Object.keys(cart.carts), [cart.carts]);

  // Menyu FAQAT blok ochilganda va faqat YETISHMAYOTGAN restoranlar
  // uchun so'raladi: savat tugmasi har sahifada turadi, har yuklashda
  // so'rov yuborish bekorga bo'lardi.
  useEffect(() => {
    if (!open) return;
    const missing = cartIds.filter((id) => !data[id]);
    if (missing.length === 0) return;
    let cancelled = false;
    void Promise.all(
      missing.map(async (id) => [id, await fetchRestaurantMenu(id)] as const),
    ).then((pairs) => {
      if (cancelled) return;
      setData((prev) => ({ ...prev, ...Object.fromEntries(pairs) }));
    });
    return () => {
      cancelled = true;
    };
  }, [open, cartIds, data]);

  // Tashqariga bosilsa yoki Escape bosilsa yopiladi.
  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  if (!cart.hydrated || cartIds.length === 0) return null;

  // Bitta savatda mahsulot soni ko'proq ma'lumot beradi; bir nechta
  // bo'lsa — savatlar soni (namunadagi "Корзины · 3" kabi).
  const label =
    cart.cartCount > 1
      ? `Savatlar · ${cart.cartCount}`
      : `Savat · ${cart.totalItems}`;

  return (
    <div ref={rootRef} className="relative shrink-0">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex h-11 items-center gap-2 rounded-full bg-brand px-4 text-sm font-bold text-white transition-colors hover:bg-brand-light"
      >
        <ShoppingBag size={17} />
        {label}
      </button>

      {open && (
        // ┌─ BALANDLIK VA SKROLL ────────────────────────────────────┐
        // Balandlik EKRANGA nisbatan cheklanadi: navbar `sticky`
        // bo'lgani uchun ro'yxat ekrandan chiqib ketsa, sahifani
        // skroll qilib ham unga yetib bo'lmaydi (blok navbar bilan
        // birga joyida qoladi) — pastdagi savatlar butunlay
        // ko'rinmasdi.
        //
        // `calc(100vh-96px)` — navbar balandligi + pastdagi zaxira.
        // `overscroll-contain` — ro'yxat oxiriga yetganda g'ildirak
        // ORQADAGI sahifani surib yubormaydi.
        <div className="ondex-scroll absolute right-0 top-[calc(100%+10px)] flex max-h-[calc(100vh-96px)] w-[380px] flex-col overflow-y-auto overscroll-contain rounded-2xl bg-[#1e1e1e] text-white shadow-2xl ring-1 ring-white/10">
          {cartIds.map((id, i) => (
            <CartBlock
              key={id}
              restaurantId={id}
              items={cart.carts[id]}
              data={data[id]}
              expanded={Boolean(expanded[id])}
              onToggleExpand={() =>
                setExpanded((prev) => ({ ...prev, [id]: !prev[id] }))
              }
              onRemove={() => {
                cart.removeCart(id);
                setExpanded((prev) => ({ ...prev, [id]: false }));
              }}
              onGo={() => {
                // Buyurtma FAOL savatdan rasmiylashtiriladi — shuning
                // uchun checkout'ga o'tishdan oldin faol savat aynan
                // shu restoranga o'tkaziladi. Busiz mijoz ikkinchi
                // savatning "Buyurtma berish"ini bosib, birinchisining
                // taomlarini rasmiylashtirib yuborardi.
                cart.setActive(id);
                setOpen(false);
              }}
              divider={i > 0}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function CartBlock({
  restaurantId,
  items,
  data,
  expanded,
  onToggleExpand,
  onRemove,
  onGo,
  divider,
}: {
  restaurantId: string;
  items: Record<string, number>;
  data?: MenuData;
  expanded: boolean;
  onToggleExpand: () => void;
  onRemove: () => void;
  onGo: () => void;
  divider: boolean;
}) {
  const checkout = useDesktopCheckout();

  const productsById = useMemo(
    () => new Map((data?.menu ?? []).map((p) => [p.id, p])),
    [data],
  );

  const lines = useMemo(
    () =>
      Object.entries(items)
        .filter(([, qty]) => qty > 0)
        .map(([id, qty]) => ({ id, qty, product: productsById.get(id) }))
        .filter((l): l is { id: string; qty: number; product: Product } =>
          Boolean(l.product),
        ),
    [items, productsById],
  );

  // Summa mijoz tomonida hisoblanadi — bu FAQAT ko'rish uchun. Yakuniy
  // narxni (yetkazish, aksiya shartlari) server beradi (`/checkout`).
  const subtotal = useMemo(() => {
    let sum = 0;
    for (const { product, qty } of lines) {
      const discount = computeProductDiscount(product, data?.promotions ?? []);
      const price = discount?.discountedPriceTiyin ?? product.price_tiyin ?? 0;
      sum += price * qty;
    }
    return sum;
  }, [lines, data]);

  const restaurant = data?.restaurant ?? null;
  const eta =
    restaurant && restaurant.eta_min_minutes > 0 && restaurant.eta_max_minutes > 0
      ? `${restaurant.eta_min_minutes}–${restaurant.eta_max_minutes} daq`
      : null;

  const hidden = Math.max(0, lines.length - THUMBS_PER_ROW);
  const visible = expanded ? lines : lines.slice(0, THUMBS_PER_ROW);

  return (
    // `shrink-0` — ota element `flex-col` (skroll uchun): busiz ko'p
    // savatda bloklar siqilib, rasm va tugmalar ezilib qolardi.
    <div
      className={
        divider ? "shrink-0 border-t border-white/10 p-4" : "shrink-0 p-4"
      }
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-[17px] font-bold">
            {restaurant?.name ?? "Savat"}
          </p>
          <p className="mt-0.5 text-[13px] text-white/50">
            {data ? formatSum(subtotal) : "Yuklanmoqda…"}
            {eta && ` · ${eta}`}
          </p>
        </div>

        <button
          type="button"
          onClick={onRemove}
          aria-label="Savatni o'chirish"
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-white/50 transition-colors hover:bg-white/10 hover:text-white"
        >
          <Trash2 size={17} />
        </button>
      </div>

      {/* Rasmlar. Sig'magani "+N" katagiga yig'iladi; u BOSILSA barcha
          mahsulotlar pastki qatorlarda to'liq ochiladi (avval ular
          umuman ko'rinmasdi — ya'ni savatning bir qismi yashirin
          qolardi). */}
      <div className="mt-3.5 flex flex-wrap gap-2">
        {!data ? (
          <div className="h-[62px] w-full animate-pulse rounded-xl bg-white/5" />
        ) : (
          <>
            {visible.map(({ id, product, qty }) => (
              <Thumb key={id} product={product} qty={qty} />
            ))}
            {!expanded && hidden > 0 && (
              <button
                type="button"
                onClick={onToggleExpand}
                aria-label={`Yana ${hidden} ta mahsulotni ko'rsatish`}
                className="flex h-[62px] w-[62px] shrink-0 items-center justify-center rounded-xl bg-white/10 text-sm font-bold text-white/70 transition-colors hover:bg-white/[0.18] hover:text-white"
              >
                +{hidden}
              </button>
            )}
          </>
        )}
      </div>

      {expanded && hidden > 0 && (
        <button
          type="button"
          onClick={onToggleExpand}
          className="mt-2 text-[13px] font-semibold text-white/50 hover:text-white"
        >
          Yig&apos;ish
        </button>
      )}

      <div className="mt-4 flex gap-2.5">
        <Link
          href={`/restaurants/${restaurantId}`}
          onClick={onGo}
          className="flex h-11 flex-1 items-center justify-center rounded-xl bg-white/10 text-sm font-semibold transition-colors hover:bg-white/[0.16]"
        >
          Menyuga
        </Link>
        {/* Kompyuterda rasmiylashtirish — OYNA, sahifa emas
            (`desktop-checkout-context.tsx`). Provider topilmasa
            (kutilmagan holat) eski `/checkout` sahifasiga tushadi —
            ya'ni tugma hech qachon "o'lik" bo'lmaydi. */}
        {checkout ? (
          <button
            type="button"
            onClick={() => {
              onGo();
              checkout.openCheckout(restaurantId);
            }}
            className="flex h-11 flex-1 items-center justify-center rounded-xl bg-brand text-sm font-bold text-white transition-colors hover:bg-brand-light"
          >
            Buyurtma berish
          </button>
        ) : (
          <Link
            href="/checkout"
            onClick={onGo}
            className="flex h-11 flex-1 items-center justify-center rounded-xl bg-brand text-sm font-bold text-white transition-colors hover:bg-brand-light"
          >
            Buyurtma berish
          </Link>
        )}
      </div>
    </div>
  );
}

function Thumb({ product, qty }: { product: Product; qty: number }) {
  return (
    <div className="relative h-[62px] w-[62px] shrink-0 overflow-hidden rounded-xl bg-white/5">
      {product.image_url ? (
        // eslint-disable-next-line @next/next/no-img-element -- manzil muhitga
        // qarab dinamik (R2/lokal disk), `next/image` uchun `remotePatterns`
        // oldindan belgilab bo'lmaydi
        <img
          src={fullImageUrl(product.image_url)}
          alt={product.name}
          title={product.name}
          className="h-full w-full object-cover"
        />
      ) : (
        <div
          title={product.name}
          className="flex h-full w-full items-center justify-center text-white/25"
        >
          <UtensilsCrossed size={20} />
        </div>
      )}

      {/* Miqdor — namunada yo'q, lekin bir xil rasmli ikki qator
          ("2 ta burger" va "1 ta burger") aks holda farqlanmasdi. */}
      {qty > 1 && (
        <span className="absolute bottom-0 right-0 rounded-tl-lg bg-black/75 px-1.5 py-0.5 text-[11px] font-bold">
          {qty}
        </span>
      )}
    </div>
  );
}
