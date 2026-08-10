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
