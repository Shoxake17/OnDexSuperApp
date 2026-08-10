"use client";

import { useEffect, useMemo, useState } from "react";
import { computeProductDiscount } from "./promotions";
import type { ActivePromotion, Product } from "./types";

// menu_screen.dart/cart_screen.dart'dagi _loadQuote() bilan bir xil naqsh:
// login qilingan bo'lsa HAQIQIY server summasi (POST /restaurants/{id}/quote),
// aks holda (anonim — 401, yoki tarmoq xatosi) vizual (promo-hisoblangan)
// taxminga tushadi. Menyu VA Savat sahifalari ikkalasi ham shu hook'ni
// ishlatadi — bitta manba, ikki joyda ko'chirilgan kod yo'q.
export function useQuote(
  restaurantId: string | null,
  cartItems: Record<string, number>,
  productsById: Map<string, Product>,
  promotions: ActivePromotion[],
) {
  const [quoteTotal, setQuoteTotal] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);

  const subtotalTiyin = useMemo(() => {
    let total = 0;
    for (const [id, qty] of Object.entries(cartItems)) {
      const p = productsById.get(id);
      if (p) total += p.price_tiyin * qty;
    }
    return total;
  }, [cartItems, productsById]);

  const visualTotalTiyin = useMemo(() => {
    let total = 0;
    for (const [id, qty] of Object.entries(cartItems)) {
      const p = productsById.get(id);
      if (!p) continue;
      const d = computeProductDiscount(p, promotions);
      total += (d ? d.discountedPriceTiyin : p.price_tiyin) * qty;
    }
    return total;
  }, [cartItems, productsById, promotions]);

  const itemsKey = JSON.stringify(cartItems);

  useEffect(() => {
    const items = Object.entries(cartItems)
      .filter(([, qty]) => qty > 0)
      .map(([product_id, qty]) => ({ product_id, qty }));
    if (!restaurantId || items.length === 0) {
      setQuoteTotal(0);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    fetch(`/api/proxy/restaurants/${restaurantId}/quote`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ items }),
    })
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((q) => {
        if (cancelled) return;
        // `q.total_tiyin ?? null` YETARLI EMAS EDI: `??` faqat
        // null/undefined ni ushlaydi, `0` ni EMAS. Server noto'g'ri
        // chegirma sozlamasi tufayli 0 qaytarganda UI uni jimgina
        // qabul qilib "0 so'm" chizardi va savat tugmasi faol
        // qolardi — ya'ni bepul buyurtma. Endi 0 (yoki manfiy, yoki
        // son bo'lmagan qiymat) "narx aniqlanmadi" deb qaraladi va
        // vizual taxminga tushiladi; checkout esa uni bloklaydi.
        const t = q?.total_tiyin;
        setQuoteTotal(typeof t === "number" && t > 0 ? t : null);
      })
      .catch(() => {
        if (!cancelled) setQuoteTotal(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [restaurantId, itemsKey]);

  const totalTiyin = quoteTotal ?? visualTotalTiyin;

  return { totalTiyin, subtotalTiyin, loading, quoteTotal };
}
