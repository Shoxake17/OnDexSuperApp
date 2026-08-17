"use client";

import { useEffect } from "react";
import { applyTableSession } from "@/lib/open-table";
import type { TableSession } from "@/lib/table-session";

// Mijoz ilovasidagi (Flutter) QR skaneri bilan sahifa o'rtasidagi
// ko'prik.
//
// ┌─ ISH TAQSIMOTI ───────────────────────────────────────────────────┐
// Flutter tomoni:  kamera, QR o'qish, tokenni yechish (`/tables/resolve`
//                  — native, autentifikatsiyalangan so'rov) va xato
//                  xabarlari (native oyna).
// Sahifa tomoni:   stol seansini yozish, savat qoidasi va menyuga
//                  o'tish — ya'ni AYNAN Telegram Mini App'dagi bilan
//                  bir xil kod (`lib/open-table.ts`).
//
// Shu bo'linish sababi: seans `sessionStorage` da va savat
// `localStorage` da — ikkalasi ham SAHIFAGA tegishli, Flutter ularga
// to'g'ridan-to'g'ri tegsa ikki joyda ikki xil qoida paydo bo'lardi.
// └───────────────────────────────────────────────────────────────────┘
//
// ┌─ XAVFSIZLIK ──────────────────────────────────────────────────────┐
// Funksiya FAQAT ilova qobig'ida (`/api/bridge` orqali ochilgan
// sessiyada) ro'yxatdan o'tadi — Telegram'da va oddiy brauzerda u
// umuman mavjud bo'lmaydi.
//
// U hech qanday YANGI huquq bermaydi: sahifadagi istalgan skript
// allaqachon `sessionStorage`/`localStorage` ga yoza oladi. Bu yerda
// esa qiymat shakli tekshiriladi (`restaurantId` va `token` bo'lishi
// shart), ya'ni buzilgan kirish holatni buzmaydi.
// └───────────────────────────────────────────────────────────────────┘

declare global {
  interface Window {
    /**
     * Flutter chaqiradi: `__ondexOpenTable(JSON.stringify(session))`.
     * `true` — seans qabul qilindi va menyu ochilmoqda.
     */
    __ondexOpenTable?: (sessionJson: string) => boolean;
  }
}

export default function AppTableBridge({ enabled }: { enabled: boolean }) {
  useEffect(() => {
    if (!enabled) return;

    window.__ondexOpenTable = (sessionJson: string) => {
      try {
        const raw = JSON.parse(sessionJson) as Partial<TableSession>;
        if (!raw?.token || !raw?.restaurantId) return false;
        applyTableSession({
          token: raw.token,
          tableLabel: raw.tableLabel ?? "",
          restaurantId: raw.restaurantId,
          restaurantName: raw.restaurantName,
        });
        return true;
      } catch {
        return false;
      }
    };

    return () => {
      delete window.__ondexOpenTable;
    };
  }, [enabled]);

  return null;
}
