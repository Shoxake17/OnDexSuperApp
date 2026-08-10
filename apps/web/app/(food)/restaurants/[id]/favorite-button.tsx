"use client";

import { Heart } from "lucide-react";
import { useEffect, useState } from "react";

// product_grid.dart'dagi FavoriteButton bilan bir xil: bosilganda DARHOL
// (optimistik) holatni almashtiradi, fon rejimida serverga so'rov
// yuboradi; xato bo'lsa (masalan login qilinmagan — 401, yoki tarmoq)
// holat ORQAGA qaytariladi.
export default function FavoriteButton({
  productId,
  initialFavorited,
  onChanged,
}: {
  productId: string;
  initialFavorited: boolean;
  onChanged?: (productId: string, favorited: boolean) => void;
}) {
  const [favorited, setFavorited] = useState(initialFavorited);
  const [busy, setBusy] = useState(false);

  // Grid qayta render bo'lganda (masalan favoriteIds boshqa joyda
  // yangilangan) ham to'g'ri sinxronlanadi.
  useEffect(() => setFavorited(initialFavorited), [initialFavorited]);

  async function toggle() {
    if (busy) return;
    const next = !favorited;
    setFavorited(next);
    setBusy(true);
    try {
      const res = await fetch(`/api/proxy/favorites/${productId}`, {
        method: next ? "POST" : "DELETE",
      });
      if (!res.ok) throw new Error("xato");
      onChanged?.(productId, next);
    } catch {
      setFavorited(!next);
    } finally {
      setBusy(false);
    }
  }

  return (
    <button
      onClick={toggle}
      className="flex h-9 w-9 items-center justify-center rounded-full bg-white shadow-md"
      aria-label={favorited ? "Istaklardan olib tashlash" : "Istaklarga qo'shish"}
    >
      <Heart
        size={20}
        className={favorited ? "text-[#E53935]" : "text-black"}
        fill={favorited ? "#E53935" : "none"}
      />
    </button>
  );
}
