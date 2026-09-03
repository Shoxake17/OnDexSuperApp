import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { safeRedirect } from "@/lib/safe-redirect";
import LoginClient from "./login-client";

export const metadata = { title: "Kirish" };

// ┌─ NEGA `(food)` GURUHI TASHQARISIDA ────────────────────────────────┐
// `(food)/layout.tsx` savat/agent kontekstini va pastki menyuni
// qo'shadi — bu sahifaga ikkalasi ham kerak emas, faqat chalg'itadi.
// Root `layout.tsx` (TelegramAuth va h.k.) baribir qamrab oladi.
// └───────────────────────────────────────────────────────────────────┘
//
// ┌─ NEGA TELEGRAM/ILOVA HAQIDA HECH NARSA BILISHNING HOJATI YO'Q ─────┐
// Ular ichida bu sahifaga tashrif buyurish AMALDA sodir bo'lmaydi:
// Telegram — root layout'dagi to'suvchi modal auth tugamaguncha boshqa
// hech qayerga o'tkazmaydi; ilova — `/api/bridge` cookie'ni SAHIFA
// render bo'lishidan OLDIN o'rnatadi. Shuning uchun bu yerga tushgan
// HAR KIM — oddiy brauzer mehmoni (`lib/require-auth.ts` bilan bir xil
// mantiq).
// └───────────────────────────────────────────────────────────────────┘

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ next?: string }>;
}) {
  const { next } = await searchParams;
  const target = safeRedirect(next);

  // Allaqachon kirgan bo'lsa — forma o'rniga darhol maqsadga.
  if (await getSessionToken()) {
    redirect(target);
  }

  return <LoginClient next={target} />;
}
