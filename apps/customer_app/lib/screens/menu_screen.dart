import 'package:flutter/material.dart';

import '../api.dart';
import '../data/cart_store.dart';
import '../data/catalog_repository.dart';
import 'cart_screen.dart';
import 'catalog_screen.dart' show kBrand;

/// Restoran menyusi — NATIVE (ilgari Next.js sahifasi edi).
///
/// ┌─ RESTORAN MA'LUMOTI PARAMETR SIFATIDA ────────────────────────────┐
/// Katalog kartasi bosilganda restoran `Map`i shu yerga UZATILADI,
/// qaytadan so'ralmaydi. Ikki sabab:
///   * qo'shimcha tarmoq so'rovi yo'q — ekran bir zumda ochiladi;
///   * internet bo'lmasa ham sarlavha to'g'ri chiziladi (katalog
///     keshdan kelgan bo'lsa, menyu ham keshdan keladi).
/// └───────────────────────────────────────────────────────────────────┘
class MenuScreen extends StatefulWidget {
  final Map<String, dynamic> restaurant;

  const MenuScreen({super.key, required this.restaurant});

  /// Restoran menyusini FAQAT ID bo'yicha ochadi.
  ///
  /// ┌─ NEGA YORDAMCHI KERAK ──────────────────────────────────────────┐
  /// Menyuga uch joydan kelinadi: katalog kartasi, stol QR kodi va
  /// sevimlilar ro'yxati. Katalogda restoran `Map`i qo'lda bor,
  /// qolgan ikkitasida esa faqat ID.
  ///
  /// Har birida "keshdan topib, topilmasa bo'sh Map yasash" mantig'i
  /// qayta yozilsa — bu STACK ICHIDA dublikat bo'lardi. Shuning uchun
  /// u BIR joyda.
  /// └─────────────────────────────────────────────────────────────────┘
  ///
  /// Kesh ishlatiladi, tarmoq EMAS: menyu ekrani baribir o'z
  /// so'rovini yuboradi, sarlavha uchun qo'shimcha kutish shart emas.
  static Future<void> open(
    BuildContext context,
    String restaurantId, {
    String? fallbackName,
  }) async {
    Map<String, dynamic>? found;
    try {
      final cached = await Repos.restaurants().peek();
      found = cached?.cast<Map<String, dynamic>>().firstWhere(
            (r) => r['id'] == restaurantId,
            orElse: () => <String, dynamic>{},
          );
      if (found != null && found.isEmpty) found = null;
    } catch (_) {
      // Kesh o'qilmasa ham menyu ochilaveradi — sarlavhada faqat nom
      // bo'ladi, rasm va reyting bo'lmaydi.
    }
    if (!context.mounted) return;

    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => MenuScreen(
          restaurant: found ??
              {'id': restaurantId, 'name': fallbackName ?? '', 'open': true},
        ),
      ),
    );
  }

  @override
  State<MenuScreen> createState() => _MenuScreenState();
}

class _MenuScreenState extends State<MenuScreen> {
  late final _repo = Repos.menu(_id);
  final _cart = CartStore.instance;
  int _reloadTick = 0;

  String get _id => (widget.restaurant['id'] as String?) ?? '';
  String get _name => (widget.restaurant['name'] as String?) ?? '';

  @override
  void initState() {
    super.initState();
    // Savat o'zgarsa pastki panel va kartochkalardagi miqdorlar
    // yangilanadi.
    _cart.addListener(_onCart);
  }

  @override
  void dispose() {
    _cart.removeListener(_onCart);
    super.dispose();
  }

  void _onCart() {
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: RefreshIndicator(
        color: kBrand,
        onRefresh: () async => setState(() => _reloadTick++),
        child: StreamBuilder<Cached<List<dynamic>>>(
          key: ValueKey('menu-$_id-$_reloadTick'),
          stream: _repo.observe(force: _reloadTick > 0),
          builder: (context, snap) {
            final c = snap.data ?? const Cached<List<dynamic>>(refreshing: true);

            final products = (c.value ?? const [])
                .cast<Map<String, dynamic>>()
                // Mavjud bo'lmagan taom menyuda KO'RSATILMAYDI —
                // mijoz uni savatga qo'shib, checkout'da rad javob
                // olmasligi kerak.
                .where((p) => p['available'] != false)
                .toList();

            final sections = _groupByCategory(products);

            return CustomScrollView(
              physics: const AlwaysScrollableScrollPhysics(),
              slivers: [
                _CoverBar(restaurant: widget.restaurant),

                if (c.error != null)
                  const SliverToBoxAdapter(child: _OfflineStrip()),

                if (c.showSpinner)
                  const SliverFillRemaining(
                    hasScrollBody: false,
                    child: Center(child: CircularProgressIndicator()),
                  )
                else if (products.isEmpty)
                  const SliverFillRemaining(
                    hasScrollBody: false,
                    child: Center(
                      child: Padding(
                        padding: EdgeInsets.all(32),
                        child: Text('Menyu hozircha bo\'sh'),
                      ),
                    ),
                  )
                else
                  for (final s in sections) ...[
                    SliverToBoxAdapter(child: _SectionTitle(s.title)),
                    SliverPadding(
                      padding: const EdgeInsets.symmetric(horizontal: 16),
                      sliver: SliverList.separated(
                        itemCount: s.items.length,
                        separatorBuilder: (_, __) => const Divider(height: 24),
                        itemBuilder: (_, i) => _ProductRow(
                          product: s.items[i],
                          restaurantId: _id,
                          restaurantName: _name,
                        ),
                      ),
                    ),
                  ],

                // Pastki panel kontentni bosib qolmasin.
                const SliverToBoxAdapter(child: SizedBox(height: 96)),
              ],
            );
          },
        ),
      ),
      bottomNavigationBar: _CartBar(restaurantId: _id),
    );
  }

  /// Taomlarni turkum bo'yicha guruhlaydi.
  ///
  /// Tartib MENYUDAGI tartibda qoladi (alifbo bo'yicha saralanmaydi):
  /// restoran taomlarni ataylab shunday joylashtirgan bo'lishi mumkin.
  static List<_Section> _groupByCategory(List<Map<String, dynamic>> items) {
    final order = <String>[];
    final map = <String, List<Map<String, dynamic>>>{};
    for (final p in items) {
      final cat = ((p['category'] as String?) ?? '').trim();
      final key = cat.isEmpty ? 'Boshqa' : cat;
      if (!map.containsKey(key)) {
        map[key] = [];
        order.add(key);
      }
      map[key]!.add(p);
    }
    return [for (final k in order) _Section(k, map[k]!)];
  }
}

class _Section {
  final String title;
  final List<Map<String, dynamic>> items;
  const _Section(this.title, this.items);
}

// ═══════════════════════════════════════════════════════════════════
// SARLAVHA
// ═══════════════════════════════════════════════════════════════════

class _CoverBar extends StatelessWidget {
  final Map<String, dynamic> restaurant;
  const _CoverBar({required this.restaurant});

  @override
  Widget build(BuildContext context) {
    final cover = (restaurant['cover_url'] as String?) ?? '';
    final name = (restaurant['name'] as String?) ?? '';
    final rating = (restaurant['rating'] as num?)?.toDouble() ?? 0;
    final etaMin = (restaurant['eta_min_minutes'] as num?)?.toInt() ?? 0;
    final etaMax = (restaurant['eta_max_minutes'] as num?)?.toInt() ?? 0;

    return SliverAppBar(
      expandedHeight: 200,
      pinned: true,
      backgroundColor: Colors.white,
      foregroundColor: Colors.white,
      flexibleSpace: FlexibleSpaceBar(
        title: Text(
          name,
          style: const TextStyle(
            color: Colors.white,
            fontSize: 16,
            fontWeight: FontWeight.bold,
            shadows: [Shadow(color: Colors.black54, blurRadius: 6)],
          ),
        ),
        background: Stack(
          fit: StackFit.expand,
          children: [
            Container(color: const Color(0xFF3A3A3A)),
            if (cover.isNotEmpty)
              Image.network(
                fullImageUrl(cover),
                fit: BoxFit.cover,
                errorBuilder: (_, __, ___) => const SizedBox.shrink(),
              ),
            // Sarlavha matni rasm ustida o'qilishi uchun.
            const DecoratedBox(
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.bottomCenter,
                  end: Alignment.center,
                  colors: [Color(0xCC000000), Color(0x00000000)],
                ),
              ),
            ),
            if (rating > 0 || (etaMin > 0 && etaMax > 0))
              Positioned(
                left: 16,
                bottom: 52,
                child: Row(
                  children: [
                    if (etaMin > 0 && etaMax > 0)
                      _MiniChip(
                        icon: Icons.schedule,
                        text: '$etaMin–$etaMax daq',
                      ),
                    if (rating > 0) ...[
                      const SizedBox(width: 8),
                      _MiniChip(
                        icon: Icons.star,
                        iconColor: kBrand,
                        text: rating.toStringAsFixed(1),
                      ),
                    ],
                  ],
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _MiniChip extends StatelessWidget {
  final IconData icon;
  final Color? iconColor;
  final String text;
  const _MiniChip({required this.icon, this.iconColor, required this.text});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(20),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 13, color: iconColor ?? const Color(0xFF444444)),
          const SizedBox(width: 4),
          Text(text,
              style: const TextStyle(
                  fontSize: 11.5,
                  fontWeight: FontWeight.w600,
                  color: Color(0xFF262626))),
        ],
      ),
    );
  }
}

class _SectionTitle extends StatelessWidget {
  final String title;
  const _SectionTitle(this.title);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 22, 16, 6),
      child: Text(
        title,
        style: const TextStyle(fontSize: 19, fontWeight: FontWeight.bold),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// TAOM QATORI
// ═══════════════════════════════════════════════════════════════════

class _ProductRow extends StatelessWidget {
  final Map<String, dynamic> product;
  final String restaurantId;
  final String restaurantName;

  const _ProductRow({
    required this.product,
    required this.restaurantId,
    required this.restaurantName,
  });

  @override
  Widget build(BuildContext context) {
    final cart = CartStore.instance;
    final id = (product['id'] as String?) ?? '';
    final name = (product['name'] as String?) ?? '';
    final desc = ((product['description'] as String?) ?? '').trim();
    final image = (product['image_url'] as String?) ?? '';
    final price = (product['price_tiyin'] as num?)?.toInt() ?? 0;
    final discount = (product['discount_price_tiyin'] as num?)?.toInt() ?? 0;

    // ┌─ CHEGIRMA: FAQAT MAHSULOT MAYDONI ──────────────────────────┐
    // Bu yerda AKSIYA hisobi qilinmaydi (veb tomondagi
    // `computeProductDiscount` ga o'xshash mantiq YO'Q). Sabab:
    // aksiya qoidalari murakkab va ular Dart'da qayta yozilsa
    // TS versiyasidan asta-sekin uzoqlashardi.
    //
    // Yakuniy summa BARIBIR serverdan keladi
    // (`POST /restaurants/{id}/quote`, checkout ekranida). Bu yerda
    // ko'rsatilgan narx — mahsulotning o'z chegirmasi, ya'ni
    // serverning o'zi bergan maydon.
    // └──────────────────────────────────────────────────────────────┘
    final hasDiscount = discount > 0 && discount < price;
    final effective = hasDiscount ? discount : price;

    final qty = cart.qtyOf(id);

    return InkWell(
      onTap: () => _showDetail(context, product),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name,
                    style: const TextStyle(
                        fontSize: 15.5, fontWeight: FontWeight.w600)),
                if (desc.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 3),
                    child: Text(
                      desc,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                          fontSize: 13, color: Color(0xFF757575), height: 1.3),
                    ),
                  ),
                const SizedBox(height: 8),
                Row(
                  children: [
                    Text(formatSum(effective),
                        style: const TextStyle(
                            fontSize: 15, fontWeight: FontWeight.bold)),
                    if (hasDiscount) ...[
                      const SizedBox(width: 8),
                      Text(
                        formatSum(price),
                        style: const TextStyle(
                          fontSize: 13,
                          color: Color(0xFF9E9E9E),
                          decoration: TextDecoration.lineThrough,
                        ),
                      ),
                    ],
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          Column(
            children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(12),
                child: SizedBox(
                  width: 92,
                  height: 92,
                  child: image.isEmpty
                      ? Container(
                          color: const Color(0xFFF5F5F5),
                          child: const Icon(Icons.restaurant_menu,
                              color: Color(0xFFBDBDBD)),
                        )
                      : Image.network(
                          fullImageUrl(image),
                          fit: BoxFit.cover,
                          errorBuilder: (_, __, ___) => Container(
                            color: const Color(0xFFF5F5F5),
                            child: const Icon(Icons.restaurant_menu,
                                color: Color(0xFFBDBDBD)),
                          ),
                        ),
                ),
              ),
              const SizedBox(height: 6),
              _QtyControl(
                qty: qty,
                onAdd: () => cart.increment(
                  restaurantId: restaurantId,
                  productId: id,
                  restaurantName: restaurantName,
                ),
                onRemove: () => cart.decrement(
                  restaurantId: restaurantId,
                  productId: id,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  void _showDetail(BuildContext context, Map<String, dynamic> p) {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (_) => _ProductSheet(
        product: p,
        restaurantId: restaurantId,
        restaurantName: restaurantName,
      ),
    );
  }
}

/// "+" tugmasi yoki "− 2 +" boshqaruvi.
class _QtyControl extends StatelessWidget {
  final int qty;
  final VoidCallback onAdd;
  final VoidCallback onRemove;

  const _QtyControl({
    required this.qty,
    required this.onAdd,
    required this.onRemove,
  });

  @override
  Widget build(BuildContext context) {
    if (qty == 0) {
      return SizedBox(
        width: 92,
        height: 34,
        child: OutlinedButton(
          onPressed: onAdd,
          style: OutlinedButton.styleFrom(
            padding: EdgeInsets.zero,
            foregroundColor: kBrand,
            side: const BorderSide(color: Color(0xFFE0E0E0)),
            shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(10)),
          ),
          child: const Icon(Icons.add, size: 20),
        ),
      );
    }

    return Container(
      width: 92,
      height: 34,
      decoration: BoxDecoration(
        color: kBrand,
        borderRadius: BorderRadius.circular(10),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          _SmallIcon(icon: Icons.remove, onTap: onRemove),
          Text('$qty',
              style: const TextStyle(
                  color: Colors.white, fontWeight: FontWeight.bold)),
          _SmallIcon(icon: Icons.add, onTap: onAdd),
        ],
      ),
    );
  }
}

class _SmallIcon extends StatelessWidget {
  final IconData icon;
  final VoidCallback onTap;
  const _SmallIcon({required this.icon, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkResponse(
      onTap: onTap,
      radius: 20,
      child: SizedBox(
        width: 30,
        height: 34,
        child: Icon(icon, size: 18, color: Colors.white),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// TAOM TAFSILOTI
// ═══════════════════════════════════════════════════════════════════

class _ProductSheet extends StatefulWidget {
  final Map<String, dynamic> product;
  final String restaurantId;
  final String restaurantName;

  const _ProductSheet({
    required this.product,
    required this.restaurantId,
    required this.restaurantName,
  });

  @override
  State<_ProductSheet> createState() => _ProductSheetState();
}

class _ProductSheetState extends State<_ProductSheet> {
  final _cart = CartStore.instance;

  @override
  Widget build(BuildContext context) {
    final p = widget.product;
    final id = (p['id'] as String?) ?? '';
    final image = (p['image_url'] as String?) ?? '';
    final price = (p['price_tiyin'] as num?)?.toInt() ?? 0;
    final discount = (p['discount_price_tiyin'] as num?)?.toInt() ?? 0;
    final hasDiscount = discount > 0 && discount < price;
    final effective = hasDiscount ? discount : price;

    final weight = (p['weight'] as num?)?.toDouble() ?? 0;
    final unit = ((p['weight_unit'] as String?) ?? '').trim();
    final prep = ((p['prep_time_text'] as String?) ?? '').trim();
    final desc = ((p['description'] as String?) ?? '').trim();
    final qty = _cart.qtyOf(id);

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (image.isNotEmpty)
              ClipRRect(
                borderRadius: BorderRadius.circular(16),
                child: AspectRatio(
                  aspectRatio: 16 / 10,
                  child: Image.network(
                    fullImageUrl(image),
                    fit: BoxFit.cover,
                    errorBuilder: (_, __, ___) => const SizedBox.shrink(),
                  ),
                ),
              ),
            const SizedBox(height: 14),
            Text((p['name'] as String?) ?? '',
                style: const TextStyle(
                    fontSize: 20, fontWeight: FontWeight.bold)),
            if (weight > 0 || prep.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Text(
                  [
                    if (weight > 0)
                      '${weight.toStringAsFixed(weight % 1 == 0 ? 0 : 1)} $unit'
                          .trim(),
                    if (prep.isNotEmpty) prep,
                  ].join(' · '),
                  style:
                      const TextStyle(fontSize: 13, color: Color(0xFF757575)),
                ),
              ),
            if (desc.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 10),
                child: Text(desc,
                    style: const TextStyle(fontSize: 14, height: 1.45)),
              ),
            const SizedBox(height: 18),
            Row(
              children: [
                Text(formatSum(effective),
                    style: const TextStyle(
                        fontSize: 20, fontWeight: FontWeight.bold)),
                if (hasDiscount) ...[
                  const SizedBox(width: 10),
                  Text(
                    formatSum(price),
                    style: const TextStyle(
                      fontSize: 15,
                      color: Color(0xFF9E9E9E),
                      decoration: TextDecoration.lineThrough,
                    ),
                  ),
                ],
                const Spacer(),
                _QtyControl(
                  qty: qty,
                  onAdd: () => setState(() => _cart.increment(
                        restaurantId: widget.restaurantId,
                        productId: id,
                        restaurantName: widget.restaurantName,
                      )),
                  onRemove: () => setState(() => _cart.decrement(
                        restaurantId: widget.restaurantId,
                        productId: id,
                      )),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

// ═══════════════════════════════════════════════════════════════════
// PASTKI SAVAT PANELI
// ═══════════════════════════════════════════════════════════════════

class _CartBar extends StatelessWidget {
  final String restaurantId;
  const _CartBar({required this.restaurantId});

  @override
  Widget build(BuildContext context) {
    final cart = CartStore.instance;

    // Savat bo'sh yoki BOSHQA restoranniki bo'lsa panel chizilmaydi —
    // begona savat summasini bu menyuda ko'rsatish chalkash bo'lardi.
    if (cart.isEmpty || cart.restaurantId != restaurantId) {
      return const SizedBox.shrink();
    }

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
        child: SizedBox(
          height: 52,
          child: FilledButton(
            style: FilledButton.styleFrom(
              backgroundColor: kBrand,
              shape:
                  RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
            ),
            onPressed: () => Navigator.of(context).push(
              MaterialPageRoute(builder: (_) => const CartScreen()),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Text('Savatga o\'tish',
                    style: TextStyle(
                        fontSize: 16, fontWeight: FontWeight.bold)),
                const SizedBox(width: 10),
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 9, vertical: 2),
                  decoration: BoxDecoration(
                    color: Colors.white24,
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: Text('${cart.totalQty}',
                      style: const TextStyle(fontWeight: FontWeight.bold)),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _OfflineStrip extends StatelessWidget {
  const _OfflineStrip();

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.fromLTRB(16, 12, 16, 0),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF4E5),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: const Color(0xFFFFD9A8)),
      ),
      child: const Row(
        children: [
          Icon(Icons.cloud_off, size: 16, color: Color(0xFF9A5B00)),
          SizedBox(width: 8),
          Expanded(
            child: Text('Menyu yangilanmadi — saqlangan nusxa',
                style: TextStyle(fontSize: 12.5, color: Color(0xFF9A5B00))),
          ),
        ],
      ),
    );
  }
}
