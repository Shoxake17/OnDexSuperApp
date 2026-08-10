import { formatSum } from "./format";
import type { ActivePromotion, Product } from "./types";

// apps/customer_app/lib/widgets/product_grid.dart'dagi categoryOf() bilan
// bir xil.
export function categoryOf(p: Pick<Product, "category">): string {
  const c = p.category?.trim();
  return c ? c : "Boshqa";
}

export type ProductDiscount = {
  discountedPriceTiyin: number;
  label: string;
};

// product_grid.dart'dagi computeProductDiscount() bilan BIR XIL mantiq
// (taxminiy/vizual — YAKUNIY chegirma har doim checkout'da serverda,
// internal/promotions/apply.go orqali hisoblanadi). Faqat mahsulot/turkum
// darajasidagi percent/fixed_amount aksiyalar hisobga olinadi — butun
// buyurtmaga yoki BOGO/to'plam/sodiqlik aksiyalari uchun "bitta
// mahsulotning yangi narxi" tushunchasi ma'noga ega emas.
export function computeProductDiscount(
  product: Product,
  promotions: ActivePromotion[],
): ProductDiscount | null {
  const id = product.id;
  const category = categoryOf(product);
  const price = product.price_tiyin ?? 0;
  if (price <= 0) return null;

  // Mahsulotning O'Z chegirma narxi ham NOMZOD sifatida qatnashadi.
  //
  // Avval u bu yerda UMUMAN hisobga olinmasdi: menyuda faqat
  // "narx − aksiya" ko'rsatilardi. Natijada ekranda 13 000 so'm
  // yozilgan mahsulotni server 50 so'mga narxlardi — ko'rsatilgan va
  // olinadigan narx bir-biriga umuman mos kelmasdi.
  //
  // Server bilan BIR XIL qoida: chegirmalar QO'SHILMAYDI, eng
  // foydalisi tanlanadi (internal/orders/service.go priceCart).
  let bestDiscount = 0;
  let bestLabel: string | null = null;

  const productDiscountPrice = product.discount_price_tiyin ?? 0;
  if (productDiscountPrice > 0 && productDiscountPrice < price) {
    bestDiscount = price - productDiscountPrice;
    bestLabel = `-${formatSum(bestDiscount)}`;
  }

  for (const p of promotions) {
    if (p.type !== "percent" && p.type !== "fixed_amount") continue;
    if (p.applies_to_orders) continue;

    const matchesProduct =
      p.applies_to_products && (p.target_product_ids ?? []).includes(id);
    const matchesCategory =
      p.applies_to_categories &&
      (p.target_categories ?? []).includes(category);
    if (!matchesProduct && !matchesCategory) continue;

    const unit = p.discount_unit || "percent";
    const value = p.discount_value ?? 0;
    if (value <= 0) continue;

    let lineDiscount =
      unit === "amount" ? value : Math.floor((price * value) / 100);
    if (lineDiscount > price) lineDiscount = price;
    if (lineDiscount <= bestDiscount) continue;

    bestDiscount = lineDiscount;
    bestLabel = unit === "amount" ? `-${formatSum(lineDiscount)}` : `-${value}%`;
  }

  if (bestDiscount <= 0 || !bestLabel) return null;
  return { discountedPriceTiyin: price - bestDiscount, label: bestLabel };
}
