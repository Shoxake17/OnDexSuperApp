"use client";

import { useSyncExternalStore } from "react";

// Joriy vaqt — soniya aniqligi shart bo'lmagan ko'rinishlar uchun
// (restoran holati, kuryer yetib kelish vaqti).
//
// ┌─ NEGA UMUMIY DO'KON ───────────────────────────────────────────────┐
// Sahifadagi yigirmata restoran kartasi yigirmata `setInterval` ochmasin:
// taymer BITTA, obunachi qolmasa to'xtaydi.
//
// Server va gidratsiya paytida `null` qaytadi: aks holda keshlangan
// HTML'dagi matn ("09:00 da ochiladi") brauzerdagi soat bilan farq qilib,
// React "mos kelmadi" deb xato berardi. Keyin darhol haqiqiy vaqt bilan
// qayta chiziladi.
// └────────────────────────────────────────────────────────────────────┘

const STEP_MS = 15_000;

let snapshot = 0;
const listeners = new Set<() => void>();
let timer: ReturnType<typeof setInterval> | null = null;

function read(): number {
  const t = Math.floor(Date.now() / STEP_MS) * STEP_MS;
  if (t !== snapshot) snapshot = t;
  return snapshot;
}

function subscribe(onChange: () => void): () => void {
  listeners.add(onChange);
  if (!timer) {
    timer = setInterval(() => {
      const before = snapshot;
      if (read() !== before) listeners.forEach((l) => l());
    }, 1_000);
  }
  return () => {
    listeners.delete(onChange);
    if (listeners.size === 0 && timer) {
      clearInterval(timer);
      timer = null;
    }
  };
}

/** Joriy vaqt (15 soniya qadam bilan); server/gidratsiyada `null`. */
export function useNow(): Date | null {
  const t = useSyncExternalStore(subscribe, read, () => 0);
  return t === 0 ? null : new Date(t);
}
