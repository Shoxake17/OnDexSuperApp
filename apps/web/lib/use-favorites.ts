"use client";

import { useCallback, useEffect, useState } from "react";

// Istaklar (yurak) holati — Menyu va Savat sahifalari IKKALASI ham
// shu hook'ni ishlatadi.
//
// NEGA UMUMIY: avval har ikkala sahifa `favoriteIds` to'plamini o'zi
// yuklab, o'zgarishni ham o'zi qo'lda qo'shib/o'chirib yozardi — bir xil
// mantiq ikki nusxada edi. Bundan tashqari kartochkadagi yurak bosilganda
// ota-komponentga xabar berilmasa, MAHSULOT TAFSILOTI oynasi eski
// holatni ko'rsatardi (ikkalasi ham shu to'plamdan oziqlanadi).
export function useFavorites() {
  const [favoriteIds, setFavoriteIds] = useState<Set<string>>(new Set());

  useEffect(() => {
    let cancelled = false;
    fetch("/api/proxy/favorites/ids")
      .then((r) => (r.ok ? r.json() : []))
      .then((ids: string[]) => {
        if (!cancelled) setFavoriteIds(new Set(ids));
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  /// Yurak SERVERGA saqlangandan keyin chaqiriladi.
  const onFavoriteChange = useCallback((productId: string, favorited: boolean) => {
    setFavoriteIds((prev) => {
      const next = new Set(prev);
      if (favorited) next.add(productId);
      else next.delete(productId);
      return next;
    });
  }, []);

  return { favoriteIds, onFavoriteChange };
}
