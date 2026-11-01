import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import '../data/qurt_cart.dart';
import '../widgets/promo_banners.dart';
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
        body: SafeArea(
          bottom: false,
          child: Column(
            children: [
              _topBar(),
              if (_showSearch) _searchField(),
              Expanded(
                child: Stack(
                  children: [
                    RefreshIndicator(
                      color: kBrand,
                      onRefresh: _refresh,
                      child: _content(),
                    ),
                    if (!_cart.isEmpty)
                      Positioned(
                        left: 16,
                        right: 16,
                        bottom: 12 + MediaQuery.of(context).padding.bottom,
                        child: _cartBar(),
                      ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// Pastki "Savatcha" paneli (maket: `image/qurt_menu.png`) — ishlaydi: savatga o'tadi.
  Widget _cartBar() => Material(
    color: kBrand,
    borderRadius: BorderRadius.circular(18),
    elevation: 4,
    child: InkWell(
      key: const ValueKey('qurt-cart-bar'),
      borderRadius: BorderRadius.circular(18),
      onTap: _openCart,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        child: Row(
          children: [
            const Icon(Icons.shopping_basket_outlined, color: Colors.white),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Text(
                    'Savatcha',
                    style: TextStyle(
                      color: Colors.white,
                      fontWeight: FontWeight.w800,
                      fontSize: 14.5,
                    ),
                  ),
                  Text(
                    '${_cart.totalQty} ta mahsulot · ${formatSum(_cart.totalTiyin)}',
                    style: const TextStyle(color: Colors.white70, fontSize: 12),
                  ),
                ],
              ),
            ),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              decoration: BoxDecoration(
                color: Colors.white,
                borderRadius: BorderRadius.circular(14),
              ),
              child: const Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    'Buyurtma',
                    style: TextStyle(
                      color: kBrand,
                      fontWeight: FontWeight.w700,
                      fontSize: 12.5,
                    ),
                  ),
                  SizedBox(width: 4),
                  Icon(Icons.arrow_forward, size: 15, color: kBrand),
                ],
              ),
            ),
          ],
        ),
      ),
    ),
  );

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
          Material(
            color: Colors.white,
            shape: const CircleBorder(side: BorderSide(color: _line)),
            child: InkWell(
              key: const ValueKey('qurt-search-toggle'),
              customBorder: const CircleBorder(),
              onTap:
                  () => setState(() {
                    _showSearch = !_showSearch;
                    if (!_showSearch && _query.isNotEmpty) {
                      _query = '';
                      _load();
                    }
                  }),
              child: Padding(
                padding: const EdgeInsets.all(11),
                child: Icon(
                  _showSearch ? Icons.close : Icons.search,
                  size: 22,
                  color: _ink,
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          _circleButton(
            const ValueKey('qurt-orders-open'),
            Icons.receipt_long_outlined,
            _openOrders,
            tooltip: 'Buyurtmalarim',
          ),
          const SizedBox(width: 8),
          _circleButton(
            const ValueKey('qurt-cart-open'),
            Icons.shopping_basket_outlined,
            _openCart,
            tooltip: 'Savat',
            badge: _cart.totalQty,
          ),
        ],
      ),
    );
  }

  Widget _circleButton(
    Key key,
    IconData icon,
    VoidCallback onTap, {
    required String tooltip,
    int badge = 0,
  }) {
    return Tooltip(
      message: tooltip,
      child: Stack(
        clipBehavior: Clip.none,
        children: [
          Material(
            color: Colors.white,
            shape: const CircleBorder(side: BorderSide(color: _line)),
            child: InkWell(
              key: key,
              customBorder: const CircleBorder(),
              onTap: onTap,
              child: Padding(
                padding: const EdgeInsets.all(11),
                child: Icon(icon, size: 22, color: _ink),
              ),
            ),
          ),
          if (badge > 0)
            Positioned(
              right: -2,
              top: -2,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                decoration: BoxDecoration(
                  color: kBrand,
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Text(
                  badge > 99 ? '99+' : '$badge',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 10.5,
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
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
        // Pastki "Savatcha" paneli ustiga chiqmasligi uchun joy qoldiriladi.
        padding: EdgeInsets.fromLTRB(16, 0, 16, _cart.isEmpty ? 28 : 96),
        // Kartochka balandligi kontentga mos (bo'sh joy qolmaydi): rasm (2.4:1) +
        // nom + do'kon + narx. Eni ekranga qarab o'zgargani uchun LayoutBuilder.
        child: LayoutBuilder(
          builder: (context, c) {
            final cardW = (c.maxWidth - 12) / 2;
            final extent = 8 + (cardW - 16) / _kProductImageRatio + 6 + 82 + 8;
            return GridView.builder(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 2,
                mainAxisSpacing: 12,
                crossAxisSpacing: 12,
                mainAxisExtent: extent,
              ),
              itemCount: items.length,
              itemBuilder:
                  (_, i) => _QurtCard(
                    item: items[i],
                    qty: _cart.qtyOf((items[i]['id'] ?? '').toString()),
                    onTap: () => _showDetail(items[i]),
                    onAdd: () => _add(items[i]),
                    onMinus:
                        () => _cart.setQty(
                          (items[i]['id'] ?? '').toString(),
                          _cart.qtyOf((items[i]['id'] ?? '').toString()) - 1,
                        ),
                  ),
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

class _QurtCard extends StatelessWidget {
  const _QurtCard({
    required this.item,
    required this.onTap,
    required this.qty,
    required this.onAdd,
    required this.onMinus,
  });
  final Map<String, dynamic> item;
  final VoidCallback onTap;
  final int qty;
  final VoidCallback onAdd;
  final VoidCallback onMinus;

  @override
  Widget build(BuildContext context) {
    final grams = _grams(item['weight_grams']);
    final description = (item['description'] ?? '').toString();
    final subtitle =
        description.isNotEmpty
            ? description
            : (item['store_name'] ?? '').toString();
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(18),
      child: Container(
        padding: const EdgeInsets.all(8),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(18),
          border: Border.all(color: _line),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Mahsulot rasmi — cho'zinchoq (maket: image/qurt_menu.png), 2.4 : 1.
            AspectRatio(
              aspectRatio: _kProductImageRatio,
              child: _image(item['image_url'] as String?, radius: 12),
            ),
            const SizedBox(height: 8),
            Text(
              (item['name'] ?? '').toString(),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                fontSize: 14.5,
                fontWeight: FontWeight.w800,
                color: _ink,
              ),
            ),
            const SizedBox(height: 1),
            SizedBox(
              height: 16,
              child: Text(
                subtitle,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontSize: 11.5,
                  height: 1.25,
                  color: _muted,
                ),
              ),
            ),
            const SizedBox(height: 6),
            Row(
              children: [
                Expanded(
                  child: FittedBox(
                    fit: BoxFit.scaleDown,
                    alignment: Alignment.centerLeft,
                    child: Text.rich(
                      TextSpan(
                        children: [
                          TextSpan(
                            text: formatSum(
                              ((item['price_tiyin'] as num?) ?? 0).toInt(),
                            ),
                            style: const TextStyle(
                              fontSize: 15,
                              fontWeight: FontWeight.w800,
                              color: kBrand,
                            ),
                          ),
                          if (grams.isNotEmpty)
                            TextSpan(
                              text: '  / $grams',
                              style: const TextStyle(
                                fontSize: 11,
                                fontWeight: FontWeight.w500,
                                color: _muted,
                              ),
                            ),
                        ],
                      ),
                      maxLines: 1,
                    ),
                  ),
                ),
                const SizedBox(width: 6),
                _control(),
              ],
            ),
          ],
        ),
      ),
    );
  }

  /// "+" (savatda yo'q) yoki  − N +  (savatda bor). Qoldiq tugagan bo'lsa — yozuv.
  Widget _control() {
    final id = (item['id'] ?? '').toString();
    final left = (item['stock_left'] as num?)?.toInt();
    if (left != null && left < 1) {
      return const Text(
        'Tugagan',
        style: TextStyle(
          color: _muted,
          fontSize: 11.5,
          fontWeight: FontWeight.w600,
        ),
      );
    }
    Widget btn(Key k, IconData icon, VoidCallback? tap) => InkResponse(
      key: k,
      onTap: tap,
      radius: 18,
      child: Container(
        width: 28,
        height: 28,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: tap == null ? const Color(0xFFF3F4F6) : kBrand,
        ),
        child: Icon(
          icon,
          size: 17,
          color: tap == null ? Colors.black26 : Colors.white,
        ),
      ),
    );
    if (qty < 1) return btn(ValueKey('qurt-add-$id'), Icons.add, onAdd);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        btn(ValueKey('qurt-minus-$id'), Icons.remove, onMinus),
        SizedBox(
          width: 22,
          child: Text(
            '$qty',
            textAlign: TextAlign.center,
            style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 13),
          ),
        ),
        btn(ValueKey('qurt-plus-$id'), Icons.add, onAdd),
      ],
    );
  }
}

class _QurtDetail extends StatelessWidget {
  const _QurtDetail({required this.item, required this.onAdd});
  final Map<String, dynamic> item;
  final VoidCallback onAdd;

  /// Pastki panel: narx + savatga qo'shish / miqdor tanlagich (savat o'zgarsa yangilanadi).
  Widget _addBar(BuildContext context) {
    final cart = QurtCart.instance;
    final id = (item['id'] ?? '').toString();
    final left = (item['stock_left'] as num?)?.toInt();
    final soldOut = left != null && left < 1;
    return AnimatedBuilder(
      animation: cart,
      builder: (_, _) {
        final qty = cart.qtyOf(id);
        final atMax = qty >= cart.limitOf(id) || (left != null && qty >= left);
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
                        'Savatga qo\'shish · ${formatSum(((item['price_tiyin'] as num?) ?? 0).toInt())}',
                        style: const TextStyle(
                          fontWeight: FontWeight.w700,
                          fontSize: 15,
                        ),
                      ),
                    ),
                  )
                  : Row(
                    children: [
                      IconButton.filled(
                        key: const ValueKey('qurt-detail-minus'),
                        style: IconButton.styleFrom(backgroundColor: kBrand),
                        onPressed: () => cart.setQty(id, qty - 1),
                        icon: const Icon(Icons.remove),
                      ),
                      Expanded(
                        child: Text(
                          '$qty dona · ${formatSum(qty * ((item['price_tiyin'] as num?) ?? 0).toInt())}',
                          textAlign: TextAlign.center,
                          style: const TextStyle(
                            fontWeight: FontWeight.w800,
                            fontSize: 16,
                          ),
                        ),
                      ),
                      IconButton.filled(
                        key: const ValueKey('qurt-detail-plus'),
                        style: IconButton.styleFrom(backgroundColor: kBrand),
                        onPressed: atMax ? null : onAdd,
                        icon: const Icon(Icons.add),
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
