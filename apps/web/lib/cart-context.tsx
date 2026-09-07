"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

// ┌─ HAR RESTORAN UCHUN ALOHIDA SAVAT (2026-09-04) ─────────────────────┐
// AVVAL bitta savat bor edi va boshqa restorandan mahsulot qo'shilsa
// eskisi JIMGINA o'chirilardi:
//
//     const items = prev.restaurantId === id ? {...prev.items} : {};
//
// Qoidaning o'zi to'g'ri: bitta BUYURTMA = bitta restoran
// (`internal/orders` dagi `ErrMixedRestaurants`). Lekin undan
// "boshqa restoranga qarasang, yig'ganingni yo'qotasan" degan xulosa
// KELIB CHIQMAYDI — mijoz ikkinchi kafeda bitta narsani ko'rib qo'shsa,
// birinchi savati butunlay yo'qolardi va bu haqda ogohlantirish ham
// yo'q edi.
//
// Endi savatlar restoran bo'yicha alohida saqlanadi; "faol" savat —
// oxirgi tegilgani. Buyurtma baribir bittasidan rasmiylashtiriladi,
// ya'ni server qoidasi buzilmaydi — shunchaki qolganlari o'chmaydi
// (namuna: image/savat.png — Yandex Eats'da ham savatlar ro'yxati).
// └────────────────────────────────────────────────────────────────────┘
const STORAGE_KEY = "chust_cart_v2";
// Eski (bitta savatli) format — birinchi ochilishda ko'chiriladi.
const LEGACY_KEY = "chust_cart_v1";

/** restoran id -> (mahsulot id -> soni) */
type Carts = Record<string, Record<string, number>>;

type CartState = {
  activeRestaurantId: string | null;
  carts: Carts;
};

const EMPTY: CartState = { activeRestaurantId: null, carts: {} };

function readStored(): CartState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<CartState>;
      if (parsed && typeof parsed === "object" && parsed.carts) {
        return {
          activeRestaurantId: parsed.activeRestaurantId ?? null,
          carts: parsed.carts,
        };
      }
    }
    // ── Eski formatdan ko'chirish ──
    const legacy = localStorage.getItem(LEGACY_KEY);
    if (legacy) {
      const old = JSON.parse(legacy) as {
        restaurantId?: string | null;
        items?: Record<string, number>;
      };
      localStorage.removeItem(LEGACY_KEY);
      if (old?.restaurantId && old.items && Object.keys(old.items).length > 0) {
        return {
          activeRestaurantId: old.restaurantId,
          carts: { [old.restaurantId]: old.items },
        };
      }
    }
  } catch {
    // Buzilgan yozuv — bo'sh savat bilan davom etamiz.
  }
  return EMPTY;
}

/**
 * setStoredActiveRestaurant — saqlangan savatlar ichidan SHU restoranni
 * faol qiladi (boshqalarini O'CHIRMAYDI).
 *
 * ┌─ NEGA BOR ─────────────────────────────────────────────────────────┐
 * Stol QR kodi skanerlanganda niyat aniq: "men SHU restoranning SHU
 * stolidaman". Shuning uchun faol savat o'shanga o'tishi kerak, aks
 * holda mijoz savatga kirib BEGONA restoran taomlarini ko'rardi.
 *
 * Avval bu funksiya boshqa restoran savatini O'CHIRARDI
 * (`clearStoredCartIfOtherRestaurant`) — endi buning hojati yo'q,
 * savatlar yonma-yon yashaydi.
 *
 * React holatiga emas, `localStorage` ga to'g'ridan-to'g'ri tegamiz: bu
 * funksiya `CartProvider` dan TASHQARIDA chaqiriladi va undan keyin
 * sahifa TO'LIQ qayta yuklanadi (`lib/open-table.ts`).
 * └────────────────────────────────────────────────────────────────────┘
 */
export function setStoredActiveRestaurant(restaurantId: string): void {
  try {
    const state = readStored();
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ ...state, activeRestaurantId: restaurantId }),
    );
  } catch {
    // Saqlash imkoni bo'lmasa oqim to'xtamaydi — savat shunchaki
    // shu sahifada bo'sh ko'rinadi.
  }
}

type CartContextValue = {
  /** Faol savat — oxirgi tegilgan restoran. */
  restaurantId: string | null;
  /** Faol savatdagi mahsulotlar. */
  items: Record<string, number>;
  /** Faol savatdagi mahsulotlar soni. */
  totalItems: number;
  hydrated: boolean;
  /** Barcha savatlar (bo'shlari saqlanmaydi). */
  carts: Carts;
  /** Nechta restoranda savat bor. */
  cartCount: number;
  /**
   * ANIQ shu restoran savati (faol bo'lishi shart emas).
   *
   * ┌─ NEGA KERAK ─────────────────────────────────────────────────┐
   * Menyu sahifasi miqdorlarni ko'rsatishda avval `restaurantId ===
   * active ? items : {}` deb tekshirardi. Savat bitta bo'lganda bu
   * to'g'ri edi, ko'p savatda esa XATO: mijoz A restoranida savat
   * yig'ib, B ga o'tsa (faol savat B bo'ladi), keyin A menyusiga
   * qaytganda hamma miqdor 0 ko'rinardi — savat esa joyida turardi.
   * └──────────────────────────────────────────────────────────────┘
   */
  itemsFor: (restaurantId: string) => Record<string, number>;
  setQty: (restaurantId: string, productId: string, qty: number) => void;
  /** Faol savatni bo'shatadi. */
  clear: () => void;
  /** Bitta restoran savatini butunlay o'chiradi. */
  removeCart: (restaurantId: string) => void;
  /** Faol savatni almashtiradi. */
  setActive: (restaurantId: string) => void;
};

const CartContext = createContext<CartContextValue | null>(null);

/** Bo'sh savatlarni tashlab, faol savatni tirik qiymatga tenglashtiradi. */
function normalize(state: CartState): CartState {
  const carts: Carts = {};
  for (const [id, items] of Object.entries(state.carts)) {
    const kept: Record<string, number> = {};
    for (const [productId, qty] of Object.entries(items)) {
      if (qty > 0) kept[productId] = qty;
    }
    if (Object.keys(kept).length > 0) carts[id] = kept;
  }
  const active =
    state.activeRestaurantId && carts[state.activeRestaurantId]
      ? state.activeRestaurantId
      : (Object.keys(carts)[0] ?? null);
  return { activeRestaurantId: active, carts };
}

export function CartProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<CartState>(EMPTY);
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    setState(normalize(readStored()));
    setHydrated(true);
  }, []);

  useEffect(() => {
    if (!hydrated) return;
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  }, [state, hydrated]);

  const setQty = useCallback(
    (restaurantId: string, productId: string, qty: number) => {
      setState((prev) => {
        // Faqat SHU restoran savati o'zgaradi — qolganlariga tegilmaydi.
        const items = { ...(prev.carts[restaurantId] ?? {}) };
        if (qty <= 0) delete items[productId];
        else items[productId] = qty;

        const carts = { ...prev.carts };
        if (Object.keys(items).length > 0) carts[restaurantId] = items;
        else delete carts[restaurantId];

        return normalize({ activeRestaurantId: restaurantId, carts });
      });
    },
    [],
  );

  const removeCart = useCallback((restaurantId: string) => {
    setState((prev) => {
      const carts = { ...prev.carts };
      delete carts[restaurantId];
      return normalize({ activeRestaurantId: prev.activeRestaurantId, carts });
    });
  }, []);

  const clear = useCallback(() => {
    setState((prev) => {
      if (!prev.activeRestaurantId) return prev;
      const carts = { ...prev.carts };
      delete carts[prev.activeRestaurantId];
      return normalize({ activeRestaurantId: null, carts });
    });
  }, []);

  const setActive = useCallback((restaurantId: string) => {
    setState((prev) => normalize({ ...prev, activeRestaurantId: restaurantId }));
  }, []);

  const items = useMemo(
    () =>
      state.activeRestaurantId
        ? (state.carts[state.activeRestaurantId] ?? {})
        : {},
    [state],
  );

  const totalItems = useMemo(
    () => Object.values(items).reduce((a, b) => a + b, 0),
    [items],
  );

  const itemsFor = useCallback(
    (restaurantId: string) => state.carts[restaurantId] ?? {},
    [state.carts],
  );

  const value = useMemo(
    () => ({
      restaurantId: state.activeRestaurantId,
      items,
      totalItems,
      hydrated,
      carts: state.carts,
      cartCount: Object.keys(state.carts).length,
      itemsFor,
      setQty,
      clear,
      removeCart,
      setActive,
    }),
    [
      state,
      items,
      totalItems,
      hydrated,
      itemsFor,
      setQty,
      clear,
      removeCart,
      setActive,
    ],
  );

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>;
}

export function useCart(): CartContextValue {
  const ctx = useContext(CartContext);
  if (!ctx) throw new Error("useCart CartProvider ichida ishlatilishi kerak");
  return ctx;
}
