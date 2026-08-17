import { Wallet } from "lucide-react";
import MobileSheet from "../mobile-sheet";

// Hamyon — HALI QURILMAGAN.
//
// ┌─ NEGA SAHIFA BOR, LEKIN FUNKSIYA YO'Q ─────────────────────────────┐
// Maketda (image/restarant.png) sarlavhada hamyon ikoni turibdi va u
// interfeysning bir qismi. Lekin backendda balans, tranzaksiya daftari
// yoki to'lov tizimi UMUMAN yo'q — na jadval, na endpoint.
//
// Ikonni bosganda hech narsa bo'lmasligi yoki bo'sh ekran ochilishi eng
// yomon variant: mijoz buni ilovaning nosozligi deb biladi. Shuning
// uchun sahifa holatni OCHIQ aytadi.
//
// Haqiqiy hamyon qurilganda shu fayl almashtiriladi — ikon, marshrut va
// sarlavhadagi joy allaqachon tayyor bo'ladi.
// └────────────────────────────────────────────────────────────────────┘
export const metadata = { title: "Hamyon" };

export default function WalletPage() {
  return (
    // `pb-10` — `bottom-nav.tsx` bu sahifada menyuni ko'rsatmaydi.
    <MobileSheet className="px-4 pb-10 pt-3">
      <h1 className="text-2xl font-bold">Hamyon</h1>

      <div className="flex flex-col items-center px-6 py-16 text-center">
        <div className="flex h-20 w-20 items-center justify-center rounded-full bg-neutral-100">
          <Wallet size={36} className="text-neutral-400" />
        </div>
        <p className="mt-5 text-lg font-semibold">Tayyorlanmoqda</p>
        <p className="mt-2 max-w-xs text-sm leading-relaxed text-neutral-500">
          Balans, to&apos;ldirish va bonuslar — to&apos;lov tizimi
          (Payme/Click) ulanganidan keyin shu yerda bo&apos;ladi. Hozircha
          buyurtma uchun to&apos;lov kuryerga naqd yoki karta bilan
          amalga oshiriladi.
        </p>
      </div>
    </MobileSheet>
  );
}
