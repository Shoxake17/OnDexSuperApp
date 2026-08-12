import type { Metadata, Viewport } from "next";
import "./globals.css";
import TelegramAuth from "./telegram-auth";
import TableInit from "./table-init";
import { getSessionToken } from "@/lib/session";

export const metadata: Metadata = {
  title: "ChustApp",
  description: "Chust bo'ylab taom yetkazib berish",
};

// MUHIM (haqiqiy Android qurilmada topilgan bug): viewport meta tegi
// bo'lmasa, mobil WebView sahifani "desktop" kengligida (~980px) deb
// hisoblab, butun sahifani kichraytirib ko'rsatadi — vizual hammasi joyida
// ko'rinadi-yu, lekin haqiqiy ekran koordinatalari bilan brauzer
// hisoblagan (kichraytirilgan) koordinatalar mos kelmay qoladi, natijada
// tugmalar bosilganda umuman ishlamaydi (foydalanuvchi haqiqiy qurilmada
// aynan shu holatni uchratdi — "hech qaysi icon ishlamayapti").
export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 1,
  viewportFit: "cover",
};

export default async function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  // ┌─ TELEGRAM MINI APP AVTOMATIK KIRISHI ─────────────────────────────┐
  // Sessiya BOR-YO'QLIGI serverda aniqlanadi va klientga faqat `true/
  // false` beriladi — tokenning o'zi brauzerga hech qachon chiqmaydi
  // (u httpOnly cookie'da).
  //
  // `TelegramAuth` Telegram TASHQARISIDA hech narsa qilmaydi: oddiy
  // brauzerda va Flutter WebView'da `initData` bo'lmaydi, shuning
  // uchun u jimgina chetga chiqadi va mavjud kirish oqimlariga
  // (bridge, SMS) umuman xalaqit bermaydi.
  // └───────────────────────────────────────────────────────────────────┘
  const signedIn = Boolean(await getSessionToken());

  return (
    <html lang="uz">
      <body>
        <TelegramAuth signedIn={signedIn} />
        {/* Stol QR kodi bilan kelgan mijozni to'g'ri menyuga olib
            boradi. Telegram tashqarisida hech narsa qilmaydi. */}
        <TableInit signedIn={signedIn} />
        {children}
      </body>
    </html>
  );
}
