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
/// ┌─ BU TAXMIN, HISOB-KITOB EMAS ─────────────────────────────────────┐
/// Bu yerdagi natija FAQAT ekranga chiziladi. Haqiqiy summa har doim
/// serverda hisoblanadi (`internal/promotions/apply.go`) va mijozga
/// `POST /restaurants/{id}/quote` orqali qaytadi. Ya'ni bu funksiya
/// noto'g'ri ishlasa ham mijoz noto'g'ri summa TO'LAMAYDI — u faqat
/// noto'g'ri raqam ko'radi.
///
/// Faqat mahsulot/turkum darajasidagi `percent`/`fixed_amount`
/// aksiyalar qatnashadi: butun buyurtmaga tegishli yoki BOGO/to'plam
/// aksiyalar uchun "bitta taomning yangi narxi" tushunchasi ma'noga
/// ega emas.
///
/// Chegirmalar QO'SHILMAYDI — server bilan bir xil qoida bo'yicha eng
/// foydalisi tanlanadi (`internal/orders/service.go` `priceCart`).
/// └───────────────────────────────────────────────────────────────────┘
ProductDiscount? computeProductDiscount(
  Map<String, dynamic> product,
  List<Map<String, dynamic>> promotions,
) {
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

  for (final p in promotions) {
    final type = (p['type'] as String?) ?? '';
    if (type != 'percent' && type != 'fixed_amount') continue;
    if (p['applies_to_orders'] == true) continue;

    final matchesProduct = p['applies_to_products'] == true &&
        ((p['target_product_ids'] as List?) ?? const []).contains(id);
    final matchesCategory = p['applies_to_categories'] == true &&
        ((p['target_categories'] as List?) ?? const []).contains(category);
    if (!matchesProduct && !matchesCategory) continue;

    final unit = ((p['discount_unit'] as String?) ?? '').isEmpty
        ? 'percent'
        : p['discount_unit'] as String;
    final value = (p['discount_value'] as num?)?.toInt() ?? 0;
    if (value <= 0) continue;

    // Foizda butun bo'lish — server ham shunday yaxlitlaydi.
    var lineDiscount = unit == 'amount' ? value : (price * value) ~/ 100;
    if (lineDiscount > price) lineDiscount = price;
    if (lineDiscount <= bestDiscount) continue;

    bestDiscount = lineDiscount;
    bestLabel = unit == 'amount' ? '-${formatSum(lineDiscount)}' : '-$value%';
  }

  if (bestDiscount <= 0 || bestLabel == null) return null;
  return ProductDiscount(price - bestDiscount, bestLabel);
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
  late bool _favorited = widget.initialFavorited;
  bool _busy = false;

  @override
  void didUpdateWidget(covariant FavoriteButton old) {
    super.didUpdateWidget(old);
    if (old.productId != widget.productId ||
        old.initialFavorited != widget.initialFavorited) {
      _favorited = widget.initialFavorited;
    }
  }

  Future<void> _toggle() async {
    if (_busy) return;
    final next = !_favorited;
    setState(() {
      _favorited = next;
      _busy = true;
    });
    try {
      if (next) {
        await api.addFavorite(widget.productId);
      } else {
        await api.removeFavorite(widget.productId);
      }
      widget.onChanged?.call(next);
    } catch (_) {
      if (mounted) setState(() => _favorited = !next);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
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
            _favorited ? Icons.favorite : Icons.favorite_border,
            color: _favorited ? const Color(0xFFE53935) : Colors.black,
            size: widget.size,
          ),
        ),
      ),
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
                    child: _RoundButton(icon: Icons.add, onTap: onAdd),
                  ),
                if (available && qty != null && qty! > 0)
                  Positioned(
                    left: 8,
                    right: 8,
                    bottom: 8,
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        _RoundButton(icon: Icons.remove, onTap: onRemove),
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
                        _RoundButton(icon: Icons.add, onTap: onAdd),
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
  Widget build(BuildContext context) {
    if (url.isEmpty) return _placeholder;
    return Image.network(
      fullImageUrl(url),
      fit: BoxFit.cover,
      // Rasm kelguncha JOY EGALLANADI — aks holda kartochkalar
      // yuklanish paytida sakrab qolardi.
      loadingBuilder: (context, child, progress) =>
          progress == null ? child : const SizedBox.shrink(),
      errorBuilder: (_, __, ___) => _placeholder,
    );
  }
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

/// Rasm ustidagi oq dumaloq tugma (+ / −).
class _RoundButton extends StatelessWidget {
  final IconData icon;
  final VoidCallback? onTap;
  const _RoundButton({required this.icon, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.white,
      shape: const CircleBorder(),
      elevation: 3,
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: SizedBox(
          width: 34,
          height: 34,
          child: Icon(icon, size: 19, color: Colors.black),
        ),
      ),
    );
  }
}

/// Kartochkalar to'ri — vebdagi `grid-cols-2` bilan bir xil.
///
/// `childAspectRatio` ATAYLAB yo'q: kartochka balandligi matn
/// uzunligiga qarab o'zgaradi (uzun nom ikki qatorga tushadi) va qat'iy
/// nisbat qo'yilsa qisqa nomli kartochkalarda ortiqcha bo'shliq,
/// uzunlarida esa kesilish paydo bo'lardi. `SliverGrid` o'rniga
/// `MasonryGrid` kerak bo'lmasligi uchun eng baland element bo'yicha
/// tenglashtiriladi — buni `mainAxisExtent` bilan emas, o'lchov
/// natijasida qilamiz.
const productGridDelegate = SliverGridDelegateWithMaxCrossAxisExtent(
  maxCrossAxisExtent: 240,
  crossAxisSpacing: 14,
  mainAxisSpacing: 20,
  // Kvadrat rasm + narx + ikki qatorli nom + og'irlik uchun.
  childAspectRatio: 0.62,
);
