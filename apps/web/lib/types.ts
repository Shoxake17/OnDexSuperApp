// internal/catalog/catalog.go'dagi Go struct'lariga mos JSON shakllar.

export type Restaurant = {
  id: string;
  name: string;
  address: string;
  lat: number;
  lng: number;
  /** Restoranning QO'LDA bosadigan tugmasi. Ko'rsatish uchun ishlatilmaydi — `lib/restaurant-status.ts`. */
  open: boolean;
  /** HOZIR buyurtma qabul qiladimi (tugma VA ish vaqti) — server hisoblaydi. */
  open_now?: boolean;
  closed_reason?: "manual" | "hours";
  /** Holat o'z-o'zidan o'zgaradigan payt (ISO, UTC). */
  open_changes_at?: string;
  logo_url: string;
  cover_url: string;
  tags: string;

  // 0 = "ma'lumot yo'q", "yomon" EMAS. Kartada 0 bo'lgan ko'rsatkich
  // UMUMAN chizilmaydi — "0.0 ★" yoki "0 daqiqa" ko'rsatilmaydi.
  // Qiymatlar hozircha admin panelda qo'lda kiritiladi.
  rating: number;
  rating_count: number;
  eta_min_minutes: number;
  eta_max_minutes: number;
};

export type Product = {
  id: string;
  restaurant_id: string;
  name: string;
  category: string;
  price_tiyin: number;
  discount_price_tiyin: number;
  stock: number;
  weight: number;
  weight_unit: string;
  description: string;
  prep_time_text: string;
  image_url: string;
  available: boolean;
};

export type ProductSearchResult = Product & {
  restaurant_name: string;
  restaurant_logo_url: string;
  restaurant_open: boolean;
};

// GET /restaurants/{id}/active-promotions natijasi (faqat mijozga
// tegishli/xavfsiz maydonlar — internal/httpapi/routes_promotions.go).
//
// Chegirma CHEKLOVLARI ham shu javobda keladi: ularsiz menyudagi narx
// serverning narxidan farq qilardi (`max_discount_amount_tiyin` avval
// umuman yuborilmasdi va "30%, lekin ko'pi bilan 50 000 so'm" aksiyasi
// menyuda to'liq 30% bo'lib ko'rinardi).
export type ActivePromotion = {
  id: string;
  name: string;
  type: string; // percent | fixed_amount | bogo | bundle | free_delivery | loyalty
  discount_unit: string; // percent | amount
  discount_value: number;
  // Cheklovlar — 0 = cheklov yo'q. Ikkalasi ham HISOBGA OLINISHI SHART:
  // aks holda menyuda ko'rsatilgan narx serverning narxidan farq qiladi
  // (max_discount_amount_tiyin avval API javobida umuman yo'q edi).
  min_order_amount_tiyin: number;
  max_discount_amount_tiyin: number;
  applies_to_orders: boolean;
  applies_to_products: boolean;
  applies_to_categories: boolean;
  target_product_ids: string[];
  target_categories: string[];
};
