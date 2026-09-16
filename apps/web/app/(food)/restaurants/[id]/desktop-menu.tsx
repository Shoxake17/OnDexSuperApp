"use client";

import {
  ArrowLeft,
  ChevronLeft,
  ChevronRight,
  Clock,
  MapPin,
  Minus,
  Plus,
  Star,
  UtensilsCrossed,
} from "lucide-react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useCart } from "@/lib/cart-context";
import { formatSum } from "@/lib/format";
import { fullImageUrl } from "@/lib/images";
import { categoryOf, computeProductDiscount } from "@/lib/promotions";
import { closedMessage } from "@/lib/restaurant-status";
import type { ActivePromotion, Product, Restaurant } from "@/lib/types";
import { useRestaurantOpenStatus } from "@/lib/use-restaurant-open-status";
import { useDesktopCheckout } from "../../desktop-checkout-context";
import DesktopNavbar from "../../desktop-navbar";
import FavoriteButton from "./favorite-button";

// Restoran menyusining KOMPYUTER ko'rinishi — image/resto_menu.png.
//
// ┌─ TUZILISH ─────────────────────────────────────────────────────────┐
// Chapda: restoran sarlavhasi (nom, teglar, reyting/vaqt chiplari),
// turkum yorliqlari va taomlar to'ri. O'ngda: YOPISHIB turadigan savat
// paneli — namunadagi kabi. Mobil ko'rinish (`menu-content.tsx`)
// o'zgarishsiz qoladi va `md:` dan kichik ekranda ishlaydi.
// └────────────────────────────────────────────────────────────────────┘
//
// ┌─ NEGA SAVAT O'NGDA, ALOHIDA PANEL ─────────────────────────────────┐
// Telefonda savat pastdagi suzuvchi tugma orqali ochiladi — u yerda
// ekran tor. Kompyuterda esa keng bo'sh joy bor va mijoz taom qo'shib
// borganda savat DARHOL ko'rinib turishi kerak: nima qo'shilgani,
// summa qancha bo'lgani. Har safar boshqa sahifaga o'tish shu oqimni
// uzardi.
// └────────────────────────────────────────────────────────────────────┘
export default function DesktopMenu({
  restaurant,
  menu,
  promotions,
  signedIn,
  favoriteIds,
  onFavoriteChange,
}: {
  restaurant: Restaurant;
  menu: Product[];
  promotions: ActivePromotion[];
  signedIn: boolean;
  favoriteIds: Set<string>;
  onFavoriteChange: (productId: string, favorited: boolean) => void;
}) {
  const router = useRouter();
  const cart = useCart();
  const checkout = useDesktopCheckout();
  const [activeCategory, setActiveCategory] = useState<string | null>(null);

  // ANIQ shu restoran savati — faol savat boshqa restoranniki bo'lsa
  // ham (`lib/cart-context.tsx` dagi `itemsFor` izohiga qarang).
  const items = cart.itemsFor(restaurant.id);

  const categories = useMemo(() => {
    const list: string[] = [];
    for (const p of menu) {
      const c = categoryOf(p);
      if (!list.includes(c)) list.push(c);
    }
    return list;
  }, [menu]);

  const sections = useMemo(() => {
    const map = new Map<string, Product[]>();
    for (const p of menu) {
      const c = categoryOf(p);
      if (!map.has(c)) map.set(c, []);
      map.get(c)!.push(p);
    }
    if (!activeCategory) return [...map.entries()];
    return [...map.entries()].filter(([c]) => c === activeCategory);
  }, [menu, activeCategory]);

  const productsById = useMemo(
    () => new Map(menu.map((p) => [p.id, p])),
    [menu],
  );

  const hasRating = restaurant.rating > 0;
  const hasEta =
    restaurant.eta_min_minutes > 0 && restaurant.eta_max_minutes > 0;

  // HOZIR buyurtma qabul qiladimi — ish vaqti bilan (mobil menyu bilan
  // bitta hook). Avval `restaurant.open` (qo'lda tugma) ishlatilardi.
  const { status: openStatus, detail: openDetail, now } =
    useRestaurantOpenStatus(restaurant);
  const orderable = openStatus.open;

  return (
    // ┌─ SAHIFA FONI — NAVBAR BILAN BIR XIL KULRANG ─────────────────┐
    // Fon to'q kulrang (`#302F2D`, navbar bilan bir xil), menyuning
    // O'ZI esa undan to'qroq blok (`#141414`). Shunda blok fonda
    // "yotgan qog'oz" bo'lib ko'rinadi. Teskarisi (qora fon + to'qroq
    // blok) da chegara sezilmasdi va sahifa yaxlit qora dog' edi.
    //
    // Sahifa O'ZI skroll BO'LMAYDI (`h-dvh` + `overflow-hidden`):
    // skroll bloklarning ICHIDA. Shu sababli brauzerning qo'pol
    // standart polosasi umuman chiqmaydi va navbar doim joyida
    // qoladi (`.ondex-scroll` — `globals.css`).
    // └───────────────────────────────────────────────────────────────┘
    <div className="flex h-dvh flex-col overflow-hidden bg-[#302F2D] text-white">
      <DesktopNavbar
        signedIn={signedIn}
        scopeLabel={restaurant.name}
        onClearScope={() => router.push("/")}
        placeholder="Restoran ichidan qidirish"
      />

      {/* Menyu bloki ATAYLAB bosh sahifadan kengroq: bu yerda o'ngda
          savat paneli ham bor, chap ustun esa taomlar to'ri — chekinish
          katta bo'lsa bir qatorga kamroq karta sig'adi va menyu
          uzunroq skroll bo'lardi. Bosh sahifadagi chekinish (`px-12/
          20/24`) o'zgarishsiz qoladi — u yerda kontent kartalardan
          iborat va keng hoshiya yaxshi ko'rinadi. */}
      <div className="mx-auto flex w-full min-h-0 max-w-[1800px] flex-1 gap-4 px-6 py-5 xl:px-10 2xl:px-14">
        {/* ┌─ ORQAGA — BLOKDAN TASHQARIDA ─────────────────────────────┐
            Namunadagi (image/resto_menu.png) joylashuv: tugma blokning
            ichida emas, uning CHAP TOMONIDA, sahifa hoshiyasida
            "suzib" turadi. Bu shunchaki did emas — blok ichidagi
            hamma narsa restoranga tegishli (nomi, menyusi), bu tugma
            esa undan CHIQIB ketish amali. Tashqarida turgani uni
            kontentdan ajratadi va u skroll bilan ham yo'qolmaydi
            (blok ichida bo'lsa, pastga surilganda ko'rinmay qolardi).

            `router.push("/")` — `router.back()` EMAS: brauzer
            tarixidagi oldingi yozuv begona sayt bo'lishi mumkin
            (havola orqali to'g'ridan-to'g'ri kirilgan bo'lsa),
            o'shanda "orqaga" mijozni OnDex'dan chiqarib yuborardi.

            Bu ko'rinish faqat kompyuterda (`xl:`) chiziladi, ya'ni
            Flutter WebView'ida render bo'lmaydi — shuning uchun
            `lib/nav.ts` dagi `goBack` (native oynani yopish) bu
            yerda kerak emas.
            └───────────────────────────────────────────────────────────┘ */}
        <button
          type="button"
          onClick={() => router.push("/")}
          aria-label="Restoranlar ro'yxatiga qaytish"
          // Foni menyu bloki bilan BIR XIL (`#141414`), chegarasi ham —
          // shunda u kulrang sahifada "blok oilasiga" tegishli
          // ko'rinadi. `bg-white/10` bo'lganda esa fonning ochroq
          // dog'idek edi, ya'ni bosiladigan element ekani sezilmasdi.
          className="mt-1 flex h-11 w-11 shrink-0 items-center justify-center self-start rounded-full border border-white/10 bg-[#141414] text-white transition-colors hover:bg-[#1f1f1f]"
        >
          <ArrowLeft size={20} />
        </button>

        {/* Chap ustun — IKKI alohida blok: restoran ma'lumoti va menyu. */}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-4">
          {/* ┌─ 1-BLOK: RESTORAN MA'LUMOTI ──────────────────────────────┐
              Menyudan AJRATILGAN va skroll bilan ketmaydi: nom, turkum,
              manzil va holat — mijoz menyuni pastga surganda ham
              "men qayerdaman va bu yer ochiqmi" degan savolga javob
              ko'z oldida qolishi kerak.

              Har bir bo'lak MUSTAQIL chiziladi: yo'q bo'lsa joyini ham
              egallamaydi (avval chiplar qatori `hasRating || hasEta`
              sharti ichida edi va ma'lumotda ikkalasi ham nol bo'lgani
              uchun manzil hech qachon ko'rinmasdi, o'rnida esa quruq
              joy qolardi).
              └───────────────────────────────────────────────────────────┘ */}
          <div className="shrink-0 rounded-3xl border border-white/10 bg-[#141414] px-6 py-5">
            <h1 className="text-[28px] font-extrabold leading-none">
              {restaurant.name}
            </h1>

            {restaurant.tags.trim() && (
              <p className="mt-2 text-[15px] text-white/45">{restaurant.tags}</p>
            )}

            <div className="mt-3.5 flex flex-wrap gap-2.5">
                {hasRating && (
                  <Chip>
                    <Star size={15} className="fill-brand text-brand" />
                    {restaurant.rating.toFixed(1)}
                    {restaurant.rating_count > 0 &&
                      ` · ${restaurant.rating_count} baho`}
                  </Chip>
                )}
                {hasEta && (
                  <Chip>
                    <Clock size={15} className="text-white/60" />
                    {restaurant.eta_min_minutes}–{restaurant.eta_max_minutes} daq
                  </Chip>
                )}
                {restaurant.address && (
                  <Chip>
                    <MapPin size={15} className="text-white/60" />
                    {restaurant.address}
                  </Chip>
                )}
                {/* Holat chipi DOIM chiziladi (avval reyting/manzil
                    bo'lmasa u ham yo'qolardi). */}
                <Chip>
                  <span
                    className={`h-2 w-2 rounded-full ${
                      orderable ? "bg-green-500" : "bg-red-500"
                    }`}
                  />
                  {orderable ? "Ochiq" : "Yopiq"}
                  {openDetail && (
                    <span className="font-medium text-white/55">· {openDetail}</span>
                  )}
                </Chip>
            </div>

            {!orderable && (
              <p
                role="status"
                className="mt-3.5 rounded-2xl border border-amber-400/30 bg-amber-400/10 px-4 py-2.5 text-[14px] font-semibold text-amber-200"
              >
                {closedMessage(openStatus, now)}
                <span className="font-normal text-amber-100/70">
                  {" — menyuni ko'rish mumkin, buyurtma restoran ochilgach qabul qilinadi."}
                </span>
              </p>
            )}
          </div>

          {/* ┌─ 2-BLOK: MENYU (yagona skroll qiladigan qism) ────────────┐
              Chekinish blokning O'ZIDA yo'q: yorliqlar (`sticky top-0`)
              uning eng tepasiga yopishadi va ostidan kontent sizib
              o'tmaydi. Avval bu yerda `p-6` turardi va o'sha 24px
              yo'lakdan kartalar ko'rinib qolardi (image/cate.png).
              └───────────────────────────────────────────────────────────┘ */}
          <div className="ondex-scroll min-h-0 flex-1 overflow-y-auto overscroll-contain rounded-3xl border border-white/10 bg-[#141414]">
          {categories.length > 0 && (
            <CategoryTabs
              categories={categories}
              active={activeCategory}
              onSelect={setActiveCategory}
            />
          )}

          {menu.length === 0 ? (
            <p className="py-20 text-center text-white/40">
              Menyu hozircha bo&apos;sh
            </p>
          ) : (
            sections.map(([category, products]) => (
              <section key={category} className="px-6 pb-2 pt-7">
                <h2 className="mb-4 text-xl font-bold">{category}</h2>
                <div className="grid grid-cols-2 gap-x-5 gap-y-7 lg:grid-cols-3 2xl:grid-cols-4">
                  {products.map((p) => (
                    <MenuCard
                      key={p.id}
                      product={p}
                      qty={items[p.id] ?? 0}
                      discount={computeProductDiscount(p, promotions)}
                      favorited={favoriteIds.has(p.id)}
                      onFavoriteChange={onFavoriteChange}
                      onQty={(qty) => cart.setQty(restaurant.id, p.id, qty)}
                      orderable={orderable}
                    />
                  ))}
                </div>
              </section>
            ))
          )}
          </div>
        </div>

        {/* ── O'ng ustun: savat ─────────────────────────────────────── */}
        <CartPanel
          orderable={orderable}
          items={items}
          productsById={productsById}
          promotions={promotions}
          onQty={(productId, qty) => cart.setQty(restaurant.id, productId, qty)}
          onCheckout={() => {
            cart.setActive(restaurant.id);
            // Kompyuterda rasmiylashtirish OYNADA ochiladi — mijoz
            // menyudan uzoqlashmaydi. Provider bo'lmasa (kutilmagan
            // holat) eski sahifaga tushadi.
            if (checkout) checkout.openCheckout(restaurant.id);
            else router.push("/checkout");
          }}
        />
      </div>
    </div>
  );
}

function Chip({ children }: { children: React.ReactNode }) {
  return (
    <span className="flex items-center gap-2 rounded-full bg-white/10 px-3.5 py-2 text-sm font-semibold">
      {children}
    </span>
  );
}

// ── Turkum yorliqlari ────────────────────────────────────────────────
//
// Bosh sahifadagi turkum qatori bilan bir xil muammo va yechim:
// `.no-scrollbar` polosani yashiradi, shuning uchun surish tugmalari
// KERAK (`desktop-home.tsx` dagi `CategoryRow` izohiga qarang).
function CategoryTabs({
  categories,
  active,
  onSelect,
}: {
  categories: string[];
  active: string | null;
  onSelect: (c: string | null) => void;
}) {
  const trackRef = useRef<HTMLDivElement>(null);
  const [canLeft, setCanLeft] = useState(false);
  const [canRight, setCanRight] = useState(false);

  const sync = useCallback(() => {
    const el = trackRef.current;
    if (!el) return;
    setCanLeft(el.scrollLeft > 2);
    setCanRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 2);
  }, []);

  useEffect(() => {
    const el = trackRef.current;
    if (!el) return;
    sync();
    el.addEventListener("scroll", sync, { passive: true });
    window.addEventListener("resize", sync);
    return () => {
      el.removeEventListener("scroll", sync);
      window.removeEventListener("resize", sync);
    };
  }, [sync, categories.length]);

  function nudge(direction: 1 | -1) {
    const el = trackRef.current;
    if (!el) return;
    el.scrollBy({ left: direction * el.clientWidth * 0.8, behavior: "smooth" });
  }

  return (
    // `sticky` — menyu blok ICHIDA skroll bo'lgani uchun yorliqlar
    // tepada osilib qoladi: mijoz uzun ro'yxatni surganda ham qaysi
    // turkumda ekanini ko'radi va bir bosishda boshqasiga o'tadi.
    // Foni blok foni bilan bir xil, aks holda ostidan kartalar
    // ko'rinib o'tardi.
    // `top-0` — skroll idishining eng tepasi. Idishda chekinish YO'Q,
    // shuning uchun yorliqlar ostidan kontent sizib o'tmaydi.
    // `rounded-t-3xl` — yorliqlar blokning eng birinchi elementi, ya'ni
    // foni blokning dumaloq burchaklarini takrorlashi kerak; busiz
    // to'rtburchak fon burchaklardan chiqib turardi.
    <div className="sticky top-0 z-10 rounded-t-3xl border-b border-white/10 bg-[#141414] px-6 pt-5">
      <div ref={trackRef} className="no-scrollbar flex gap-7 overflow-x-auto">
        <Tab active={active === null} onClick={() => onSelect(null)}>
          Hammasi
        </Tab>
        {categories.map((c) => (
          <Tab key={c} active={active === c} onClick={() => onSelect(c)}>
            {c}
          </Tab>
        ))}
      </div>

      {canLeft && (
        <button
          type="button"
          onClick={() => nudge(-1)}
          aria-label="Chapga surish"
          className="absolute left-0 top-1/2 flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-full bg-[#302F2D] text-white shadow-lg ring-1 ring-white/10"
        >
          <ChevronLeft size={17} />
        </button>
      )}
      {canRight && (
        <button
          type="button"
          onClick={() => nudge(1)}
          aria-label="O'ngga surish"
          className="absolute right-0 top-1/2 flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-full bg-[#302F2D] text-white shadow-lg ring-1 ring-white/10"
        >
          <ChevronRight size={17} />
        </button>
      )}
    </div>
  );
}

function Tab({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`shrink-0 whitespace-nowrap border-b-2 pb-3 pt-1 text-[15px] font-semibold transition-colors ${
        active
          ? "border-white text-white"
          : "border-transparent text-white/45 hover:text-white/70"
      }`}
    >
      {children}
    </button>
  );
}

// ── Taom kartasi (to'q mavzu) ────────────────────────────────────────
function MenuCard({
  product,
  qty,
  discount,
  favorited,
  onFavoriteChange,
  onQty,
  orderable,
}: {
  product: Product;
  qty: number;
  discount: ReturnType<typeof computeProductDiscount>;
  favorited: boolean;
  onFavoriteChange: (productId: string, favorited: boolean) => void;
  onQty: (qty: number) => void;
  /** Restoran HOZIR buyurtma qabul qiladimi — yo'q bo'lsa qo'shib bo'lmaydi. */
  orderable: boolean;
}) {
  const price = discount?.discountedPriceTiyin ?? product.price_tiyin ?? 0;
  const available = product.available;

  return (
    <div className={available ? "" : "opacity-40"}>
      <div className="relative aspect-square w-full overflow-hidden rounded-2xl bg-white/5">
        {product.image_url ? (
          // eslint-disable-next-line @next/next/no-img-element -- manzil muhitga
          // qarab dinamik (R2/lokal disk)
          <img
            src={fullImageUrl(product.image_url)}
            alt={product.name}
            className="h-full w-full object-cover"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-white/20">
            <UtensilsCrossed size={36} />
          </div>
        )}

        {discount && (
          <span className="absolute left-3 top-3 rounded-full bg-brand px-2.5 py-1 text-xs font-bold">
            {discount.label}
          </span>
        )}

        <div className="absolute right-3 top-3">
          <FavoriteButton
            productId={product.id}
            initialFavorited={favorited}
            onChanged={onFavoriteChange}
          />
        </div>

        {/* Miqdor boshqaruvi rasm USTIDA — to'rda har karta bir xil
            balandlikda qoladi va tugma doim bir joyda bo'ladi. */}
        {available && (
          <div className="absolute bottom-3 right-3">
            {qty > 0 ? (
              <div className="flex items-center gap-1 rounded-full bg-white p-1 text-[#141414] shadow-lg">
                <button
                  type="button"
                  onClick={() => onQty(qty - 1)}
                  aria-label="Kamaytirish"
                  className="flex h-8 w-8 items-center justify-center rounded-full hover:bg-black/5"
                >
                  <Minus size={16} />
                </button>
                <span className="min-w-[18px] text-center text-sm font-bold">
                  {qty}
                </span>
                <button
                  type="button"
                  onClick={() => onQty(qty + 1)}
                  disabled={!orderable}
                  aria-label="Ko'paytirish"
                  className="flex h-8 w-8 items-center justify-center rounded-full hover:bg-black/5 disabled:opacity-35"
                >
                  <Plus size={16} />
                </button>
              </div>
            ) : (
              orderable && <button
                type="button"
                onClick={() => onQty(1)}
                aria-label={`${product.name} — savatga qo'shish`}
                className="flex h-10 w-10 items-center justify-center rounded-full bg-white text-[#141414] shadow-lg transition-transform active:scale-95"
              >
                <Plus size={20} />
              </button>
            )}
          </div>
        )}
      </div>

      <p className="mt-2.5 text-[15px] font-bold">
        {formatSum(price)}
        {discount && (
          <span className="ml-2 text-[13px] font-medium text-white/35 line-through">
            {formatSum(product.price_tiyin)}
          </span>
        )}
      </p>
      <p className="mt-0.5 line-clamp-2 text-[13px] leading-snug text-white/60">
        {product.name}
      </p>
      {!available && (
        <p className="mt-1 text-xs text-white/40">Hozircha mavjud emas</p>
      )}
    </div>
  );
}

// ── O'ngdagi savat paneli ────────────────────────────────────────────
function CartPanel({
  orderable,
  items,
  productsById,
  promotions,
  onQty,
  onCheckout,
}: {
  /** Restoran HOZIR buyurtma qabul qiladimi (ish vaqti bilan). */
  orderable: boolean;
  items: Record<string, number>;
  productsById: Map<string, Product>;
  promotions: ActivePromotion[];
  onQty: (productId: string, qty: number) => void;
  onCheckout: () => void;
}) {
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

  const subtotal = useMemo(() => {
    let sum = 0;
    for (const { product, qty } of lines) {
      const discount = computeProductDiscount(product, promotions);
      sum += (discount?.discountedPriceTiyin ?? product.price_tiyin ?? 0) * qty;
    }
    return sum;
  }, [lines, promotions]);

  return (
    // Balandlik qatordan olinadi (sahifa o'zi skroll bo'lmaydi) —
    // savat doim ko'z oldida, ichidagi ro'yxat esa o'zi suriladi.
    // Chegara menyu bloki bilan bir xil: ikkalasi ham fonda "yotgan"
    // alohida qog'oz bo'lib ko'rinadi.
    <aside className="hidden h-full w-[340px] shrink-0 flex-col overflow-hidden rounded-3xl border border-white/10 bg-[#141414] xl:flex">
      <h2 className="px-5 pb-3 pt-5 text-xl font-bold">Savat</h2>

      {lines.length === 0 ? (
        // Bo'sh holat — mijoz ilovasidagi bilan BIR XIL rasm
        // (`assets/empty/cart.png` dan nusxa): mijoz ikkala ilovada
        // bir xil ko'rinishni ko'radi.
        <div className="flex flex-1 flex-col items-center justify-center px-8 text-center">
          {/* eslint-disable-next-line @next/next/no-img-element -- statik fayl */}
          <img
            src="/empty-cart.png"
            alt=""
            aria-hidden="true"
            className="w-[190px] object-contain"
          />
          <p className="mt-5 text-[17px] font-bold">Savat bo&apos;sh</p>
          <p className="mt-1.5 text-[13px] leading-relaxed text-white/45">
            Menyudan biror taom qo&apos;shing — u shu yerda ko&apos;rinadi.
          </p>
        </div>
      ) : (
        <>
          <div className="ondex-scroll min-h-0 flex-1 overflow-y-auto overscroll-contain px-5">
            {lines.map(({ id, product, qty }) => {
              const discount = computeProductDiscount(product, promotions);
              const price =
                discount?.discountedPriceTiyin ?? product.price_tiyin ?? 0;
              return (
                <div key={id} className="flex gap-3 border-b border-white/5 py-3">
                  <div className="h-[54px] w-[54px] shrink-0 overflow-hidden rounded-xl bg-white/5">
                    {product.image_url ? (
                      // eslint-disable-next-line @next/next/no-img-element -- manzil dinamik
                      <img
                        src={fullImageUrl(product.image_url)}
                        alt={product.name}
                        className="h-full w-full object-cover"
                      />
                    ) : (
                      <div className="flex h-full w-full items-center justify-center text-white/25">
                        <UtensilsCrossed size={18} />
                      </div>
                    )}
                  </div>

                  <div className="min-w-0 flex-1">
                    <p className="line-clamp-2 text-[13px] leading-snug">
                      {product.name}
                    </p>
                    <p className="mt-1 text-[13px] font-bold">
                      {formatSum(price * qty)}
                    </p>
                  </div>

                  <div className="flex h-8 shrink-0 items-center gap-1 self-start rounded-full bg-white/10 px-1">
                    <button
                      type="button"
                      onClick={() => onQty(id, qty - 1)}
                      aria-label="Kamaytirish"
                      className="flex h-6 w-6 items-center justify-center rounded-full hover:bg-white/10"
                    >
                      <Minus size={14} />
                    </button>
                    <span className="min-w-[16px] text-center text-[13px] font-bold">
                      {qty}
                    </span>
                    <button
                      type="button"
                      onClick={() => onQty(id, qty + 1)}
                      disabled={!orderable}
                      aria-label="Ko'paytirish"
                      className="flex h-6 w-6 items-center justify-center rounded-full hover:bg-white/10 disabled:opacity-35"
                    >
                      <Plus size={14} />
                    </button>
                  </div>
                </div>
              );
            })}
          </div>

          <div className="border-t border-white/10 p-5">
            <div className="flex items-baseline justify-between">
              <span className="text-sm text-white/50">Taomlar</span>
              <span className="text-lg font-bold">{formatSum(subtotal)}</span>
            </div>
            {/* Yetkazish narxi SERVER hisobidan chiqadi
                (`/checkout` dagi `useQuote`) — bu yerda taxminiy raqam
                yozilsa, keyingi ekranda boshqa summa chiqib, ishonch
                yo'qolardi. */}
            <p className="mt-1 text-xs text-white/35">
              Yetkazish narxi rasmiylashtirishda hisoblanadi
            </p>

            <button
              type="button"
              onClick={onCheckout}
              disabled={!orderable}
              className="mt-4 flex h-12 w-full items-center justify-center rounded-2xl bg-brand text-[15px] font-bold text-white transition-colors hover:bg-brand-light disabled:cursor-not-allowed disabled:bg-white/10 disabled:text-white/40"
            >
              {orderable ? "Buyurtma berish" : "Restoran yopiq"}
            </button>
          </div>
        </>
      )}
    </aside>
  );
}
