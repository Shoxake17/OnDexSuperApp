"use client";

// Stol seansi — QR kod skanerlangandan keyingi holat.
//
// ┌─ NEGA `sessionStorage`, `localStorage` EMAS ──────────────────────┐
// Stolda o'tirish — VAQTINCHA holat. `localStorage` da saqlansa,
// mijoz uyiga qaytib ilovani ochganda ham "5-stol" rejimida qolardi
// va yetkazib berish buyurtmasi o'rniga yana stol buyurtmasi
// yuborardi — restoranda hech kim kutmayotgan buyurtma.
//
// `sessionStorage` esa tab/Mini App yopilishi bilan tozalanadi, ya'ni
// keyingi ochilishda odatiy yetkazish rejimi qaytadi.
// └───────────────────────────────────────────────────────────────────┘

import { useCallback, useEffect, useState } from "react";

const KEY = "ondex_table_v1";

export type TableSession = {
  /** QR kod ichidagi sir — buyurtma yaratishda serverga yuboriladi. */
  token: string;
  tableLabel: string;
  restaurantId: string;
  restaurantName?: string;
};

export function readTableSession(): TableSession | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = sessionStorage.getItem(KEY);
    if (!raw) return null;
    const v = JSON.parse(raw) as TableSession;
    // Shakl tekshiruvi: eski/buzilgan yozuv butun sahifani
    // yiqitmasligi kerak.
    if (!v?.token || !v?.restaurantId) return null;
    return v;
  } catch {
    return null;
  }
}

export function writeTableSession(v: TableSession): void {
  try {
    sessionStorage.setItem(KEY, JSON.stringify(v));
  } catch {
    // Xotira to'la / rejim taqiqlangan — stol rejimisiz davom etamiz.
  }
}

export function clearTableSession(): void {
  try {
    sessionStorage.removeItem(KEY);
  } catch {
    // e'tiborsiz
  }
}

/**
 * "Bu savat stol rejimidami?" — YAGONA javob beruvchi joy.
 *
 * ┌─ NEGA HOOK ───────────────────────────────────────────────────────┐
 * Bu qoida kamida ikki sahifada kerak: savat (tugma qayerga olib
 * boradi) va rasmiylashtirish (manzil so'ralsinmi). Avval u faqat
 * rasmiylashtirish sahifasida bor edi, savat esa SHARTSIZ
 * `/address?next=/checkout` ga yuborardi — ya'ni stolda o'tirgan mijoz
 * ham xarita ekranidan o'tishga majbur bo'lardi. Manzil unga umuman
 * kerak emas (taom stolga keladi), lekin ekranni o'tkazib yuborishning
 * yo'li ham yo'q edi.
 *
 * Endi qoida bitta joyda: keyingi sahifa ham xuddi shu javobni oladi.
 * └───────────────────────────────────────────────────────────────────┘
 *
 * MOSLIK: seans BOSHQA restoranniki bo'lsa `null` qaytadi — mijoz
 * stolda o'tirib, boshqa restoran menyusidan savat yig'ishi mumkin.
 * Server ham bu holatni rad etadi; bu yerda esa oddiygina yetkazib
 * berish rejimi qoladi.
 *
 * SSR: birinchi renderda har doim `null` (server `sessionStorage` ni
 * ko'rmaydi) — shuning uchun UI "hali noma'lum" holatini ham
 * ko'tara olishi kerak.
 */
export function useTableSession(restaurantId: string | null | undefined): {
  table: TableSession | null;
  /** Stol rejimidan chiqish (mijoz "Bekor qilish" bosganda). */
  clear: () => void;
} {
  const [table, setTable] = useState<TableSession | null>(null);

  useEffect(() => {
    const t = readTableSession();
    setTable(t && restaurantId && t.restaurantId === restaurantId ? t : null);
  }, [restaurantId]);

  const clear = useCallback(() => {
    clearTableSession();
    setTable(null);
  }, []);

  return { table, clear };
}
