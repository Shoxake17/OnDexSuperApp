import { requireAuth } from "@/lib/require-auth";
import OrderTrackingClient from "./order-client";

export default async function OrderTrackingPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  await requireAuth(`/orders/${id}`);
  // `params` shu yerda ALLAQACHON await qilingan (yuqorida) — Promise'ning
  // o'zi bir necha marta await qilinsa ham xavfsiz (natija keshlanadi),
  // shuning uchun bolaga AYNAN o'sha promise qayta uzatiladi.
  return <OrderTrackingClient params={params} />;
}
