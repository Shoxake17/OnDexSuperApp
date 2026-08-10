import type { Metadata, Viewport } from "next";
import "./globals.css";

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

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="uz">
      <body>{children}</body>
    </html>
  );
}
