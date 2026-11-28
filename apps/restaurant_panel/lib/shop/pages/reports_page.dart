import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import '../../widgets/charts.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// "Savdo statistikasi" — ShoxPos `Hisobotlar.tsx` (kunlik sotuv
/// yig'indilari, `/merchant/shop/sales`).
///
/// 1:1: Umumiy daromad / Sotilgan mahsulot turi / Sotilgan miqdor (davr
/// filtri: Barchasi, Bugun, Hafta, Oy); grafiklar — barcha sotuvlar tarixi
/// (kunlar bo'yicha), oxirgi 7 kun, kategoriyalar bo'yicha, top 10 (dona)
/// va top 10 (summa). Grafiklar ShoxPos'dagidek BUTUN tarix bo'yicha;
/// davr filtri faqat kartalarga ta'sir qiladi. Dorixonada (PharmHisobotlar.tsx)
/// — cheklar qatorlari bo'yicha.
class ReportsPage extends StatefulWidget {
  const ReportsPage({super.key, required this.ctx, this.now});
  final StoreContext ctx;
  final DateTime Function()? now;

  @override
  State<ReportsPage> createState() => _ReportsPageState();
}

enum _P { all, today, week, month }

class _ReportsPageState extends State<ReportsPage> {
  List<Map<String, dynamic>> _sales = [];
  bool _loading = true;
  _P _p = _P.all;

  DateTime _now() => widget.now?.call() ?? DateTime.now();

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final s = widget.ctx.isPharmacy ? await api.pharmSaleLines() : await api.shopSales();
      if (mounted) setState(() => _sales = s);
    } catch (e) {
      if (mounted) toast(context, errText(e), error: true);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  static DateTime? _at(Map<String, dynamic> s) => DateTime.tryParse((s['sold_at'] as String?) ?? '')?.toLocal();
  static DateTime _day(DateTime d) => DateTime(d.year, d.month, d.day);
  static String _two(int v) => v.toString().padLeft(2, '0');

  List<Map<String, dynamic>> get _filtered {
    final now = _now();
    return _sales.where((s) {
      if (_p == _P.all) return true;
      final d = _at(s);
      if (d == null) return false;
      return switch (_p) {
        _P.today => _day(d) == _day(now),
        _P.week => !d.isBefore(now.subtract(const Duration(days: 7))),
        _P.month => d.year == now.year && d.month == now.month,
        _P.all => true,
      };
    }).toList();
  }

  List<ChartPoint> _daily({int? lastDays}) {
    final byDay = <DateTime, int>{};
    for (final s in _sales) {
      final d = _at(s);
      if (d != null) byDay.update(_day(d), (v) => v + intOf(s['total_tiyin']), ifAbsent: () => intOf(s['total_tiyin']));
    }
    final List<DateTime> days;
    if (lastDays != null) {
      final today = _day(_now());
      days = [for (var i = lastDays - 1; i >= 0; i--) today.subtract(Duration(days: i))];
    } else {
      days = byDay.keys.toList()..sort();
    }
    return [
      for (final d in days)
        ChartPoint(
          axisLabel: '${_two(d.day)}.${_two(d.month)}',
          value: (byDay[d] ?? 0) / 100,
          tooltipTitle: '${_two(d.day)}.${_two(d.month)}.${d.year}',
          tooltipValue: somFromTiyin(byDay[d] ?? 0),
        ),
    ];
  }

  /// (nom, qiymat) — kamayish tartibida.
  List<(String, double)> _top({required bool bySum, int n = 10}) {
    final m = <String, (String, double)>{};
    for (final s in _sales) {
      final b = ((s['barcode'] as String?) ?? '').trim();
      final v = bySum ? intOf(s['total_tiyin']).toDouble() : numOf(s['quantity']);
      final cur = m[b];
      m[b] = ((s['name'] as String?) ?? b, (cur?.$2 ?? 0) + v);
    }
    final l = m.values.toList()..sort((a, b) => b.$2.compareTo(a.$2));
    return l.take(n).toList();
  }

  List<(String, double)> _byCategory() {
    final m = <String, double>{};
    for (final s in _sales) {
      final c = ((s['category'] as String?) ?? '').isEmpty ? 'Boshqa' : s['category'] as String;
      m.update(c, (v) => v + intOf(s['total_tiyin']), ifAbsent: () => intOf(s['total_tiyin']).toDouble());
    }
    final l = m.entries.map((e) => (e.key, e.value)).toList()..sort((a, b) => b.$2.compareTo(a.$2));
    return l;
  }

  String _compact(double som) {
    if (som >= 1e9) return '${(som / 1e9).toStringAsFixed(1)} mlrd';
    if (som >= 1e6) return '${(som / 1e6).toStringAsFixed(1)} mln';
    if (som >= 1e3) return '${(som / 1e3).toStringAsFixed(0)} ming';
    return som.toStringAsFixed(0);
  }

  Widget _chartCard(String title, Widget child, {double height = 260}) => ShopCard(
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          Text(title, style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 15)),
          const SizedBox(height: 12),
          SizedBox(height: height, child: child),
        ]),
      );

  Widget _bars(List<(String, double)> rows, Color color, String Function(double) fmt) {
    if (rows.isEmpty) return const Center(child: Text('Ma\'lumot yo\'q', style: TextStyle(color: OnDexColors.inkDim)));
    final max = rows.first.$2 <= 0 ? 1.0 : rows.first.$2;
    final total = rows.fold<double>(0, (s, r) => s + r.$2);
    return ListView(children: [
      for (final r in rows)
        Padding(
          padding: const EdgeInsets.only(bottom: 10),
          child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            Row(children: [
              Expanded(child: Text(r.$1, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(fontSize: 13))),
              Text(fmt(r.$2), style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 13)),
              if (total > 0)
                SizedBox(
                    width: 48,
                    child: Text(' ${(r.$2 * 100 / total).round()}%',
                        textAlign: TextAlign.right, style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim))),
            ]),
            const SizedBox(height: 4),
            ClipRRect(
              borderRadius: BorderRadius.circular(6),
              child: LinearProgressIndicator(
                value: (r.$2 / max).clamp(0, 1),
                minHeight: 8,
                color: color,
                backgroundColor: color.withValues(alpha: .1),
              ),
            ),
          ]),
        ),
    ]);
  }

  @override
  Widget build(BuildContext context) {
    final f = _filtered;
    final income = f.fold<int>(0, (s, x) => s + intOf(x['total_tiyin']));
    final kinds = f.map((x) => x['barcode']).toSet().length;
    final qty = f.fold<double>(0, (s, x) => s + numOf(x['quantity']));
    final wide = MediaQuery.sizeOf(context).width >= 1100;

    if (_loading) return const Center(child: CircularProgressIndicator());

    final all = _daily();
    final line = OnDexLineChart(
      points: all,
      color: OnDexColors.primary,
      axisLabel: _compact,
      unitLabel: 'so\'m',
      emptyText: 'Hali sotuv yo\'q',
    );
    final week = OnDexLineChart(
      points: _daily(lastDays: 7),
      color: OnDexColors.info,
      axisLabel: _compact,
      unitLabel: 'so\'m',
      emptyText: 'Oxirgi 7 kunda sotuv yo\'q',
    );
    final grid = <Widget>[
      _chartCard('Oxirgi 7 kunlik sotuvlar', week),
      _chartCard('Kategoriyalar bo\'yicha sotuvlar',
          _bars(_byCategory(), OnDexColors.teal, (v) => somFromTiyin(v))),
      _chartCard('Top 10 mahsulot (dona)', _bars(_top(bySum: false), OnDexColors.amber, fmtQty)),
      _chartCard('Top 10 mahsulot (summa)', _bars(_top(bySum: true), OnDexColors.success, (v) => somFromTiyin(v))),
    ];

    return ShopPage(children: [
      StatGrid(tiles: [
        StatTile(label: 'Umumiy daromad', value: somFromTiyin(income), icon: Icons.trending_up_rounded, color: OnDexColors.success),
        StatTile(label: 'Sotilgan mahsulot turi', value: '$kinds', icon: Icons.category_outlined, color: OnDexColors.info),
        StatTile(label: 'Sotilgan mahsulot miqdori', value: fmtQty(qty), icon: Icons.inventory_2_outlined, color: OnDexColors.amber),
      ]),
      const SizedBox(height: 14),
      Align(
        alignment: Alignment.centerLeft,
        child: SegmentedButton<_P>(
          key: const ValueKey('reports-period'),
          showSelectedIcon: false,
          segments: const [
            ButtonSegment(value: _P.all, label: Text('Barchasi')),
            ButtonSegment(value: _P.today, label: Text('Bugun')),
            ButtonSegment(value: _P.week, label: Text('Hafta')),
            ButtonSegment(value: _P.month, label: Text('Oy')),
          ],
          selected: {_p},
          onSelectionChanged: (s) => setState(() => _p = s.first),
        ),
      ),
      const SizedBox(height: 16),
      _chartCard(
        'Barcha sotuvlar tarixi',
        LayoutBuilder(
          builder: (context, c) => SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: SizedBox(width: (all.length * 44.0).clamp(c.maxWidth, double.infinity), child: line),
          ),
        ),
        height: 280,
      ),
      const SizedBox(height: 16),
      if (wide)
        for (var i = 0; i < grid.length; i += 2)
          Padding(
            padding: const EdgeInsets.only(bottom: 16),
            child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Expanded(child: grid[i]),
              const SizedBox(width: 16),
              Expanded(child: grid[i + 1]),
            ]),
          )
      else
        for (final g in grid) Padding(padding: const EdgeInsets.only(bottom: 16), child: g),
    ]);
  }
}
