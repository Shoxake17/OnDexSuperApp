import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// "Mahsulotlar" — ShoxPos `MaxsulotBazasi.tsx` (+ qurt uchun `QurtProducts.tsx`).
///
/// 1:1: ro'yxat faqat do'kon kategoriyalaridagi mahsulotlar; qidiruv
/// (shtrix-kod yoki nom) va kategoriya filtri; narxni joyida tahrirlash;
/// o'chirish (tasdiq bilan); "Mahsulot qo'shish" alohida sahifa.
/// Farq: ShoxPos jadvalda narxni HAR tugma bosilganda serverga yuborardi —
/// bu yerda Enter yoki maydondan chiqqanda bir marta.
class ProductsPage extends StatefulWidget {
  const ProductsPage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<ProductsPage> createState() => _ProductsPageState();
}

class _ProductsPageState extends State<ProductsPage> {
  List<Map<String, dynamic>> _all = [];
  List<String> _cats = [];
  bool _loading = true;
  String _q = '';
  String _cat = '';
  bool _showFilter = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final r = await Future.wait([api.shopProducts(), api.shopCategories()]);
      if (!mounted) return;
      final products = r[0] as List<Map<String, dynamic>>;
      final cats = r[1] as List<String>;
      setState(() {
        _cats = cats;
        // ShoxPos: kategoriyalar bo'lsa — faqat shu kategoriyalardagi mahsulotlar.
        _all = cats.isEmpty ? products : products.where((p) => cats.contains(p['category'])).toList();
      });
    } catch (e) {
      if (mounted) toast(context, errText(e), error: true);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  List<Map<String, dynamic>> get _shown {
    final q = _q.trim().toLowerCase();
    return _all.where((p) {
      if (q.isNotEmpty &&
          !(p['barcode'] as String).toLowerCase().contains(q) &&
          !((p['name'] as String?) ?? '').toLowerCase().contains(q)) {
        return false;
      }
      return _cat.isEmpty || p['category'] == _cat;
    }).toList();
  }

  Future<void> _updatePrice(Map<String, dynamic> p, int tiyin) async {
    if (tiyin == intOf(p['price_tiyin'])) return;
    final old = p['price_tiyin'];
    setState(() => p['price_tiyin'] = tiyin);
    try {
      await api.shopUpdateField(p['barcode'] as String, 'price', tiyin);
    } catch (e) {
      if (!mounted) return;
      setState(() => p['price_tiyin'] = old);
      toast(context, errText(e), error: true);
    }
  }

  Future<void> _delete(String barcode) async {
    if (!await confirmDialog(context, 'Mahsulotni o\'chirishni xohlaysizmi?')) return;
    try {
      await api.shopDelete(barcode);
      if (!mounted) return;
      toast(context, 'Muvaffaqiyatli');
      await _load();
    } catch (e) {
      if (mounted) toast(context, errText(e), error: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final narrow = MediaQuery.sizeOf(context).width < 760;
    final shown = _shown;
    return ShopPage(children: [
      Row(children: [
        Expanded(child: SearchBox(onChanged: (v) => setState(() => _q = v))),
        const SizedBox(width: 10),
        OutlinedButton.icon(
          key: const ValueKey('products-filter'),
          style: OutlinedButton.styleFrom(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
            backgroundColor: _showFilter ? OnDexColors.primary : null,
            foregroundColor: _showFilter ? Colors.white : OnDexColors.inkDim,
          ),
          onPressed: () => setState(() => _showFilter = !_showFilter),
          icon: const Icon(Icons.filter_list_rounded, size: 18),
          label: const Text('Filtr'),
        ),
        const SizedBox(width: 10),
        FilledButton.icon(
          key: const ValueKey('products-add'),
          onPressed: () => widget.ctx.go(StorePage.addProduct),
          icon: const Icon(Icons.add, size: 18),
          label: Text(narrow ? 'Qo\'shish' : 'Mahsulot qo\'shish'),
        ),
      ]),
      if (_showFilter) ...[
        const SizedBox(height: 12),
        InkWell(
          key: const ValueKey('products-category'),
          borderRadius: BorderRadius.circular(12),
          onTap: () async {
            final v = await pickCategory(context, _cats, _cat, withAll: true);
            if (v != null) setState(() => _cat = v);
          },
          child: InputDecorator(
            decoration: shopInput(''),
            child: Row(children: [
              Expanded(child: Text(categoryLabel(_cat))),
              const Icon(Icons.expand_more_rounded),
            ]),
          ),
        ),
      ],
      const SizedBox(height: 16),
      if (shown.isEmpty)
        ShopCard(child: EmptyState(loading: _loading, text: 'Mahsulot topilmadi'))
      else if (widget.ctx.isQurt)
        _QurtList(
            products: shown,
            narrow: narrow,
            onEdit: (bc) => widget.ctx.go(StorePage.addProduct, arg: bc),
            onDelete: _delete)
      else if (narrow)
        for (final p in shown)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: _ProductCard(p: p, onPrice: (v) => _updatePrice(p, v), onDelete: () => _delete(p['barcode'] as String)),
          )
      else
        _ProductTable(products: shown, onPrice: _updatePrice, onDelete: _delete),
    ]);
  }
}

class _ProductTable extends StatelessWidget {
  const _ProductTable({required this.products, required this.onPrice, required this.onDelete});
  final List<Map<String, dynamic>> products;
  final void Function(Map<String, dynamic> p, int tiyin) onPrice;
  final ValueChanged<String> onDelete;

  @override
  Widget build(BuildContext context) {
    return ShopCard(
      padding: EdgeInsets.zero,
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: ConstrainedBox(
          constraints: BoxConstraints(minWidth: MediaQuery.sizeOf(context).width - 320),
          child: DataTable(
            headingRowColor: WidgetStateProperty.all(OnDexColors.pageBg),
            headingTextStyle:
                const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.inkDim),
            columns: const [
              DataColumn(label: Text('#')),
              DataColumn(label: Text('SHTRIX-KOD')),
              DataColumn(label: Text('NOMI')),
              DataColumn(label: Text('NARXI')),
              DataColumn(label: Text('SONI')),
              DataColumn(label: Text('KATEGORIYA')),
              DataColumn(label: Text('AMALLAR')),
            ],
            rows: [
              for (var i = 0; i < products.length; i++)
                () {
                  final p = products[i];
                  final q = numOf(p['quantity']);
                  return DataRow(
                    color: WidgetStateProperty.all(q <= 0
                        ? OnDexColors.dangerBg.withValues(alpha: .5)
                        : q <= 5
                            ? OnDexColors.warningBg.withValues(alpha: .5)
                            : null),
                    cells: [
                      DataCell(Text('${i + 1}')),
                      DataCell(Text(p['barcode'] as String,
                          style: const TextStyle(fontFamily: 'monospace', fontWeight: FontWeight.w600, color: OnDexColors.info))),
                      DataCell(ConstrainedBox(
                          constraints: const BoxConstraints(maxWidth: 260),
                          child: Text((p['name'] as String?) ?? '-', overflow: TextOverflow.ellipsis))),
                      DataCell(SizedBox(
                          width: 120,
                          child: _PriceField(
                              key: ValueKey('price-${p['barcode']}-${p['price_tiyin']}'),
                              tiyin: intOf(p['price_tiyin']),
                              onCommit: (v) => onPrice(p, v)))),
                      DataCell(Text(fmtQty(q),
                          style: TextStyle(color: q <= 5 ? qtyColor(q) : null, fontWeight: q <= 0 ? FontWeight.w800 : FontWeight.w600))),
                      DataCell(CategoryChip(p['category'] as String? ?? '')),
                      DataCell(IconButton(
                        key: ValueKey('delete-${p['barcode']}'),
                        tooltip: 'O\'chirish',
                        style: IconButton.styleFrom(backgroundColor: OnDexColors.dangerBg),
                        color: OnDexColors.danger,
                        icon: const Icon(Icons.delete_outline_rounded, size: 18),
                        onPressed: () => onDelete(p['barcode'] as String),
                      )),
                    ],
                  );
                }(),
            ],
          ),
        ),
      ),
    );
  }
}

/// Narx maydoni: so'mda ko'rsatadi, Enter/fokus chiqqanda tiyinda saqlaydi.
class _PriceField extends StatefulWidget {
  const _PriceField({super.key, required this.tiyin, required this.onCommit});
  final int tiyin;
  final ValueChanged<int> onCommit;

  @override
  State<_PriceField> createState() => _PriceFieldState();
}

class _PriceFieldState extends State<_PriceField> {
  late final _c = TextEditingController(text: '${widget.tiyin ~/ 100}');
  final _f = FocusNode();

  @override
  void initState() {
    super.initState();
    _f.addListener(() {
      if (!_f.hasFocus) _commit();
    });
  }

  void _commit() {
    if (_c.text.trim().isEmpty) return;
    widget.onCommit(tiyinFromSomText(_c.text));
  }

  @override
  void dispose() {
    _c.dispose();
    _f.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => TextField(
        controller: _c,
        focusNode: _f,
        inputFormatters: kNumberInput,
        textAlign: TextAlign.center,
        onSubmitted: (_) => _commit(),
        decoration: shopInput('0').copyWith(contentPadding: const EdgeInsets.symmetric(horizontal: 8, vertical: 10)),
      );
}

class _ProductCard extends StatefulWidget {
  const _ProductCard({required this.p, required this.onPrice, required this.onDelete});
  final Map<String, dynamic> p;
  final ValueChanged<int> onPrice;
  final VoidCallback onDelete;

  @override
  State<_ProductCard> createState() => _ProductCardState();
}

class _ProductCardState extends State<_ProductCard> {
  bool _editing = false;

  @override
  Widget build(BuildContext context) {
    final p = widget.p;
    final q = numOf(p['quantity']);
    return ShopCard(
      color: q <= 0 ? const Color(0xFFFFF5F5) : q <= 5 ? const Color(0xFFFFF7ED) : null,
      padding: const EdgeInsets.all(14),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text((p['name'] as String?) ?? '—',
                  maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 15)),
              Text(p['barcode'] as String,
                  style: const TextStyle(fontFamily: 'monospace', fontSize: 11.5, color: OnDexColors.info)),
            ]),
          ),
          const SizedBox(width: 8),
          Flexible(child: CategoryChip(p['category'] as String? ?? '')),
        ]),
        const SizedBox(height: 10),
        Row(children: [
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              const Text('Narxi', style: TextStyle(fontSize: 11, color: OnDexColors.inkDim)),
              _editing
                  ? SizedBox(
                      width: 130,
                      child: _PriceField(
                          tiyin: intOf(p['price_tiyin']),
                          onCommit: (v) {
                            setState(() => _editing = false);
                            widget.onPrice(v);
                          }))
                  : Text(somFromTiyin(intOf(p['price_tiyin'])),
                      style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 15)),
            ]),
          ),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
            decoration: BoxDecoration(color: qtyColor(q).withValues(alpha: .14), borderRadius: BorderRadius.circular(20)),
            child: Text('${fmtQty(q)} dona',
                style: TextStyle(color: qtyColor(q), fontWeight: FontWeight.w700, fontSize: 12)),
          ),
          const SizedBox(width: 6),
          IconButton(
            tooltip: 'Narxni tahrirlash',
            style: IconButton.styleFrom(backgroundColor: OnDexColors.primaryTint),
            color: OnDexColors.primaryPressed,
            icon: const Icon(Icons.edit_rounded, size: 16),
            onPressed: () => setState(() => _editing = true),
          ),
          IconButton(
            tooltip: 'O\'chirish',
            style: IconButton.styleFrom(backgroundColor: OnDexColors.dangerBg),
            color: OnDexColors.danger,
            icon: const Icon(Icons.delete_outline_rounded, size: 16),
            onPressed: widget.onDelete,
          ),
        ]),
      ]),
    );
  }
}

/// Qurt do'koni ro'yxati: rasm, holat (0 dona = "Mavjud emas"), tahrir.
class _QurtList extends StatelessWidget {
  const _QurtList({required this.products, required this.narrow, required this.onEdit, required this.onDelete});
  final List<Map<String, dynamic>> products;
  final bool narrow;
  final ValueChanged<String> onEdit;
  final ValueChanged<String> onDelete;

  Widget _thumb(Map<String, dynamic> p, [double h = 40]) {
    final url = (p['image_url'] as String?) ?? '';
    return Container(
      width: (h * 2.4).roundToDouble(),
      height: h,
      decoration: BoxDecoration(color: OnDexColors.pageBg, borderRadius: BorderRadius.circular(10)),
      clipBehavior: Clip.antiAlias,
      child: url.isEmpty
          ? Icon(Icons.add_photo_alternate_outlined, size: h / 2, color: OnDexColors.inkFaint)
          : Image.network(fullImageUrl(url),
              fit: BoxFit.contain,
              errorBuilder: (_, __, ___) => Icon(Icons.broken_image_outlined, size: h / 2, color: OnDexColors.inkFaint)),
    );
  }

  Widget _badge(bool available) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 5),
        decoration: BoxDecoration(
            color: available ? OnDexColors.successBg : OnDexColors.dangerBg, borderRadius: BorderRadius.circular(999)),
        child: Text(available ? 'Mavjud' : 'Mavjud emas',
            style: TextStyle(
                fontSize: 12, fontWeight: FontWeight.w700, color: available ? OnDexColors.success : OnDexColors.danger)),
      );

  Widget _actions(Map<String, dynamic> p) => Row(mainAxisSize: MainAxisSize.min, children: [
        IconButton(
          key: ValueKey('edit-${p['barcode']}'),
          tooltip: 'Tahrirlash',
          style: IconButton.styleFrom(backgroundColor: OnDexColors.primaryTint),
          color: OnDexColors.primaryPressed,
          icon: const Icon(Icons.edit_rounded, size: 18),
          onPressed: () => onEdit(p['barcode'] as String),
        ),
        const SizedBox(width: 6),
        IconButton(
          key: ValueKey('delete-${p['barcode']}'),
          tooltip: 'O\'chirish',
          style: IconButton.styleFrom(backgroundColor: OnDexColors.dangerBg),
          color: OnDexColors.danger,
          icon: const Icon(Icons.delete_outline_rounded, size: 18),
          onPressed: () => onDelete(p['barcode'] as String),
        ),
      ]);

  @override
  Widget build(BuildContext context) {
    if (narrow) {
      return Column(children: [
        for (var i = 0; i < products.length; i++)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: ShopCard(
              padding: const EdgeInsets.all(12),
              child: Column(children: [
                Row(children: [
                  SizedBox(
                      width: 20,
                      child: Text('${i + 1}', style: const TextStyle(color: OnDexColors.inkDim, fontWeight: FontWeight.w700))),
                  _thumb(products[i]),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Text((products[i]['name'] as String?) ?? '-',
                          maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(fontWeight: FontWeight.w700)),
                      Text(products[i]['category'] as String? ?? '',
                          style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                    ]),
                  ),
                  _badge(numOf(products[i]['quantity']) > 0),
                ]),
                const SizedBox(height: 10),
                Row(children: [
                  Expanded(
                    child: Text.rich(TextSpan(children: [
                      TextSpan(
                          text: somFromTiyin(intOf(products[i]['price_tiyin'])),
                          style: const TextStyle(fontWeight: FontWeight.w800)),
                      TextSpan(
                          text: ' · ${fmtQty(numOf(products[i]['quantity']))} dona',
                          style: const TextStyle(color: OnDexColors.inkDim)),
                    ])),
                  ),
                  _actions(products[i]),
                ]),
              ]),
            ),
          ),
      ]);
    }
    return ShopCard(
      padding: EdgeInsets.zero,
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: DataTable(
          dataRowMinHeight: 60,
          dataRowMaxHeight: 64,
          headingRowColor: WidgetStateProperty.all(OnDexColors.pageBg),
          headingTextStyle: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.inkDim),
          columns: const [
            DataColumn(label: Text('№')),
            DataColumn(label: Text('RASM')),
            DataColumn(label: Text('NOMI')),
            DataColumn(label: Text('NARXI')),
            DataColumn(label: Text('MIQDORI')),
            DataColumn(label: Text('KATEGORIYA')),
            DataColumn(label: Text('HOLATI')),
            DataColumn(label: Text('AMALLAR')),
          ],
          rows: [
            for (var i = 0; i < products.length; i++)
              DataRow(cells: [
                DataCell(Text('${i + 1}', style: const TextStyle(color: OnDexColors.inkDim))),
                DataCell(_thumb(products[i])),
                DataCell(Text((products[i]['name'] as String?) ?? '-', style: const TextStyle(fontWeight: FontWeight.w600))),
                DataCell(Text(somFromTiyin(intOf(products[i]['price_tiyin'])))),
                DataCell(Text(fmtQty(numOf(products[i]['quantity'])), style: const TextStyle(fontWeight: FontWeight.w600))),
                DataCell(CategoryChip(products[i]['category'] as String? ?? '')),
                DataCell(_badge(numOf(products[i]['quantity']) > 0)),
                DataCell(_actions(products[i])),
              ]),
          ],
        ),
      ),
    );
  }
}
