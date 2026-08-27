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

// product_grid.dart'dagi discountLineLabel() bilan bir xil qoida.
//
// Chegirma IKKI manbadan kelishi mumkin (aksiya va mahsulotning o'z
// chegirma narxi) va bitta savatda ular ARALASH bo'lishi mumkin —
// server har qatorga eng foydalisini beradi. Shunday holatda
// "Aksiya: <nom>" deb yozish noto'g'ri bo'lardi, chunki summaning bir
// qismi o'sha aksiyadan emas. Nom faqat chegirmaning HAMMASI aksiyadan
// bo'lganda ko'rsatiladi.
export function discountLineLabel(
  discountTiyin: number,
  promotionDiscountTiyin: number,
  promotionName?: string | null,
): string {
  const name = (promotionName ?? "").trim();
  if (!name || promotionDiscountTiyin < discountTiyin) return "Chegirma";
  return `Aksiya: ${name}`;
}

// product_grid.dart'dagi computeProductDiscount() bilan BIR XIL mantiq
// (to'liq izoh o'sha faylda).
//
// ┌─ KARTOCHKADAGI NARX NIMANI ANGLATADI ────────────────────────────┐
// Server HAR QATORGA o'zining eng foydali chegirmasini beradi, ya'ni
// kartochkadagi narx savatdagi narx bilan mos tushadi. Yakuniy raqam
// baribir serverdan: savat/checkout sahifalari `quote.lines` javobini
// chizadi (lib/use-quote.ts), bu funksiya esa menyu kartochkasi uchun.
//
// 1+1, to'plam va sodiqlik aksiyalari bu yerda hisoblanmaydi — ularning
// qiymati savatning qolgan qismiga bog'liq; savatda ular qo'llanadi va
// narx kutilganidan ARZONROQ chiqadi.
// └──────────────────────────────────────────────────────────────────┘
//
// Server bilan bir xil qoidalar: mexanika TUR bo'yicha (`discount_unit`
// emas), `min_order_amount_tiyin` va `max_discount_amount_tiyin`
// hisobga olinadi, chegirmalar qo'shilmaydi — eng foydalisi tanlanadi.
//
// cartSubtotalTiyin — joriy savatning CHEGIRMASIZ summasi (0 =
// noma'lum), faqat minimal buyurtma shartini tekshirish uchun.
export function computeProductDiscount(
  product: Product,
  promotions: ActivePromotion[],
  cartSubtotalTiyin = 0,
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

  // "Minimal buyurtma summasi" sharti shu savatga nisbatan tekshiriladi
  // (savat bo'sh bo'lsa — kamida shu taomning o'zi).
  const basis = Math.max(cartSubtotalTiyin, price);

  for (const p of promotions) {
    if (p.type !== "percent" && p.type !== "fixed_amount") continue;

    // Butun buyurtmaga tegishli aksiya ham HAR QATORGA tushadi (server
    // uni shu qatorning ulushi sifatida hisoblaydi) — shuning uchun u
    // ham nomzod.
    const matchesProduct =
      p.applies_to_products && (p.target_product_ids ?? []).includes(id);
    const matchesCategory =
      p.applies_to_categories &&
      (p.target_categories ?? []).includes(category);
    if (!p.applies_to_orders && !matchesProduct && !matchesCategory) continue;

    const minOrder = p.min_order_amount_tiyin ?? 0;
    if (minOrder > 0 && basis < minOrder) continue;

    const value = p.discount_value ?? 0;
    if (value <= 0) continue;

    // Mexanika TUR bo'yicha — saqlangan `discount_unit` turga zid
    // bo'lishi mumkin edi (promotions.Promotion.EffectiveUnit).
    let lineDiscount =
      p.type === "fixed_amount" ? value : Math.floor((price * value) / 100);
    if (lineDiscount > price) lineDiscount = price;

    // Maksimal chegirma chegarasi ishlasa, "-20%" yozuvi yolg'on bo'lib
    // qoladi — yorliq summaga o'tadi.
    const maxDiscount = p.max_discount_amount_tiyin ?? 0;
    let capped = false;
    if (maxDiscount > 0 && lineDiscount > maxDiscount) {
      lineDiscount = maxDiscount;
      capped = true;
    }
    if (lineDiscount <= bestDiscount) continue;

    bestDiscount = lineDiscount;
    bestLabel =
      p.type === "percent" && !capped
        ? `-${value}%`
        : `-${formatSum(lineDiscount)}`;
  }

  if (bestDiscount <= 0 || !bestLabel) return null;
  return { discountedPriceTiyin: price - bestDiscount, label: bestLabel };
}
