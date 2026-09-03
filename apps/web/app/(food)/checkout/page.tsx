import { requireAuth } from "@/lib/require-auth";
import CheckoutClient from "./checkout-client";

export default async function CheckoutPage() {
  await requireAuth("/checkout");
  return <CheckoutClient />;
}
