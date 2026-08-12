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
