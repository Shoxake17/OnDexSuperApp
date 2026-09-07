import { CartProvider } from "@/lib/cart-context";
import { isAppShell } from "@/lib/session";
import { AgentActivityProvider } from "./agent-activity";
import BottomNav from "./bottom-nav";
import { DesktopCheckoutProvider } from "./desktop-checkout-context";
import WebMcpTools from "./webmcp-tools";

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
      <AgentActivityProvider>
        {/* ┌─ WEBMCP ─────────────────────────────────────────────────┐
            Sahifaning amallarini brauzer agentiga ochadi. Hech narsa
            chizmaydi va WebMCP qo'llanmagan brauzerda BUTUNLAY jim
            turadi (`webmcpAvailable`), ya'ni oddiy foydalanuvchi uchun
            hech qanday farq yo'q.

            Bu yerda — `CartProvider` ICHIDA: amallar savat holatini
            o'qiydi va o'zgartiradi, ya'ni odam bosgandagi bilan
            aynan bir xil yo'ldan.
            └──────────────────────────────────────────────────────────┘ */}
        <WebMcpTools />
        {/* Rasmiylashtirish oynasi (kompyuter) — BITTA nusxa, ikki
            joydan ochiladi (`desktop-checkout-context.tsx`). Mobil
            ko'rinish bu oynani umuman ishlatmaydi. */}
        <DesktopCheckoutProvider>{children}</DesktopCheckoutProvider>
        {/* Pastki menyu O'ZI qaysi sahifada ko'rinishini hal qiladi
            (`bottom-nav.tsx` izohiga qarang) — shuning uchun uni har bir
            sahifada alohida chizish shart emas. */}
        {!inApp && <BottomNav />}
      </AgentActivityProvider>
    </CartProvider>
  );
}
