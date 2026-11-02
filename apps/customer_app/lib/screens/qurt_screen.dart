import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import '../data/qurt_cart.dart';
import '../widgets/header_action.dart';
import '../widgets/product_grid.dart';
import '../widgets/promo_banners.dart';
import '../widgets/qty_stepper.dart';
import '../widgets/order_blocks.dart' show CartSummaryBar;
import '../widgets/sheet_page.dart';
import 'catalog_screen.dart' show kBrand;
import 'qurt_cart_screen.dart';
import 'qurt_orders_screen.dart';

/// Qurt sahifasi (maket: `image/qurt_menu.png`).
///
/// Tepada Do'kon bannerlari (admin panel > "Do'kon Banner"), ostida kategoriya
/// qatori, saralash va qurt mahsulotlari to'ri. Mahsulotlar qurt sotuvchilari
/// POS panelida yozgan ma'lumotdan keladi (`GET /qurt/products`); ulgurji narx
/// serverdan HECH QACHON kelmaydi.
///
/// ┌─ MAKETDAGI AYRIM BELGILAR QASDAN YO'Q ───────────────────────────────┐
/// Maketda savat, "+" tugmasi va "Mashhur/Yangi" yorliqlari bor. Ular
/// QO'SHILMADI: qurt uchun buyurtma/savat backendi hali yo'q, yorliqlar uchun
/// esa haqiqiy ma'lumot (sotuv soni, yaratilgan sana) yo'q. Bosilganda hech
/// narsa qilmaydigan "+" yoki soxta "Mashhur" — loyihada qoida bilan
/// taqiqlangan (`catalog_screen.dart` dagi izohga qarang).
/// └──────────────────────────────────────────────────────────────────────┘
class QurtScreen extends StatefulWidget {
  const QurtScreen({super.key});

  @override
  State<QurtScreen> createState() => _QurtScreenState();
}

enum _Sort { recommended, priceAsc, priceDesc, name }

const _sortTitles = {
  _Sort.recommended: 'Tavsiya etilgan',
  _Sort.priceAsc: 'Narx: arzonroq',
  _Sort.priceDesc: 'Narx: qimmatroq',
  _Sort.name: 'Nomi bo\'yicha',
};

const _ink = Color(0xFF111827);
const _muted = Color(0xFF6B7280);
const _line = Color(0xFFEDEDED);

class _QurtScreenState extends State<QurtScreen> {
  List<Map<String, dynamic>> _items = [];
  List<String> _categories = [];
  String? _category; // null = barchasi
  String _query = '';
  _Sort _sort = _Sort.recommended;
  bool _showSearch = false;
  bool _loading = true;
  String? _error;
  int _bannerTick = 0;
  Timer? _debounce;

  final _cart = QurtCart.instance;

  @override
  void initState() {
    super.initState();
    _cart.addListener(_onCart);
    _cart.ensureRestored();
    _load();
  }

  @override
  void dispose() {
    _cart.removeListener(_onCart);
    _debounce?.cancel();
    super.dispose();
  }

  void _onCart() {
    if (mounted) setState(() {});
  }

  /// Savatga 1 dona qo'shadi; qoldiq/chegara to'lgan bo'lsa tushuntiradi.
  void _add(Map<String, dynamic> product) {
    if (_cart.add(product)) return;
    final left = (product['stock_left'] as num?)?.toInt();
    final msg =
        (left != null && left < 1)
            ? 'Bu mahsulot tugagan'
            : 'Bundan ko\'p qolmagan';
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(msg)));
  }

  Future<void> _openCart() async {
    await Navigator.of(context).push(sheetRoute(const QurtCartScreen()));
    if (mounted) _load(); // qoldiq o'zgargan bo'lishi mumkin
  }

  Future<void> _openOrders() =>
      Navigator.of(context).push(sheetRoute(const QurtOrdersScreen()));

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final results = await Future.wait([
        api.qurtProducts(category: _category, query: _query),
        api.qurtCategories(),
      ]);
      if (!mounted) return;
      setState(() {
        _items =
            results[0]
                .whereType<Map>()
                .map((e) => Map<String, dynamic>.from(e))
                .toList();
        _categories = results[1].map((e) => e.toString()).toList();
        // Tanlangan kategoriya yo'qolgan bo'lsa (hamma mahsuloti tugagan) — barchasi.
        if (_category != null && !_categories.contains(_category)) {
          _category = null;
        }
      });
    } catch (e) {
      if (!mounted) return;
      setState(
        () => _error = errorText(e, 'Qurt mahsulotlarini yuklab bo\'lmadi'),
      );
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _refresh() async {
    setState(() => _bannerTick++);
    await _load();
  }

  void _onSearch(String v) {
    _query = v.trim();
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 350), _load);
  }

  void _pickCategory(String? c) {
    if (c == _category) return;
    setState(() => _category = c);
    _load();
  }

  /// Saralash mijozda: server tartibi ("Tavsiya etilgan") o'zgarmaydi.
  List<Map<String, dynamic>> get _sorted {
    final list = [..._items];
    int price(Map<String, dynamic> p) =>
        (p['price_tiyin'] as num?)?.toInt() ?? 0;
    switch (_sort) {
      case _Sort.recommended:
        break;
      case _Sort.priceAsc:
        list.sort((a, b) => price(a).compareTo(price(b)));
      case _Sort.priceDesc:
        list.sort((a, b) => price(b).compareTo(price(a)));
      case _Sort.name:
        list.sort(
          (a, b) => (a['name'] ?? '').toString().toLowerCase().compareTo(
            (b['name'] ?? '').toString().toLowerCase(),
          ),
        );
    }
    return list;
  }

  @override
  Widget build(BuildContext context) {
    return SheetPage(
      child: Scaffold(
        backgroundColor: Colors.white,
        // Pastdagi "Savatcha — N ta mahsulot  [Buyurtmani davom ettirish]" paneli
        // (maket: image/qurtplus.png; umumiy blok).
        bottomNavigationBar:
            _cart.isEmpty
                ? null
                : CartSummaryBar(
                  count: _cart.totalQty,
                  onTap: _openCart,
                ),
        body: SafeArea(
          bottom: false,
          child: Column(
            children: [
              _topBar(),
              if (_showSearch) _searchField(),
              Expanded(
                child: RefreshIndicator(
                  color: kBrand,
                  onRefresh: _refresh,
                  child: _content(),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  // ── Yuqori panel: orqaga, OnDex, qidiruv ──
  Widget _topBar() {
    return Padding(
      padding: const EdgeInsets.fromLTRB(8, 6, 16, 6),
      child: Row(
        children: [
          IconButton(
            tooltip: 'Orqaga',
            icon: const Icon(Icons.arrow_back_ios_new_rounded, size: 20),
            onPressed: () => Navigator.of(context).maybePop(),
          ),
          const SizedBox(width: 4),
          const Text.rich(
            TextSpan(
              children: [
                TextSpan(text: 'On', style: TextStyle(color: _ink)),
                TextSpan(text: 'Dex', style: TextStyle(color: kBrand)),
              ],
            ),
            style: TextStyle(fontSize: 30, fontWeight: FontWeight.w800),
          ),
          const Spacer(),
          // Restoran sahifasidagi bilan bir xil dumaloq tugmalar (umumiy blok).
          HeaderActionButton(
            key: const ValueKey('qurt-search-toggle'),
            icon: _showSearch ? Icons.close : Icons.search,
            label: 'Qidirish',
            onTap:
                () => setState(() {
                  _showSearch = !_showSearch;
                  if (!_showSearch && _query.isNotEmpty) {
                    _query = '';
                    _load();
                  }
                }),
          ),
          const SizedBox(width: 8),
          HeaderActionButton(
            key: const ValueKey('qurt-orders-open'),
            icon: Icons.receipt_long_outlined,
            label: 'Buyurtmalarim',
            onTap: _openOrders,
          ),
          const SizedBox(width: 8),
          HeaderActionButton(
            key: const ValueKey('qurt-cart-open'),
            icon: Icons.shopping_basket_outlined,
            label: 'Savat',
            badge: _cart.totalQty,
            filled: true,
            onTap: _openCart,
          ),
        ],
      ),
    );
  }

  Widget _searchField() => Padding(
    padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
    child: TextField(
      autofocus: true,
      onChanged: _onSearch,
      textInputAction: TextInputAction.search,
      decoration: InputDecoration(
        hintText: 'Qurt qidirish',
        prefixIcon: const Icon(Icons.search),
        isDense: true,
        filled: true,
        fillColor: const Color(0xFFF3F4F6),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(14),
          borderSide: BorderSide.none,
        ),
      ),
    ),
  );

  Widget _content() {
    final header = <Widget>[
      Padding(
        padding: const EdgeInsets.fromLTRB(16, 4, 16, 0),
        child: PromoBanners(
          placement: 'store',
          // Do'kon banneri FAQAT admin paneldan ("Do'kon Banner") qo'yiladi; qo'yilmagan
          // bo'lsa joy bo'sh qoladi (qattiq yozilgan rasm yo'q). Tayyor grafika (3.2:1)
          // to'liq ko'rinishi uchun nisbat shunga teng.
          aspectRatio: 3.2,
          carousel: true,
          tick: _bannerTick,
        ),
      ),
      const SizedBox(height: 10),
      if (_categories.isNotEmpty)
        SizedBox(
          height: 34,
          child: ListView(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 12),
            children: [
              _chip('Barchasi', _category == null, () => _pickCategory(null)),
              for (final c in _categories)
                _chip(c, _category == c, () => _pickCategory(c)),
            ],
          ),
        ),
      Padding(
        padding: const EdgeInsets.fromLTRB(16, 10, 16, 8),
        child: Row(
          children: [
            const Expanded(
              child: Text(
                'Qurt mahsulotlari',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.w800,
                  color: _ink,
                ),
              ),
            ),
            _sortButton(),
          ],
        ),
      ),
    ];

    Widget body;
    if (_loading && _items.isEmpty) {
      body = const Padding(
        padding: EdgeInsets.symmetric(vertical: 48),
        child: Center(child: CircularProgressIndicator()),
      );
    } else if (_error != null && _items.isEmpty) {
      body = Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          children: [
            Text(_error!, textAlign: TextAlign.center),
            const SizedBox(height: 12),
            FilledButton(onPressed: _load, child: const Text('Qayta urinish')),
          ],
        ),
      );
    } else if (_items.isEmpty) {
      body = const Padding(
        padding: EdgeInsets.symmetric(vertical: 48),
        child: Center(
          child: Text(
            'Hozircha qurt mahsulotlari yo\'q',
            style: TextStyle(color: _muted),
          ),
        ),
      );
    } else {
      final items = _sorted;
      body = Padding(
        padding: const EdgeInsets.fromLTRB(16, 0, 16, 24),
        // Restoran menyusidagi BIR XIL kartochka va to'r (ProductCard, productGridOf),
        // lekin Qurt maketi (image/qurtplus.png) ko'rinishida: ramkali karta, cho'zinchoq
        // rasm (2.4:1, kesilmaydi), nom, tavsif, narx va o'ngda "+" / "− n +".
        child: GridView.builder(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          gridDelegate: productGridOf(
            context,
            layout: ProductCardLayout.wide,
            imageAspectRatio: _kProductImageRatio,
          ),
          itemCount: items.length,
          itemBuilder: (_, i) {
            final p = items[i];
            final id = (p['id'] ?? '').toString();
            return ProductCard(
              product: _asCardProduct(p),
              qty: _cart.qtyOf(id),
              layout: ProductCardLayout.wide,
              imageAspectRatio: _kProductImageRatio,
              imageFit: BoxFit.contain,
              subtitle: _cardSubtitle(p),
              addKey: ValueKey('qurt-add-$id'),
              onAdd: () => _add(p),
              onRemove: () => _cart.setQty(id, _cart.qtyOf(id) - 1),
              onTap: () => _showDetail(p),
            );
          },
        ),
      );
    }
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: EdgeInsets.zero,
      children: [...header, body],
    );
  }

  Widget _chip(String label, bool selected, VoidCallback onTap) => Padding(
    padding: const EdgeInsets.symmetric(horizontal: 3, vertical: 2),
    child: ChoiceChip(
      label: Text(label),
      selected: selected,
      onSelected: (_) => onTap(),
      selectedColor: kBrand,
      backgroundColor: const Color(0xFFF3F4F6),
      side: BorderSide.none,
      shape: const StadiumBorder(),
      materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
      visualDensity: VisualDensity.compact,
      padding: const EdgeInsets.symmetric(horizontal: 6),
      labelStyle: TextStyle(
        color: selected ? Colors.white : _muted,
        fontWeight: FontWeight.w700,
        fontSize: 13,
      ),
      showCheckmark: false,
    ),
  );

  Widget _sortButton() => PopupMenuButton<_Sort>(
    key: const ValueKey('qurt-sort'),
    tooltip: 'Saralash',
    initialValue: _sort,
    onSelected: (s) => setState(() => _sort = s),
    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
    itemBuilder:
        (_) => [
          for (final e in _sortTitles.entries)
            PopupMenuItem(value: e.key, child: Text(e.value)),
        ],
    child: Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: _line),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.tune, size: 16, color: kBrand),
          const SizedBox(width: 5),
          Text(
            _sortTitles[_sort]!,
            style: const TextStyle(
              color: kBrand,
              fontWeight: FontWeight.w700,
              fontSize: 12,
            ),
          ),
          const Icon(Icons.keyboard_arrow_down, size: 16, color: kBrand),
        ],
      ),
    ),
  );

  void _showDetail(Map<String, dynamic> p) {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(22)),
      ),
      builder: (_) => _QurtDetail(item: p, onAdd: () => _add(p)),
    );
  }
}

/// Mahsulot rasmi nisbati (kenglik : balandlik). Server ham rasmni shu nisbatga
/// kesadi (`images.ProcessQurtProductImage`) va Merchant panel ham shunday ko'rsatadi.
const double _kProductImageRatio = 2.4;

String _grams(dynamic g) {
  final n = (g as num?)?.toInt() ?? 0;
  if (n <= 0) return '';
  return n >= 1000 && n % 100 == 0 ? '${n / 1000} kg' : '$n g';
}

Widget _image(String? url, {double? height, double radius = 14}) {
  final full = fullImageUrl(url ?? '');
  const fallback = Icon(Icons.bakery_dining, size: 40, color: Colors.black26);
  return ClipRRect(
    borderRadius: BorderRadius.circular(radius),
    child: Container(
      height: height,
      width: double.infinity,
      color: const Color(0xFFF3F4F6),
      child:
          full.isEmpty
              ? fallback
              : Image.network(
                full,
                // Kesilmaydi: rasm qanday yuklangan bo'lsa shundayligicha ko'rinadi.
                fit: BoxFit.contain,
                errorBuilder: (_, _, _) => fallback,
              ),
    ),
  );
}

/// Karta tavsifi: mahsulot tavsifi; yo'q bo'lsa do'kon nomi.
String _cardSubtitle(Map<String, dynamic> p) {
  final d = (p['description'] ?? '').toString().trim();
  return d.isNotEmpty ? d : (p['store_name'] ?? '').toString();
}

/// Qurt mahsulotini umumiy `ProductCard` kutadigan shaklga keltiradi: og'irlik
/// (`weight` + `weight_unit`) va "tugadi" holati (qoldiq 0 bo'lsa).
Map<String, dynamic> _asCardProduct(Map<String, dynamic> p) {
  final left = (p['stock_left'] as num?)?.toInt();
  return {
    ...p,
    'weight': (p['weight_grams'] as num?)?.toInt() ?? 0,
    'weight_unit': 'g',
    'available': p['available'] != false && (left == null || left > 0),
  };
}

class _QurtDetail extends StatelessWidget {
  const _QurtDetail({required this.item, required this.onAdd});
  final Map<String, dynamic> item;
  final VoidCallback onAdd;

  /// Pastki panel: narx + savatga qo'shish / miqdor tanlagich (savat o'zgarsa yangilanadi).
  /// Miqdor tanlagich — restoran savatidagi bilan BIR XIL `QtyStepper`.
  Widget _addBar(BuildContext context) {
    final cart = QurtCart.instance;
    final id = (item['id'] ?? '').toString();
    final left = (item['stock_left'] as num?)?.toInt();
    final soldOut = left != null && left < 1;
    final price = ((item['price_tiyin'] as num?) ?? 0).toInt();
    return AnimatedBuilder(
      animation: cart,
      builder: (_, _) {
        final qty = cart.qtyOf(id);
        return Container(
          padding: EdgeInsets.fromLTRB(
            20,
            10,
            20,
            12 + MediaQuery.of(context).padding.bottom,
          ),
          decoration: const BoxDecoration(
            color: Colors.white,
            border: Border(top: BorderSide(color: _line)),
          ),
          child:
              soldOut
                  ? const Center(
                    child: Text(
                      'Bu mahsulot tugagan',
                      style: TextStyle(color: _muted),
                    ),
                  )
                  : qty < 1
                  ? SizedBox(
                    width: double.infinity,
                    child: FilledButton(
                      key: const ValueKey('qurt-detail-add'),
                      onPressed: onAdd,
                      style: FilledButton.styleFrom(
                        backgroundColor: kBrand,
                        padding: const EdgeInsets.symmetric(vertical: 15),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(16),
                        ),
                      ),
                      child: Text(
                        'Savatga qo\'shish · ${formatSum(price)}',
                        style: const TextStyle(
                          fontWeight: FontWeight.w700,
                          fontSize: 15,
                        ),
                      ),
                    ),
                  )
                  : Row(
                    children: [
                      QtyStepper(
                        qty: qty,
                        size: 40,
                        onAdd: onAdd,
                        onRemove: () => cart.setQty(id, qty - 1),
                      ),
                      Expanded(
                        child: Text(
                          formatSum(qty * price),
                          textAlign: TextAlign.right,
                          style: const TextStyle(
                            fontWeight: FontWeight.w800,
                            fontSize: 18,
                            color: kBrand,
                          ),
                        ),
                      ),
                    ],
                  ),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final grams = _grams(item['weight_grams']);
    final description = (item['description'] ?? '').toString();
    final composition = (item['composition'] ?? '').toString();
    return DraggableScrollableSheet(
      expand: false,
      initialChildSize: 0.8,
      maxChildSize: 0.95,
      minChildSize: 0.5,
      builder:
          (_, controller) => Column(
            children: [
              Expanded(
                child: ListView(
                  controller: controller,
                  padding: const EdgeInsets.fromLTRB(20, 20, 20, 24),
                  children: [
                    AspectRatio(
                      aspectRatio: _kProductImageRatio,
                      child: _image(item['image_url'] as String?),
                    ),
                    const SizedBox(height: 16),
                    Text(
                      (item['name'] ?? '').toString(),
                      style: const TextStyle(
                        fontSize: 22,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      [
                        if (grams.isNotEmpty) grams,
                        (item['category'] ?? '').toString(),
                        (item['store_name'] ?? '').toString(),
                      ].where((s) => s.isNotEmpty).join(' · '),
                      style: const TextStyle(color: _muted),
                    ),
                    const SizedBox(height: 10),
                    Text(
                      formatSum(((item['price_tiyin'] as num?) ?? 0).toInt()),
                      style: const TextStyle(
                        fontSize: 24,
                        fontWeight: FontWeight.w800,
                        color: kBrand,
                      ),
                    ),
                    if (description.isNotEmpty) ...[
                      const SizedBox(height: 18),
                      const Text(
                        'Tavsif',
                        style: TextStyle(
                          fontSize: 15,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(description, style: const TextStyle(height: 1.4)),
                    ],
                    if (composition.isNotEmpty) ...[
                      const SizedBox(height: 18),
                      const Text(
                        'Tarkibi',
                        style: TextStyle(
                          fontSize: 15,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(composition, style: const TextStyle(height: 1.4)),
                    ],
                  ],
                ),
              ),
              _addBar(context),
            ],
          ),
    );
  }
}
