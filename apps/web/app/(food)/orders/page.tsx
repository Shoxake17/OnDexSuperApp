import { getSessionToken } from "@/lib/session";
import { requireAuth } from "@/lib/require-auth";
import DesktopOrders from "./desktop-orders";
import OrdersClient from "./orders-client";

export default async function OrdersPage() {
  await requireAuth("/orders");
  // `requireAuth` shu yergacha o'tkazgan bo'lsa sessiya BOR — navbar
  // uchun qiymat shundan aniq, qo'shimcha tekshiruv shart emas.
  const signedIn = Boolean(await getSessionToken());

  return (
    <>
      {/* Kompyuter va mobil ko'rinishlar — bosh sahifadagi bilan bir xil
          naqsh: ikkalasi ham DOM'da, ko'rinishini CSS hal qiladi. */}
      <div className="hidden xl:block">
        <DesktopOrders signedIn={signedIn} />
      </div>
      <div className="xl:hidden">
        <OrdersClient />
      </div>
    </>
  );
}
