"use client";

import { Minus, Plus, Trash2, UtensilsCrossed } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { useCart } from "@/lib/cart-context";
import { fetchRestaurantMenu } from "@/lib/client-fetch";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import { categoryOf, computeProductDiscount } from "@/lib/promotions";
import { useFavorites } from "@/lib/use-favorites";
import { useQuote } from "@/lib/use-quote";
import { useTableSession } from "@/lib/table-session";
import type { ActivePromotion, Product, Restaurant } from "@/lib/types";
import ProductCard from "../restaurants/[id]/product-card";
import DiscountedTotal from "../discounted-total";
import MobileSheet from "../mobile-sheet";
import { AppButton, AppButtonLink, BackButton } from "../ui";

// cart_screen.dart bilan parity: savat qatorlari + miqdor, "Menyuni
// ochish" tugmasi, "Yana nimadir kerakmi?" (to'liq menyu, turkumlarga
// bo'lingan), pastda "Buyurtma berish". Savat GLOBAL Context'dan
// (lib/cart-context.tsx) o'qiladi — robots.txt bu sahifani indekslamaydi,
// shuning uchun to'liq client-side (SSR shart emas).
export default function CartPage() {
  const router = useRouter();
  const cart = useCart();
  const [restaurant, setRestaurant] = useState<Restaurant | null>(null);
  const [menu, setMenu] = useState<Product[]>([]);
  const [promotions, setPromotions] = useState<ActivePromotion[]>([]);
  const { favoriteIds, onFavoriteChange } = useFavorites();
  const [confirmingClear, setConfirmingClear] = useState(false);
  const [loading, setLoading] = useState(true);

  const restaurantId = cart.restaurantId;
  // Stol rejimi (QR kod) — pastdagi tugma qayerga olib borishini
  // belgilaydi. Qoida `lib/table-session.ts` da, rasmiylashtirish
  // sahifasi bilan BIR XIL manbadan.
  const { table } = useTableSession(restaurantId);

  useEffect(() => {
    if (!restaurantId) {
      setLoading(false);
      return;
    }
    setLoading(true);
    fetchRestaurantMenu(restaurantId).then(({ restaurant, menu, promotions }) => {
      setRestaurant(restaurant);
      setMenu(menu);
      setPromotions(promotions);
      setLoading(false);
    });
  }, [restaurantId]);

  const productsById = useMemo(
    () => new Map(menu.map((p) => [p.id, p])),
    [menu],
  );

  const {
    totalTiyin,
    subtotalTiyin,
    loading: quoteLoading,
    quoteLines,
  } = useQuote(restaurantId, cart.items, productsById, promotions);

  const cartEntries = useMemo(
    () =>
      Object.entries(cart.items)
        .filter(([, qty]) => qty > 0)
        .map(([id, qty]) => ({ id, qty, product: productsById.get(id) }))
        .filter((e) => e.product) as { id: string; qty: number; product: Product }[],
    [cart.items, productsById],
  );

  const menuByCategory = useMemo(() => {
    const map = new Map<string, Product[]>();
    for (const p of menu) {
      const c = categoryOf(p);
      if (!map.has(c)) map.set(c, []);
      map.get(c)!.push(p);
    }
    return map;
  }, [menu]);

  if (!cart.hydrated || loading) {
    return (
      <main className="flex min-h-screen items-center justify-center">
        <p className="text-neutral-500">Yuklanmoqda...</p>
      </main>
    );
  }

  if (!restaurantId || cartEntries.length === 0) {
    return (
      <main className="mx-auto flex min-h-screen max-w-2xl flex-col items-center justify-center gap-3 px-4 text-center">
        <p className="text-lg font-semibold">Savat bo&apos;sh</p>
        <p className="text-sm text-neutral-500">
          Buyurtma berish uchun avval restoran menyusidan taom tanlang.
        </p>
        <Link href="/" className="mt-2 text-[#E53935] underline">
          Bosh sahifaga qaytish
        </Link>
      </main>
    );
  }

  return (
    <MobileSheet className="pb-28">
        <div className="sticky top-0 z-10 flex items-center gap-2 border-b border-neutral-200 bg-white/95 px-4 pb-3 pt-2.5 backdrop-blur dark:border-neutral-800 dark:bg-[#1A1A1A]/95 md:dark:bg-[#121212]/95">
          <BackButton onClick={() => router.push(`/restaurants/${restaurantId}`)} />
          <div className="min-w-0 flex-1">
            <p className="truncate text-base font-semibold">
              {restaurant?.name ?? ""}
            </p>
            <DiscountedTotal
              totalTiyin={totalTiyin}
              subtotalTiyin={subtotalTiyin}
              className="text-sm font-bold text-green-600 dark:text-green-400"
            />
          </div>
          <button
            onClick={() => setConfirmingClear(true)}
            className="shrink-0 rounded-full p-2 text-neutral-500 hover:bg-neutral-100 dark:hover:bg-neutral-900"
            aria-label="Savatni tozalash"
          >
            <Trash2 size={20} />
          </button>
        </div>

        <div className="divide-y divide-neutral-100 dark:divide-neutral-900">
          {cartEntries.map(({ id, qty, product }) => {
            // Qator narxi AVVAL serverdan (quote.lines) — checkout'da
            // olinadigan pul ham o'sha. Javob yo'q bo'lsa (anonim
            // foydalanuvchi/tarmoq xatosi) mahalliy taxminga tushamiz.
            const lineSubtotal = product.price_tiyin * qty;
            const estimate = computeProductDiscount(
              product,
              promotions,
              subtotalTiyin,
            );
            const lineTotal =
              quoteLines.get(id) ??
              (estimate ? estimate.discountedPriceTiyin * qty : lineSubtotal);
            const weightUnit =
              product.weight_unit === "l" ? "L" : product.weight_unit;
            return (
              <div key={id} className="flex gap-3 px-4 py-3">
                <div className="h-16 w-16 shrink-0 overflow-hidden rounded-xl bg-neutral-100 dark:bg-neutral-800">
                  {product.image_url ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={fullImageUrl(product.image_url)}
                      alt={product.name}
                      className="h-full w-full object-cover"
                    />
                  ) : (
                    <div className="flex h-full items-center justify-center text-neutral-400">
                      <UtensilsCrossed size={24} />
                    </div>
                  )}
                </div>
                <div className="flex-1">
                  <p className="font-semibold">
                    {product.name}
                    {product.weight > 0 && (
                      <span className="font-normal text-neutral-400">
                        {"  "}
                        {product.weight % 1 === 0
                          ? product.weight
                          : product.weight.toFixed(1)}{" "}
                        {weightUnit}
                      </span>
                    )}
                  </p>
                  {lineTotal < lineSubtotal ? (
                    <div className="mt-1 flex items-baseline gap-1.5">
                      <span className="font-bold text-[#E53935]">
                        {formatSum(lineTotal)}
                      </span>
                      <span className="text-xs text-neutral-500 line-through">
                        {formatSum(lineSubtotal)}
                      </span>
                    </div>
                  ) : (
                    <p className="mt-1 font-bold">{formatSum(lineTotal)}</p>
                  )}
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <button
                    onClick={() => cart.setQty(restaurantId, id, qty - 1)}
                    className="flex h-8 w-8 items-center justify-center rounded-full bg-white shadow-md active:scale-95 dark:bg-neutral-800"
                  >
                    <Minus size={18} />
                  </button>
                  <span className="w-4 text-center font-bold">{qty}</span>
                  <button
                    onClick={() => cart.setQty(restaurantId, id, qty + 1)}
                    className="flex h-8 w-8 items-center justify-center rounded-full bg-white shadow-md active:scale-95 dark:bg-neutral-800"
                  >
                    <Plus size={18} />
                  </button>
                </div>
              </div>
            );
          })}
        </div>

        <div className="px-4 py-3">
          <AppButtonLink href={`/restaurants/${restaurantId}`} variant="outline">
            Menyuni ochish
          </AppButtonLink>
        </div>

        {menuByCategory.size > 0 && (
          <div className="px-4">
            <h2 className="pb-2 pt-6 text-xl font-bold">Yana nimadir kerakmi?</h2>
            {[...menuByCategory.entries()].map(([category, items]) => (
              <section key={category}>
                <h3 className="pb-2 pt-4 text-base font-semibold">{category}</h3>
                <div className="grid grid-cols-2 gap-x-3.5 gap-y-5">
                  {items.map((p) => {
                    const discount = computeProductDiscount(
                      p,
                      promotions,
                      subtotalTiyin,
                    );
                    const qty = cart.items[p.id] ?? 0;
                    return (
                      <ProductCard
                        key={p.id}
                        product={p}
                        qty={qty}
                        promoted={discount !== null}
                        discount={discount}
                        favorited={favoriteIds.has(p.id)}
                        onAdd={() => cart.setQty(restaurantId, p.id, qty + 1)}
                        onRemove={() => cart.setQty(restaurantId, p.id, qty - 1)}
                        onQtyChange={(newQty) => cart.setQty(restaurantId, p.id, newQty)}
                        onFavoriteChange={onFavoriteChange}
                      />
                    );
                  })}
                </div>
              </section>
            ))}
          </div>
        )}

        <div className="safe-bottom fixed bottom-0 left-0 right-0 border-t border-neutral-200 bg-white px-4 pb-2 pt-1.5 dark:border-neutral-800 dark:bg-[#1A1A1A] md:dark:bg-[#121212]">
          {/* ┌─ QAYERGA OLIB BORADI ─────────────────────────────────┐
              Yetkazishda — avval manzil (xarita), Yandex Go naqshi:
              yetkazish nuqtasi rasmiylashtirishdan OLDIN aniqlanadi.

              Stolda (QR kod) — TO'G'RIDAN-TO'G'RI rasmiylashtirishga.
              Taom stolga keladi, ya'ni manzilning ma'nosi yo'q; avval
              bu shart emasligi FAQAT rasmiylashtirish sahifasida
              hisobga olinardi va mijoz baribir xarita ekranidan
              o'tishga majbur bo'lardi.
              └───────────────────────────────────────────────────────┘ */}
          <AppButtonLink
            href={table ? "/checkout" : "/address?next=/checkout"}
            spread
            className="mx-auto max-w-2xl"
          >
            <span>Buyurtma rasmiylashtirish</span>
            <span className="flex items-center gap-2">
              {quoteLoading && (
                <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-black/40 border-t-black" />
              )}
              <DiscountedTotal
                totalTiyin={totalTiyin}
                subtotalTiyin={subtotalTiyin}
                className="font-bold"
                align="right"
              />
            </span>
          </AppButtonLink>
        </div>

        {confirmingClear && (
          <div className="fixed inset-0 z-20 flex items-center justify-center bg-black/50 p-4">
            <div className="w-full max-w-sm rounded-2xl bg-white p-5 dark:bg-neutral-900">
              <h3 className="text-lg font-bold">Savat tozalansinmi?</h3>
              <p className="mt-1 text-sm text-neutral-500">
                Barcha tanlangan taomlar savatdan olib tashlanadi.
              </p>
              <div className="mt-4 flex gap-3">
                <AppButton variant="outline" onClick={() => setConfirmingClear(false)}>
                  Yo&apos;q
                </AppButton>
                <AppButton
                  variant="danger"
                  onClick={() => {
                    cart.clear();
                    setConfirmingClear(false);
                  }}
                >
                  Ha, tozalash
                </AppButton>
              </div>
            </div>
          </div>
        )}
    </MobileSheet>
  );
}
