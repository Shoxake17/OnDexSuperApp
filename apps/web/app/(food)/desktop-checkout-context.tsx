"use client";

import { createContext, useCallback, useContext, useMemo, useState } from "react";
import DesktopCheckoutDialog from "./desktop-checkout-dialog";

// Rasmiylashtirish oynasini ochish uchun YAGONA nuqta.
//
// ┌─ NEGA KONTEKST ────────────────────────────────────────────────────┐
// Oynani ikki joydan ochish kerak: navbardagi savat bloki
// (`desktop-cart-menu.tsx`) va restoran menyusidagi savat paneli
// (`restaurants/[id]/desktop-menu.tsx`). Ular bir-birining ichida
// emas, ya'ni holatni props orqali uzatib bo'lmaydi.
//
// Har birida alohida nusxa render qilish ham to'g'ri emas: ikki oyna
// bir vaqtda ochilib qolishi va savat holati ikki joyda alohida
// yuklanishi mumkin edi. Shuning uchun oyna BITTA joyda —
// `(food)/layout.tsx` da — turadi, ochish esa shu hook orqali.
// └────────────────────────────────────────────────────────────────────┘
//
// Mobil ko'rinish bu hook'ni CHAQIRMAYDI — u eski `/checkout`
// sahifasiga o'tadi va o'zgarishsiz qoladi.

type Ctx = { openCheckout: (restaurantId: string) => void };

const CheckoutContext = createContext<Ctx | null>(null);

export function DesktopCheckoutProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [restaurantId, setRestaurantId] = useState<string | null>(null);

  const openCheckout = useCallback((id: string) => setRestaurantId(id), []);
  const value = useMemo(() => ({ openCheckout }), [openCheckout]);

  return (
    <CheckoutContext.Provider value={value}>
      {children}
      {restaurantId && (
        <DesktopCheckoutDialog
          restaurantId={restaurantId}
          onClose={() => setRestaurantId(null)}
        />
      )}
    </CheckoutContext.Provider>
  );
}

/**
 * Provider bo'lmasa `null` qaytaradi — chaqiruvchi o'shanda oddiy
 * `/checkout` havolasiga tushadi. Bu ataylab: hook mobil daraxtda ham
 * chaqirilishi mumkin va u yerda xato tashlash sahifani yiqitardi.
 */
export function useDesktopCheckout(): Ctx | null {
  return useContext(CheckoutContext);
}
