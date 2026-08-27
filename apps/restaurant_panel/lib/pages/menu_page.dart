import 'dart:typed_data';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';
import '../theme.dart';
import '../widgets/page_header.dart';

enum _StatusFilter { all, active, inactive }

const _pageSizeOptions = [10, 20, 50];

/// Menyu boshqaruvi — image/menyu.png (ro'yxat) va image/maxqosh.png
/// (mahsulot qo'shish sahifasi) namunalariga mos: 4 ta statistika
/// kartochkasi, bir qatorli qidiruv+filtr, ustunli jadval, o'rtalangan
/// sahifalash va to'liq alohida sahifa sifatida mahsulot qo'shish/tahrirlash.
class MenuPage extends StatefulWidget {
  const MenuPage({super.key});

  @override
  State<MenuPage> createState() => MenuPageState();
}

class MenuPageState extends State<MenuPage> {
  List<Map<String, dynamic>> _menu = [];
  List<Map<String, dynamic>> _orders = [];
  bool _loading = true;
  // Backend'dan kelguncha yoki so'rov muvaffaqiyatsiz bo'lsa ham "Mahsulot
  // qo'shish" ishlayversin uchun zaxira sifatida _fallbackCategories bilan
  // boshlanadi — server ulanishi bilan haqiqiy ro'yxat bilan almashtiriladi.
  List<String> _categories = _fallbackCategories;

  final _searchCtrl = TextEditingController();
  String _search = '';
  String? _categoryFilter;
  _StatusFilter _statusFilter = _StatusFilter.all;
  int _page = 0;
  int _pageSize = _pageSizeOptions[0];

  // Mahsulot qo'shish/tahrirlash endi modal oyna emas, alohida to'liq
  // sahifa (image/maxqosh.png namunasiga mos) — shu ikki maydon orqali
  // ro'yxat va forma orasida almashiladi.
  bool _showForm = false;
  Map<String, dynamic>? _editingProduct;

  /// Jonli yangilanish — 3D model generatsiyasi UZOQ davom etadi
  /// (bir necha daqiqa) va u fon rejimida tugaydi. Busiz restoran
  /// "tayyorlanmoqda" yozuviga qarab turib, sahifani qo'lda qayta
  /// yuklashga majbur bo'lardi.
  ///
  /// Qayta ulanish mantiqi bu yerda YO'Q — u `LiveBus` ichida
  /// (buyurtmalar sahifasidagi bilan bir xil naqsh).
  late final LiveRefresher _live;

  @override
  void initState() {
    super.initState();
    _load();
    _loadCategories();
    _live = LiveRefresher(
      bus: restaurantLive,
      onRefresh: _load,
      types: const {'model3d_updated', 'menu_updated'},
      offlineInterval: const Duration(seconds: 30),
    )..start();
    _searchCtrl.addListener(() {
      setState(() {
        _search = _searchCtrl.text.trim().toLowerCase();
        _page = 0;
      });
    });
  }

  @override
  void dispose() {
    _live.dispose();
    _searchCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      // Buyurtmalar — "Sotilgan" ustuni HAQIQIY buyurtma tarixidan
      // hisoblanadi (dashboarddagi "Eng ko'p sotilgan mahsulotlar" bilan
      // bir xil manba/usul — GET /restaurants/{id}/orders oxirgi 100 ta
      // buyurtmani qaytaradi, shuning uchun bu son ATAYLAB "so'nggi ~100
      // buyurtma bo'yicha" chegaralangan, lekin 100% haqiqiy — soxta emas).
      final results = await Future.wait([api.menu(), api.orders()]);
      if (!mounted) return;
      setState(() {
        _menu = results[0].cast<Map<String, dynamic>>();
        _orders = results[1].cast<Map<String, dynamic>>();
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _loading = false);
      _snack('Xato: $e');
    }
  }

  Future<void> _loadCategories() async {
    try {
      final l = await api.categories();
      if (!mounted || l.isEmpty) return;
      setState(() => _categories = l.cast<String>());
    } catch (_) {
      // Jim ketadi — _fallbackCategories bilan ishlayveradi, "Mahsulot
      // qo'shish" hech qachon server bilan bog'liq bu xato tufayli
      // buzilib qolmasligi kerak.
    }
  }

  void _snack(String msg) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }

  Future<void> _toggleAvailable(Map<String, dynamic> p) async {
    final next = !(p['available'] == true);
    try {
      await api.saveProduct(
        id: p['id'],
        name: p['name'],
        priceTiyin: (p['price_tiyin'] ?? 0) as int,
        available: next,
        category: p['category'] ?? '',
        wholesalePriceTiyin: (p['wholesale_price_tiyin'] ?? 0) as int,
        stock: (p['stock'] ?? 0) as int,
        weight: ((p['weight'] ?? 0) as num).toDouble(),
        weightUnit: (p['weight_unit'] as String?) ?? 'g',
        description: p['description'] ?? '',
        prepTimeText: (p['prep_time_text'] as String?) ?? '',
        imageUrl: p['image_url'] ?? '',
      );
      _load();
    } catch (e) {
      _snack('Xato: $e');
    }
  }

  // ---------- 3D model ----------

  /// Generatsiyani boshlaydi.
  ///
  /// Javob TEZ qaytadi — model fon rejimida tayyorlanadi. Ro'yxat
  /// darhol yangilanadi (holat "tayyorlanmoqda" bo'ladi), tayyor
  /// bo'lganda esa `LiveRefresher` uni o'zi qayta yuklaydi.
  Future<void> _generate3D(Map<String, dynamic> p) async {
    try {
      await api.generateModel3D(p['id'] as String);
      _snack('3D model tayyorlanmoqda — bir necha daqiqa vaqt oladi');
      _load();
    } on ApiException catch (e) {
      _snack(e.message);
    } catch (e) {
      _snack('Xato: $e');
    }
  }

  Future<void> _remove3D(Map<String, dynamic> p) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('3D modelni o\'chirish'),
        content: Text('"${p['name']}" uchun 3D model mijoz ilovasida '
            'ko\'rsatilmaydigan bo\'ladi. Keyin qayta yaratsangiz bo\'ladi.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Bekor qilish')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: TextButton.styleFrom(foregroundColor: OnDexColors.danger),
            child: const Text('O\'chirish'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await api.removeModel3D(p['id'] as String);
      _snack('3D model o\'chirildi');
      _load();
    } catch (e) {
      _snack('Xato: $e');
    }
  }

  /// Shell'dagi tashqi tugma (agar chaqirilsa) shu metodni chaqiradi.
  void openAddDialog() => _openForm();

  void _openForm({Map<String, dynamic>? existing}) {
    setState(() {
      _editingProduct = existing;
      _showForm = true;
    });
  }

  void _closeForm() {
    setState(() {
      _showForm = false;
      _editingProduct = null;
    });
  }

  void _onFormSaved({required bool wasEdit, required bool continueAdding}) {
    _snack(wasEdit ? 'Saqlandi' : 'Mahsulot qo\'shildi');
    _load();
    if (!continueAdding) _closeForm();
  }

  Future<void> _confirmDelete(Map<String, dynamic> p) async {
    final yes = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        icon: const Icon(Icons.warning_amber_rounded, color: Colors.red, size: 40),
        title: Text('"${p['name']}" o\'chirilsinmi?'),
        content: const Text(
            'Taom menyudan butunlay o\'chib ketadi. Bu amalni ortga qaytarib bo\'lmaydi.\n\n'
            'Eslatma: bu taom bilan avval berilgan buyurtmalar tarixiga ta\'sir qilmaydi.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Yo\'q, bekor qilish')),
          FilledButton(
              style: FilledButton.styleFrom(backgroundColor: Colors.red),
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Ha, o\'chirilsin')),
        ],
      ),
    );
    if (yes != true) return;
    try {
      await api.deleteProduct(p['id']);
      _snack('"${p['name']}" o\'chirildi');
      _load();
    } catch (e) {
      _snack('Xato: $e');
    }
  }

  void _showPreview(Map<String, dynamic> p) {
    showDialog(
      context: context,
      builder: (_) => _ProductPreviewDialog(product: p, soldQty: _soldQtyFor(p['id'] as String? ?? '')),
    );
  }

  // ---------------------------------------------------------------------
  // Hisob-kitob: sotilgan son (haqiqiy buyurtma tarixidan)
  // ---------------------------------------------------------------------

  Map<String, int> get _soldQtyByProduct {
    final map = <String, int>{};
    for (final o in _orders) {
      final status = o['status'] as String? ?? '';
      if (status == 'rejected' || status == 'cancelled') continue;
      final items = (o['items'] as List?) ?? [];
      for (final raw in items) {
        if (raw is! Map) continue;
        final id = raw['product_id'] as String? ?? '';
        if (id.isEmpty) continue;
        map[id] = (map[id] ?? 0) + ((raw['qty'] ?? 0) as int);
      }
    }
    return map;
  }

  int _soldQtyFor(String productId) => _soldQtyByProduct[productId] ?? 0;

  List<_CategoryAgg> get _categoryAggs {
    final counts = <String, int>{};
    for (final p in _menu) {
      final c = (p['category'] as String?)?.trim() ?? '';
      if (c.isEmpty) continue;
      counts[c] = (counts[c] ?? 0) + 1;
    }
    final list = counts.entries.map((e) => _CategoryAgg(e.key, e.value)).toList()
      ..sort((a, b) => b.count.compareTo(a.count));
    return list;
  }

  List<Map<String, dynamic>> get _filtered {
    var list = _menu;
    if (_search.isNotEmpty) {
      list = list.where((p) => (p['name'] as String? ?? '').toLowerCase().contains(_search)).toList();
    }
    if (_categoryFilter != null) {
      list = list.where((p) => (p['category'] as String? ?? '') == _categoryFilter).toList();
    }
    if (_statusFilter == _StatusFilter.active) {
      list = list.where((p) => p['available'] == true).toList();
    } else if (_statusFilter == _StatusFilter.inactive) {
      list = list.where((p) => p['available'] != true).toList();
    }
    return list;
  }

  void _clearFilters() {
    setState(() {
      _searchCtrl.clear();
      _search = '';
      _categoryFilter = null;
      _statusFilter = _StatusFilter.all;
      _page = 0;
    });
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_showForm) {
      // Shu restoran ilgari o'zi qo'shgan turkumlar (boshqa restoranlarga
      // ko'rinmaydi — chunki har biri faqat o'z menyusidan hisoblanadi).
      final ownCategories = _menu
          .map((p) => (p['category'] as String?)?.trim() ?? '')
          .where((c) => c.isNotEmpty)
          .toSet()
          .toList()
        ..sort();
      return SingleChildScrollView(
        padding: kPagePadding,
        child: _ProductFormPage(
          key: ValueKey(_editingProduct?['id'] ?? '__new__'),
          existing: _editingProduct,
          ownCategories: ownCategories,
          predefinedCategories: _categories,
          onCancel: _closeForm,
          onSaved: (continueAdding) => _onFormSaved(
              wasEdit: _editingProduct != null, continueAdding: continueAdding),
        ),
      );
    }

    final total = _menu.length;
    final active = _menu.where((p) => p['available'] == true).length;
    final inactive = total - active;
    final categoryAggs = _categoryAggs;
    final filtered = _filtered;
    final totalPages = filtered.isEmpty ? 1 : ((filtered.length - 1) ~/ _pageSize) + 1;
    final page = _page.clamp(0, totalPages - 1);
    final pageItems = filtered.skip(page * _pageSize).take(_pageSize).toList();
    final soldMap = _soldQtyByProduct;

    return RefreshIndicator(
      onRefresh: _load,
      child: SingleChildScrollView(
        padding: kPagePadding,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _Header(onAddProduct: () => _openForm()),
            const SizedBox(height: 20),
            Row(
              children: [
                Expanded(
                  child: _StatCard(
                    label: 'Jami mahsulotlar',
                    value: '$total',
                    icon: Icons.restaurant_menu_rounded,
                    color: OnDexColors.info,
                    bg: OnDexColors.infoBg,
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: _StatCard(
                    label: 'Faol mahsulotlar',
                    value: '$active',
                    icon: Icons.check_circle_rounded,
                    color: OnDexColors.success,
                    bg: OnDexColors.successBg,
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: _StatCard(
                    label: 'Nofaol mahsulotlar',
                    value: '$inactive',
                    icon: Icons.cancel_rounded,
                    color: OnDexColors.danger,
                    bg: OnDexColors.dangerBg,
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: _StatCard(
                    label: 'Kategoriyalar',
                    value: '${categoryAggs.length}',
                    icon: Icons.category_rounded,
                    color: OnDexColors.purple,
                    bg: OnDexColors.purpleBg,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 20),
            _FilterRow(
              searchCtrl: _searchCtrl,
              categories: categoryAggs.map((c) => c.name).toList(),
              categoryFilter: _categoryFilter,
              onCategoryChanged: (c) => setState(() {
                _categoryFilter = c;
                _page = 0;
              }),
              statusFilter: _statusFilter,
              onStatusChanged: (s) => setState(() {
                _statusFilter = s;
                _page = 0;
              }),
              onClear: _clearFilters,
            ),
            const SizedBox(height: 20),
            if (_menu.isEmpty)
              const _EmptyState(text: 'Menyu bo\'sh — yuqoridagi "Mahsulot qo\'shish" tugmasini bosing')
            else if (filtered.isEmpty)
              const _EmptyState(text: 'Filtrga mos mahsulot topilmadi')
            else
              _ProductTable(
                products: pageItems,
                soldMap: soldMap,
                onEdit: (p) => _openForm(existing: p),
                onPreview: _showPreview,
                onToggleAvailable: _toggleAvailable,
                onDelete: _confirmDelete,
                onModel3D: (p, remove) =>
                    remove ? _remove3D(p) : _generate3D(p),
              ),
            if (filtered.isNotEmpty) ...[
              const SizedBox(height: 16),
              _PaginationRow(
                totalCount: filtered.length,
                page: page,
                totalPages: totalPages,
                pageSize: _pageSize,
                onPageChanged: (p) => setState(() => _page = p),
                onPageSizeChanged: (s) => setState(() {
                  _pageSize = s;
                  _page = 0;
                }),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _CategoryAgg {
  final String name;
  final int count;
  _CategoryAgg(this.name, this.count);
}

// ---------------------------------------------------------------------------
// Sarlavha
// ---------------------------------------------------------------------------

/// Sarlavha bloki — dizayn `widgets/page_header.dart` da, BARCHA
/// bo'limlar bilan bitta manbadan (avval bu yerda va "Aksiyalar"da
/// deyarli bir xil kod ikki nusxada yozilgan edi).
class _Header extends StatelessWidget {
  final VoidCallback onAddProduct;
  const _Header({required this.onAddProduct});

  @override
  Widget build(BuildContext context) => PageHeader(
        title: 'Menyu',
        subtitle: 'Menyudagi mahsulotlarni boshqarish',
        narrowBelow: 700,
        action: PageActionButton(
          icon: Icons.add_rounded,
          label: 'Mahsulot qo\'shish',
          onPressed: onAddProduct,
        ),
      );
}

// ---------------------------------------------------------------------------
// Statistika kartochkalari
// ---------------------------------------------------------------------------

class _StatCard extends StatelessWidget {
  final String label;
  final String value;
  final IconData icon;
  final Color color;
  final Color bg;
  const _StatCard(
      {required this.label,
      required this.value,
      required this.icon,
      required this.color,
      required this.bg});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(18),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim, fontWeight: FontWeight.w600)),
              const SizedBox(height: 8),
              Text(value,
                  style: const TextStyle(fontSize: 24, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
            ],
          ),
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(color: bg, shape: BoxShape.circle),
            child: Icon(icon, size: 19, color: color),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Qidiruv + kategoriya/holat filtri — HAR DOIM bitta qatorda (foydalanuvchi
// so'rovi bo'yicha, avvalgi Wrap ba'zan ikkinchi qatorga tushib qolardi).
// ---------------------------------------------------------------------------

class _FilterRow extends StatelessWidget {
  final TextEditingController searchCtrl;
  final List<String> categories;
  final String? categoryFilter;
  final ValueChanged<String?> onCategoryChanged;
  final _StatusFilter statusFilter;
  final ValueChanged<_StatusFilter> onStatusChanged;
  final VoidCallback onClear;

  const _FilterRow({
    required this.searchCtrl,
    required this.categories,
    required this.categoryFilter,
    required this.onCategoryChanged,
    required this.statusFilter,
    required this.onStatusChanged,
    required this.onClear,
  });

  InputDecoration _dec(String hint, {IconData? icon}) => InputDecoration(
        isDense: true,
        hintText: hint,
        hintStyle: const TextStyle(fontSize: 13, color: OnDexColors.inkFaint),
        prefixIcon: icon == null ? null : Icon(icon, size: 19, color: OnDexColors.inkFaint),
        filled: true,
        fillColor: OnDexColors.cardBg,
        contentPadding: const EdgeInsets.symmetric(vertical: 12, horizontal: 14),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: OnDexColors.cardBorder),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: OnDexColors.primary),
        ),
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(10)),
      );

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: TextField(
            controller: searchCtrl,
            decoration: _dec('Mahsulot qidirish...', icon: Icons.search_rounded),
          ),
        ),
        const SizedBox(width: 12),
        SizedBox(
          width: 190,
          child: DropdownButtonFormField<String?>(
            initialValue: categoryFilter,
            isExpanded: true,
            dropdownColor: OnDexColors.cardBg,
            decoration: _dec('Barcha kategoriyalar'),
            hint: const Text('Barcha kategoriyalar',
                style: TextStyle(fontSize: 13, color: OnDexColors.inkFaint)),
            items: [
              const DropdownMenuItem<String?>(value: null, child: Text('Barcha kategoriyalar')),
              for (final c in categories) DropdownMenuItem<String?>(value: c, child: Text(c)),
            ],
            onChanged: onCategoryChanged,
          ),
        ),
        const SizedBox(width: 12),
        SizedBox(
          width: 170,
          child: DropdownButtonFormField<_StatusFilter>(
            initialValue: statusFilter,
            isExpanded: true,
            dropdownColor: OnDexColors.cardBg,
            decoration: _dec('Barcha holatlar'),
            items: const [
              DropdownMenuItem(value: _StatusFilter.all, child: Text('Barcha holatlar')),
              DropdownMenuItem(value: _StatusFilter.active, child: Text('Faol')),
              DropdownMenuItem(value: _StatusFilter.inactive, child: Text('Nofaol')),
            ],
            onChanged: (v) => onStatusChanged(v ?? _StatusFilter.all),
          ),
        ),
        const SizedBox(width: 12),
        OutlinedButton.icon(
          onPressed: onClear,
          style: OutlinedButton.styleFrom(
            foregroundColor: OnDexColors.inkDim,
            side: const BorderSide(color: OnDexColors.cardBorder),
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 13),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
          ),
          icon: const Icon(Icons.refresh_rounded, size: 16),
          label: const Text('Filtrlarni tozalash'),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Jadval
// ---------------------------------------------------------------------------

const _colCategoryWidth = 150.0;
const _colPriceWidth = 110.0;
const _colStatusWidth = 90.0;
const _colSoldWidth = 80.0;
const _colActionsWidth = 120.0;

class _ProductTable extends StatelessWidget {
  final List<Map<String, dynamic>> products;
  final Map<String, int> soldMap;
  final void Function(Map<String, dynamic>) onEdit;
  final void Function(Map<String, dynamic>) onPreview;
  final void Function(Map<String, dynamic>) onToggleAvailable;
  final void Function(Map<String, dynamic>) onDelete;

  /// 3D model amali. `remove: true` — modelni uzish, aks holda
  /// generatsiyani boshlash. Ikkita alohida callback o'rniga bitta:
  /// ular doim juft yuradi va parametrlar ro'yxatini shishirmasin.
  final void Function(Map<String, dynamic> product, bool remove) onModel3D;

  const _ProductTable({
    required this.products,
    required this.soldMap,
    required this.onEdit,
    required this.onPreview,
    required this.onToggleAvailable,
    required this.onDelete,
    required this.onModel3D,
  });

  @override
  Widget build(BuildContext context) {
    // MUHIM: bu Column avval gorizontal SingleChildScrollView +
    // ConstrainedBox(minWidth) ichida edi, crossAxisAlignment.stretch bilan
    // birga — bu "BoxConstraints forces an infinite width" xatosi bilan
    // butun sahifani qulatgan edi (Row ichidagi Expanded'li "Mahsulot"
    // ustuni cheksiz enga cho'zilishga urinardi). Buyurtmalar sahifasidagi
    // _ActiveOrdersList bilan bir xil, sodda naqsh qo'llanildi: gorizontal
    // scroll YO'Q, jadval konteyner enini to'liq egallaydi.
    return Container(
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        children: [
          const _TableHeaderRow(),
          const Divider(height: 1, color: OnDexColors.cardBorder),
          for (var i = 0; i < products.length; i++) ...[
            if (i > 0) const Divider(height: 1, color: OnDexColors.cardBorder),
            _ProductRow(
              product: products[i],
              soldQty: soldMap[products[i]['id']] ?? 0,
              onEdit: () => onEdit(products[i]),
              onPreview: () => onPreview(products[i]),
              onToggleAvailable: () => onToggleAvailable(products[i]),
              onDelete: () => onDelete(products[i]),
              onModel3D: (remove) => onModel3D(products[i], remove),
            ),
          ],
        ],
      ),
    );
  }
}

class _TableHeaderRow extends StatelessWidget {
  const _TableHeaderRow();

  @override
  Widget build(BuildContext context) {
    const style =
        TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: OnDexColors.inkDim);
    return const Padding(
      padding: EdgeInsets.symmetric(horizontal: 18, vertical: 14),
      child: Row(
        children: [
          Expanded(flex: 3, child: Text('Mahsulot', style: style)),
          SizedBox(width: _colCategoryWidth, child: Text('Kategoriya', style: style)),
          SizedBox(width: _colPriceWidth, child: Text('Narxi', style: style)),
          SizedBox(width: _colStatusWidth, child: Text('Holati', style: style)),
          SizedBox(
              width: _colSoldWidth,
              child: Text('Sotilgan', style: style, textAlign: TextAlign.center)),
          SizedBox(
              width: _colActionsWidth,
              child: Text('Amallar', style: style, textAlign: TextAlign.right)),
        ],
      ),
    );
  }
}

class _ProductRow extends StatelessWidget {
  final Map<String, dynamic> product;
  final int soldQty;
  final VoidCallback onEdit;
  final VoidCallback onPreview;
  final VoidCallback onToggleAvailable;
  final VoidCallback onDelete;
  final void Function(bool remove) onModel3D;

  const _ProductRow({
    required this.product,
    required this.soldQty,
    required this.onEdit,
    required this.onPreview,
    required this.onToggleAvailable,
    required this.onDelete,
    required this.onModel3D,
  });

  @override
  Widget build(BuildContext context) {
    final imgUrl = product['image_url'] as String? ?? '';
    final name = product['name'] as String? ?? '';
    final desc = (product['description'] as String? ?? '').trim();
    final category = (product['category'] as String? ?? '').trim();
    final available = product['available'] == true;
    final (catColor, catBg) = _categoryStyle(category);
    // 3D model holati — backend `model_3d_status` va `model_3d_url`
    // beradi (panel `GET /restaurants/{id}/products` dan TO'LIQ
    // obyektni oladi; mijozga faqat havola ko'rinadi).
    final model3DStatus = (product['model_3d_status'] as String?) ?? '';
    final hasModel3D =
        ((product['model_3d_url'] as String?) ?? '').isNotEmpty;

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 10),
      child: Row(
        children: [
          Expanded(
            flex: 3,
            child: Row(
              children: [
                ClipRRect(
                  borderRadius: BorderRadius.circular(10),
                  child: SizedBox(
                    width: 44,
                    height: 44,
                    child: imgUrl.isEmpty
                        ? Container(
                            color: OnDexColors.pageBg,
                            child: const Icon(Icons.restaurant_rounded, size: 18, color: OnDexColors.inkFaint),
                          )
                        : Image.network(
                            fullImageUrl(imgUrl),
                            fit: BoxFit.cover,
                            errorBuilder: (_, __, ___) => Container(
                              color: OnDexColors.pageBg,
                              child: const Icon(Icons.restaurant_rounded, size: 18, color: OnDexColors.inkFaint),
                            ),
                          ),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Row(
                        children: [
                          Flexible(
                            child: Text(name,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                          ),
                          if (model3DStatus.isNotEmpty || hasModel3D) ...[
                            const SizedBox(width: 6),
                            _Model3DBadge(status: model3DStatus, ready: hasModel3D),
                          ],
                        ],
                      ),
                      if (desc.isNotEmpty)
                        Text(desc,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(fontSize: 12, color: OnDexColors.inkFaint)),
                    ],
                  ),
                ),
              ],
            ),
          ),
          SizedBox(
            width: _colCategoryWidth,
            child: Align(
              alignment: Alignment.centerLeft,
              child: category.isEmpty
                  ? const Text('—', style: TextStyle(fontSize: 12.5, color: OnDexColors.inkFaint))
                  : Container(
                      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                      decoration: BoxDecoration(color: catBg, borderRadius: BorderRadius.circular(999)),
                      child: Text(category,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(fontSize: 11.5, fontWeight: FontWeight.w700, color: catColor)),
                    ),
            ),
          ),
          SizedBox(
            width: _colPriceWidth,
            child: Text(formatSum((product['price_tiyin'] ?? 0) as int),
                style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: OnDexColors.ink)),
          ),
          SizedBox(
            width: _colStatusWidth,
            child: Align(
              alignment: Alignment.centerLeft,
              child: InkWell(
                onTap: onToggleAvailable,
                borderRadius: BorderRadius.circular(999),
                child: Tooltip(
                  message: available ? 'Bosing — nofaol qilish' : 'Bosing — faol qilish',
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: available ? OnDexColors.successBg : OnDexColors.dangerBg,
                      borderRadius: BorderRadius.circular(999),
                    ),
                    child: Text(available ? 'Faol' : 'Nofaol',
                        style: TextStyle(
                            fontSize: 11.5,
                            fontWeight: FontWeight.w700,
                            color: available ? OnDexColors.success : OnDexColors.danger)),
                  ),
                ),
              ),
            ),
          ),
          SizedBox(
            width: _colSoldWidth,
            child: Text('$soldQty',
                textAlign: TextAlign.center,
                style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
          ),
          SizedBox(
            width: _colActionsWidth,
            child: Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                _RowIconButton(icon: Icons.edit_rounded, tooltip: 'Tahrirlash', onTap: onEdit, accent: true),
                _RowIconButton(icon: Icons.visibility_outlined, tooltip: 'Ko\'rish', onTap: onPreview),
                PopupMenuButton<String>(
                  tooltip: 'Ko\'proq',
                  icon: const Icon(Icons.more_vert_rounded, size: 18, color: OnDexColors.inkDim),
                  onSelected: (v) {
                    if (v == 'toggle') onToggleAvailable();
                    if (v == 'delete') onDelete();
                    if (v == 'gen3d') onModel3D(false);
                    if (v == 'rm3d') onModel3D(true);
                  },
                  itemBuilder: (context) => [
                    PopupMenuItem(
                      value: 'toggle',
                      child: Text(available ? 'Nofaol qilish' : 'Faol qilish'),
                    ),
                    // 3D amali holatga qarab: tayyor bo'lsa "qayta
                    // yaratish" + "o'chirish", tayyorlanayotgan bo'lsa
                    // umuman ko'rsatilmaydi (ikkinchi marta bosish
                    // faqat kredit sarflardi).
                    if (model3DStatus != 'pending') ...[
                      PopupMenuItem(
                        value: 'gen3d',
                        enabled: imgUrl.isNotEmpty,
                        child: Text(
                          hasModel3D ? '3D modelni qayta yaratish' : '3D model yaratish',
                          style: TextStyle(
                            color: imgUrl.isEmpty ? OnDexColors.inkFaint : null,
                          ),
                        ),
                      ),
                      if (hasModel3D)
                        const PopupMenuItem(
                          value: 'rm3d',
                          child: Text('3D modelni o\'chirish'),
                        ),
                    ],
                    const PopupMenuItem(
                      value: 'delete',
                      child: Text('O\'chirish', style: TextStyle(color: OnDexColors.danger)),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// "3D model yaratamizmi?" taklif oynasi.
///
/// Rasm yuklangandan keyin chiqadi. Ikkala tanlov ham to'liq
/// ishlaydigan natija beradi — "Yo'q" hech narsani buzmaydi, mahsulot
/// oddiy rasm bilan menyuda qoladi.
class _Model3DOfferDialog extends StatelessWidget {
  final String productName;
  const _Model3DOfferDialog({required this.productName});

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      shape:
          RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
      title: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: OnDexColors.purpleBg,
              borderRadius: BorderRadius.circular(10),
            ),
            child: const Icon(Icons.view_in_ar_rounded,
                color: OnDexColors.purple, size: 20),
          ),
          const SizedBox(width: 12),
          const Expanded(child: Text('3D model yaratilsinmi?')),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text.rich(
            TextSpan(children: [
              const TextSpan(text: 'Rasm asosida '),
              TextSpan(
                text: '«$productName»',
                style: const TextStyle(fontWeight: FontWeight.w700),
              ),
              const TextSpan(text: ' ning haqiqiy 3D modelini yaratamizmi?'),
            ]),
            style: const TextStyle(fontSize: 14, height: 1.45),
          ),
          const SizedBox(height: 12),
          const _OfferPoint(
            icon: Icons.photo_camera_rounded,
            text: 'Mijoz stolda buyurtma bergach, taomni telefon kamerasi '
                'orqali STOL USTIDA ko\'radi',
          ),
          const _OfferPoint(
            icon: Icons.schedule_rounded,
            text: 'Tayyorlash bir necha daqiqa vaqt oladi — menyu shu '
                'vaqtda ham normal ishlayveradi',
          ),
          const _OfferPoint(
            icon: Icons.info_outline_rounded,
            text: '"Yo\'q" desangiz taom oddiy rasm bilan saqlanadi. '
                'Keyinroq "Ko\'proq" menyusidan yaratsangiz ham bo\'ladi',
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context, false),
          style: TextButton.styleFrom(foregroundColor: OnDexColors.inkDim),
          child: const Text('Yo\'q, oddiy rasm'),
        ),
        FilledButton.icon(
          onPressed: () => Navigator.pop(context, true),
          icon: const Icon(Icons.auto_awesome_rounded, size: 18),
          label: const Text('Ha, yaratilsin'),
        ),
      ],
    );
  }
}

class _OfferPoint extends StatelessWidget {
  final IconData icon;
  final String text;
  const _OfferPoint({required this.icon, required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 15, color: OnDexColors.inkFaint),
          const SizedBox(width: 8),
          Expanded(
            child: Text(text,
                style: const TextStyle(
                    fontSize: 12.5, color: OnDexColors.inkDim, height: 1.4)),
          ),
        ],
      ),
    );
  }
}

/// 3D model holati belgisi — mahsulot nomi yonida.
///
/// Restoran menyuga qarab qaysi taomda 3D bor, qaysinisi hali
/// tayyorlanmoqda, qaysinisi yiqilganini BIR QARASHDA ko'rishi kerak:
/// generatsiya pul sarflaydi va uzoq davom etadi, ya'ni bu holat
/// menyudagi muhim ma'lumot.
class _Model3DBadge extends StatelessWidget {
  final String status;
  final bool ready;

  const _Model3DBadge({required this.status, required this.ready});

  @override
  Widget build(BuildContext context) {
    final (label, color, bg, icon) = switch (status) {
      'pending' => (
          'Tayyorlanmoqda',
          OnDexColors.warning,
          OnDexColors.warningBg,
          Icons.hourglass_top_rounded,
        ),
      'failed' => (
          '3D xato',
          OnDexColors.danger,
          OnDexColors.dangerBg,
          Icons.error_outline_rounded,
        ),
      _ when ready => (
          '3D',
          OnDexColors.purple,
          OnDexColors.purpleBg,
          Icons.view_in_ar_rounded,
        ),
      _ => ('', OnDexColors.inkFaint, OnDexColors.pageBg, Icons.help_outline),
    };
    if (label.isEmpty) return const SizedBox.shrink();

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 11, color: color),
          const SizedBox(width: 3),
          Text(label,
              style: TextStyle(
                  fontSize: 10.5, fontWeight: FontWeight.w700, color: color)),
        ],
      ),
    );
  }
}

class _RowIconButton extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final VoidCallback onTap;
  final bool accent;
  const _RowIconButton({required this.icon, required this.tooltip, required this.onTap, this.accent = false});

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(8),
        child: Padding(
          padding: const EdgeInsets.all(6),
          child: Icon(icon, size: 17, color: accent ? OnDexColors.primary : OnDexColors.inkDim),
        ),
      ),
    );
  }
}

// Kategoriya nomiga qarab barqaror (deterministik) rang tanlaydi — bir xil
// nom har doim bir xil rangga ega bo'ladi, alohida rang jadvali saqlashga
// hojat yo'q (kategoriyalar ochiq ro'yxat, har restoran o'zi ham qo'sha oladi).
const _categoryPalette = [
  (OnDexColors.primary, OnDexColors.primaryTint),
  (OnDexColors.info, OnDexColors.infoBg),
  (OnDexColors.purple, OnDexColors.purpleBg),
  (OnDexColors.amber, OnDexColors.amberBg),
  (OnDexColors.success, OnDexColors.successBg),
  (OnDexColors.danger, OnDexColors.dangerBg),
];

(Color, Color) _categoryStyle(String category) {
  if (category.isEmpty) return (OnDexColors.inkDim, OnDexColors.pageBg);
  final sum = category.codeUnits.fold<int>(0, (a, c) => a + c);
  return _categoryPalette[sum % _categoryPalette.length];
}

// ---------------------------------------------------------------------------
// Bo'sh holat
// ---------------------------------------------------------------------------

class _EmptyState extends StatelessWidget {
  final String text;
  const _EmptyState({required this.text});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 48),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Center(child: Text(text, style: const TextStyle(color: OnDexColors.inkDim))),
    );
  }
}

// ---------------------------------------------------------------------------
// Sahifalash — "< 1 >" markazdan boshlanadi (foydalanuvchi so'rovi bo'yicha)
// ---------------------------------------------------------------------------

class _PaginationRow extends StatelessWidget {
  final int totalCount;
  final int page;
  final int totalPages;
  final int pageSize;
  final ValueChanged<int> onPageChanged;
  final ValueChanged<int> onPageSizeChanged;

  const _PaginationRow({
    required this.totalCount,
    required this.page,
    required this.totalPages,
    required this.pageSize,
    required this.onPageChanged,
    required this.onPageSizeChanged,
  });

  @override
  Widget build(BuildContext context) {
    final countText = Text('Jami $totalCount ta mahsulot',
        style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim));
    final pageControls = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        _PageArrow(icon: Icons.chevron_left_rounded, enabled: page > 0, onTap: () => onPageChanged(page - 1)),
        for (final p in _visiblePages(page, totalPages))
          if (p == null)
            const Padding(
              padding: EdgeInsets.symmetric(horizontal: 6),
              child: Text('…', style: TextStyle(color: OnDexColors.inkFaint)),
            )
          else
            _PageNumberButton(number: p, selected: p == page, onTap: () => onPageChanged(p)),
        _PageArrow(
            icon: Icons.chevron_right_rounded,
            enabled: page < totalPages - 1,
            onTap: () => onPageChanged(page + 1)),
      ],
    );
    final pageSizeControl = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        const Text('Har sahifada:', style: TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
        const SizedBox(width: 8),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 10),
          decoration: BoxDecoration(
            color: OnDexColors.cardBg,
            border: Border.all(color: OnDexColors.cardBorder),
            borderRadius: BorderRadius.circular(8),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<int>(
              value: pageSize,
              isDense: true,
              items: [
                for (final s in _pageSizeOptions) DropdownMenuItem(value: s, child: Text('$s')),
              ],
              onChanged: (v) => onPageSizeChanged(v ?? _pageSizeOptions[0]),
            ),
          ),
        ),
      ],
    );

    // MUHIM: sahifa raqamlari ("< 1 >") aynan KONTEYNER O'RTASIDAN
    // boshlanishi kerak (chap/o'ng tomondagi matn kengligidan qat'iy
    // nazar) — shuning uchun 3 ta TENG (flex:1) Expanded ustun ishlatiladi:
    // chap (matn, chapga tekislangan), o'rta (sahifa tugmalari, markazda),
    // o'ng (sahifa hajmi tanlagichi, o'ngga tekislangan). Agar shunchaki
    // Row+Spacer ishlatilsa, o'rtadagi blok chap/o'ng blok kengliklariga
    // qarab siljib ketadi — bu yerda esa doim aniq markazda turadi.
    return LayoutBuilder(builder: (context, c) {
      if (c.maxWidth < 720) {
        return Wrap(
          spacing: 16,
          runSpacing: 12,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [countText, pageControls, pageSizeControl],
        );
      }
      return Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Expanded(child: Align(alignment: Alignment.centerLeft, child: countText)),
          Center(child: pageControls),
          Expanded(child: Align(alignment: Alignment.centerRight, child: pageSizeControl)),
        ],
      );
    });
  }
}

List<int?> _visiblePages(int page, int totalPages) {
  if (totalPages <= 7) return [for (var i = 0; i < totalPages; i++) i];
  final result = <int?>{0, totalPages - 1, page, page - 1, page + 1}
      .where((p) => p != null && p >= 0 && p < totalPages)
      .cast<int>()
      .toList()
    ..sort();
  final out = <int?>[];
  for (var i = 0; i < result.length; i++) {
    if (i > 0 && result[i] - result[i - 1] > 1) out.add(null);
    out.add(result[i]);
  }
  return out;
}

class _PageArrow extends StatelessWidget {
  final IconData icon;
  final bool enabled;
  final VoidCallback onTap;
  const _PageArrow({required this.icon, required this.enabled, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: enabled ? onTap : null,
      borderRadius: BorderRadius.circular(8),
      child: Container(
        width: 32,
        height: 32,
        margin: const EdgeInsets.symmetric(horizontal: 2),
        decoration: BoxDecoration(
          border: Border.all(color: OnDexColors.cardBorder),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Icon(icon, size: 18, color: enabled ? OnDexColors.ink : OnDexColors.inkFaint),
      ),
    );
  }
}

class _PageNumberButton extends StatelessWidget {
  final int number;
  final bool selected;
  final VoidCallback onTap;
  const _PageNumberButton({required this.number, required this.selected, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8),
      child: Container(
        width: 32,
        height: 32,
        margin: const EdgeInsets.symmetric(horizontal: 2),
        alignment: Alignment.center,
        decoration: BoxDecoration(
          color: selected ? OnDexColors.primary : Colors.transparent,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Text('${number + 1}',
            style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w700,
                color: selected ? Colors.white : OnDexColors.inkDim)),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Mahsulotni ko'rish (eye icon) — o'qish uchun, tahrirlanmaydi
// ---------------------------------------------------------------------------

class _ProductPreviewDialog extends StatelessWidget {
  final Map<String, dynamic> product;
  final int soldQty;
  const _ProductPreviewDialog({required this.product, required this.soldQty});

  @override
  Widget build(BuildContext context) {
    final imgUrl = product['image_url'] as String? ?? '';
    final category = (product['category'] as String? ?? '').trim();
    final available = product['available'] == true;
    final weight = ((product['weight'] ?? 0) as num).toDouble();
    final weightUnit = (product['weight_unit'] as String?) ?? 'g';
    final desc = (product['description'] as String? ?? '').trim();
    final (catColor, catBg) = _categoryStyle(category);

    return AlertDialog(
      title: Text(product['name'] as String? ?? ''),
      content: SizedBox(
        width: 360,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Center(
              child: ClipRRect(
                borderRadius: BorderRadius.circular(16),
                child: SizedBox(
                  width: 200,
                  height: 200,
                  child: imgUrl.isEmpty
                      ? Container(
                          color: OnDexColors.pageBg,
                          child: const Icon(Icons.restaurant_rounded, size: 40, color: OnDexColors.inkFaint),
                        )
                      : Image.network(fullImageUrl(imgUrl), fit: BoxFit.cover),
                ),
              ),
            ),
            const SizedBox(height: 16),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                if (category.isNotEmpty)
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(color: catBg, borderRadius: BorderRadius.circular(999)),
                    child: Text(category, style: TextStyle(fontSize: 11.5, fontWeight: FontWeight.w700, color: catColor)),
                  ),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(
                      color: available ? OnDexColors.successBg : OnDexColors.dangerBg,
                      borderRadius: BorderRadius.circular(999)),
                  child: Text(available ? 'Faol' : 'Nofaol',
                      style: TextStyle(
                          fontSize: 11.5,
                          fontWeight: FontWeight.w700,
                          color: available ? OnDexColors.success : OnDexColors.danger)),
                ),
              ],
            ),
            const SizedBox(height: 14),
            _PreviewRow(label: 'Narxi', value: formatSum((product['price_tiyin'] ?? 0) as int)),
            if (weight > 0)
              _PreviewRow(
                  label: 'Miqdor',
                  value: '${weight % 1 == 0 ? weight.toInt() : weight} $weightUnit'),
            _PreviewRow(label: 'Sotilgan', value: '$soldQty ta'),
            if (desc.isNotEmpty) ...[
              const SizedBox(height: 10),
              const Text('Tasvif', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: OnDexColors.inkDim)),
              const SizedBox(height: 4),
              Text(desc, style: const TextStyle(fontSize: 13, color: OnDexColors.ink)),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: const Text('Yopish')),
      ],
    );
  }
}

class _PreviewRow extends StatelessWidget {
  final String label;
  final String value;
  const _PreviewRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        children: [
          Text(label, style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
          const Spacer(),
          Text(value, style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
        ],
      ),
    );
  }
}

// Zaxira ro'yxat: server hali javob bermagan yoki so'rov muvaffaqiyatsiz
// bo'lgan holatda ishlatiladi (haqiqiy manba — backend GET /categories,
// bir joyda saqlanadi, mijoz ilovasi ham shundan foydalanadi).
const _fallbackCategories = [
  'Fast Food',
  'Ichimliklar',
  'Shirinliklar',
  'Milliy taomlar',
  'Yevropa taomlar',
  'Steyklar',
  'Pizza',
  'Burgerlar',
  'KFC',
  'Norin',
  'Suyuq ovqatlar',
  'Quyuq ovqatlar',
  'Salatlar',
  'Gazaklar',
  'Desertlar',
];

// Tanlovda "Yangi turkum qo'shish" varianti uchun maxsus belgi.
const _customCategorySentinel = '__custom__';

// Solishtirma birlik uchun ruxsat etilgan qiymatlar (backend'dagi
// catalog.AllowedWeightUnits bilan bir xil bo'lishi kerak).
const _weightUnits = ['g', 'kg', 'ml', 'l', 'dona', 'porsiya'];

// Backend'dagi catalog.MaxPrepTimeTextLength bilan bir xil bo'lishi kerak.
const _maxPrepTimeTextLength = 50;

// ---------------------------------------------------------------------------
// Mahsulot qo'shish/tahrirlash — TO'LIQ SAHIFA (image/maxqosh.png namunasiga
// mos, avval modal oyna edi). Ikki ustunli joylashuv: chapda asosiy
// ma'lumotlar+holat, o'ngda rasm+jonli oldindan ko'rish.
// ---------------------------------------------------------------------------

class _ProductFormPage extends StatefulWidget {
  final Map<String, dynamic>? existing;
  final List<String> ownCategories;
  final List<String> predefinedCategories;
  final VoidCallback onCancel;
  // continueAdding=true bo'lsa — "Saqlash va davom ettirish" bosilgan,
  // forma o'zini tozalab shu sahifada qoladi; false bo'lsa oddiy "Saqlash"
  // — chaqiruvchi ro'yxatga qaytaradi.
  final ValueChanged<bool> onSaved;

  const _ProductFormPage({
    super.key,
    this.existing,
    required this.ownCategories,
    required this.predefinedCategories,
    required this.onCancel,
    required this.onSaved,
  });

  @override
  State<_ProductFormPage> createState() => _ProductFormPageState();
}

class _ProductFormPageState extends State<_ProductFormPage> {
  late final TextEditingController _name;
  late final TextEditingController _customCategory;
  late final TextEditingController _price;
  /// Ulgurji narx — IXTIYORIY ichki maydon (mijozga ko'rsatilmaydi).
  /// Avval bu yerda "Chegirma narxi" turardi; chegirmalar endi faqat
  /// "Aksiyalar" bo'limi orqali beriladi.
  late final TextEditingController _wholesalePrice;
  late final TextEditingController _prepTime;
  late final TextEditingController _description;
  String _weightUnit = 'g';
  // Miqdor (Weight) maydoni endi formada ko'rsatilmaydi (foydalanuvchi
  // so'rovi bo'yicha olib tashlandi) — lekin tahrirlashda eski qiymat
  // yo'qolib ketmasligi uchun jim saqlab qolinadi va o'zgarishsiz qaytadan
  // yuboriladi (yangi mahsulotda har doim 0).
  double _existingWeight = 0;
  bool _available = true;

  late List<String> _categoryOptions;
  String? _selectedCategory;

  Uint8List? _pickedBytes;
  String? _pickedName;
  String _existingImageUrl = '';
  bool _saving = false;
  String? _error;

  bool get _isEdit => widget.existing != null;

  @override
  void initState() {
    super.initState();
    _name = TextEditingController();
    _customCategory = TextEditingController();
    _price = TextEditingController();
    _wholesalePrice = TextEditingController();
    _prepTime = TextEditingController();
    _description = TextEditingController();
    _rebuildCategoryOptions();
    _loadFrom(widget.existing);
  }

  void _rebuildCategoryOptions() {
    // Bo'shliq/registr farqi tufayli "FastFood" va "Fast Food" kabi bir xil
    // kategoriya ikki marta chiqib qolmasligi uchun normallashtirib
    // solishtiramiz (faqat harf/raqam, kichik harf).
    String normalize(String s) => s.toLowerCase().replaceAll(RegExp(r'[^a-z0-9]'), '');
    final combined = <String>[...widget.predefinedCategories];
    final usedNormalized = combined.map(normalize).toSet();
    for (final c in widget.ownCategories) {
      if (usedNormalized.add(normalize(c))) combined.add(c);
    }
    _categoryOptions = combined;
  }

  void _loadFrom(Map<String, dynamic>? e) {
    _name.text = e?['name'] ?? '';
    _customCategory.text = '';
    _price.text = e == null ? '' : ((e['price_tiyin'] as int) ~/ 100).toString();
    final existingWholesale = ((e?['wholesale_price_tiyin'] ?? 0) as num).toInt();
    _wholesalePrice.text =
        existingWholesale > 0 ? (existingWholesale ~/ 100).toString() : '';
    _prepTime.text = e?['prep_time_text'] as String? ?? '';
    _existingWeight = ((e?['weight'] ?? 0) as num).toDouble();
    _weightUnit = (e?['weight_unit'] as String?) ?? 'g';
    if (!_weightUnits.contains(_weightUnit)) _weightUnit = 'g';
    _description.text = e?['description'] as String? ?? '';
    _existingImageUrl = e?['image_url'] as String? ?? '';
    _available = e == null ? true : e['available'] == true;
    _pickedBytes = null;
    _pickedName = null;
    _error = null;

    final existingCategory = (e?['category'] as String?)?.trim();
    if (existingCategory != null && existingCategory.isNotEmpty) {
      if (_categoryOptions.contains(existingCategory)) {
        _selectedCategory = existingCategory;
      } else {
        // Ro'yxatda topilmasa (ehtiyot chorasi) — maxsus tur sifatida ko'rsatamiz.
        _selectedCategory = _customCategorySentinel;
        _customCategory.text = existingCategory;
      }
    } else {
      _selectedCategory = null;
    }
  }

  /// "Saqlash va davom ettirish"dan keyin — kategoriya/birlik/holat SAQLAB
  /// qolinadi (bir xil turdagi bir nechta mahsulotni ketma-ket qo'shish tez
  /// bo'lishi uchun), qolgan maydonlar tozalanadi.
  void _resetForNext() {
    _name.clear();
    _price.clear();
    _wholesalePrice.clear();
    _prepTime.clear();
    _description.clear();
    _existingWeight = 0;
    _pickedBytes = null;
    _pickedName = null;
    _existingImageUrl = '';
    _error = null;
  }

  Future<void> _pickImage() async {
    final result = await FilePicker.platform.pickFiles(type: FileType.image, withData: true);
    final f = result?.files.firstOrNull;
    if (f?.bytes == null) return;
    setState(() {
      _pickedBytes = f!.bytes;
      _pickedName = f.name;
    });
  }

  Future<void> _submit({required bool continueAdding}) async {
    final name = _name.text.trim();
    final category = _selectedCategory == _customCategorySentinel
        ? _customCategory.text.trim()
        : (_selectedCategory ?? '');
    final sum = int.tryParse(_price.text.trim());
    final wholesaleText = _wholesalePrice.text.trim();
    final wholesaleSum = wholesaleText.isEmpty ? 0 : int.tryParse(wholesaleText);
    final prepTime = _prepTime.text.trim();
    // Miqdor (stock) maydoni endi ushbu formada ko'rsatilmaydi — lekin agar
    // taom avval (eski versiyada) stock bilan saqlangan bo'lsa, tahrirlashda
    // shu qiymat jim saqlanib qoladi (0'ga tushirib yubormaslik uchun).
    final stock = (widget.existing?['stock'] ?? 0) as int;

    if (name.isEmpty) {
      setState(() => _error = 'Mahsulot nomini kiriting');
      return;
    }
    if (category.isEmpty) {
      setState(() => _error = 'Kategoriyani tanlang yoki yangi kategoriya kiriting');
      return;
    }
    if (sum == null || sum <= 0) {
      setState(() => _error = 'Narxni to\'g\'ri kiriting');
      return;
    }
    // Ulgurji narx — ixtiyoriy. Asosiy narx bilan TAQQOSLANMAYDI: u
    // undan yuqori ham bo'lishi mumkin (bu restoranning o'z hisob-kitobi,
    // mijoz to'laydigan summaga umuman ta'sir qilmaydi).
    if (wholesaleText.isNotEmpty && (wholesaleSum == null || wholesaleSum < 0)) {
      setState(() => _error = 'Ulgurji narxni to\'g\'ri kiriting');
      return;
    }

    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      var imageUrl = _existingImageUrl;
      if (_pickedBytes != null) {
        imageUrl = await api.uploadImage(_pickedBytes!, _pickedName ?? 'rasm.jpg');
      }
      final saved = await api.saveProduct(
        id: widget.existing?['id'],
        name: name,
        category: category,
        priceTiyin: sum * 100,
        wholesalePriceTiyin: (wholesaleSum ?? 0) * 100,
        stock: stock,
        weight: _existingWeight,
        weightUnit: _weightUnit,
        description: _description.text.trim(),
        prepTimeText: prepTime,
        imageUrl: imageUrl,
        available: _available,
      );
      if (!mounted) return;

      // ── 3D model taklifi ──
      //
      // Taklif SAQLANGANDAN KEYIN chiqadi: mahsulot allaqachon
      // menyuda, ya'ni restoran "Yo'q" desa ham hech narsa
      // yo'qolmaydi — oddiy rasm bilan saqlangan holicha qoladi.
      await _maybeOfferModel3D(saved, imageUrl);
      if (!mounted) return;
      if (continueAdding) {
        setState(() {
          _resetForNext();
          _rebuildCategoryOptions();
        });
      }
      widget.onSaved(continueAdding);
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (_) {
      setState(() => _error = 'Xato yuz berdi, qayta urinib ko\'ring');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  /// Rasm yuklangan mahsulot uchun 3D model taklif qiladi.
  ///
  /// ┌─ QACHON SO'RALADI ────────────────────────────────────────────┐
  /// Taklif FAQAT quyidagi hollarda chiqadi:
  ///   * mahsulotda rasm bor (rasmsiz model yasab bo'lmaydi);
  ///   * modeli hali yo'q va tayyorlanayotgani ham yo'q.
  ///
  /// Har saqlashda qayta so'ralsa — restoran allaqachon "Yo'q" degan
  /// taomni tahrirlaganda yana bezovta bo'lardi. Model bor bo'lsa esa
  /// qayta yaratish "Ko'proq" menyusi orqali, ATAYLAB qo'lda
  /// chaqiriladi: har generatsiya PUL sarflaydi.
  /// └───────────────────────────────────────────────────────────────┘
  Future<void> _maybeOfferModel3D(
      Map<String, dynamic> saved, String imageUrl) async {
    if (imageUrl.trim().isEmpty) return;
    if (((saved['model_3d_url'] as String?) ?? '').isNotEmpty) return;
    if (saved['model_3d_status'] == 'pending') return;

    final productId = (saved['id'] as String?) ?? '';
    if (productId.isEmpty) return;

    final name = (saved['name'] as String?) ?? '';
    final wants = await showDialog<bool>(
      context: context,
      builder: (ctx) => _Model3DOfferDialog(productName: name),
    );
    if (wants != true || !mounted) return;

    try {
      await api.generateModel3D(productId);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('3D model tayyorlanmoqda — bir necha daqiqa vaqt '
              'oladi. Tayyor bo\'lganda menyuda «3D» belgisi paydo bo\'ladi.'),
          duration: Duration(seconds: 5),
        ),
      );
    } on ApiException catch (e) {
      if (!mounted) return;
      // Xato saqlashni BUZMAYDI: mahsulot allaqachon menyuda.
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('3D model: ${e.message}')));
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('3D modelni boshlab bo\'lmadi')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(_isEdit ? 'Mahsulotni tahrirlash' : 'Mahsulot qo\'shish',
            style: const TextStyle(fontSize: 27, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
        const SizedBox(height: 4),
        Text(
          _isEdit ? 'Mahsulot ma\'lumotlarini yangilang' : 'Yangi mahsulotni menyuga qo\'shing',
          style: const TextStyle(fontSize: 14, color: OnDexColors.inkDim),
        ),
        const SizedBox(height: 24),
        LayoutBuilder(builder: (context, c) {
          final stacked = c.maxWidth < 900;
          final left = _FormCard(
            title: 'Asosiy ma\'lumotlar',
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      flex: 2,
                      child: _LabeledField(
                        label: 'Mahsulot nomi',
                        required: true,
                        child: TextField(
                          controller: _name,
                          onChanged: (_) => setState(() {}),
                          decoration: _formDec(hint: 'Masalan: Lavash mini'),
                        ),
                      ),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: _LabeledField(
                        label: 'Kategoriya',
                        required: true,
                        child: DropdownButtonFormField<String>(
                          initialValue: _selectedCategory,
                          isExpanded: true,
                          dropdownColor: OnDexColors.cardBg,
                          decoration: _formDec(hint: 'Kategoriyani tanlang'),
                          items: [
                            for (final cat in _categoryOptions)
                              DropdownMenuItem(value: cat, child: Text(cat)),
                            const DropdownMenuItem(
                              value: _customCategorySentinel,
                              child: Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Icon(Icons.add, size: 17),
                                  SizedBox(width: 6),
                                  Text('Yangi kategoriya'),
                                ],
                              ),
                            ),
                          ],
                          onChanged: _saving ? null : (v) => setState(() => _selectedCategory = v),
                        ),
                      ),
                    ),
                  ],
                ),
                if (_selectedCategory == _customCategorySentinel) ...[
                  const SizedBox(height: 14),
                  _LabeledField(
                    label: 'Yangi kategoriya nomi',
                    child: TextField(
                      controller: _customCategory,
                      autofocus: true,
                      decoration: _formDec(
                          hint: 'Masalan: Tandir non',
                          helper: 'Bu kategoriya faqat sizning restoraningizda ko\'rinadi'),
                    ),
                  ),
                ],
                const SizedBox(height: 14),
                _LabeledField(
                  label: 'Qisqacha tavsif',
                  child: TextField(
                    controller: _description,
                    maxLines: 3,
                    maxLength: 200,
                    onChanged: (_) => setState(() {}),
                    decoration: _formDec(hint: 'Masalan: Mini lavash tandirda pishirilgan'),
                  ),
                ),
                const SizedBox(height: 6),
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: _LabeledField(
                        label: 'Narxi',
                        required: true,
                        child: TextField(
                          controller: _price,
                          keyboardType: TextInputType.number,
                          onChanged: (_) => setState(() {}),
                          decoration: _formDec(hint: '0', suffix: 'so\'m'),
                        ),
                      ),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: _LabeledField(
                        // Chegirma narxi bu yerdan OLIB TASHLANDI:
                        // chegirmalar faqat "Aksiyalar" bo'limida
                        // beriladi (bitta joyda boshqariladi, muddati
                        // va qamrovi bilan). Ulgurji narx esa —
                        // restoranning o'z hisob-kitobi uchun, mijozga
                        // umuman ko'rsatilmaydi.
                        label: 'Ulgurji narx',
                        child: TextField(
                          controller: _wholesalePrice,
                          keyboardType: TextInputType.number,
                          onChanged: (_) => setState(() {}),
                          decoration: _formDec(
                              hint: '0',
                              suffix: 'so\'m',
                              helper: 'Ixtiyoriy, faqat siz ko\'rasiz'),
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 14),
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: _LabeledField(
                        label: 'Tayyorlanish vaqti',
                        child: TextField(
                          controller: _prepTime,
                          maxLength: _maxPrepTimeTextLength,
                          onChanged: (_) => setState(() {}),
                          decoration: _formDec(
                              hint: 'Masalan: 20-30 daqiqa',
                              helper: 'Mijozga ko\'rsatiladigan tayyorlanish vaqti'),
                        ),
                      ),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: _LabeledField(
                        label: 'Solishtirma birlik',
                        child: DropdownButtonFormField<String>(
                          initialValue: _weightUnit,
                          dropdownColor: OnDexColors.cardBg,
                          decoration: _formDec(),
                          items: [
                            for (final u in _weightUnits) DropdownMenuItem(value: u, child: Text(u)),
                          ],
                          onChanged: _saving ? null : (v) => setState(() => _weightUnit = v ?? 'g'),
                        ),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          );

          final holatCard = _FormCard(
            title: 'Holat',
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Padding(
                  padding: EdgeInsets.only(bottom: 8),
                  child: Text.rich(
                    TextSpan(
                      text: 'Mavjudlik',
                      style: TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: OnDexColors.ink),
                      children: [
                        TextSpan(text: ' *', style: TextStyle(color: OnDexColors.danger)),
                      ],
                    ),
                  ),
                ),
                Row(
                  children: [
                    _AvailabilityRadio(
                      label: 'Mavjud',
                      selected: _available,
                      onTap: () => setState(() => _available = true),
                    ),
                    const SizedBox(width: 20),
                    _AvailabilityRadio(
                      label: 'Mavjud emas',
                      selected: !_available,
                      onTap: () => setState(() => _available = false),
                    ),
                  ],
                ),
              ],
            ),
          );

          final imageCard = _FormCard(
            title: 'Rasm',
            child: _ImageDropZone(
              pickedBytes: _pickedBytes,
              existingUrl: _existingImageUrl,
              onPick: _saving ? null : _pickImage,
            ),
          );

          final previewCard = _FormCard(
            title: 'Oldindan ko\'rish',
            child: _LivePreview(
              imageBytes: _pickedBytes,
              imageUrl: _existingImageUrl,
              name: _name.text,
              description: _description.text,
              category: _selectedCategory == _customCategorySentinel
                  ? _customCategory.text
                  : (_selectedCategory ?? ''),
              priceText: _price.text,
              prepTimeText: _prepTime.text,
            ),
          );

          if (stacked) {
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                left,
                const SizedBox(height: 16),
                holatCard,
                const SizedBox(height: 16),
                imageCard,
                const SizedBox(height: 16),
                previewCard,
              ],
            );
          }
          return Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                flex: 2,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [left, const SizedBox(height: 16), holatCard],
                ),
              ),
              const SizedBox(width: 20),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [imageCard, const SizedBox(height: 16), previewCard],
                ),
              ),
            ],
          );
        }),
        if (_error != null) ...[
          const SizedBox(height: 16),
          Text(_error!, style: const TextStyle(color: OnDexColors.danger)),
        ],
        const SizedBox(height: 24),
        Wrap(
          alignment: WrapAlignment.spaceBetween,
          spacing: 12,
          runSpacing: 12,
          children: [
            OutlinedButton(
              onPressed: _saving ? null : widget.onCancel,
              style: OutlinedButton.styleFrom(
                foregroundColor: OnDexColors.inkDim,
                side: const BorderSide(color: OnDexColors.cardBorder),
                padding: const EdgeInsets.symmetric(horizontal: 26, vertical: 18),
                textStyle: const TextStyle(fontSize: 14.5, fontWeight: FontWeight.w700),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
              ),
              child: const Text('Bekor qilish'),
            ),
            Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [
                if (!_isEdit)
                  OutlinedButton(
                    onPressed: _saving ? null : () => _submit(continueAdding: true),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: OnDexColors.primary,
                      side: const BorderSide(color: OnDexColors.primary),
                      padding: const EdgeInsets.symmetric(horizontal: 26, vertical: 18),
                      textStyle: const TextStyle(fontSize: 14.5, fontWeight: FontWeight.w700),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                    ),
                    child: const Text('Saqlash va davom ettirish'),
                  ),
                FilledButton(
                  onPressed: _saving ? null : () => _submit(continueAdding: false),
                  style: FilledButton.styleFrom(
                    padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 18),
                    textStyle: const TextStyle(fontSize: 14.5, fontWeight: FontWeight.w700),
                  ),
                  child: Text(_saving ? 'Saqlanmoqda...' : 'Saqlash'),
                ),
              ],
            ),
          ],
        ),
      ],
    );
  }

  InputDecoration _formDec({String? hint, String? suffix, String? helper}) => InputDecoration(
        isDense: true,
        hintText: hint,
        hintStyle: const TextStyle(fontSize: 13, color: OnDexColors.inkFaint),
        suffixText: suffix,
        helperText: helper,
        helperMaxLines: 2,
        filled: true,
        fillColor: OnDexColors.cardBg,
        contentPadding: const EdgeInsets.symmetric(vertical: 13, horizontal: 14),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: OnDexColors.cardBorder),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: OnDexColors.primary),
        ),
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(10)),
      );
}

class _FormCard extends StatelessWidget {
  final String title;
  final Widget child;
  const _FormCard({required this.title, required this.child});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(title,
              style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
          const SizedBox(height: 16),
          child,
        ],
      ),
    );
  }
}

class _LabeledField extends StatelessWidget {
  final String label;
  final bool required;
  final Widget child;
  const _LabeledField({required this.label, required this.child, this.required = false});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text.rich(
          TextSpan(
            text: label,
            style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: OnDexColors.ink),
            children: [
              if (required) const TextSpan(text: ' *', style: TextStyle(color: OnDexColors.danger)),
            ],
          ),
        ),
        const SizedBox(height: 6),
        child,
      ],
    );
  }
}

class _AvailabilityRadio extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;
  const _AvailabilityRadio({required this.label, required this.selected, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 6, horizontal: 4),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 20,
              height: 20,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(color: selected ? OnDexColors.primary : OnDexColors.cardBorder, width: 2),
              ),
              child: selected
                  ? Center(
                      child: Container(
                        width: 10,
                        height: 10,
                        decoration: const BoxDecoration(color: OnDexColors.primary, shape: BoxShape.circle),
                      ),
                    )
                  : null,
            ),
            const SizedBox(width: 8),
            Text(label, style: const TextStyle(fontSize: 13.5, color: OnDexColors.ink)),
          ],
        ),
      ),
    );
  }
}

class _ImageDropZone extends StatelessWidget {
  final Uint8List? pickedBytes;
  final String existingUrl;
  final VoidCallback? onPick;
  const _ImageDropZone({required this.pickedBytes, required this.existingUrl, required this.onPick});

  @override
  Widget build(BuildContext context) {
    final hasImage = pickedBytes != null || existingUrl.isNotEmpty;
    return InkWell(
      onTap: onPick,
      borderRadius: BorderRadius.circular(12),
      child: Container(
        height: 190,
        decoration: BoxDecoration(
          color: OnDexColors.pageBg,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: OnDexColors.cardBorder),
        ),
        clipBehavior: Clip.antiAlias,
        child: hasImage
            ? Stack(
                fit: StackFit.expand,
                children: [
                  pickedBytes != null
                      ? Image.memory(pickedBytes!, fit: BoxFit.cover)
                      : Image.network(fullImageUrl(existingUrl), fit: BoxFit.cover),
                  Positioned(
                    right: 8,
                    bottom: 8,
                    child: Material(
                      color: Colors.black.withValues(alpha: 0.55),
                      borderRadius: BorderRadius.circular(8),
                      child: const Padding(
                        padding: EdgeInsets.symmetric(horizontal: 10, vertical: 8),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Icon(Icons.edit_rounded, size: 14, color: Colors.white),
                            SizedBox(width: 6),
                            Text('Almashtirish', style: TextStyle(fontSize: 12, color: Colors.white)),
                          ],
                        ),
                      ),
                    ),
                  ),
                ],
              )
            : Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  const Icon(Icons.cloud_upload_outlined, size: 30, color: OnDexColors.inkFaint),
                  const SizedBox(height: 10),
                  const Text('Mahsulot rasmini yuklang',
                      style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: OnDexColors.ink)),
                  const SizedBox(height: 3),
                  const Text('PNG, JPG format. Maksimal 5MB',
                      style: TextStyle(fontSize: 11.5, color: OnDexColors.inkFaint)),
                  const SizedBox(height: 14),
                  OutlinedButton(
                    onPressed: onPick,
                    style: OutlinedButton.styleFrom(
                      foregroundColor: OnDexColors.primary,
                      side: const BorderSide(color: OnDexColors.primary),
                      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                    ),
                    child: const Text('Rasm tanlash'),
                  ),
                ],
              ),
      ),
    );
  }
}

class _LivePreview extends StatelessWidget {
  final Uint8List? imageBytes;
  final String imageUrl;
  final String name;
  final String description;
  final String category;
  final String priceText;
  final String prepTimeText;

  const _LivePreview({
    required this.imageBytes,
    required this.imageUrl,
    required this.name,
    required this.description,
    required this.category,
    required this.priceText,
    this.prepTimeText = '',
  });

  @override
  Widget build(BuildContext context) {
    final sum = int.tryParse(priceText.trim()) ?? 0;
    final hasImage = imageBytes != null || imageUrl.isNotEmpty;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: OnDexColors.pageBg,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(10),
            child: SizedBox(
              width: 56,
              height: 56,
              child: !hasImage
                  ? Container(
                      color: OnDexColors.cardBg,
                      child: const Icon(Icons.image_outlined, size: 22, color: OnDexColors.inkFaint),
                    )
                  : imageBytes != null
                      ? Image.memory(imageBytes!, fit: BoxFit.cover)
                      : Image.network(fullImageUrl(imageUrl), fit: BoxFit.cover),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name.isEmpty ? 'Mahsulot nomi' : name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                        fontSize: 13.5,
                        fontWeight: FontWeight.w700,
                        color: name.isEmpty ? OnDexColors.inkFaint : OnDexColors.ink)),
                const SizedBox(height: 2),
                Text(description.isEmpty ? 'Qisqacha tavsif' : description,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkFaint)),
                if ((category.isNotEmpty && category != _customCategorySentinel) ||
                    prepTimeText.trim().isNotEmpty) ...[
                  const SizedBox(height: 6),
                  Wrap(
                    spacing: 6,
                    runSpacing: 4,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: [
                      if (category.isNotEmpty && category != _customCategorySentinel)
                        Builder(builder: (context) {
                          final (color, bg) = _categoryStyle(category);
                          return Container(
                            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                            decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(999)),
                            child: Text(category, style: TextStyle(fontSize: 10.5, fontWeight: FontWeight.w700, color: color)),
                          );
                        }),
                      if (prepTimeText.trim().isNotEmpty)
                        Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            const Icon(Icons.schedule_rounded, size: 12, color: OnDexColors.inkFaint),
                            const SizedBox(width: 3),
                            Text(prepTimeText.trim(),
                                style: const TextStyle(fontSize: 10.5, color: OnDexColors.inkFaint)),
                          ],
                        ),
                    ],
                  ),
                ],
              ],
            ),
          ),
          const SizedBox(width: 8),
          // Bu — MIJOZ ko'radigan kartochka, shuning uchun faqat sotuv
          // narxi. Ulgurji narx bu yerda UMUMAN ko'rsatilmaydi, chegirma
          // esa "Aksiyalar" bo'limidan kelib chiqadi (mahsulot formasida
          // chegirma narxi endi yo'q).
          Text(formatSum(sum * 100),
              style: const TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w800,
                  color: OnDexColors.ink)),
        ],
      ),
    );
  }
}
