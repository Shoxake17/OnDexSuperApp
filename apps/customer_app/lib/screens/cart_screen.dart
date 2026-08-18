import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import '../data/cart_store.dart';
import '../data/catalog_repository.dart';
import '../widgets/product_grid.dart';
import '../widgets/qty_stepper.dart';
import 'catalog_screen.dart' show kBrand;
import 'checkout_screen.dart';

/// Savat — NATIVE.
///
/// Veb bilan parity: `apps/web/app/(food)/cart/page.tsx` — yopishqoq
/// sarlavhada restoran nomi va jami, qatorlarda QATOR SUMMASI
/// (miqdor × narx), "Menyuni ochish" tugmasi, pastda "Yana nimadir
/// kerakmi?" bo'limi (to'liq menyu, turkumlarga bo'lingan).
///
/// ┌─ NARX: NIMA VIZUAL, NIMA HAQIQIY ─────────────────────────────────┐
/// Ro'yxatdagi har taom yonidagi narx — MENYUDAN olingan, ya'ni
/// taxminiy. Pastdagi JAMI esa serverdan keladi
/// (`POST /restaurants/{id}/quote`) va u buyurtma yaratilganda
/// yoziladigan summa bilan AYNAN bir xil (bir xil backend mantig'i).
///
/// Ikkisi farq qilishi MUMKIN — aksiya, yetkazish haqi yoki narx
/// o'zgargan bo'lsa. Shuning uchun tugmadagi raqam HAR DOIM serverniki:
/// mijoz bosgan summa va hisobdan yechilgan summa bir xil bo'lishi shart.
/// └───────────────────────────────────────────────────────────────────┘
class CartScreen extends StatefulWidget {
  const CartScreen({super.key});

  @override
  State<CartScreen> createState() => _CartScreenState();
}

class _CartScreenState extends State<CartScreen> {
  final _cart = CartStore.instance;

  List<Map<String, dynamic>> _menu = const [];
  List<Map<String, dynamic>> _promos = const [];
  Set<String> _favorites = <String>{};
  StreamSubscription<Cached<List<dynamic>>>? _menuSub;
  StreamSubscription<Cached<List<dynamic>>>? _promoSub;

  /// Serverdan kelgan yakuniy summa. `null` — hali olinmagan.
  int? _quoteTiyin;

  /// Hisob tafsilotlari — rasmiylashtirish ekraniga uzatiladi, u
  /// darhol to'liq hisobni chizsin va qayta so'rov kutmasin.
  int? _quoteSubtotal;
  int? _quoteDiscount;
  String? _quotePromotionName;

  bool _quoting = false;
  String? _quoteError;

  /// Eng oxirgi so'rovni belgilash uchun — tez o'zgartirishlarda
  /// eskirgan javob yangisini bosib ketmasligi kerak.
  int _quoteSeq = 0;

  @override
  void initState() {
    super.initState();
    _cart.addListener(_onCart);
    _subscribe();
    _loadFavorites();
    _refreshQuote();
  }

  @override
  void dispose() {
    _cart.removeListener(_onCart);
    _menuSub?.cancel();
    _promoSub?.cancel();
    super.dispose();
  }

  void _subscribe() {
    final rid = _cart.restaurantId;
    if (rid == null) return;
    _menuSub?.cancel();
    _promoSub?.cancel();
    _menuSub = Repos.menu(rid).observe().listen((c) {
      if (!mounted || c.value == null) return;
      setState(() => _menu = c.value!.cast<Map<String, dynamic>>());
    });
    _promoSub = Repos.promotions(rid).observe().listen((c) {
      if (!mounted || c.value == null) return;
      setState(() => _promos = c.value!.cast<Map<String, dynamic>>());
    });
  }

  Future<void> _loadFavorites() async {
    try {
      final ids = await api.favoriteIds();
      if (mounted) setState(() => _favorites = ids);
    } catch (_) {
      // Anonim foydalanuvchida 401 — bu normal holat.
    }
  }

  void _onCart() {
    if (!mounted) return;
    setState(() {});
    _refreshQuote();
  }

  /// Serverdan yakuniy summani so'raydi.
  ///
  /// Har savat o'zgarishida qayta chaqiriladi. Eskirgan javoblar
  /// `_quoteSeq` bilan filtrlanadi: mijoz tez "+" bosganda javoblar
  /// tartibsiz kelishi mumkin va eskisi yangisini bosib ketardi —
  /// natijada ekranda NOTO'G'RI summa qolardi.
  Future<void> _refreshQuote() async {
    final rid = _cart.restaurantId;
    if (rid == null || _cart.isEmpty) {
      setState(() {
        _quoteTiyin = null;
        _quoteError = null;
      });
      return;
    }

    final seq = ++_quoteSeq;
    setState(() {
      _quoting = true;
      _quoteError = null;
    });

    try {
      final res = await api.quote(rid, _itemsPayload());
      if (!mounted || seq != _quoteSeq) return;
      setState(() {
        _quoteTiyin = (res['total_tiyin'] as num?)?.toInt();
        _quoteSubtotal = (res['subtotal_tiyin'] as num?)?.toInt();
        _quoteDiscount = (res['discount_tiyin'] as num?)?.toInt();
        _quotePromotionName = res['promotion_name'] as String?;
        _quoting = false;
      });
    } catch (e) {
      if (!mounted || seq != _quoteSeq) return;
      setState(() {
        _quoting = false;
        // Summani KO'RSATMAYMIZ: noto'g'ri raqam ko'rsatgandan ko'ra
        // "hisoblab bo'lmadi" deyish to'g'ri.
        _quoteTiyin = null;
        _quoteError =
            e is ApiException ? e.message : 'Summani hisoblab bo\'lmadi';
      });
    }
  }

  List<Map<String, dynamic>> _itemsPayload() => [
        for (final e in _cart.items.entries)
          {'product_id': e.key, 'qty': e.value}
      ];

  Map<String, Map<String, dynamic>> get _byId =>
      {for (final p in _menu) (p['id'] as String? ?? ''): p};

  void _onFavoriteChanged(String productId, bool favorited) {
    setState(() {
      if (favorited) {
        _favorites.add(productId);
      } else {
        _favorites.remove(productId);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final rid = _cart.restaurantId;

    if (_cart.isEmpty || rid == null) {
      return Scaffold(
        backgroundColor: Colors.white,
        appBar: AppBar(
          backgroundColor: Colors.white,
          surfaceTintColor: Colors.transparent,
          foregroundColor: Colors.black,
          elevation: 0,
          title: const Text('Savat'),
        ),
        body: const _EmptyCart(),
      );
    }

    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        backgroundColor: Colors.white,
        surfaceTintColor: Colors.transparent,
        foregroundColor: Colors.black,
        elevation: 0,
        scrolledUnderElevation: 0.5,
        titleSpacing: 0,
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Text(
              _cart.restaurantName ?? 'Savat',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
            if (_quoteTiyin != null)
              Text(
                formatSum(_quoteTiyin!),
                style: const TextStyle(
                    fontSize: 13.5,
                    fontWeight: FontWeight.bold,
                    color: Color(0xFF16A34A)),
              ),
          ],
        ),
        actions: [
          IconButton(
            onPressed: _confirmClear,
            icon: const Icon(Icons.delete_outline),
            tooltip: 'Savatni tozalash',
          ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.only(bottom: 24),
        children: [
          if (_cart.isDineIn)
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
              child: _TableBanner(label: _cart.tableLabel),
            ),

          // ── Savatdagi taomlar ────────────────────────────────────
          for (final e in _cart.items.entries)
            _CartRow(
              product: _byId[e.key],
              productId: e.key,
              qty: e.value,
              restaurantId: rid,
              promotions: _promos,
            ),

          // ── Menyuga qaytish ──────────────────────────────────────
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 10, 16, 0),
            child: SizedBox(
              height: 48,
              child: OutlinedButton(
                onPressed: () => Navigator.of(context).pop(),
                style: OutlinedButton.styleFrom(
                  foregroundColor: const Color(0xFF171717),
                  side: const BorderSide(color: Color(0xFFE0E0E0)),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(14)),
                ),
                child: const Text('Menyuni ochish',
                    style: TextStyle(
                        fontSize: 15, fontWeight: FontWeight.w600)),
              ),
            ),
          ),

          // ── Qo'shimcha taklif ────────────────────────────────────
          //
          // Vebdagi "Yana nimadir kerakmi?" bo'limi. Nativda umuman
          // yo'q edi — mijoz savatga o'tgach menyuga qaytmasdan hech
          // narsa qo'sha olmasdi.
          if (_menu.isNotEmpty) ..._upsellSections(rid),
        ],
      ),
      bottomNavigationBar: _Bottom(
        quoteTiyin: _quoteTiyin,
        subtotalTiyin: _quoteSubtotal,
        discountTiyin: _quoteDiscount,
        promotionName: _quotePromotionName,
        quoting: _quoting,
        error: _quoteError,
        onRetry: _refreshQuote,
      ),
    );
  }

  List<Widget> _upsellSections(String restaurantId) {
    final order = <String>[];
    final byCategory = <String, List<Map<String, dynamic>>>{};
    for (final p in _menu) {
      final c = categoryOf(p);
      if (!byCategory.containsKey(c)) {
        byCategory[c] = [];
        order.add(c);
      }
      byCategory[c]!.add(p);
    }

    return [
      const Padding(
        padding: EdgeInsets.fromLTRB(16, 26, 16, 0),
        child: Text('Yana nimadir kerakmi?',
            style: TextStyle(fontSize: 19, fontWeight: FontWeight.bold)),
      ),
      for (final c in order) ...[
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
          child: Text(c,
              style:
                  const TextStyle(fontSize: 15, fontWeight: FontWeight.w600)),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: GridView.builder(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            gridDelegate: productGridOf(context),
            itemCount: byCategory[c]!.length,
            itemBuilder: (_, i) {
              final p = byCategory[c]![i];
              final id = (p['id'] as String?) ?? '';
              final discount = computeProductDiscount(p, _promos);
              return ProductCard(
                product: p,
                qty: _cart.qtyOf(id),
                discount: discount,
                promoted: PromotionIndex(_promos).covers(p, discount),
                favorited: _favorites.contains(id),
                onAdd: () => _cart.increment(
                  restaurantId: restaurantId,
                  productId: id,
                  restaurantName: _cart.restaurantName,
                ),
                onRemove: () => _cart.decrement(
                    restaurantId: restaurantId, productId: id),
                onFavoriteChanged: (fav) => _onFavoriteChanged(id, fav),
              );
            },
          ),
        ),
      ],
    ];
  }

  Future<void> _confirmClear() async {
    final yes = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Savat tozalansinmi?'),
        content: const Text(
            'Barcha tanlangan taomlar savatdan olib tashlanadi.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Yo\'q'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Ha, tozalash',
                style: TextStyle(color: Color(0xFFDC2626))),
          ),
        ],
      ),
    );
    if (yes == true) _cart.clear();
  }
}

// ═══════════════════════════════════════════════════════════════════
// SAVAT QATORI
// ═══════════════════════════════════════════════════════════════════

/// Savatdagi taom. Nom va narx MENYU keshidan olinadi — savatda faqat
/// ID va miqdor saqlanadi.
///
/// NEGA SAVATDA NARX SAQLANMAYDI: saqlansa u eskirardi va mijoz eski
/// narxni ko'rib, checkout'da boshqasini olardi. Menyu keshi esa
/// o'zi yangilanadi.
class _CartRow extends StatelessWidget {
  final Map<String, dynamic>? product;
  final String productId;
  final int qty;
  final String restaurantId;
  final List<Map<String, dynamic>> promotions;

  const _CartRow({
    required this.product,
    required this.productId,
    required this.qty,
    required this.restaurantId,
    required this.promotions,
  });

  @override
  Widget build(BuildContext context) {
    final cart = CartStore.instance;
    final p = product;

    // Menyu keshi hali kelmagan bo'lsa nom o'rniga joy egallovchi
    // ko'rsatiladi — qator YO'QOLMAYDI, aks holda mijoz savatidan
    // taom o'chib ketgandek tuyulardi.
    final name = (p?['name'] as String?) ?? 'Yuklanmoqda…';
    final price = (p?['price_tiyin'] as num?)?.toInt() ?? 0;
    final image = (p?['image_url'] as String?) ?? '';
    final weight = (p?['weight'] as num?)?.toDouble() ?? 0;
    final unit = formatWeightUnit((p?['weight_unit'] as String?) ?? '');
    final discount =
        p == null ? null : computeProductDiscount(p, promotions);

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
      child: Row(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: SizedBox(
              width: 64,
              height: 64,
              child: image.isEmpty
                  ? Container(
                      color: const Color(0xFFF5F5F5),
                      child: const Icon(Icons.restaurant_menu,
                          size: 24, color: Color(0xFFBDBDBD)),
                    )
                  : Image.network(
                      fullImageUrl(image),
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) => Container(
                        color: const Color(0xFFF5F5F5),
                        child: const Icon(Icons.restaurant_menu,
                            size: 24, color: Color(0xFFBDBDBD)),
                      ),
                    ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text.rich(
                  TextSpan(
                    text: name,
                    style: const TextStyle(fontWeight: FontWeight.w600),
                    children: [
                      if (weight > 0)
                        TextSpan(
                          text:
                              '  ${weight % 1 == 0 ? weight.toInt() : weight.toStringAsFixed(1)} $unit'
                                  .trimRight(),
                          style: const TextStyle(
                              fontWeight: FontWeight.normal,
                              color: Color(0xFF9E9E9E)),
                        ),
                    ],
                  ),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 4),
                // QATOR SUMMASI (miqdor × narx) — birlik narxi emas.
                // Vebda ham shunday: mijoz uchun "bu taom uchun
                // qancha to'layman" degan savol muhimroq.
                if (discount != null)
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Text(
                        formatSum(discount.discountedPriceTiyin * qty),
                        style: const TextStyle(
                            fontWeight: FontWeight.bold,
                            color: Color(0xFFE53935)),
                      ),
                      const SizedBox(width: 6),
                      Flexible(
                        child: Text(
                          formatSum(price * qty),
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
                  Text(formatSum(price * qty),
                      style: const TextStyle(fontWeight: FontWeight.bold)),
              ],
            ),
          ),
          const SizedBox(width: 8),
          QtyStepper(
            qty: qty,
            onAdd: () => cart.increment(
                restaurantId: restaurantId, productId: productId),
            onRemove: () => cart.decrement(
                restaurantId: restaurantId, productId: productId),
          ),
        ],
      ),
    );
  }
}

/// Stol rejimi belgisi — mijoz restoranda o'tirgani aniq ko'rinsin.
class _TableBanner extends StatelessWidget {
  final String? label;
  const _TableBanner({required this.label});

  @override
  Widget build(BuildContext context) {
    final text = (label == null || label!.isEmpty)
        ? 'Stoldan buyurtma'
        : '$label-stol · stoldan buyurtma';
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: const Color(0xFFFDEDE7),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: [
          const Icon(Icons.qr_code_2, size: 18, color: kBrand),
          const SizedBox(width: 8),
          Expanded(
            child: Text(text,
                style: const TextStyle(
                    fontSize: 13.5,
                    fontWeight: FontWeight.w600,
                    color: Color(0xFFB23A12))),
          ),
        ],
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// PASTKI PANEL
// ═══════════════════════════════════════════════════════════════════

class _Bottom extends StatelessWidget {
  final int? quoteTiyin;
  final int? subtotalTiyin;
  final int? discountTiyin;
  final String? promotionName;
  final bool quoting;
  final String? error;
  final VoidCallback onRetry;

  const _Bottom({
    required this.quoteTiyin,
    required this.subtotalTiyin,
    required this.discountTiyin,
    required this.promotionName,
    required this.quoting,
    required this.error,
    required this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    // ┌─ SUMMA YO'Q BO'LSA TUGMA O'CHIQ ────────────────────────────┐
    // Mijoz summani KO'RMASDAN buyurtma bera olmasligi kerak. Bu
    // shunchaki qulaylik emas: u nima to'lashini bilmagan holda
    // rozilik bergan bo'lardi.
    // └─────────────────────────────────────────────────────────────┘
    final ready = quoteTiyin != null && !quoting;

    return SafeArea(
      child: Container(
        decoration: const BoxDecoration(
          color: Colors.white,
          border: Border(top: BorderSide(color: Color(0xFFE5E5E5))),
        ),
        padding: const EdgeInsets.fromLTRB(16, 10, 16, 10),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (error != null)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Row(
                  children: [
                    const Icon(Icons.error_outline,
                        size: 16, color: Color(0xFFB3261E)),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(error!,
                          style: const TextStyle(
                              fontSize: 12.5, color: Color(0xFFB3261E))),
                    ),
                    TextButton(
                      onPressed: onRetry,
                      child: const Text('Qayta'),
                    ),
                  ],
                ),
              ),
            SizedBox(
              height: 52,
              width: double.infinity,
              child: FilledButton(
                style: FilledButton.styleFrom(
                  backgroundColor: kBrand,
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(14)),
                ),
                onPressed: ready
                    ? () => Navigator.of(context).push(
                          MaterialPageRoute(
                            builder: (_) => CheckoutScreen(
                              quoteTiyin: quoteTiyin!,
                              subtotalTiyin: subtotalTiyin,
                              discountTiyin: discountTiyin,
                              promotionName: promotionName,
                            ),
                          ),
                        )
                    : null,
                child: quoting
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                            strokeWidth: 2, color: Colors.white),
                      )
                    : Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          const Text('Buyurtma rasmiylashtirish',
                              style: TextStyle(
                                  fontSize: 15.5,
                                  fontWeight: FontWeight.bold)),
                          Text(
                            ready ? formatSum(quoteTiyin!) : 'Summa yo\'q',
                            style: const TextStyle(
                                fontSize: 15.5, fontWeight: FontWeight.bold),
                          ),
                        ],
                      ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _EmptyCart extends StatelessWidget {
  const _EmptyCart();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 80,
              height: 80,
              decoration: const BoxDecoration(
                shape: BoxShape.circle,
                color: Color(0xFFF5F5F5),
              ),
              child: const Icon(Icons.shopping_bag_outlined,
                  size: 34, color: Color(0xFF9E9E9E)),
            ),
            const SizedBox(height: 18),
            const Text('Savat bo\'sh',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
            const SizedBox(height: 6),
            const Text(
              'Buyurtma berish uchun avval restoran menyusidan taom tanlang.',
              textAlign: TextAlign.center,
              style: TextStyle(color: Color(0xFF757575)),
            ),
          ],
        ),
      ),
    );
  }
}
