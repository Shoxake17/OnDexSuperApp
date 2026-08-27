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
  // Serverning QATOR bo'yicha hisobi: product_id -> chegirmadan keyingi
  // qator summasi. Savat qatorlari AYNAN shundan chiziladi — server
  // butun savatga bitta aksiya qo'llaydi, ya'ni "har taomga o'zining
  // eng yaxshi chegirmasi" hisobi qatorlar yig'indisini jamidan
  // ajratib yuborardi (19 000 va 24 000 so'm).
  const [quoteLines, setQuoteLines] = useState<Map<string, number>>(
    () => new Map(),
  );
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
      const d = computeProductDiscount(p, promotions, subtotalTiyin);
      total += (d ? d.discountedPriceTiyin : p.price_tiyin) * qty;
    }
    return total;
  }, [cartItems, productsById, promotions, subtotalTiyin]);

  const itemsKey = JSON.stringify(cartItems);

  useEffect(() => {
    const items = Object.entries(cartItems)
      .filter(([, qty]) => qty > 0)
      .map(([product_id, qty]) => ({ product_id, qty }));
    if (!restaurantId || items.length === 0) {
      setQuoteTotal(0);
      setQuoteLines(new Map());
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
        const ok = typeof t === "number" && t > 0;
        setQuoteTotal(ok ? t : null);
        // Qator narxlari FAQAT jami ishonchli bo'lganda ishlatiladi —
        // aks holda savat qatorlari va pastdagi jami turli manbadan
        // kelib qolardi.
        const lines = new Map<string, number>();
        if (ok && Array.isArray(q?.lines)) {
          for (const l of q.lines) {
            if (typeof l?.product_id === "string" && typeof l?.total_tiyin === "number") {
              lines.set(l.product_id, l.total_tiyin);
            }
          }
        }
        setQuoteLines(lines);
      })
      .catch(() => {
        if (!cancelled) {
          setQuoteTotal(null);
          setQuoteLines(new Map());
        }
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

  return { totalTiyin, subtotalTiyin, loading, quoteTotal, quoteLines };
}
