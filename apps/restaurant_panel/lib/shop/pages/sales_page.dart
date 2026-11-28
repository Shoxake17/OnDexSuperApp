import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// "Sotuvlar" — ShoxPos `Sotilgan.tsx`: oxirgi 30 kun sotuv qatorlari
/// (`/merchant/shop/sold-lines`, ShoxPos `sold-buckets`).
///
/// 1:1: Umumiy summa / Turli mahsulotlar soni / Sotilgan mahsulotlar;
/// qidiruv (nom yoki shtrix-kod); davr: Barchasi, Kunlik, Haftalik
/// (oxirgi 7 kun), Oylik (joriy oy); yangisi birinchi; kichik ekranda
/// kunlar bo'yicha guruh ("Bugun", "Kecha") va to'lov turi belgisi.
/// Dorixonada (ShoxPos PharmSale.tsx) — oxirgi 100 chek qatorlari.
class SalesPage extends StatefulWidget {
  const SalesPage({super.key, required this.ctx, this.now});
  final StoreContext ctx;

  /// Testlar uchun "hozir".
  final DateTime Function()? now;

  @override
  State<SalesPage> createState() => _SalesPageState();
}

enum _Period { all, today, week, month }

const _periodLabel = {
  _Period.all: 'Barchasi',
  _Period.today: 'Kunlik',
  _Period.week: 'Haftalik',
  _Period.month: 'Oylik',
};

String paymentLabel(String? m) => switch ((m ?? '').toLowerCase()) {
      '' => '—',
      'naqd' || 'cash' => 'Naqd',
      'plastik' || 'card' => 'Plastik',
      'qarz' => 'Qarz',
      final v => v,
    };

Color paymentColor(String? m) => switch ((m ?? '').toLowerCase()) {
      'naqd' || 'cash' => OnDexColors.success,
      'plastik' || 'card' => OnDexColors.info,
      'qarz' => OnDexColors.amber,
      _ => OnDexColors.inkDim,
    };

String _two(int v) => v.toString().padLeft(2, '0');
String fmtDateTime(DateTime d) => '${_two(d.day)}.${_two(d.month)}.${d.year} ${_two(d.hour)}:${_two(d.minute)}';

class _SalesPageState extends State<SalesPage> {
  List<Map<String, dynamic>> _lines = [];
  bool _loading = true;
  String _q = '';
  _Period _period = _Period.all;

  DateTime _now() => widget.now?.call() ?? DateTime.now();

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final l = widget.ctx.isPharmacy ? await api.pharmSaleLines() : await api.shopSoldLines();
      if (mounted) setState(() => _lines = l);
    } catch (e) {
      if (mounted) toast(context, errText(e), error: true);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  static DateTime? _at(Map<String, dynamic> l) => DateTime.tryParse((l['sold_at'] as String?) ?? '')?.toLocal();

  List<Map<String, dynamic>> get _shown {
    final q = _q.trim().toLowerCase();
    final now = _now();
    final out = _lines.where((l) {
      if (q.isNotEmpty &&
          !((l['name'] as String?) ?? '').toLowerCase().contains(q) &&
          !((l['barcode'] as String?) ?? '').contains(q)) {
        return false;
      }
      if (_period == _Period.all) return true;
      final d = _at(l);
      if (d == null) return false;
      return switch (_period) {
        _Period.today => d.year == now.year && d.month == now.month && d.day == now.day,
        _Period.week => !d.isBefore(now.subtract(const Duration(days: 7))),
        _Period.month => d.year == now.year && d.month == now.month,
        _Period.all => true,
      };
    }).toList()
      ..sort((a, b) => (_at(b) ?? DateTime(0)).compareTo(_at(a) ?? DateTime(0)));
    return out;
  }

  String _dayLabel(DateTime d) {
    final now = _now();
    final s = '${_two(d.day)}.${_two(d.month)}.${d.year}';
    bool same(DateTime a, DateTime b) => a.year == b.year && a.month == b.month && a.day == b.day;
    if (same(d, now)) return 'Bugun — $s';
    if (same(d, now.subtract(const Duration(days: 1)))) return 'Kecha — $s';
    return s;
  }

  @override
  Widget build(BuildContext context) {
    final shown = _shown;
    final total = shown.fold<int>(0, (s, l) => s + intOf(l['total_tiyin']));
    final unique = shown.map((l) => l['barcode']).toSet().length;
    final qty = shown.fold<double>(0, (s, l) => s + numOf(l['quantity']));
    final narrow = MediaQuery.sizeOf(context).width < 760;

    return ShopPage(children: [
      StatGrid(tiles: [
        StatTile(label: 'Umumiy summa', value: somFromTiyin(total), icon: Icons.trending_up_rounded, color: OnDexColors.success),
        StatTile(label: 'Turli mahsulotlar soni', value: '$unique', icon: Icons.description_outlined, color: OnDexColors.info),
        StatTile(label: 'Sotilgan mahsulotlar', value: fmtQty(qty), icon: Icons.inventory_2_outlined, color: OnDexColors.amber),
      ]),
      const SizedBox(height: 16),
      Wrap(spacing: 10, runSpacing: 10, crossAxisAlignment: WrapCrossAlignment.center, children: [
        SizedBox(
          width: narrow ? double.infinity : 380,
          child: SearchBox(hint: 'Shtrix-kod va nomi orqali qidiring', onChanged: (v) => setState(() => _q = v)),
        ),
        SegmentedButton<_Period>(
          key: const ValueKey('sales-period'),
          showSelectedIcon: false,
          segments: [for (final p in _Period.values) ButtonSegment(value: p, label: Text(_periodLabel[p]!))],
          selected: {_period},
          onSelectionChanged: (s) => setState(() => _period = s.first),
        ),
      ]),
      const SizedBox(height: 16),
      if (shown.isEmpty)
        ShopCard(
            child: EmptyState(loading: _loading, text: 'Sotuvlar topilmadi — filtrlarni o\'zgartiring'))
      else if (narrow)
        ..._grouped(shown)
      else
        ShopCard(
          padding: EdgeInsets.zero,
          child: SizedBox(
            width: double.infinity,
            child: DataTable(
              headingRowColor: WidgetStateProperty.all(OnDexColors.pageBg),
              headingTextStyle: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.inkDim),
              columns: const [
                DataColumn(label: Text('MAHSULOTLAR')),
                DataColumn(label: Text('SHTRIX-KOD')),
                DataColumn(label: Text('SONI'), numeric: true),
                DataColumn(label: Text('NARXI'), numeric: true),
                DataColumn(label: Text('JAMI'), numeric: true),
                DataColumn(label: Text('TO\'LOV')),
                DataColumn(label: Text('VAQTI')),
              ],
              rows: [
                for (final l in shown)
                  DataRow(cells: [
                    DataCell(Text((l['name'] as String?) ?? '', style: const TextStyle(fontWeight: FontWeight.w600))),
                    DataCell(Text((l['barcode'] as String?) ?? '',
                        style: const TextStyle(fontFamily: 'monospace', fontSize: 13, color: OnDexColors.info))),
                    DataCell(Text(fmtQty(numOf(l['quantity'])), style: const TextStyle(fontWeight: FontWeight.w600))),
                    DataCell(Text(somFromTiyin(intOf(l['price_tiyin'])), style: const TextStyle(color: OnDexColors.inkDim))),
                    DataCell(Text(somFromTiyin(intOf(l['total_tiyin'])),
                        style: const TextStyle(fontWeight: FontWeight.w800, color: OnDexColors.success))),
                    DataCell(_PayBadge(l['payment_method'] as String?)),
                    DataCell(Text(_at(l) == null ? '—' : fmtDateTime(_at(l)!),
                        style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim))),
                  ]),
              ],
            ),
          ),
        ),
    ]);
  }

  List<Widget> _grouped(List<Map<String, dynamic>> shown) {
    final out = <Widget>[];
    String? last;
    for (final l in shown) {
      final d = _at(l);
      final label = d == null ? 'Noma\'lum' : _dayLabel(d);
      if (label != last) {
        out.add(Padding(
          padding: const EdgeInsets.symmetric(vertical: 10),
          child: Row(children: [
            const Expanded(child: Divider()),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 10),
              child: Text(label, style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: OnDexColors.inkDim)),
            ),
            const Expanded(child: Divider()),
          ]),
        ));
        last = label;
      }
      out.add(Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: ShopCard(
          padding: const EdgeInsets.all(12),
          child: Row(children: [
            Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text((l['name'] as String?) ?? '',
                    maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(fontWeight: FontWeight.w700)),
                Text('${fmtQty(numOf(l['quantity']))} × ${somFromTiyin(intOf(l['price_tiyin']))}'
                    '${d == null ? '' : ' · ${_two(d.hour)}:${_two(d.minute)}'}',
                    style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
              ]),
            ),
            Column(crossAxisAlignment: CrossAxisAlignment.end, children: [
              Text(somFromTiyin(intOf(l['total_tiyin'])),
                  style: const TextStyle(fontWeight: FontWeight.w800, color: OnDexColors.success)),
              const SizedBox(height: 4),
              _PayBadge(l['payment_method'] as String?),
            ]),
          ]),
        ),
      ));
    }
    return out;
  }
}

class _PayBadge extends StatelessWidget {
  const _PayBadge(this.method);
  final String? method;

  @override
  Widget build(BuildContext context) {
    final c = paymentColor(method);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 3),
      decoration: BoxDecoration(color: c.withValues(alpha: .12), borderRadius: BorderRadius.circular(20)),
      child: Text(paymentLabel(method), style: TextStyle(fontSize: 11.5, fontWeight: FontWeight.w700, color: c)),
    );
  }
}
