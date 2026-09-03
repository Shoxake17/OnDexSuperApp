import { requireAuth } from "@/lib/require-auth";
import OrdersClient from "./orders-client";

export default async function OrdersPage() {
  await requireAuth("/orders");
  return <OrdersClient />;
}
