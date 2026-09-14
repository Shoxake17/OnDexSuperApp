/// Menyu kartochkasidagi narx va aksiya mantig'i — BITTA nusxa.
///
/// Mijoz ilovasi (`apps/customer_app`) va affitsiant ilovasi
/// (`apps/waiter_app`) shu faylni ishlatadi. Vebdagi mos fayl —
/// `apps/web/lib/promotions.ts`; qoidalar ATAYLAB bir xil.
library;

import 'package:ondex_core/ondex_core.dart' show formatSum;

// ═══════════════════════════════════════════════════════════════════
// TURKUM VA CHEGIRMA
// ═══════════════════════════════════════════════════════════════════

/// `apps/web/lib/promotions.ts` dagi `categoryOf()` bilan bir xil.
String categoryOf(Map<String, dynamic> product) {
  final c = ((product['category'] as String?) ?? '').trim();
  return c.isEmpty ? 'Boshqa' : c;
}

/// Kartochkada ko'rsatiladigan chegirma.
class ProductDiscount {
  /// Chegirmadan KEYINGI birlik narxi (tiyin).
  final int discountedPriceTiyin;

  /// Lenta yozuvi — `-20%` yoki `-5 000 so'm`.
  final String label;

  const ProductDiscount(this.discountedPriceTiyin, this.label);
}

/// `apps/web/lib/promotions.ts` dagi `computeProductDiscount()` bilan
/// BIR XIL mantiq.
///
/// ┌─ KARTOCHKADAGI NARX NIMANI ANGLATADI ─────────────────────────────┐
/// Server HAR QATORGA o'zining eng foydali chegirmasini beradi
/// (`promotions.Apply`), shuning uchun kartochkadagi narx —
/// "shu taomni olsam, u shu narxda bo'ladi" degani. Menyudagi narxlar
/// yig'indisi savatdagi jami bilan mos tushishi SHART.
///
/// Yakuniy raqam baribir SERVERDAN olinadi: savat va rasmiylashtirish
/// ekranlari `POST /restaurants/{id}/quote` javobidagi `lines`
/// massivini chizadi (bu funksiya faqat MENYU kartochkasi uchun).
/// └───────────────────────────────────────────────────────────────────┘
///
/// Server bilan bir xil qoidalar:
///   • mexanika TUR (`type`) bo'yicha aniqlanadi — `discount_unit`
///     emas (`promotions.Promotion.EffectiveUnit`). Avval klient
///     birlikka qarardi va tur bilan zid yozuvda menyuda "-20%" yozib,
///     server 20 tiyin chegirma berardi;
///   • `min_order_amount_tiyin` — savat summasi yetmasa aksiya
///     KO'RSATILMAYDI (server ham qo'llamaydi);
///   • `max_discount_amount_tiyin` — chegirma shu chegaradan oshmaydi;
///   • chegirmalar QO'SHILMAYDI — bitta taomga eng foydalisi tanlanadi.
///
/// Qatnashadigan aksiyalar: `percent` va `fixed_amount` — mahsulotga,
/// turkumga yoki BUTUN BUYURTMAGA tegishli bo'lishidan qat'i nazar.
///
/// 1+1 (BOGO), to'plam va sodiqlik aksiyalari ATAYLAB hisoblanmaydi:
/// ularning qiymati savatning qolgan qismiga bog'liq (nechta dona
/// olingani, to'plamning boshqa taomlari, oldingi buyurtmalar soni),
/// ya'ni bitta kartochkada halol ko'rsatib bo'lmaydi. Ular faqat
/// "Aksiya" lentasi bilan belgilanadi va savatda HAQIQIY narx bilan
/// qo'llanadi — ya'ni mijoz kutganidan ko'ra ARZONROQ chiqadi, aksincha
/// emas.
///
/// [cartSubtotalTiyin] — joriy savatning CHEGIRMASIZ summasi (0 =
/// noma'lum/bo'sh). Faqat `min_order_amount_tiyin` shartini tekshirish
/// uchun kerak.
ProductDiscount? computeProductDiscount(
  Map<String, dynamic> product,
  List<Map<String, dynamic>> promotions, {
  int cartSubtotalTiyin = 0,
}) {
  final id = (product['id'] as String?) ?? '';
  final category = categoryOf(product);
  final price = (product['price_tiyin'] as num?)?.toInt() ?? 0;
  if (price <= 0) return null;

  var bestDiscount = 0;
  String? bestLabel;

  // Mahsulotning O'Z chegirma narxi ham nomzod — aks holda menyuda
  // aksiyasiz, lekin chegirmali taom to'liq narxda ko'rinardi.
  final own = (product['discount_price_tiyin'] as num?)?.toInt() ?? 0;
  if (own > 0 && own < price) {
    bestDiscount = price - own;
    bestLabel = '-${formatSum(bestDiscount)}';
  }

  // "Minimal buyurtma summasi" sharti: savat allaqachon shu summadan
  // katta bo'lsa — savat summasi, aks holda kamida shu taomning o'zi
  // (kartochka ma'nosi: "yolg'iz olsam").
  final basis = cartSubtotalTiyin > price ? cartSubtotalTiyin : price;

  for (final p in promotions) {
    final type = (p['type'] as String?) ?? '';
    if (type != 'percent' && type != 'fixed_amount') continue;

    // Butun buyurtmaga tegishli aksiya HAR QATORGA ham tushadi (server
    // uni shu qatorning ulushi sifatida hisoblaydi), shuning uchun u
    // ham nomzod. Avval o'tkazib yuborilardi va kartochka aksiyani
    // ko'rsatmasdi — savatda esa narx arzonlab, ikki xil raqam chiqardi.
    final appliesToOrders = p['applies_to_orders'] == true;
    final matchesProduct = p['applies_to_products'] == true &&
        ((p['target_product_ids'] as List?) ?? const []).contains(id);
    final matchesCategory = p['applies_to_categories'] == true &&
        ((p['target_categories'] as List?) ?? const []).contains(category);
    if (!appliesToOrders && !matchesProduct && !matchesCategory) continue;

    final minOrder = (p['min_order_amount_tiyin'] as num?)?.toInt() ?? 0;
    if (minOrder > 0 && basis < minOrder) continue;

    final value = (p['discount_value'] as num?)?.toInt() ?? 0;
    if (value <= 0) continue;

    // Foizda butun bo'lish — server ham shunday yaxlitlaydi.
    var lineDiscount = type == 'fixed_amount' ? value : (price * value) ~/ 100;
    if (lineDiscount > price) lineDiscount = price;

    // Maksimal chegirma chegarasi — chegara ishlaganda "-20%" yozuvi
    // yolg'on bo'lib qoladi, shuning uchun yorliq summaga o'tadi.
    final maxDiscount = (p['max_discount_amount_tiyin'] as num?)?.toInt() ?? 0;
    var capped = false;
    if (maxDiscount > 0 && lineDiscount > maxDiscount) {
      lineDiscount = maxDiscount;
      capped = true;
    }
    if (lineDiscount <= bestDiscount) continue;

    bestDiscount = lineDiscount;
    bestLabel = (type == 'percent' && !capped)
        ? '-$value%'
        : '-${formatSum(lineDiscount)}';
  }

  if (bestDiscount <= 0 || bestLabel == null) return null;
  return ProductDiscount(price - bestDiscount, bestLabel);
}

/// Hisob-kitobdagi chegirma qatorining yozuvi.
///
/// Chegirma IKKI manbadan kelishi mumkin — aksiya va mahsulotning o'z
/// chegirma narxi — va bitta savatda ular ARALASH bo'lishi mumkin
/// (server har qatorga eng foydalisini beradi). Shunday holatda
/// "Aksiya: <nom>" deb yozish mijozga noto'g'ri ma'lumot berardi:
/// summaning bir qismi umuman o'sha aksiyadan emas.
///
/// Shuning uchun nom FAQAT chegirmaning HAMMASI aksiyadan bo'lganda
/// ko'rsatiladi. `apps/web/lib/promotions.ts` da bir xil qoida.
String discountLineLabel({
  required int discountTiyin,
  required int promotionDiscountTiyin,
  String? promotionName,
}) {
  final name = (promotionName ?? '').trim();
  if (name.isEmpty || promotionDiscountTiyin < discountTiyin) return 'Chegirma';
  return 'Aksiya: $name';
}

/// Aksiya BOR-YO'QLIGINI aniqlaydi (lenta uchun).
///
/// [computeProductDiscount] dan farqi: bu yerda butun buyurtmaga
/// tegishli va BOGO kabi aksiyalar ham hisobga olinadi — ular narxni
/// o'zgartirmasa ham, mijoz "bu taomda aksiya bor" deb bilishi kerak.
class PromotionIndex {
  final Set<String> _productIds;
  final Set<String> _categories;
  final bool _orderWide;

  const PromotionIndex._(this._productIds, this._categories, this._orderWide);

  static const empty = PromotionIndex._({}, {}, false);

  factory PromotionIndex(List<Map<String, dynamic>> promotions) {
    final productIds = <String>{};
    final categories = <String>{};
    var orderWide = false;
    for (final p in promotions) {
      if (p['applies_to_orders'] == true) orderWide = true;
      if (p['applies_to_products'] == true) {
        for (final id in (p['target_product_ids'] as List?) ?? const []) {
          if (id is String) productIds.add(id);
        }
      }
      if (p['applies_to_categories'] == true) {
        for (final c in (p['target_categories'] as List?) ?? const []) {
          if (c is String) categories.add(c);
        }
      }
    }
    return PromotionIndex._(productIds, categories, orderWide);
  }

  bool covers(Map<String, dynamic> product, ProductDiscount? discount) =>
      _orderWide ||
      discount != null ||
      _productIds.contains((product['id'] as String?) ?? '') ||
      _categories.contains(categoryOf(product));
}
