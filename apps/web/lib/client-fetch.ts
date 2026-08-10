"use client";

import type { ActivePromotion, Product, Restaurant } from "./types";

// Client komponentlar uchun katalog so'rovlari — BFF proksisi orqali.
//
// AVVAL bu yerda `NEXT_PUBLIC_API_URL` bilan Go backend'ga TO'G'RIDAN-
// TO'G'RI murojaat qilinardi. Bu butun BFF arxitekturasidan chetga
// chiqish edi: brauzer origin'idan kelgan bu so'rovlar ishlashi uchun
// Go tomonda CORS ochiq qolishi kerak bo'lardi — aynan BFF yopmoqchi
// bo'lgan teshik. Endi hamma narsa bitta yo'ldan (`/api/proxy/...`)
// o'tadi, ya'ni Go backend faqat Next.js serveridan so'rov qabul
// qilsa yetarli.
export async function fetchRestaurantMenu(restaurantId: string): Promise<{
  restaurant: Restaurant | null;
  menu: Product[];
  promotions: ActivePromotion[];
}> {
  const id = encodeURIComponent(restaurantId);
  try {
    const [rRes, mRes, pRes] = await Promise.all([
      fetch(`/api/proxy/restaurants/${id}`),
      fetch(`/api/proxy/restaurants/${id}/menu`),
      fetch(`/api/proxy/restaurants/${id}/active-promotions`),
    ]);
    return {
      restaurant: rRes.ok ? await rRes.json() : null,
      menu: mRes.ok ? await mRes.json() : [],
      promotions: pRes.ok ? await pRes.json() : [],
    };
  } catch {
    return { restaurant: null, menu: [], promotions: [] };
  }
}
