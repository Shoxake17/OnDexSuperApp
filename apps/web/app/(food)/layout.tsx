import { CartProvider } from "@/lib/cart-context";
import { isAppShell } from "@/lib/session";
import BottomNav from "./bottom-nav";

export default async function FoodLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  // ┌─ ILOVA ICHIDA PASTKI MENYU CHIZILMAYDI ───────────────────────┐
  // Mijoz ilovasi (Flutter) shu sahifalarni WebView orqali ko'rsatadi
  // va uning O'Z pastki menyusi bor. Ikkalasi birga chizilganda ekran
  // pastida ikkita menyu ustma-ust turardi.
  //
  // Tekshiruv SERVERDA: menyu bir zumga ko'rinib keyin yo'qolmaydi.
  // Belgi `/api/bridge` da qo'yiladi — u yerga faqat ilova murojaat
  // qiladi (`lib/session.ts` dagi izoh). Telegram Mini App va oddiy
  // brauzerda menyu HAR DOIM qoladi: u yerda boshqa navigatsiya yo'q.
  // └───────────────────────────────────────────────────────────────┘
  const inApp = await isAppShell();

  return (
    <CartProvider>
      {children}
      {/* Pastki menyu O'ZI qaysi sahifada ko'rinishini hal qiladi
          (`bottom-nav.tsx` izohiga qarang) — shuning uchun uni har bir
          sahifada alohida chizish shart emas. */}
      {!inApp && <BottomNav />}
    </CartProvider>
  );
}
