import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// "Dorilar bazasi" — ShoxPos `PharmBase.tsx`: dorixonadagi dorilar.
///
/// 1:1: qidiruv (shtrix-kod yoki nom) va kategoriya filtri; jadval (№,
/// shtrix-kod, nomi, narxi — joyida tahrirlash, soni — 0 qizil, 1..5 to'q
/// sariq, kategoriya, o'chirish tasdiq bilan); tor ekranda kartalar.
class PharmBasePage extends StatefulWidget {
  const PharmBasePage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<PharmBasePage> createState() => _PharmBasePageState();
}

class _PharmBasePageState extends State<PharmBasePage> {
  List<Map<String, dynamic>> _all = [];
  bool _loading = true;
  String _q = '';
  String _cat = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final l = await api.pharmProducts();
      if (mounted) setState(() => _all = l);
    } catch (e) {
      if (mounted) toast(context, errText(e), error: true);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _price(Map<String, dynamic> p, int tiyin) async {
    final old = p['price_tiyin'];
    setState(() => p['price_tiyin'] = tiyin);
    try {
      await api.pharmUpdate(p['barcode'] as String, {'price_tiyin': tiyin});
      if (mounted) toast(context, 'Narx yangilandi');
    } catch (e) {
      if (!mounted) return;
      setState(() => p['price_tiyin'] = old);
      toast(context, errText(e), error: true);
    }
  }

  Future<void> _delete(Map<String, dynamic> p) async {
    if (!await confirmDialog(context, 'Dorini o\'chirishni xohlaysizmi?')) return;
    try {
      await api.pharmDelete(p['barcode'] as String);
      if (!mounted) return;
      setState(() => _all.remove(p));
      toast(context, 'Dori o\'chirildi');
    } catch (e) {
      if (mounted) toast(context, errText(e), error: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final cats = {for (final p in _all) (p['category'] as String?) ?? ''}.where((c) => c.isNotEmpty).toList()..sort();
    final q = _q.trim().toLowerCase();
    final shown = _all.where((p) {
      if (q.isNotEmpty &&
          !(p['barcode'] as String).toLowerCase().contains(q) &&
          !((p['name'] as String?) ?? '').toLowerCase().contains(q)) {
        return false;
      }
      return _cat.isEmpty || p['category'] == _cat;
    }).toList();
    final narrow = MediaQuery.sizeOf(context).width < 760;

    return ShopPage(children: [
      Row(children: [
        Expanded(child: SearchBox(onChanged: (v) => setState(() => _q = v))),
        const SizedBox(width: 10),
        OutlinedButton.icon(
          style: OutlinedButton.styleFrom(padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16)),
          onPressed: () async {
            final v = await pickCategory(context, cats, _cat, withAll: true);
            if (v != null) setState(() => _cat = v);
          },
          icon: const Icon(Icons.filter_list_rounded, size: 18),
          label: Text(_cat.isEmpty ? 'Filtr' : _cat),
        ),
        const SizedBox(width: 10),
        FilledButton.icon(
          onPressed: () => widget.ctx.go(StorePage.pharmAdd),
          icon: const Icon(Icons.add, size: 18),
          label: const Text('Dori qo\'shish'),
        ),
      ]),
      const SizedBox(height: 16),
      if (shown.isEmpty)
        ShopCard(child: EmptyState(loading: _loading, text: 'Dori topilmadi'))
      else if (narrow)
        for (final p in shown)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: ShopCard(
              padding: const EdgeInsets.all(12),
              child: Row(children: [
                Expanded(
                  child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                    Text((p['name'] as String?) ?? '—', style: const TextStyle(fontWeight: FontWeight.w700)),
                    Text('${p['barcode']} · ${p['category'] ?? ''}',
                        style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                    Text(somFromTiyin(intOf(p['price_tiyin'])), style: const TextStyle(fontWeight: FontWeight.w800)),
                  ]),
                ),
                Text('${fmtQty(numOf(p['quantity']))} dona',
                    style: TextStyle(color: qtyColor(numOf(p['quantity'])), fontWeight: FontWeight.w700)),
                IconButton(
                    color: OnDexColors.danger, onPressed: () => _delete(p), icon: const Icon(Icons.delete_outline_rounded)),
              ]),
            ),
          )
      else
        ShopCard(
          padding: EdgeInsets.zero,
          child: SizedBox(
            width: double.infinity,
            child: DataTable(
              headingRowColor: WidgetStateProperty.all(OnDexColors.pageBg),
              headingTextStyle: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.inkDim),
              columns: const [
                DataColumn(label: Text('№')),
                DataColumn(label: Text('SHTRIX-KOD')),
                DataColumn(label: Text('NOMI')),
                DataColumn(label: Text('NARXI')),
                DataColumn(label: Text('SONI')),
                DataColumn(label: Text('KATEGORIYA')),
                DataColumn(label: Text('AMALLAR')),
              ],
              rows: [
                for (var i = 0; i < shown.length; i++)
                  DataRow(cells: [
                    DataCell(Text('${i + 1}')),
                    DataCell(Text(shown[i]['barcode'] as String,
                        style: const TextStyle(fontFamily: 'monospace', fontWeight: FontWeight.w600, color: OnDexColors.info))),
                    DataCell(Text((shown[i]['name'] as String?) ?? '—')),
                    DataCell(CommitField(
                        key: ValueKey('pharm-price-${shown[i]['barcode']}'),
                        initial: '${intOf(shown[i]['price_tiyin']) ~/ 100}',
                        onCommit: (v) => _price(shown[i], tiyinFromSomText(v)))),
                    DataCell(Text(fmtQty(numOf(shown[i]['quantity'])),
                        style: TextStyle(
                            fontWeight: FontWeight.w700,
                            color: numOf(shown[i]['quantity']) <= 5 ? qtyColor(numOf(shown[i]['quantity'])) : null))),
                    DataCell(CategoryChip((shown[i]['category'] as String?) ?? '')),
                    DataCell(IconButton(
                      key: ValueKey('pharm-delete-${shown[i]['barcode']}'),
                      style: IconButton.styleFrom(backgroundColor: OnDexColors.dangerBg),
                      color: OnDexColors.danger,
                      onPressed: () => _delete(shown[i]),
                      icon: const Icon(Icons.delete_outline_rounded, size: 18),
                    )),
                  ]),
              ],
            ),
          ),
        ),
    ]);
  }
}
