import 'package:flutter/material.dart';

import '../api.dart';
import '../live.dart';

/// Statistika — BARCHA restoranlar bo'yicha umumiy ko'rsatkichlar.
///
/// Manba: `GET /admin/stats?period=` — hisob serverda (ombor ichida), shuning
/// uchun raqamlar so'nggi N ta buyurtma bilan cheklanmaydi. Yuqorida jami
/// kartalar, o'rtada kunlik grafik va holatlar, pastda har restoran jadvali.
///
/// Tasniflash restoran panelidagi statistika bilan bir xil:
///   * qabul qilgan = qabul qilinib bekor/rad etilmagan (jarayonda + bajarilgan);
///   * yetkazilgan = yetkazildi + stolga xizmat qilindi (pul tushgan);
///   * tushum = FAQAT yetkazilgan buyurtmalar summasi.
class StatsPage extends StatefulWidget {
  const StatsPage({super.key});

  @override
  State<StatsPage> createState() => _StatsPageState();
}

/// Davr tanlovi: server `period` qiymati va yorlig'i.
const _periods = <({String value, String label})>[
  (value: 'today', label: 'Bugun'),
  (value: '7d', label: '7 kun'),
  (value: '30d', label: '30 kun'),
  (value: 'all', label: 'Hammasi'),
];

int _i(Object? v) => v is num ? v.toInt() : 0;

/// Bitta restoranning (yoki jami) davrdagi yig'indilari.
class _Totals {
  const _Totals({
    this.orders = 0,
    this.newOrders = 0,
    this.inProgress = 0,
    this.accepted = 0,
    this.completed = 0,
    this.cancelled = 0,
    this.revenue = 0,
    this.avgCheck = 0,
    this.completionRate = 0,
  });

  factory _Totals.fromJson(Map<String, dynamic> j) => _Totals(
        orders: _i(j['orders']),
        newOrders: _i(j['new']),
        inProgress: _i(j['in_progress']),
        accepted: _i(j['accepted']),
        completed: _i(j['completed']),
        cancelled: _i(j['cancelled']),
        revenue: _i(j['revenue_tiyin']),
        avgCheck: _i(j['avg_check_tiyin']),
        completionRate: _i(j['completion_rate']),
      );

  final int orders, newOrders, inProgress, accepted, completed, cancelled;
  final int revenue, avgCheck;

  /// Foiz, 0..100: bajarilgan / (bajarilgan + bekor). Yakunlangan buyurtma
  /// bo'lmasa — 0 (jadvalda "—" ko'rsatiladi).
  final int completionRate;

  bool get hasFinished => completed + cancelled > 0;
}

class _RestaurantRow {
  _RestaurantRow(Map<String, dynamic> j)
      : id = '${j['id'] ?? ''}',
        name = '${j['name'] ?? ''}',
        logoUrl = '${j['logo_url'] ?? ''}',
        open = j['open'] == true,
        deleted = j['deleted'] == true,
        totals = _Totals.fromJson(j);

  final String id, name, logoUrl;
  final bool open, deleted;
  final _Totals totals;

  String get title => deleted ? 'O\'chirilgan restoranlar' : name;
}

class _Day {
  _Day(Map<String, dynamic> j)
      : date = '${j['date'] ?? ''}',
        orders = _i(j['orders']),
        completed = _i(j['completed']),
        revenue = _i(j['revenue_tiyin']);

  final String date;
  final int orders, completed, revenue;
}

class _StatsPageState extends State<StatsPage> {
  Map<String, dynamic>? _data;
  String? _error;
  bool _loading = true;
  String _period = '30d';
  late final LiveRefresher _live;

  // Jadval: tartiblash va qidiruv.
  int _sortColumn = _defaultSortColumn; // Tushum
  bool _sortAsc = false;
  String _query = '';

  // Grafik: nimani ko'rsatish.
  bool _chartRevenue = true;

  @override
  void initState() {
    super.initState();
    _load();
    // Ko'rsatkichlar JONLI kanaldan yangilanadi; so'rov sikli esa faqat
    // zaxira (`live.dart` dagi izoh).
    _live = LiveRefresher(
      bus: adminLive,
      onRefresh: _load,
      types: const {
        'new_order',
        'order_status',
        'courier_assigned',
        'courier_registered',
        'courier_status',
      },
    )..start();
  }

  @override
  void dispose() {
    _live.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final period = _period;
    try {
      final s = await api.stats(period: period);
      // Davr o'zgargan bo'lsa bu javob eskirgan.
      if (!mounted || period != _period) return;
      setState(() {
        _data = s;
        _error = null;
        _loading = false;
      });
    } catch (e) {
      if (!mounted || period != _period) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  void _setPeriod(String p) {
    if (p == _period) return;
    setState(() {
      _period = p;
      _loading = true;
    });
    _load();
  }

  // ── Ma'lumotni o'qish ───────────────────────────────────────────────

  _Totals get _totals => _Totals.fromJson(
      Map<String, dynamic>.from((_data?['totals'] as Map?) ?? const {}));

  List<_RestaurantRow> get _rows =>
      ((_data?['restaurants'] as List?) ?? const [])
          .whereType<Map>()
          .map((e) => _RestaurantRow(Map<String, dynamic>.from(e)))
          .toList();

  List<_Day> get _days => ((_data?['daily'] as List?) ?? const [])
      .whereType<Map>()
      .map((e) => _Day(Map<String, dynamic>.from(e)))
      .toList();

  Map<String, int> get _statuses => {
        for (final e in ((_data?['statuses'] as Map?) ?? const {}).entries)
          '${e.key}': _i(e.value),
      };

  String _rangeLabel() {
    final from = '${_data?['from'] ?? ''}';
    final to = '${_data?['to'] ?? ''}';
    if (_period == 'all') {
      return 'Butun davr${to.isEmpty ? '' : ' · $to gacha'}';
    }
    if (from.isEmpty) return '';
    return from == to ? _fmtDate(from) : '${_fmtDate(from)} — ${_fmtDate(to)}';
  }

  // ── Qurilish ────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final body = _body(context);
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            crossAxisAlignment: WrapCrossAlignment.center,
            spacing: 16,
            runSpacing: 8,
            children: [
              Text('Statistika', style: theme.textTheme.headlineMedium),
              SegmentedButton<String>(
                key: const ValueKey('stats-period'),
                showSelectedIcon: false,
                segments: [
                  for (final p in _periods)
                    ButtonSegment(
                      value: p.value,
                      label: Text(p.label,
                          key: ValueKey('stats-period-${p.value}')),
                    ),
                ],
                selected: {_period},
                onSelectionChanged: (s) => _setPeriod(s.first),
              ),
              if (_data != null)
                Text(_rangeLabel(), style: theme.textTheme.bodyMedium),
              IconButton(
                tooltip: 'Yangilash',
                onPressed: _load,
                icon: const Icon(Icons.refresh),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            'Barcha restoranlar bo\'yicha. Yangi buyurtma kelishi bilan avtomatik yangilanadi.',
            style: theme.textTheme.bodySmall,
          ),
          const SizedBox(height: 12),
          if (_loading && _data != null)
            const LinearProgressIndicator(minHeight: 2),
          Expanded(child: body),
        ],
      ),
    );
  }

  Widget _body(BuildContext context) {
    if (_data == null) {
      if (_error != null) {
        return Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.error_outline, color: Colors.red, size: 40),
              const SizedBox(height: 8),
              Text('Statistikani yuklab bo\'lmadi: $_error',
                  textAlign: TextAlign.center),
              const SizedBox(height: 8),
              FilledButton(
                  onPressed: _load, child: const Text('Qayta urinish')),
            ],
          ),
        );
      }
      return const Center(child: CircularProgressIndicator());
    }
    // Eski backend (davr statistikasi qo'shilishidan oldingi) `totals` ni
    // bermaydi. Bunda nollarni "hech narsa yo'q" deb ko'rsatish yolg'on
    // bo'lardi — sababini ochiq aytamiz.
    if (_data!['totals'] is! Map) {
      return Center(
        key: const ValueKey('stats-server-outdated'),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.system_update_alt, size: 40, color: Colors.orange),
            const SizedBox(height: 8),
            const Text(
              'Server hali yangilanmagan: statistika uchun backend\'ning yangi versiyasi kerak.',
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 8),
            FilledButton(
                onPressed: _load, child: const Text('Qayta tekshirish')),
          ],
        ),
      );
    }
    final totals = _totals;
    // Butun sahifa bir marta quriladi (ListView emas): jadval pastda turadi va
    // ListView uni ko'rinmaguncha qurmasdi.
    return SingleChildScrollView(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Text(
                  'Yangilab bo\'lmadi (eski ma\'lumot ko\'rsatilmoqda): $_error',
                  style: const TextStyle(color: Colors.red)),
            ),
          _kpiGrid(totals),
          const SizedBox(height: 24),
          _chartAndStatuses(context),
          const SizedBox(height: 24),
          _restaurantsTable(context, totals),
          const SizedBox(height: 24),
        ],
      ),
    );
  }

  // ── KPI kartalar ────────────────────────────────────────────────────

  Widget _kpiGrid(_Totals t) {
    final d = _data!;
    final cards = <_Kpi>[
      _Kpi(
        key: 'orders',
        title: 'Buyurtmalar',
        value: '${t.orders}',
        note: 'Qabul qilingan: ${t.accepted}',
        icon: Icons.receipt_long,
        color: Colors.blue,
      ),
      _Kpi(
        key: 'revenue',
        title: 'Tushum',
        value: formatSum(t.revenue),
        note: t.completed == 0
            ? 'Yetkazilgan yo\'q'
            : 'O\'rtacha chek: ${formatSum(t.avgCheck)}',
        icon: Icons.payments,
        color: Colors.green,
      ),
      _Kpi(
        key: 'delivered',
        title: 'Yetkazilgan',
        value: '${t.completed}',
        note: t.hasFinished
            ? 'Bajarilish: ${t.completionRate}%'
            : 'Hali yakunlangani yo\'q',
        icon: Icons.done_all,
        color: Colors.teal,
      ),
      _Kpi(
        key: 'cancelled',
        title: 'Bekor / rad etilgan',
        value: '${t.cancelled}',
        note: 'Jarayonda: ${t.inProgress + t.newOrders}',
        icon: Icons.cancel_outlined,
        color: Colors.grey,
      ),
      _Kpi(
        key: 'couriers-online',
        title: 'Kuryerlar online',
        value: '${_i(d['couriers_online'])}',
        note: 'Jami kuryerlar: ${_i(d['couriers_total'])}',
        icon: Icons.delivery_dining,
        color: Colors.orange,
      ),
      _Kpi(
        key: 'couriers-pending',
        title: 'Tasdiq kutayotgan kuryerlar',
        value: '${_i(d['couriers_pending'])}',
        note: _i(d['couriers_pending']) > 0
            ? 'OnDex kuryerlari bo\'limida tasdiqlang'
            : 'Ariza yo\'q',
        icon: Icons.hourglass_top,
        color: Colors.red,
      ),
      _Kpi(
        key: 'restaurants',
        title: 'Restoranlar',
        value: '${_i(d['restaurants_total'])}',
        note: 'Hozir ochiq: ${_i(d['restaurants_open'])}',
        icon: Icons.storefront,
        color: Colors.purple,
      ),
      _Kpi(
        key: 'new',
        title: 'Yangi (ko\'rilmagan)',
        value: '${t.newOrders}',
        note: t.newOrders > 0
            ? 'Restoran hali qabul qilmagan'
            : 'Hammasi ko\'rilgan',
        icon: Icons.fiber_new,
        color: Colors.indigo,
      ),
    ];
    return LayoutBuilder(builder: (context, box) {
      const gap = 16.0;
      final perRow = box.maxWidth >= 1000 ? 4 : (box.maxWidth >= 560 ? 2 : 1);
      final w = (box.maxWidth - gap * (perRow - 1)) / perRow;
      return Wrap(
        spacing: gap,
        runSpacing: gap,
        children: [for (final c in cards) _KpiCard(kpi: c, width: w)],
      );
    });
  }

  // ── Grafik va holatlar ──────────────────────────────────────────────

  Widget _chartAndStatuses(BuildContext context) {
    final chart = _card(
      context,
      title: 'So\'nggi 14 kun',
      trailing: SegmentedButton<bool>(
        key: const ValueKey('stats-chart-metric'),
        showSelectedIcon: false,
        style: const ButtonStyle(visualDensity: VisualDensity.compact),
        segments: const [
          ButtonSegment(value: true, label: Text('Tushum')),
          ButtonSegment(value: false, label: Text('Buyurtmalar')),
        ],
        selected: {_chartRevenue},
        onSelectionChanged: (s) => setState(() => _chartRevenue = s.first),
      ),
      child: SizedBox(
        height: 200,
        child: _DailyChart(days: _days, revenue: _chartRevenue),
      ),
    );
    final statuses = _card(
      context,
      title: 'Buyurtmalar holati',
      child: _StatusBreakdown(counts: _statuses),
    );
    return LayoutBuilder(builder: (context, box) {
      if (box.maxWidth < 900) {
        return Column(children: [chart, const SizedBox(height: 16), statuses]);
      }
      return Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(flex: 3, child: chart),
          const SizedBox(width: 16),
          Expanded(flex: 2, child: statuses),
        ],
      );
    });
  }

  Widget _card(BuildContext context,
      {required String title, Widget? trailing, required Widget child}) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                    child: Text(title, style: theme.textTheme.titleMedium)),
                if (trailing != null) trailing,
              ],
            ),
            const SizedBox(height: 12),
            child,
          ],
        ),
      ),
    );
  }

  // ── Restoranlar jadvali ─────────────────────────────────────────────

  /// Ustun tartibi jadval sarlavhasi bilan AYNAN bir xil.
  ///
  /// Tushum nomdan keyin turadi: tor oynada jadval yon tomonga aylantirilsa
  /// ham eng muhim raqam ko'rinib turadi.
  static const _columns = <String>[
    'Restoran',
    'Tushum',
    'Jami',
    'Qabul qilgan',
    'Yetkazilgan',
    'Bekor / rad',
    'Jarayonda',
    'Yangi',
    'Bajarilish',
    'O\'rtacha chek',
  ];

  /// Boshlang'ich tartib: tushum ustuni.
  static const _defaultSortColumn = 1;

  num _sortValue(_RestaurantRow r, int col) {
    final t = r.totals;
    return switch (col) {
      1 => t.revenue,
      2 => t.orders,
      3 => t.accepted,
      4 => t.completed,
      5 => t.cancelled,
      6 => t.inProgress,
      7 => t.newOrders,
      8 => t.hasFinished ? t.completionRate : -1,
      9 => t.avgCheck,
      _ => 0,
    };
  }

  Widget _restaurantsTable(BuildContext context, _Totals total) {
    final theme = Theme.of(context);
    final q = _query.trim().toLowerCase();
    var rows = _rows
        .where((r) => q.isEmpty || r.title.toLowerCase().contains(q))
        .toList();
    rows.sort((a, b) {
      final primary = _sortColumn == 0
          ? a.title.toLowerCase().compareTo(b.title.toLowerCase())
          : _sortValue(a, _sortColumn).compareTo(_sortValue(b, _sortColumn));
      if (primary != 0) return _sortAsc ? primary : -primary;
      // Teng bo'lsa nom bo'yicha — tartib barqaror.
      return a.title.toLowerCase().compareTo(b.title.toLowerCase());
    });

    DataColumn col(int i) => DataColumn(
          label: Text(_columns[i],
              style: const TextStyle(fontWeight: FontWeight.w700)),
          numeric: i != 0,
          onSort: (idx, asc) => setState(() {
            _sortColumn = idx;
            _sortAsc = asc;
          }),
        );

    String pct(_Totals t) => t.hasFinished ? '${t.completionRate}%' : '—';

    DataRow line(_RestaurantRow r) {
      final t = r.totals;
      return DataRow(
        cells: [
          DataCell(_RestaurantCell(
              key: ValueKey('stats-row-${r.deleted ? 'deleted' : r.id}'),
              row: r)),
          DataCell(Text(formatSum(t.revenue),
              style: const TextStyle(fontWeight: FontWeight.w600))),
          DataCell(Text('${t.orders}')),
          DataCell(Text('${t.accepted}')),
          DataCell(Text('${t.completed}')),
          DataCell(Text('${t.cancelled}')),
          DataCell(Text('${t.inProgress}')),
          DataCell(Text('${t.newOrders}')),
          DataCell(Text(pct(t))),
          DataCell(Text(t.completed == 0 ? '—' : formatSum(t.avgCheck))),
        ],
      );
    }

    // Jami qatori — faqat qidiruv bo'lmaganda: filtrlangan jadvalda "jami"
    // kartalardagi bilan mos kelmay chalkashtirardi.
    final footer = q.isEmpty && rows.isNotEmpty
        ? DataRow(
            color: WidgetStatePropertyAll(
                theme.colorScheme.surfaceContainerHighest),
            cells: [
              const DataCell(Text('JAMI',
                  key: ValueKey('stats-row-total'),
                  style: TextStyle(fontWeight: FontWeight.w800))),
              for (final v in [
                formatSum(total.revenue),
                '${total.orders}',
                '${total.accepted}',
                '${total.completed}',
                '${total.cancelled}',
                '${total.inProgress}',
                '${total.newOrders}',
                pct(total),
                total.completed == 0 ? '—' : formatSum(total.avgCheck),
              ])
                DataCell(Text(v,
                    style: const TextStyle(fontWeight: FontWeight.w800))),
            ],
          )
        : null;

    return _card(
      context,
      title: 'Restoranlar bo\'yicha',
      trailing: SizedBox(
        width: 260,
        child: TextField(
          key: const ValueKey('stats-search'),
          decoration: const InputDecoration(
            isDense: true,
            prefixIcon: Icon(Icons.search),
            hintText: 'Restoran nomi bo\'yicha',
            border: OutlineInputBorder(),
          ),
          onChanged: (v) => setState(() => _query = v),
        ),
      ),
      child: rows.isEmpty
          ? Padding(
              padding: const EdgeInsets.symmetric(vertical: 32),
              child: Center(
                child: Text(
                    _query.isEmpty ? 'Hozircha restoran yo\'q' : 'Topilmadi'),
              ),
            )
          : LayoutBuilder(
              builder: (context, box) => SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                child: ConstrainedBox(
                  constraints: BoxConstraints(minWidth: box.maxWidth),
                  child: DataTable(
                    key: const ValueKey('stats-table'),
                    sortColumnIndex: _sortColumn,
                    sortAscending: _sortAsc,
                    columnSpacing: 14,
                    horizontalMargin: 12,
                    columns: [for (var i = 0; i < _columns.length; i++) col(i)],
                    rows: [
                      for (final r in rows) line(r),
                      if (footer != null) footer
                    ],
                  ),
                ),
              ),
            ),
    );
  }
}

String _fmtDate(String iso) {
  // "2026-09-21" -> "21.09.2026"
  final p = iso.split('-');
  return p.length == 3 ? '${p[2]}.${p[1]}.${p[0]}' : iso;
}

// ── Kichik vidjetlar ────────────────────────────────────────────────────

class _Kpi {
  const _Kpi({
    required this.key,
    required this.title,
    required this.value,
    required this.note,
    required this.icon,
    required this.color,
  });

  final String key, title, value, note;
  final IconData icon;
  final Color color;
}

class _KpiCard extends StatelessWidget {
  const _KpiCard({required this.kpi, required this.width});

  final _Kpi kpi;
  final double width;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return SizedBox(
      width: width,
      child: Card(
        key: ValueKey('stats-kpi-${kpi.key}'),
        margin: EdgeInsets.zero,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(16),
          side: BorderSide(color: theme.colorScheme.outlineVariant),
        ),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Container(
                    padding: const EdgeInsets.all(6),
                    decoration: BoxDecoration(
                      color: kpi.color.withAlpha(30),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Icon(kpi.icon, color: kpi.color, size: 18),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(kpi.title,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: theme.textTheme.bodyMedium),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              // Uzun summa tor kartada kesilmasin — kichraytiriladi.
              FittedBox(
                fit: BoxFit.scaleDown,
                alignment: Alignment.centerLeft,
                child: Text(kpi.value,
                    key: ValueKey('stats-kpi-${kpi.key}-value'),
                    style: theme.textTheme.headlineMedium
                        ?.copyWith(fontWeight: FontWeight.w700)),
              ),
              const SizedBox(height: 4),
              Text(kpi.note,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.bodySmall
                      ?.copyWith(color: theme.colorScheme.onSurfaceVariant)),
            ],
          ),
        ),
      ),
    );
  }
}

class _RestaurantCell extends StatelessWidget {
  const _RestaurantCell({super.key, required this.row});

  final _RestaurantRow row;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    Widget avatar() {
      final fallback = Container(
        color: theme.colorScheme.primaryContainer,
        alignment: Alignment.center,
        child: Icon(row.deleted ? Icons.delete_outline : Icons.storefront,
            size: 16, color: theme.colorScheme.onPrimaryContainer),
      );
      return ClipOval(
        child: SizedBox(
          width: 28,
          height: 28,
          child: row.logoUrl.isEmpty
              ? fallback
              : Image.network(imageUrl(row.logoUrl),
                  fit: BoxFit.cover, errorBuilder: (_, __, ___) => fallback),
        ),
      );
    }

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        avatar(),
        const SizedBox(width: 10),
        Text(row.title,
            style: row.deleted
                ? const TextStyle(
                    fontStyle: FontStyle.italic, color: Colors.grey)
                : const TextStyle(fontWeight: FontWeight.w600)),
        if (!row.deleted) ...[
          const SizedBox(width: 8),
          Tooltip(
            message: row.open ? 'Hozir ochiq' : 'Yopiq',
            child: Icon(Icons.circle,
                size: 9, color: row.open ? Colors.green : Colors.grey),
          ),
        ],
      ],
    );
  }
}

/// So'nggi kunlar grafigi: har kun uchun ustun; ustiga borilsa aniq qiymat.
/// Tashqi kutubxonasiz — 14 ta ustun uchun `Row` yetadi.
class _DailyChart extends StatelessWidget {
  const _DailyChart({required this.days, required this.revenue});

  final List<_Day> days;

  /// true — tushum, false — buyurtmalar soni.
  final bool revenue;

  int _value(_Day d) => revenue ? d.revenue : d.orders;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (days.isEmpty) return const Center(child: Text('Ma\'lumot yo\'q'));
    final max = days.fold<int>(0, (m, d) => _value(d) > m ? _value(d) : m);
    if (max == 0) {
      return Center(
        child: Text('Bu davrda ${revenue ? 'tushum' : 'buyurtma'} yo\'q',
            style: theme.textTheme.bodyMedium
                ?.copyWith(color: theme.colorScheme.onSurfaceVariant)),
      );
    }
    return LayoutBuilder(builder: (context, box) {
      const labelH = 18.0, valueH = 16.0;
      final barArea = box.maxHeight - labelH - valueH;
      final showValues = box.maxWidth / days.length >= 32;
      return Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          for (var i = 0; i < days.length; i++)
            Expanded(
              child: Tooltip(
                message: '${_fmtDate(days[i].date)}\n'
                    'Buyurtmalar: ${days[i].orders}\n'
                    'Yetkazilgan: ${days[i].completed}\n'
                    'Tushum: ${formatSum(days[i].revenue)}',
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    SizedBox(
                      height: valueH,
                      child: showValues && _value(days[i]) > 0
                          ? FittedBox(
                              child: Text(
                                  revenue
                                      ? _compactSum(days[i].revenue)
                                      : '${days[i].orders}',
                                  style: theme.textTheme.labelSmall),
                            )
                          : null,
                    ),
                    Container(
                      key: ValueKey('stats-bar-${days[i].date}'),
                      margin: const EdgeInsets.symmetric(horizontal: 4),
                      height: _value(days[i]) == 0
                          ? 2
                          : (barArea * _value(days[i]) / max)
                              .clamp(3.0, barArea),
                      decoration: BoxDecoration(
                        color: i == days.length - 1
                            ? theme.colorScheme.primary
                            : theme.colorScheme.primary.withAlpha(120),
                        borderRadius: const BorderRadius.vertical(
                            top: Radius.circular(4)),
                      ),
                    ),
                    SizedBox(
                      height: labelH,
                      child: Center(
                        child: Text(_shortDay(days[i].date),
                            style: theme.textTheme.labelSmall),
                      ),
                    ),
                  ],
                ),
              ),
            ),
        ],
      );
    });
  }
}

/// "2026-09-21" -> "21.09"
String _shortDay(String iso) {
  final p = iso.split('-');
  return p.length == 3 ? '${p[2]}.${p[1]}' : iso;
}

/// Grafik ustidagi qisqa summa: tiyin -> "1.2 mln" / "350 ming".
String _compactSum(int tiyin) {
  final som = tiyin ~/ 100;
  if (som >= 1000000) return '${(som / 1000000).toStringAsFixed(1)} mln';
  if (som >= 1000) return '${som ~/ 1000} ming';
  return '$som';
}

/// Holatlar bo'yicha taqsimot: rangli chiziq + ro'yxat. Ranglar buyurtmalar
/// jadvalidagi bilan bir xil.
class _StatusBreakdown extends StatelessWidget {
  const _StatusBreakdown({required this.counts});

  final Map<String, int> counts;

  static const _colors = {
    'created': Colors.blue,
    'accepted': Colors.indigo,
    'preparing': Colors.orange,
    'ready': Colors.amber,
    'picked_up': Colors.teal,
    'delivered': Colors.green,
    'served': Colors.green,
    'cancelled': Colors.grey,
    'rejected': Colors.red,
  };

  /// Buyurtma hayot tsikli tartibida.
  static const _order = [
    'created',
    'accepted',
    'preparing',
    'ready',
    'picked_up',
    'delivered',
    'served',
    'cancelled',
    'rejected',
  ];

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final keys = [
      ..._order.where((k) => (counts[k] ?? 0) > 0),
      // Yangi (noma'lum) holat yashirilmaydi — admin darhol ko'radi.
      ...counts.keys.where((k) => !_order.contains(k) && (counts[k] ?? 0) > 0),
    ];
    final total = keys.fold<int>(0, (s, k) => s + (counts[k] ?? 0));
    if (total == 0) {
      return SizedBox(
        height: 120,
        child: Center(
          child: Text('Bu davrda buyurtma yo\'q',
              style: theme.textTheme.bodyMedium
                  ?.copyWith(color: theme.colorScheme.onSurfaceVariant)),
        ),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(6),
          child: SizedBox(
            height: 12,
            child: Row(
              children: [
                for (final k in keys)
                  Expanded(
                    flex: counts[k]!,
                    child: Container(color: _colors[k] ?? Colors.blueGrey),
                  ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 12),
        for (final k in keys)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 3),
            child: Row(
              key: ValueKey('stats-status-$k'),
              children: [
                Icon(Icons.circle,
                    size: 10, color: _colors[k] ?? Colors.blueGrey),
                const SizedBox(width: 8),
                Expanded(child: Text(orderStatusLabel(k))),
                Text('${counts[k]}',
                    style: const TextStyle(fontWeight: FontWeight.w700)),
                SizedBox(
                  width: 52,
                  child: Text('${(counts[k]! * 100 / total).round()}%',
                      textAlign: TextAlign.right,
                      style: theme.textTheme.bodySmall),
                ),
              ],
            ),
          ),
      ],
    );
  }
}
