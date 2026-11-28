import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// "Kirim bazasi" — ShoxPos `KirimBazasi.tsx`.
///
/// 1:1: kartalar sotuvlardan (Umumiy kirim = narx×miqdor, Umumiy chiqim =
/// ulgurji×miqdor, Daromad = farq, Sotilgan miqdor); mahsulotlar jadvali —
/// chiqim (ulgurji) narxi va miqdorni joyida tahrirlash, umumiy chiqim =
/// ulgurji × miqdor (miqdor 0 bo'lsa 1 deb — ShoxPos bilan bir xil).
/// Dorixonada (PharmKirimBase.tsx) jadval faqat ko'rish uchun.
class IncomingPage extends StatefulWidget {
  const IncomingPage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<IncomingPage> createState() => _IncomingPageState();
}

class _IncomingPageState extends State<IncomingPage> {
  List<Map<String, dynamic>> _products = [];
  List<Map<String, dynamic>> _sales = [];
  List<String> _cats = [];
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
      final pharm = widget.ctx.isPharmacy;
      final r = await Future.wait([
        pharm ? api.pharmProducts() : api.shopProducts(),
        pharm ? api.pharmSaleLines() : api.shopSales(),
        pharm ? Future.value(<String>[]) : api.shopCategories(),
      ]);
      if (!mounted) return;
      setState(() {
        _products = r[0] as List<Map<String, dynamic>>;
        _sales = r[1] as List<Map<String, dynamic>>;
        _cats = pharm
            ? ({for (final p in _products) (p['category'] as String?) ?? ''}.where((c) => c.isNotEmpty).toList()..sort())
            : r[2] as List<String>;
      });
    } catch (e) {
      if (mounted) toast(context, errText(e), error: true);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _update(Map<String, dynamic> p, String field, num value) async {
    final key = field == 'base_price' ? 'base_price_tiyin' : 'quantity';
    final old = p[key];
    setState(() => p[key] = value);
    try {
      await api.shopUpdateField(p['barcode'] as String, field, value);
      if (mounted && field == 'quantity') toast(context, 'Miqdor yangilandi');
    } catch (e) {
      if (!mounted) return;
      setState(() => p[key] = old);
      toast(context, errText(e), error: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    double q1(Map<String, dynamic> x) => numOf(x['quantity']) == 0 ? 1 : numOf(x['quantity']);
    final chiqim = _sales.fold<double>(0, (s, x) => s + intOf(x['base_price_tiyin']) * q1(x));
    final kirim = _sales.fold<double>(0, (s, x) => s + intOf(x['price_tiyin']) * q1(x));
    final daromad = kirim - chiqim;
    final qty = _sales.fold<double>(0, (s, x) => s + q1(x));
    final q = _q.trim().toLowerCase();
    final shown = _products.where((p) {
      if (q.isNotEmpty &&
          !(p['barcode'] as String).toLowerCase().contains(q) &&
          !((p['name'] as String?) ?? '').toLowerCase().contains(q)) {
        return false;
      }
      return _cat.isEmpty || p['category'] == _cat;
    }).toList();
    final narrow = MediaQuery.sizeOf(context).width < 760;

    return ShopPage(children: [
      StatGrid(tiles: [
        StatTile(label: 'Umumiy kirim', value: somFromTiyin(kirim), icon: Icons.trending_up_rounded, color: OnDexColors.success),
        StatTile(label: 'Umumiy chiqim', value: somFromTiyin(chiqim), icon: Icons.trending_down_rounded, color: OnDexColors.danger),
        StatTile(
            label: 'Daromad',
            value: somFromTiyin(daromad),
            icon: Icons.attach_money_rounded,
            color: daromad >= 0 ? OnDexColors.success : OnDexColors.danger),
        StatTile(label: 'Sotilgan miqdor', value: fmtQty(qty), icon: Icons.inventory_2_outlined, color: OnDexColors.amber),
      ]),
      const SizedBox(height: 16),
      Row(children: [
        Expanded(child: SearchBox(onChanged: (v) => setState(() => _q = v))),
        const SizedBox(width: 10),
        OutlinedButton.icon(
          style: OutlinedButton.styleFrom(padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16)),
          onPressed: () async {
            final v = await pickCategory(context, _cats, _cat, withAll: true);
            if (v != null) setState(() => _cat = v);
          },
          icon: const Icon(Icons.filter_list_rounded, size: 18),
          label: Text(_cat.isEmpty ? 'Kategoriya' : _cat, overflow: TextOverflow.ellipsis),
        ),
      ]),
      const SizedBox(height: 16),
      if (shown.isEmpty)
        ShopCard(child: EmptyState(loading: _loading))
      else if (narrow)
        for (final p in shown)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: ShopCard(
              padding: const EdgeInsets.all(12),
              child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                Row(children: [
                  Expanded(
                      child: Text((p['name'] as String?) ?? '—',
                          maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(fontWeight: FontWeight.w700))),
                  Flexible(child: CategoryChip(p['category'] as String? ?? '')),
                ]),
                Text(p['barcode'] as String, style: const TextStyle(fontFamily: 'monospace', fontSize: 12, color: OnDexColors.info)),
                const SizedBox(height: 8),
                Row(children: [
                  const Text('Chiqim narx ', style: TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                  if (widget.ctx.isPharmacy)
                    Text(somFromTiyin(intOf(p['base_price_tiyin'])))
                  else
                    CommitField(
                        initial: '${intOf(p['base_price_tiyin']) ~/ 100}',
                        onCommit: (v) => _update(p, 'base_price', tiyinFromSomText(v))),
                  const Spacer(),
                  const Text('Miqdor ', style: TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                  if (widget.ctx.isPharmacy)
                    Text(fmtQty(numOf(p['quantity'])))
                  else
                    CommitField(
                        width: 80,
                        initial: fmtQty(numOf(p['quantity'])),
                        onCommit: (v) => _update(p, 'quantity', double.tryParse(v) ?? 0)),
                ]),
                const SizedBox(height: 6),
                Text('Umumiy chiqim: ${somFromTiyin(intOf(p['base_price_tiyin']) * q1(p))}',
                    style: const TextStyle(fontWeight: FontWeight.w700, color: OnDexColors.success)),
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
                DataColumn(label: Text('MAHSULOT NOMI')),
                DataColumn(label: Text('CHIQIM NARX')),
                DataColumn(label: Text('MIQDORI')),
                DataColumn(label: Text('UMUMIY CHIQIM')),
                DataColumn(label: Text('KATEGORIYA')),
              ],
              rows: [
                for (var i = 0; i < shown.length; i++)
                  DataRow(cells: [
                    DataCell(Text('${i + 1}', style: const TextStyle(color: OnDexColors.inkDim))),
                    DataCell(Text(shown[i]['barcode'] as String,
                        style: const TextStyle(fontFamily: 'monospace', fontWeight: FontWeight.w600, color: OnDexColors.info))),
                    DataCell(Text((shown[i]['name'] as String?) ?? '—')),
                    DataCell(widget.ctx.isPharmacy
                        ? Text(somFromTiyin(intOf(shown[i]['base_price_tiyin'])))
                        : CommitField(
                        key: ValueKey('base-${shown[i]['barcode']}'),
                        initial: '${intOf(shown[i]['base_price_tiyin']) ~/ 100}',
                        onCommit: (v) => _update(shown[i], 'base_price', tiyinFromSomText(v)))),
                    DataCell(widget.ctx.isPharmacy
                        ? Text(fmtQty(numOf(shown[i]['quantity'])))
                        : CommitField(
                        key: ValueKey('qty-${shown[i]['barcode']}'),
                        width: 90,
                        initial: fmtQty(numOf(shown[i]['quantity'])),
                        onCommit: (v) => _update(shown[i], 'quantity', double.tryParse(v) ?? 0))),
                    DataCell(Text(somFromTiyin(intOf(shown[i]['base_price_tiyin']) * q1(shown[i])),
                        style: const TextStyle(fontWeight: FontWeight.w800, color: OnDexColors.success))),
                    DataCell(CategoryChip(shown[i]['category'] as String? ?? '')),
                  ]),
              ],
            ),
          ),
        ),
    ]);
  }
}
