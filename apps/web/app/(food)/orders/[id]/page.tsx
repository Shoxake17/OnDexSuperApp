import { getSessionToken } from "@/lib/session";
import { requireAuth } from "@/lib/require-auth";
import DesktopOrder from "./desktop-order";
import OrderTrackingClient from "./order-client";

export default async function OrderTrackingPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  await requireAuth(`/orders/${id}`);
  const signedIn = Boolean(await getSessionToken());

  return (
    <>
      {/* Kompyuter — ikki ustunli kuzatuv (`desktop-order.tsx`), mobil
          — avvalgi ko'rinish. Ikkalasi ham `lib/use-order-tracking.ts`
          dan foydalanadi, ya'ni jonli yangilanish mantiqi bitta. */}
      <div className="hidden lg:block">
        <DesktopOrder id={id} signedIn={signedIn} />
      </div>
      <div className="lg:hidden">
        {/* `params` shu yerda ALLAQACHON await qilingan — bola
            komponent uni `use()` bilan ochadi, promise qayta
            await qilinsa ham xavfsiz (natija keshlanadi). */}
        <OrderTrackingClient params={params} />
      </div>
    </>
  );
}
