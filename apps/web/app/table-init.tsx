"use client";

import { useEffect, useRef } from "react";
import { getTelegramWebApp } from "@/lib/telegram";
import { openTableFromToken } from "@/lib/open-table";
import { readTableSession } from "@/lib/table-session";

// Stol QR kodidan kelgan mijozni to'g'ri menyuga olib boradi.
//
// ┌─ OQIM ────────────────────────────────────────────────────────────┐
// 1. Mijoz stoldagi QR kodni kamera bilan skanerlaydi;
// 2. Kamera `https://t.me/<bot>/<short_name>?startapp=<token>` ni ochadi
//    (short_name serverda — `miniAppShortName`, hozir `ondex`);
// 3. Telegram Mini App'ni ochib, `<token>` ni `start_param` sifatida
//    beradi;
// 4. Bu komponent tokenni serverga yuboradi va qaysi restoran/stol
//    ekanini aniqlaydi;
// 5. O'sha restoran menyusi ochiladi, savat "stol rejimi"da ishlaydi.
// └───────────────────────────────────────────────────────────────────┘
//
// Telegram TASHQARISIDA hech narsa qilmaydi.
export default function TableInit({ signedIn }: { signedIn: boolean }) {
  // React `useEffect` ni development'da IKKI MARTA chaqiradi
  // (StrictMode) — busiz server ikki marta so'rov olardi.
  const started = useRef(false);

  useEffect(() => {
    // Kirish SHART: `/tables/resolve` autentifikatsiya talab qiladi
    // (tokenlarni birma-bir sinab ko'rishga qarshi). Mijoz hali
    // kirmagan bo'lsa, `TelegramAuth` avval o'z ishini tugatadi va
    // sahifa qayta yuklanadi — o'shanda bu effekt qaytadan ishlaydi.
    if (!signedIn || started.current) return;

    const wa = getTelegramWebApp();
    const token = wa?.initDataUnsafe?.start_param?.trim();
    if (!token) return;

    // Shu stol allaqachon ochilgan bo'lsa qayta yo'naltirmaymiz —
    // aks holda mijoz savatdan menyuga har qaytganda sahifa
    // o'z-o'zidan sakrab turardi.
    const existing = readTableSession();
    if (existing?.token === token) return;

    started.current = true;

    // Yechish, seansni yozish, savatni tekshirish va menyuni ochish —
    // hammasi `lib/open-table.ts` da (mijoz ilovasi va Mini App
    // kamerasi ham AYNAN o'sha funksiyani chaqiradi).
    //
    // Bu yerda xato KO'RSATILMAYDI: foydalanuvchi hech narsa
    // bosmagan, u shunchaki ilovani ochgan. Yaroqsiz QR bo'lsa
    // odatiy bosh sahifa qoladi.
    void openTableFromToken(token);
  }, [signedIn]);

  return null;
}
