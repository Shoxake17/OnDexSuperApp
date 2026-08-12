"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

// Bitta buyurtma = bitta restoran (internal/orders'dagi ErrMixedRestaurants
// bilan mos). Flutter'da savat MenuScreen'ning LOKAL widget state'i edi —
// boshqa restoran menyusiga o'tilganda avtomatik "toza" boshlanardi. Bu
// yerda global Context + localStorage ishlatiladi (WebView/brauzer sahifa
// almashtirsa ham — Next.js'ning o'z client-side router'i — savat
// yo'qolmasin uchun), lekin xatti-harakat bir xil: boshqa restoran uchun
// miqdor o'rnatilsa, ESKI restoran savati jimgina TOZALANADI (Flutter'da
// bu allaqachon "bepul" — endi ANIQ shu yerda takrorlanadi).
const STORAGE_KEY = "chust_cart_v1";

/**
 * clearStoredCartIfOtherRestaurant — saqlangan savat BOSHQA restoranga
 * tegishli bo'lsa uni o'chiradi.
 *
 * ┌─ NEGA KERAK ──────────────────────────────────────────────────────┐
 * Mijoz "B" restorani savatini yig'ib qo'ygan bo'lishi, keyin "A"
 * restoranida stolga o'tirib QR skanerlashi mumkin. O'shanda savat
 * hamon B'niki bo'lib qolardi va mijoz savatga kirsa BEGONA
 * restoran taomlarini ko'rardi.
 *
 * `setQty` allaqachon boshqa restoran uchun savatni tozalaydi, lekin
 * u FAQAT mijoz biror taom qo'shganda ishlaydi — menyuga kirib,
 * to'g'ridan savatga o'tsa eski holat ko'rinardi.
 *
 * React holatiga emas, `localStorage` ga to'g'ridan-to'g'ri tegamiz:
 * bu funksiya `CartProvider` dan TASHQARIDA (`app/table-init.tsx`,
 * root layout'da) chaqiriladi va undan keyin sahifa TO'LIQ qayta
 * yuklanadi, ya'ni provider yangi holatni o'qiydi.
 * └───────────────────────────────────────────────────────────────────┘
 */
export function clearStoredCartIfOtherRestaurant(restaurantId: string): void {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return;
    const parsed = JSON.parse(raw) as Partial<CartState>;
    if (parsed?.restaurantId && parsed.restaurantId !== restaurantId) {
      localStorage.removeItem(STORAGE_KEY);
    }
  } catch {
    // Buzilgan yozuv — o'chirib yuborish eng xavfsizi.
    try {
      localStorage.removeItem(STORAGE_KEY);
    } catch {
      // e'tiborsiz
    }
  }
}

type CartState = {
  restaurantId: string | null;
  items: Record<string, number>;
};

type CartContextValue = {
  restaurantId: string | null;
  items: Record<string, number>;
  totalItems: number;
  hydrated: boolean;
  setQty: (restaurantId: string, productId: string, qty: number) => void;
  clear: () => void;
};

const CartContext = createContext<CartContextValue | null>(null);

export function CartProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<CartState>({
    restaurantId: null,
    items: {},
  });
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw) setState(JSON.parse(raw));
    } catch {
      // buzilgan/eski format — bo'sh savat bilan davom etamiz.
    }
    setHydrated(true);
  }, []);

  useEffect(() => {
    if (!hydrated) return;
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  }, [state, hydrated]);

  const setQty = useCallback(
    (restaurantId: string, productId: string, qty: number) => {
      setState((prev) => {
        const items =
          prev.restaurantId === restaurantId ? { ...prev.items } : {};
        if (qty <= 0) {
          delete items[productId];
        } else {
          items[productId] = qty;
        }
        return { restaurantId, items };
      });
    },
    [],
  );

  const clear = useCallback(
    () => setState({ restaurantId: null, items: {} }),
    [],
  );

  const totalItems = useMemo(
    () => Object.values(state.items).reduce((a, b) => a + b, 0),
    [state.items],
  );

  const value = useMemo(
    () => ({
      restaurantId: state.restaurantId,
      items: state.items,
      totalItems,
      hydrated,
      setQty,
      clear,
    }),
    [state, totalItems, hydrated, setQty, clear],
  );

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>;
}

export function useCart(): CartContextValue {
  const ctx = useContext(CartContext);
  if (!ctx) throw new Error("useCart CartProvider ichida ishlatilishi kerak");
  return ctx;
}
