import { CartProvider } from "@/lib/cart-context";

export default function FoodLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return <CartProvider>{children}</CartProvider>;
}
