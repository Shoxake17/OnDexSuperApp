"use client";

import { useFavorites } from "@/lib/use-favorites";
import type { ActivePromotion, Product, Restaurant } from "@/lib/types";
import DesktopMenu from "./desktop-menu";
import MenuContent from "./menu-content";

// Menyu sahifasining ikki ko'rinishi.
//
// Mobil (`menu-content.tsx`) — Flutter WebView va Telegram Mini App
// ICHIDA ham ishlatiladi, shuning uchun unga TEGILMAYDI. Kompyuter
// ko'rinishi (`desktop-menu.tsx`) faqat `xl:` dan boshlanadi: unda
// o'ngda savat paneli bor va u ~1100px dan tor ekranda menyuni
// qisib qo'yardi.
//
// Ikkalasi ham DOM'da bo'ladi, ko'rinishini CSS hal qiladi — bosh
// sahifadagi (`home-content.tsx`) bilan bir xil naqsh.
//
// `useFavorites` SHU YERDA: ikkala ko'rinish bitta ro'yxatni baham
// ko'rishi kerak, aks holda kompyuterda bosilgan yurak mobil
// ko'rinishda eski holatda qolardi (bu bug avval bir marta uchragan —
// `product-card.tsx` dagi `onFavoriteChange` izohiga qarang).
export default function MenuShell({
  restaurant,
  menu,
  promotions,
  signedIn,
}: {
  restaurant: Restaurant;
  menu: Product[];
  promotions: ActivePromotion[];
  signedIn: boolean;
}) {
  const { favoriteIds, onFavoriteChange } = useFavorites();

  return (
    <>
      <div className="hidden xl:block">
        <DesktopMenu
          restaurant={restaurant}
          menu={menu}
          promotions={promotions}
          signedIn={signedIn}
          favoriteIds={favoriteIds}
          onFavoriteChange={onFavoriteChange}
        />
      </div>

      <div className="xl:hidden">
        <MenuContent
          restaurant={restaurant}
          menu={menu}
          promotions={promotions}
        />
      </div>
    </>
  );
}
