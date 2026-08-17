import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import '../data/cart_store.dart';
import '../data/catalog_repository.dart';
import 'catalog_screen.dart' show kBrand;
import 'checkout_screen.dart';

/// Savat — NATIVE.
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

  /// Serverdan kelgan yakuniy summa. `null` — hali olinmagan.
  int? _quoteTiyin;
  bool _quoting = false;
  String? _quoteError;

  /// Eng oxirgi so'rovni belgilash uchun — tez o'zgartirishlarda
  /// eskirgan javob yangisini bosib ketmasligi kerak.
  int _quoteSeq = 0;

  @override
  void initState() {
    super.initState();
    _cart.addListener(_onCart);
    _refreshQuote();
  }

  @override
  void dispose() {
    _cart.removeListener(_onCart);
    super.dispose();
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
        _quoting = false;
      });
    } catch (e) {
      if (!mounted || seq != _quoteSeq) return;
      setState(() {
        _quoting = false;
        // Summani KO'RSATMAYMIZ: noto'g'ri raqam ko'rsatgandan ko'ra
        // "hisoblab bo'lmadi" deyish to'g'ri.
        _quoteTiyin = null;
        _quoteError = e is ApiException ? e.message : 'Summani hisoblab bo\'lmadi';
      });
    }
  }

  List<Map<String, dynamic>> _itemsPayload() => [
        for (final e in _cart.items.entries)
          {'product_id': e.key, 'qty': e.value}
      ];

  @override
  Widget build(BuildContext context) {
    final rid = _cart.restaurantId;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Savat'),
        actions: [
          if (!_cart.isEmpty)
            TextButton(
              onPressed: _confirmClear,
              child: const Text('Tozalash'),
            ),
        ],
      ),
      body: _cart.isEmpty || rid == null
          ? const _EmptyCart()
          : _CartBody(restaurantId: rid),
      bottomNavigationBar: _cart.isEmpty || rid == null
          ? null
          : _Bottom(
              quoteTiyin: _quoteTiyin,
              quoting: _quoting,
              error: _quoteError,
              onRetry: _refreshQuote,
            ),
    );
  }

  Future<void> _confirmClear() async {
    final yes = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Savatni tozalash'),
        content: const Text('Barcha taomlar olib tashlanadimi?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Yo\'q'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Ha'),
          ),
        ],
      ),
    );
    if (yes == true) _cart.clear();
  }
}

// ═══════════════════════════════════════════════════════════════════
// RO'YXAT
// ═══════════════════════════════════════════════════════════════════

/// Savatdagi taomlar. Nom va narx MENYU keshidan olinadi — savatda
/// faqat ID va miqdor saqlanadi.
///
/// NEGA SAVATDA NARX SAQLANMAYDI: saqlansa u eskirardi va mijoz eski
/// narxni ko'rib, checkout'da boshqasini olardi. Menyu keshi esa
/// o'zi yangilanadi.
class _CartBody extends StatelessWidget {
  final String restaurantId;
  const _CartBody({required this.restaurantId});

  @override
  Widget build(BuildContext context) {
    final cart = CartStore.instance;

    return StreamBuilder<Cached<List<dynamic>>>(
      stream: Repos.menu(restaurantId).observe(),
      builder: (context, snap) {
        final menu = (snap.data?.value ?? const [])
            .cast<Map<String, dynamic>>();
        final byId = {for (final p in menu) (p['id'] as String? ?? ''): p};

        final entries = cart.items.entries.toList();

        return ListView(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
          children: [
            if (cart.isDineIn) _TableBanner(label: cart.tableLabel),
            if (cart.restaurantName != null &&
                cart.restaurantName!.isNotEmpty) ...[
              Text(
                cart.restaurantName!,
                style: const TextStyle(fontSize: 17, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 12),
            ],
            for (final e in entries)
              _CartRow(
                product: byId[e.key],
                productId: e.key,
                qty: e.value,
                restaurantId: restaurantId,
              ),
          ],
        );
      },
    );
  }
}

class _CartRow extends StatelessWidget {
  final Map<String, dynamic>? product;
  final String productId;
  final int qty;
  final String restaurantId;

  const _CartRow({
    required this.product,
    required this.productId,
    required this.qty,
    required this.restaurantId,
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
    final discount = (p?['discount_price_tiyin'] as num?)?.toInt() ?? 0;
    final effective = (discount > 0 && discount < price) ? discount : price;
    final image = (p?['image_url'] as String?) ?? '';

    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Row(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(10),
            child: SizedBox(
              width: 56,
              height: 56,
              child: image.isEmpty
                  ? Container(
                      color: const Color(0xFFF5F5F5),
                      child: const Icon(Icons.restaurant_menu,
                          size: 20, color: Color(0xFFBDBDBD)),
                    )
                  : Image.network(
                      fullImageUrl(image),
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) => Container(
                        color: const Color(0xFFF5F5F5),
                        child: const Icon(Icons.restaurant_menu,
                            size: 20, color: Color(0xFFBDBDBD)),
                      ),
                    ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontWeight: FontWeight.w600)),
                if (effective > 0)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(formatSum(effective),
                        style: const TextStyle(
                            fontSize: 13, color: Color(0xFF757575))),
                  ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          _QtyBox(
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

class _QtyBox extends StatelessWidget {
  final int qty;
  final VoidCallback onAdd;
  final VoidCallback onRemove;

  const _QtyBox({
    required this.qty,
    required this.onAdd,
    required this.onRemove,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 34,
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xFFE0E0E0)),
        borderRadius: BorderRadius.circular(10),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          InkResponse(
            onTap: onRemove,
            child: const SizedBox(
              width: 34,
              height: 34,
              child: Icon(Icons.remove, size: 17),
            ),
          ),
          SizedBox(
            width: 22,
            child: Text('$qty',
                textAlign: TextAlign.center,
                style: const TextStyle(fontWeight: FontWeight.bold)),
          ),
          InkResponse(
            onTap: onAdd,
            child: const SizedBox(
              width: 34,
              height: 34,
              child: Icon(Icons.add, size: 17, color: kBrand),
            ),
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
      margin: const EdgeInsets.only(bottom: 14),
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
  final bool quoting;
  final String? error;
  final VoidCallback onRetry;

  const _Bottom({
    required this.quoteTiyin,
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
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
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
                            builder: (_) =>
                                CheckoutScreen(quoteTiyin: quoteTiyin!),
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
                    : Text(
                        ready
                            ? 'Rasmiylashtirish · ${formatSum(quoteTiyin!)}'
                            : 'Summa hisoblanmadi',
                        style: const TextStyle(
                            fontSize: 16, fontWeight: FontWeight.bold),
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
              'Menyudan taom tanlang — u shu yerda paydo bo\'ladi.',
              textAlign: TextAlign.center,
              style: TextStyle(color: Color(0xFF757575)),
            ),
          ],
        ),
      ),
    );
  }
}
