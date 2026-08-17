// internal/catalog/catalog.go'dagi Go struct'lariga mos JSON shakllar.

export type Restaurant = {
  id: string;
  name: string;
  address: string;
  lat: number;
  lng: number;
  open: boolean;
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
// tegishli/xavfsiz maydonlar — cmd/api/main.go'ga qarang). DIQQAT:
// max_discount_amount_tiyin bu javobda YO'Q (faqat backend'ning ichki
// hisob-kitobida ishlatiladi) — shuning uchun bu yerdagi vizual taxmin
// (computeProductDiscount) uni cheklovsiz hisoblaydi; YAKUNIY chegirma
// har doim checkout'da serverda (max bilan) hisoblanadi.
export type ActivePromotion = {
  id: string;
  name: string;
  type: string; // percent | fixed_amount | bogo | bundle | free_delivery | loyalty
  discount_unit: string; // percent | amount
  discount_value: number;
  applies_to_orders: boolean;
  applies_to_products: boolean;
  applies_to_categories: boolean;
  target_product_ids: string[];
  target_categories: string[];
};
