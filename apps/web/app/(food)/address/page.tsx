import { requireAuth } from "@/lib/require-auth";
import AddressClient from "./address-client";

// Autentifikatsiya SHU YERDA tekshiriladi — mahsulotni "rasmiylashtirish"
// (manzil tanlash -> checkout) zanjirining boshlanish nuqtasi. Menyuni
// ko'rish kirishsiz ochiq qoladi, lekin manzil = akkaunt kerak.
export default async function AddressPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  await requireAuth("/address", await searchParams);
  return <AddressClient />;
}
