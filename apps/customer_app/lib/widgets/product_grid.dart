/// Taom kartochkasi va u bilan bog'liq narx mantig'i — BITTA nusxa.
///
/// ┌─ NEGA BU FAYL ────────────────────────────────────────────────────┐
/// Kartochka uch joyda kerak: menyu, menyudagi qidiruv va
/// "Istaklarim". Uchalasida alohida yozilsa — bu Flutter STEKI ICHIDA
/// dublikat bo'lardi (loyiha qoidasi buni taqiqlaydi: steklar orasida
/// takrorlash mumkin, stek ichida yo'q).
///
/// Vebdagi mos fayl — `apps/web/app/(food)/restaurants/[id]/
/// product-card.tsx`. Ikkalasi ATAYLAB bir xil ko'rinadi.
/// └───────────────────────────────────────────────────────────────────┘
library;

import 'package:flutter/material.dart';

import '../api.dart';
import '../data/favorites_store.dart';
import 'common.dart';
import '../screens/catalog_screen.dart' show kBrand;

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

// ═══════════════════════════════════════════════════════════════════
// SEVIMLI TUGMASI
// ═══════════════════════════════════════════════════════════════════

/// "Istaklarim" yurak belgisi — oq doira, elevatsiyali; o'z holatini o'zi
/// boshqaradi: bosilganda DARHOL (optimistik) qizarib/oqarib, fon rejimida
/// serverga so'rov yuboradi. Xato bo'lsa (masalan tarmoq uzilsa) holat
/// ORQAGA qaytariladi — foydalanuvchi hech qachon soxta/yolg'on holat
/// ko'rmaydi.
class FavoriteButton extends StatefulWidget {
  final String productId;

  /// ESKIRGAN: holat endi [FavoritesStore] dan olinadi. Parametr
  /// chaqiruvchilarni buzmaslik uchun qoldirilgan va E'TIBORGA
  /// OLINMAYDI.
  final bool initialFavorited;
  // onChanged — muvaffaqiyatli o'zgarishdan KEYIN chaqiriladi (masalan
  // "Istaklarim" sahifasi shu orqali mahsulotni ro'yxatdan olib tashlaydi).
  final ValueChanged<bool>? onChanged;
  final double size;

  const FavoriteButton({
    super.key,
    required this.productId,
    required this.initialFavorited,
    this.onChanged,
    this.size = 20,
  });

  @override
  State<FavoriteButton> createState() => _FavoriteButtonState();
}

class _FavoriteButtonState extends State<FavoriteButton> {
  bool _busy = false;

  /// ┌─ HOLAT BU YERDA SAQLANMAYDI ────────────────────────────────────┐
  /// Ilgari har tugma o'z `_favorited` ini saqlardi. Shuning uchun
  /// bitta mahsulot ikki joyda ko'rinsa (menyu to'ri va taom tavsifi
  /// paneli) ular bir-biridan bexabar qolardi va biri eskirgan belgini
  /// ko'rsatib turardi.
  ///
  /// Endi yagona manba — [FavoritesStore]. Tugma unga QULOQ SOLADI,
  /// ya'ni qayerda bosilishidan qat'i nazar hamma nusxa bir vaqtda
  /// o'zgaradi.
  /// └─────────────────────────────────────────────────────────────────┘
  Future<void> _toggle() async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await FavoritesStore.instance.toggle(widget.productId);
      widget.onChanged
          ?.call(FavoritesStore.instance.contains(widget.productId));
    } catch (_) {
      // Belgi `FavoritesStore` da allaqachon o'z holiga qaytarilgan.
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: FavoritesStore.instance,
      builder: (context, _) {
        final favorited = FavoritesStore.instance.contains(widget.productId);
        return Material(
          color: Colors.white,
          shape: const CircleBorder(),
          elevation: 3,
          child: InkWell(
            customBorder: const CircleBorder(),
            onTap: _toggle,
            child: Padding(
              padding: const EdgeInsets.all(8),
              child: Icon(
                favorited ? Icons.favorite : Icons.favorite_border,
                color: favorited ? const Color(0xFFE53935) : Colors.black,
                size: widget.size,
              ),
            ),
          ),
        );
      },
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// TAOM KARTOCHKASI
// ═══════════════════════════════════════════════════════════════════

/// Kvadrat rasm, chap yuqorida yurak, o'ng yuqorida aksiya lentasi,
/// pastda "+" yoki "− n +" boshqaruvi; rasm ostida narx, nom, og'irlik.
///
/// Veb bilan parity: `product-card.tsx`.
class ProductCard extends StatelessWidget {
  final Map<String, dynamic> product;

  /// Savatdagi miqdor. `null` — miqdor boshqaruvi UMUMAN chizilmaydi
  /// ("Istaklarim" sahifasi shunday: u yerdan savatga qo'shilmaydi,
  /// chunki bitta buyurtma bitta restorandan bo'lishi shart).
  final int? qty;

  final ProductDiscount? discount;
  final bool promoted;
  final bool favorited;

  final VoidCallback? onAdd;

  /// "+" tugmasining kaliti.
  ///
  /// ┌─ NEGA KERAK ────────────────────────────────────────────────────┐
  /// Shaddiy buyurtmani ko'rsatib berganda "barmoq" AYNAN shu tugma
  /// ustida turishi kerak. Ilgari u kartochka MARKAZIGA qo'yilardi va
  /// miqdor raqamining ustiga tushib qolardi — foydalanuvchi noto'g'ri
  /// tugma bosilyapti deb o'ylardi.
  ///
  /// Tugmaning joyi savatdagi miqdorga qarab O'ZGARADI (bo'shda —
  /// o'ng pastda, qo'shilgach — miqdor qatorining o'ng chetida),
  /// shuning uchun joy har bosishdan oldin qaytadan o'lchanadi.
  /// └─────────────────────────────────────────────────────────────────┘
  final Key? addKey;

  final VoidCallback? onRemove;
  final VoidCallback? onTap;
  final ValueChanged<bool>? onFavoriteChanged;

  /// Nom/narx ostiga qo'shiladigan qator (masalan "Istaklarim" dagi
  /// restoran nomi). Menyuda ishlatilmaydi.
  final Widget? footer;

  const ProductCard({
    super.key,
    required this.product,
    this.qty,
    this.discount,
    this.promoted = false,
    this.favorited = false,
    this.onAdd,
    this.addKey,
    this.onRemove,
    this.onTap,
    this.onFavoriteChanged,
    this.footer,
  });

  @override
  Widget build(BuildContext context) {
    final id = (product['id'] as String?) ?? '';
    final name = (product['name'] as String?) ?? '';
    final image = (product['image_url'] as String?) ?? '';
    final price = (product['price_tiyin'] as num?)?.toInt() ?? 0;
    final available = product['available'] != false;
    final weight = (product['weight'] as num?)?.toDouble() ?? 0;
    final unit = formatWeightUnit((product['weight_unit'] as String?) ?? '');

    return Opacity(
      opacity: available ? 1 : 0.4,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          AspectRatio(
            aspectRatio: 1,
            child: Stack(
              children: [
                Positioned.fill(
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(16),
                    child: Material(
                      color: const Color(0xFFF5F5F5),
                      child: InkWell(
                        onTap: onTap,
                        child: _ProductImage(url: image),
                      ),
                    ),
                  ),
                ),
                Positioned(
                  left: 8,
                  top: 8,
                  child: FavoriteButton(
                    productId: id,
                    initialFavorited: favorited,
                    onChanged: onFavoriteChanged,
                  ),
                ),
                if (promoted && available)
                  Positioned(
                    right: 8,
                    top: 8,
                    child: _Badge(
                      text: discount?.label ?? 'Aksiya',
                      background: kBrand,
                      foreground: Colors.white,
                    ),
                  ),
                if (!available)
                  const Positioned(
                    left: 8,
                    bottom: 8,
                    child: _Badge(
                      text: 'Tugadi',
                      background: Color(0xD9000000),
                      foreground: Colors.white,
                    ),
                  ),
                if (available && qty != null && qty == 0)
                  Positioned(
                    right: 8,
                    bottom: 8,
                    child: RoundIconButton(
                        key: addKey,
                        icon: Icons.add,
                        onTap: onAdd,
                        iconSize: 19),
                  ),
                if (available && qty != null && qty! > 0)
                  Positioned(
                    left: 8,
                    right: 8,
                    bottom: 8,
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        RoundIconButton(icon: Icons.remove, onTap: onRemove, iconSize: 19),
                        Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 10, vertical: 5),
                          decoration: BoxDecoration(
                            color: Colors.white,
                            borderRadius: BorderRadius.circular(20),
                            boxShadow: const [
                              BoxShadow(
                                color: Color(0x33000000),
                                blurRadius: 4,
                                offset: Offset(0, 2),
                              ),
                            ],
                          ),
                          child: Text(
                            '$qty',
                            style: const TextStyle(
                                fontWeight: FontWeight.bold, fontSize: 13),
                          ),
                        ),
                        RoundIconButton(
                            key: addKey,
                            icon: Icons.add,
                            onTap: onAdd,
                            iconSize: 19),
                      ],
                    ),
                  ),
              ],
            ),
          ),
          const SizedBox(height: 8),
          InkWell(
            onTap: onTap,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (discount != null)
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Text(
                        formatSum(discount!.discountedPriceTiyin),
                        style: const TextStyle(
                            fontWeight: FontWeight.bold,
                            fontSize: 15,
                            color: Color(0xFFE53935)),
                      ),
                      const SizedBox(width: 6),
                      Flexible(
                        child: Text(
                          formatSum(price),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontSize: 12,
                            color: Color(0xFF757575),
                            decoration: TextDecoration.lineThrough,
                          ),
                        ),
                      ),
                    ],
                  )
                else
                  Text(
                    formatSum(price),
                    style: const TextStyle(
                        fontWeight: FontWeight.bold, fontSize: 15),
                  ),
                const SizedBox(height: 2),
                Text(
                  name,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 13, height: 1.25),
                ),
                if (weight > 0)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(
                      '${weight % 1 == 0 ? weight.toInt() : weight.toStringAsFixed(1)} $unit'
                          .trim(),
                      style: const TextStyle(
                          fontSize: 12, color: Color(0xFF757575)),
                    ),
                  ),
                if (footer != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: footer,
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _ProductImage extends StatelessWidget {
  final String url;
  const _ProductImage({required this.url});

  static const _placeholder = Center(
    child: Icon(Icons.restaurant_menu, size: 30, color: Color(0xFFBDBDBD)),
  );

  @override
  Widget build(BuildContext context) =>
      RemoteImage(url: url, placeholder: _placeholder);
}

class _Badge extends StatelessWidget {
  final String text;
  final Color background;
  final Color foreground;

  const _Badge({
    required this.text,
    required this.background,
    required this.foreground,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 3),
      decoration: BoxDecoration(
        color: background,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        text,
        style: TextStyle(
            fontSize: 11, fontWeight: FontWeight.bold, color: foreground),
      ),
    );
  }
}

/// Rasm ostidagi matn blokining balandligi (piksel).
///
/// Narx (~20) + oraliqlar (8+2) + ikki qatorli nom (13px × 1.25 × 2 ≈ 34)
/// + og'irlik (17) + kichik zaxira. Ikki qator DOIM zaxiraga olinadi —
/// aks holda bir qatorli va ikki qatorli nomli kartochkalar turli
/// balandlikda bo'lib, to'r notekis ko'rinardi.
///
/// Qiymat QURILMADA o'lchandi: avval 82 edi va "Istaklarim" ekranida
/// "BOTTOM OVERFLOWED BY 4.0 PIXELS" bergan. Zaxira ataylab ozgina
/// ortiqcha — bir necha piksel bo'sh joy sezilmaydi, toshib ketish esa
/// darhol ko'rinadi.
const _kCardTextHeight = 88.0;

/// [ProductCard.footer] bor bo'lganda qo'shiladigan balandlik
/// (restoran nomi qatori: 16px logo + 4px chekinish).
const kCardFooterHeight = 24.0;

/// Kartochkalar to'ri — vebdagi `grid-cols-2` bilan bir xil.
///
/// ┌─ NEGA `childAspectRatio` EMAS ────────────────────────────────────┐
/// Avval bu yerda qat'iy `childAspectRatio: 0.62` turardi va u NOTO'G'RI
/// edi: nisbat kenglikka bog'liq. Kartochka balandligi = kvadrat rasm
/// (kenglikka teng) + matn bloki (kenglikdan MUSTAQIL, ~82px). Ya'ni
/// to'g'ri nisbat `w / (w + 82)` — telefon kengligiga qarab 0.70 dan
/// 0.81 gacha o'zgaradi.
///
/// Qat'iy 0.62 keng ekranda kerakdan baland katak berardi va qatorlar
/// orasida katta bo'sh joy qolardi (qurilmada aynan shu ko'rindi).
///
/// `mainAxisExtent` bilan balandlik ANIQ hisoblanadi — na bo'sh joy,
/// na kesilish.
/// └───────────────────────────────────────────────────────────────────┘
///
/// [availableWidth] — to'r egallaydigan kenglik (yon chekinishlar
/// AYIRILGAN holda).
SliverGridDelegate productGridDelegate(
  double availableWidth, {
  int columns = 2,
  double spacing = 14,
  double extraHeight = 0,
}) {
  final cell = (availableWidth - spacing * (columns - 1)) / columns;
  return SliverGridDelegateWithFixedCrossAxisCount(
    crossAxisCount: columns,
    crossAxisSpacing: spacing,
    mainAxisSpacing: 20,
    mainAxisExtent: cell + _kCardTextHeight + extraHeight,
  );
}

/// Ekran kengligidan to'r kengligini hisoblaydi — barcha chaqiruv
/// joylarida yon chekinish 16px.
///
/// [extraHeight] — kartochkada qo'shimcha qator bo'lsa
/// ("Istaklarim" dagi restoran nomi): [kCardFooterHeight].
SliverGridDelegate productGridOf(BuildContext context,
        {double extraHeight = 0}) =>
    productGridDelegate(MediaQuery.sizeOf(context).width - 32,
        extraHeight: extraHeight);
