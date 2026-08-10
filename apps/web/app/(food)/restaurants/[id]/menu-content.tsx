"use client";

import { Search, ShoppingBasket, X } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { fullImageUrl } from "@/lib/images";
import { goBack } from "@/lib/nav";
import { categoryOf, computeProductDiscount } from "@/lib/promotions";
import { useFavorites } from "@/lib/use-favorites";
import type { ActivePromotion, Product, Restaurant } from "@/lib/types";
import { useCart } from "@/lib/cart-context";
import { useQuote } from "@/lib/use-quote";
import ProductCard from "./product-card";
import DiscountedTotal from "../../discounted-total";
import MobileSheet from "../../mobile-sheet";
import { BackButton } from "../../ui";

export default function MenuContent({
  restaurant,
  menu,
  promotions,
}: {
  restaurant: Restaurant;
  menu: Product[];
  promotions: ActivePromotion[];
}) {
  const router = useRouter();
  const cart = useCart();
  const [query, setQuery] = useState("");
  const [searchOpen, setSearchOpen] = useState(false);
  const { favoriteIds, onFavoriteChange } = useFavorites();
  const [activeCategory, setActiveCategory] = useState<string>("");

  const productsById = useMemo(
    () => new Map(menu.map((p) => [p.id, p])),
    [menu],
  );

  // Bu restoranga tegishli savat miqdorlari (boshqa restoran savati bo'lsa
  // — cart-context.tsx'dagi qoidaga ko'ra — bo'sh ko'rinadi).
  const cartItems =
    cart.restaurantId === restaurant.id ? cart.items : ({} as Record<string, number>);

  // _applyPromotions bilan bir xil: order-wide/turkum darajasidagi
  // aksiyalarni oldindan hisoblab qo'yamiz (har bir kartochkada qayta
  // hisoblamaslik uchun).
  const { promotedProductIds, promotedCategories, orderWidePromo } =
    useMemo(() => {
      const productIds = new Set<string>();
      const categories = new Set<string>();
      let orderWide = false;
      for (const p of promotions) {
        if (p.applies_to_orders) orderWide = true;
        if (p.applies_to_products) {
          for (const id of p.target_product_ids ?? []) productIds.add(id);
        }
        if (p.applies_to_categories) {
          for (const c of p.target_categories ?? []) categories.add(c);
        }
      }
      return {
        promotedProductIds: productIds,
        promotedCategories: categories,
        orderWidePromo: orderWide,
      };
    }, [promotions]);

  // Qidiruv endi FAQAT ikonka bosilganda ochiladigan overlay ichida
  // ishlaydi (Bosh sahifa bilan bir xil naqsh) — asosiy sahifa har doim
  // to'liq, filtrlanmagan menyuni ko'rsatadi.
  const searchResults = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return [];
    return menu.filter((p) => p.name.toLowerCase().includes(q));
  }, [menu, query]);

  const categories = useMemo(() => {
    const list: string[] = [];
    for (const p of menu) {
      const c = categoryOf(p);
      if (!list.includes(c)) list.push(c);
    }
    return list.length > 0 ? list : ["Menyu"];
  }, [menu]);

  // Skroll-kuzatuv (scroll-spy): foydalanuvchi qaysi turkum bo'limida
  // turganini AVTOMATIK aniqlab, mos chipni belgilaydi. Avval chiplarda
  // umuman faol holat yo'q edi — bosilganda ham qaysi biri tanlangani
  // bilinmasdi. Kuzatuv MobileSheet ichidagi skroll konteyneriga
  // nisbatan ishlaydi (sahifaning o'zi emas — 79-bandga qarang).
  const chipsRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const sections = categories
      .map((c) => document.getElementById(`cat-${encodeURIComponent(c)}`))
      .filter((el): el is HTMLElement => el !== null);
    if (sections.length === 0) return;

    const scroller = sections[0].closest(".overflow-y-auto");
    if (!scroller) return;

    // `IntersectionObserver` ATAYLAB ishlatilmadi: bir vaqtda BIR NECHTA
    // bo'lim kuzatuv oynasini kesib turishi mumkin (uzun bo'lim tugab,
    // keyingisi boshlanayotgan payt) va qaysi biri "joriy" ekanini
    // ishonchli aniqlab bo'lmaydi — natijada chip noto'g'ri turkumda
    // qotib qolardi. Bu yerda oddiy va aniq qoida ishlatiladi:
    // sarlavha ostidagi chiziqdan YUQORIDA boshlangan ENG OXIRGI bo'lim
    // — joriy bo'lim.
    const HEADER_LINE = 140;
    let raf = 0;
    const update = () => {
      raf = 0;
      let current = sections[0];
      for (const s of sections) {
        if (s.getBoundingClientRect().top <= HEADER_LINE) current = s;
        else break;
      }
      // Eng pastga yetganda — MAJBURAN oxirgi turkum. Sababi: oxirgi
      // bo'lim ekrandan qisqa bo'lsa, uning tepasi HEADER_LINE'dan
      // yuqoriga HECH QACHON chiqmaydi (sahifa bundan ortiq surilmaydi)
      // va chip oldingi turkumda qotib qolardi — aynan shu holat
      // qurilmada o'lchab topildi (Fast Food top=161 > 140).
      if (
        scroller.scrollTop + scroller.clientHeight >=
        scroller.scrollHeight - 4
      ) {
        current = sections[sections.length - 1];
      }
      setActiveCategory(decodeURIComponent(current.id.replace(/^cat-/, "")));
    };
    const onScroll = () => {
      if (!raf) raf = requestAnimationFrame(update);
    };
    scroller.addEventListener("scroll", onScroll, { passive: true });
    update(); // boshlang'ich holat — foydalanuvchi skroll qilmasa ham
    return () => {
      scroller.removeEventListener("scroll", onScroll);
      if (raf) cancelAnimationFrame(raf);
    };
  }, [categories]);

  // Faol chip ko'rinish maydonidan chiqib ketmasligi uchun — gorizontal
  // ro'yxatni avtomatik suramiz.
  //
  // MUHIM: `scrollIntoView` ISHLATILMAYDI — u BARCHA skroll qiluvchi
  // ota-elementlarni, jumladan VERTIKAL konteynerni ham suradi va
  // foydalanuvchining barmoq bilan skroll qilishiga xalaqit berardi
  // ("scroll qilib pastga-tepaga tushib chiqsa swipe bo'lmayapti").
  // Buning o'rniga FAQAT gorizontal konteynerning `scrollLeft`i
  // o'zgartiriladi — vertikal skrollga umuman tegilmaydi.
  useEffect(() => {
    const box = chipsRef.current;
    if (!activeCategory || !box) return;
    const el = box.querySelector<HTMLElement>(
      `[data-cat="${CSS.escape(activeCategory)}"]`,
    );
    if (!el) return;
    const target = el.offsetLeft - (box.clientWidth - el.clientWidth) / 2;
    box.scrollTo({ left: Math.max(0, target), behavior: "smooth" });
  }, [activeCategory]);

  const totalItems = Object.values(cartItems).reduce((a, b) => a + b, 0);

  const { totalTiyin: displayTotal, subtotalTiyin } = useQuote(
    restaurant.id,
    cartItems,
    productsById,
    promotions,
  );

  function renderProductCard(p: Product) {
    const discount = computeProductDiscount(p, promotions);
    const promoted =
      orderWidePromo ||
      discount !== null ||
      promotedProductIds.has(p.id) ||
      promotedCategories.has(categoryOf(p));
    const qty = cartItems[p.id] ?? 0;
    return (
      <ProductCard
        key={p.id}
        product={p}
        qty={qty}
        promoted={promoted}
        discount={discount}
        favorited={favoriteIds.has(p.id)}
        onAdd={() => cart.setQty(restaurant.id, p.id, qty + 1)}
        onRemove={() => cart.setQty(restaurant.id, p.id, qty - 1)}
        onQtyChange={(newQty) => cart.setQty(restaurant.id, p.id, newQty)}
        onFavoriteChange={onFavoriteChange}
      />
    );
  }

  return (
    <MobileSheet className="pb-24">
      <div className="sticky top-0 z-10 border-b border-neutral-200 bg-white/95 px-4 pb-3 pt-2.5 backdrop-blur dark:border-neutral-800 dark:bg-[#1A1A1A]/95 md:dark:bg-[#121212]/95">
        <div className="flex items-center gap-2">
          <BackButton onClick={() => goBack(() => router.push("/"))} />
          {/* Logo + nom O'RTADA — chap/o'ngdagi tugmalar bir xil
              kenglikda bo'lgani uchun markaz aniq to'g'ri keladi. */}
          <div className="flex min-w-0 flex-1 items-center justify-center gap-2">
            {restaurant.logo_url && (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={fullImageUrl(restaurant.logo_url)}
                alt=""
                className="h-7 w-7 shrink-0 rounded-md object-cover"
              />
            )}
            <h1 className="truncate text-lg font-bold">{restaurant.name}</h1>
          </div>
          <button
            type="button"
            onClick={() => setSearchOpen(true)}
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-neutral-600 active:bg-neutral-200 dark:text-neutral-300 dark:active:bg-neutral-700"
            aria-label="Qidirish"
          >
            <Search size={22} />
          </button>
        </div>
        {categories.length > 1 && (
          <div ref={chipsRef} className="mt-2.5 flex gap-2 overflow-x-auto pb-0.5">
            {categories.map((c) => {
              const active = c === activeCategory;
              return (
                <a
                  key={c}
                  data-cat={c}
                  href={`#cat-${encodeURIComponent(c)}`}
                  onClick={() => setActiveCategory(c)}
                  className={`shrink-0 rounded-full border px-3 py-1.5 text-sm transition-colors ${
                    active
                      ? "border-[#FFD100] bg-[#FFD100] font-semibold text-black"
                      : "border-neutral-200 dark:border-neutral-700"
                  }`}
                >
                  {c}
                </a>
              );
            })}
          </div>
        )}
      </div>

      {categories.map((c) => {
        const items = menu.filter((p) => categoryOf(p) === c);
        if (items.length === 0) return null;
        return (
          <section key={c} id={`cat-${encodeURIComponent(c)}`} className="px-4">
            <h2 className="pb-2 pt-5 text-lg font-bold">{c}</h2>
            <div className="grid grid-cols-2 gap-x-3.5 gap-y-5">
              {items.map(renderProductCard)}
            </div>
          </section>
        );
      })}

      {totalItems > 0 && (
        <Link
          href="/cart"
          // Balandligi umumiy tugmalar bilan bir xil (52px), lekin shakli
          // ATAYLAB dumaloq/suzuvchi — bu to'liq kenglikdagi asosiy
          // tugma emas, shuning uchun AppButton ishlatilmaydi.
          className="fixed bottom-5 right-5 flex h-[52px] items-center gap-3 rounded-full bg-[#FFD100] px-6 font-bold text-black shadow-lg"
        >
          <DiscountedTotal
            totalTiyin={displayTotal}
            subtotalTiyin={subtotalTiyin}
            className="font-bold"
          />
          <span className="relative">
            <ShoppingBasket size={22} />
            <span className="absolute -right-2 -top-2 flex h-4 min-w-4 items-center justify-center rounded-full bg-black px-1 text-[10px] font-bold text-white">
              {totalItems}
            </span>
          </span>
        </Link>
      )}

      {/* Qidiruv oynasi ham qolgan sahifalar kabi "karta" ko'rinishida:
          orqa fon + yuqori burchaklari dumaloqlangan panel. */}
      {searchOpen && (
        <div className="fixed inset-0 z-50 flex flex-col bg-neutral-200 pt-1 dark:bg-[#121212]">
          <div className="mx-auto flex min-h-0 w-full max-w-2xl flex-1 flex-col overflow-hidden rounded-t-[20px] bg-white dark:bg-[#1A1A1A]">
            <div className="safe-top flex items-center gap-2 px-4 pb-3">
              <div className="flex flex-1 items-center gap-2 rounded-xl border border-neutral-200 bg-neutral-50 px-3 dark:border-neutral-700 dark:bg-neutral-800">
                <Search size={20} className="shrink-0 text-neutral-400" />
                {/* eslint-disable-next-line jsx-a11y/no-autofocus -- qidiruv oynasi ATAYLAB ochilganda klaviatura darhol tayyor bo'lishi kerak */}
                <input
                  autoFocus
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder="Taom qidirish..."
                  className="w-full bg-transparent py-2.5 text-base outline-none"
                />
              </div>
              <button
                onClick={() => {
                  setSearchOpen(false);
                  setQuery("");
                }}
                className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-neutral-600 active:bg-neutral-200 dark:text-neutral-300 dark:active:bg-neutral-700"
                aria-label="Yopish"
              >
                <X size={22} />
              </button>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-4 py-5">
              {searchResults.length === 0 ? (
                <p className="py-10 text-center text-neutral-500">
                  {query ? "Mos taom topilmadi" : "Taom nomini yozing"}
                </p>
              ) : (
                <div className="grid grid-cols-2 gap-x-3.5 gap-y-5">
                  {searchResults.map(renderProductCard)}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </MobileSheet>
  );
}
