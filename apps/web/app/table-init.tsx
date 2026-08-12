"use client";

import { useEffect, useRef } from "react";
import { getTelegramWebApp } from "@/lib/telegram";
import { clearStoredCartIfOtherRestaurant } from "@/lib/cart-context";
import {
  readTableSession,
  writeTableSession,
  type TableSession,
} from "@/lib/table-session";

// Stol QR kodidan kelgan mijozni to'g'ri menyuga olib boradi.
//
// ┌─ OQIM ────────────────────────────────────────────────────────────┐
// 1. Mijoz stoldagi QR kodni kamera bilan skanerlaydi;
// 2. Kamera `https://t.me/<bot>/app?startapp=<token>` ni ochadi;
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

    void (async () => {
      try {
        const res = await fetch(
          `/api/proxy/tables/resolve?token=${encodeURIComponent(token)}`,
        );
        if (!res.ok) return; // yaroqsiz QR — odatiy bosh sahifa qoladi
        const d = (await res.json()) as {
          table_label?: string;
          restaurant_id?: string;
          restaurant_name?: string;
        };
        if (!d.restaurant_id) return;

        const session: TableSession = {
          token,
          tableLabel: d.table_label ?? "",
          restaurantId: d.restaurant_id,
          restaurantName: d.restaurant_name,
        };
        writeTableSession(session);

        // Savat BOSHQA restoranniki bo'lsa — tozalanadi. QR skanerlash
        // aniq niyat: "men SHU restoranning SHU stolida o'tiribman".
        // Eski savat qolib ketsa, mijoz savatga kirib begona restoran
        // taomlarini ko'rardi.
        clearStoredCartIfOtherRestaurant(d.restaurant_id);

        // ┌─ TO'LIQ QAYTA YUKLASH (router.replace EMAS) ──────────────┐
        // `CartProvider` `localStorage` ni FAQAT bir marta, o'zi
        // ulanganda o'qiydi. Yuqorida savatni o'chirdik, lekin
        // provider xotirasida eski holat turibdi va u keyingi
        // o'zgarishda `localStorage` ga QAYTA yozilardi — ya'ni
        // tozalash bekor bo'lardi.
        //
        // To'liq qayta yuklash butun daraxtni qaytadan quradi va
        // provider yangi (bo'sh) savatni o'qiydi.
        //
        // `replace` ATAYLAB: orqaga tugmasi mijozni bo'sh bosh
        // sahifaga qaytarmasin.
        window.location.replace(`/restaurants/${d.restaurant_id}`);
      } catch {
        // Tarmoq xatosi — odatiy bosh sahifa ko'rinadi, ilova
        // yiqilmaydi.
      }
    })();
  }, [signedIn]);

  return null;
}
