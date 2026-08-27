import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../api.dart';
import '../theme.dart';
// `tableText` — `api.dart` orqali `ondex_core` dan keladi (u yerda
// butun yadro qayta eksport qilinadi, `formatSum` kabi).
import '../widgets/order_card_header.dart' show isDineInOrder;
import '../widgets/page_header.dart';

const _monthNamesShort = [
  'yan',
  'fev',
  'mar',
  'apr',
  'may',
  'iyun',
  'iyul',
  'avg',
  'sen',
  'okt',
  'noy',
  'dek',
];

const _terminalBad = {'rejected', 'cancelled'};
const _activeStatuses = {'accepted', 'preparing', 'ready', 'picked_up'};

/// Bosh sahifa (Dashboard) — image/bosh.png namunasiga 100% mos qilib
/// qayta qurildi. MUHIM (loyihaning doimiy qoidasi): hech qanday raqam
/// qo'lda o'ylab topilmagan — hammasi api.orders()/api.menu() orqali
/// kelgan HAQIQIY ma'lumotdan hisoblanadi. Namunadagi "Restoran reytingi"
/// kartochkasi ATAYLAB QO'SHILMADI (backend'da haqiqiy baholash tizimi
/// yo'q — bu ROADMAP'da alohida qayd etilgan, Kuryer reytingi ham hozircha
/// neytral 5.0) — o'rniga xuddi shu joyga "Tayyorlash vaqti" (haqiqiy,
/// accepted→ready farqidan hisoblangan) qo'yildi, shunday qilib 5 ta
/// mini-kartochka soni saqlanib qoldi, lekin barchasi haqiqiy.
class DashboardPage extends StatefulWidget {
  final DateTime referenceDate;
  final VoidCallback onGoToOrders;
  final VoidCallback onGoToMenu;

  const DashboardPage({
    super.key,
    required this.referenceDate,
    required this.onGoToOrders,
    required this.onGoToMenu,
  });

  @override
  State<DashboardPage> createState() => _DashboardPageState();
}

class _DashboardPageState extends State<DashboardPage> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _orders = [];
  int _chartRangeDays = 7;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final list = await api.orders();
      if (!mounted) return;
      setState(() {
        _orders = list.cast<Map<String, dynamic>>();
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = '$e';
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('Ma\'lumot yuklanmadi: $_error'),
            const SizedBox(height: 12),
            FilledButton(onPressed: _load, child: const Text('Qayta urinish')),
          ],
        ),
      );
    }

    final refDate = widget.referenceDate;
    final stats = _computeDailyStats(_orders, refDate);
    final revenueSeries =
        _computeRevenueSeries(_orders, refDate, _chartRangeDays);
    final activeOrders = _computeActiveOrders(_orders);
    final topProducts = _computeTopProducts(_orders, 5);
    final recentOrders = _recentOrders(_orders, 6);
    final orderCountSeries = _computeCountSeries(_orders, refDate, _chartRangeDays,
        (status) => !_terminalBad.contains(status));
    final cancelledSeries = _computeCountSeries(_orders, refDate, _chartRangeDays,
        (status) => _terminalBad.contains(status));
    final deliveryTime = _computeAvgDuration(_orders, 'picked_up', 'delivered');
    final prepTime = _computeAvgDuration(_orders, 'accepted', 'ready');
    final repeatPct = _computeRepeatCustomerPct(_orders, refDate, _chartRangeDays);
    final cancelledTotal = cancelledSeries.fold<int>(0, (a, b) => a + b.round());

    return RefreshIndicator(
      onRefresh: _load,
      child: SingleChildScrollView(
        padding: kPagePadding,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            // MUHIM: avval Wrap + har kartochkada qattiq width:246 edi —
            // keng ekranda 4 karta yig'indisi konteyner enidan kamroq
            // bo'lib, oxirida foydalanilmagan bo'sh joy qolardi. Endi
            // Row+Expanded — kartalar konteynerning BUTUN enini TENG
            // bo'lib egallaydi, bo'sh joy qolmaydi (5-mini-kartalar
            // qatoridagi bir xil yechim pastda ham qo'llangan).
            // MUHIM: crossAxisAlignment.stretch BU YERDA ATAYLAB
            // QO'YILMAGAN — Row scroll ichida (cheksiz balandlik
            // konteksti) bo'lgani uchun stretch "BoxConstraints forces an
            // infinite height" xatosini beradi (jonli sinovda topilgan).
            // Kartochkalar balandligi endi _StatCard ICHIDA (footer satri
            // doim band qilinib, Opacity bilan) qo'lda bir xillashtirilgan
            // — Row darajasida stretch shart emas.
            Row(
              children: [
                Expanded(
                  child: _StatCard(
                    label: 'Bugungi buyurtmalar',
                    value: '${stats.todayCount}',
                    delta: stats.countDeltaPct,
                    icon: Icons.shopping_bag_rounded,
                    iconColor: OnDexColors.primary,
                    iconBg: OnDexColors.primaryTint,
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: _StatCard(
                    label: 'Bugungi tushum',
                    value: formatSum(stats.todayRevenueTiyin),
                    delta: stats.revenueDeltaPct,
                    icon: Icons.account_balance_wallet_rounded,
                    iconColor: OnDexColors.success,
                    iconBg: OnDexColors.successBg,
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: _StatCard(
                    label: 'O\'rtacha chek',
                    value: stats.avgCheckTiyin == null
                        ? '—'
                        : formatSum(stats.avgCheckTiyin!),
                    icon: Icons.bar_chart_rounded,
                    iconColor: OnDexColors.info,
                    iconBg: OnDexColors.infoBg,
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: _StatCard(
                    label: 'Qabul qilish foizi',
                    value: stats.acceptancePct == null
                        ? '—'
                        : '${stats.acceptancePct!.round()}%',
                    delta: stats.acceptanceDeltaPct,
                    icon: Icons.workspace_premium_rounded,
                    iconColor: OnDexColors.purple,
                    iconBg: OnDexColors.purpleBg,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 20),
            LayoutBuilder(builder: (context, c) {
              final narrow = c.maxWidth < 980;
              final left = Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  _RevenueChartCard(
                    series: revenueSeries,
                    rangeDays: _chartRangeDays,
                    onRangeChanged: (v) => setState(() => _chartRangeDays = v),
                  ),
                  const SizedBox(height: 20),
                  _RecentOrdersTable(orders: recentOrders, onSeeAll: widget.onGoToOrders),
                ],
              );
              final right = Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  _ActiveOrdersCard(orders: activeOrders, onSeeAll: widget.onGoToOrders),
                  const SizedBox(height: 20),
                  _TopProductsCard(products: topProducts),
                ],
              );
              if (narrow) {
                return Column(children: [left, const SizedBox(height: 20), right]);
              }
              return IntrinsicHeight(
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Expanded(flex: 3, child: left),
                    const SizedBox(width: 20),
                    Expanded(flex: 2, child: right),
                  ],
                ),
              );
            }),
            const SizedBox(height: 20),
            // MUHIM: avval Wrap + qattiq width:246 edi — 5 karta bir
            // qatorga sig'may, oxirgisi ("Bekor qilingan buyurtmalar")
            // pastga tushib ketardi. Row+Expanded — 5 tasi HAR DOIM bitta
            // qatorda, konteyner enini teng bo'lib egallaydi (uzun
            // yorliqlar ichki Text'ning maxLines:1+ellipsis orqali
            // kesiladi, kartochka o'lchamiga ta'sir qilmaydi). Stretch
            // ATAYLAB qo'yilmagan (yuqoridagi statistika qatoridagi bilan
            // bir xil sabab — "infinite height" xatosi).
            Row(
              children: [
                Expanded(
                  child: _MiniStatCard(
                    label: 'Buyurtmalar soni',
                    value: '${orderCountSeries.fold<int>(0, (a, b) => a + b.round())}',
                    subLabel: 'So\'nggi $_chartRangeDays kun',
                    icon: Icons.shopping_bag_rounded,
                    color: OnDexColors.success,
                    bg: OnDexColors.successBg,
                    sparkline: orderCountSeries,
                  ),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: _MiniStatCard(
                    label: 'Yetkazib berish vaqti',
                    value: deliveryTime == null ? '—' : '$deliveryTime daqiqa',
                    subLabel: 'O\'rtacha vaqt',
                    icon: Icons.pedal_bike_rounded,
                    color: OnDexColors.info,
                    bg: OnDexColors.infoBg,
                  ),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: _MiniStatCard(
                    label: 'Tayyorlash vaqti',
                    value: prepTime == null ? '—' : '$prepTime daqiqa',
                    subLabel: 'O\'rtacha vaqt',
                    icon: Icons.soup_kitchen_rounded,
                    color: OnDexColors.amber,
                    bg: OnDexColors.amberBg,
                  ),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: _MiniStatCard(
                    label: 'Qayta buyurtma',
                    value: repeatPct == null ? '—' : '${repeatPct.round()}%',
                    subLabel: 'Qaytgan mijozlar',
                    icon: Icons.repeat_rounded,
                    color: OnDexColors.purple,
                    bg: OnDexColors.purpleBg,
                  ),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: _MiniStatCard(
                    label: 'Bekor qilingan buyurtmalar',
                    value: '$cancelledTotal',
                    subLabel: 'So\'nggi $_chartRangeDays kun',
                    icon: Icons.cancel_rounded,
                    color: OnDexColors.danger,
                    bg: OnDexColors.dangerBg,
                    sparkline: cancelledSeries,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Hisob-kitob (barchasi haqiqiy buyurtma ma'lumotidan, soxta raqam yo'q)
// ---------------------------------------------------------------------------

class _DailyStats {
  final int todayCount;
  final int todayRevenueTiyin;
  final int? avgCheckTiyin;
  final double? acceptancePct;
  final int rejectedToday;
  final double? countDeltaPct;
  final double? revenueDeltaPct;
  final double? acceptanceDeltaPct;
  _DailyStats({
    required this.todayCount,
    required this.todayRevenueTiyin,
    required this.avgCheckTiyin,
    required this.acceptancePct,
    required this.rejectedToday,
    required this.countDeltaPct,
    required this.revenueDeltaPct,
    required this.acceptanceDeltaPct,
  });
}

bool _isSameDay(DateTime a, DateTime b) =>
    a.year == b.year && a.month == b.month && a.day == b.day;

DateTime? _parseAt(dynamic iso) =>
    iso is String ? DateTime.tryParse(iso)?.toLocal() : null;

_DailyStats _computeDailyStats(
    List<Map<String, dynamic>> orders, DateTime refDate) {
  final prevDate = refDate.subtract(const Duration(days: 1));
  var todayCount = 0;
  var todayRevenue = 0;
  var todayGoodCount = 0;
  var rejectedToday = 0;
  var prevCount = 0;
  var prevRevenue = 0;
  var prevRejected = 0;

  for (final o in orders) {
    final createdAt = _parseAt(o['created_at']);
    if (createdAt == null) continue;
    final status = o['status'] as String? ?? '';
    final total = (o['total_tiyin'] ?? 0) as int;

    if (_isSameDay(createdAt, refDate)) {
      todayCount++;
      if (status == 'rejected') rejectedToday++;
      if (!_terminalBad.contains(status)) {
        todayRevenue += total;
        todayGoodCount++;
      }
    } else if (_isSameDay(createdAt, prevDate)) {
      prevCount++;
      if (status == 'rejected') prevRejected++;
      if (!_terminalBad.contains(status)) prevRevenue += total;
    }
  }

  double? pctDelta(num today, num prev) {
    if (prev == 0) return null;
    return ((today - prev) / prev) * 100;
  }

  final acceptancePct =
      todayCount > 0 ? ((todayCount - rejectedToday) / todayCount) * 100 : null;
  final prevAcceptancePct =
      prevCount > 0 ? ((prevCount - prevRejected) / prevCount) * 100 : null;

  return _DailyStats(
    todayCount: todayCount,
    todayRevenueTiyin: todayRevenue,
    avgCheckTiyin: todayGoodCount > 0 ? todayRevenue ~/ todayGoodCount : null,
    acceptancePct: acceptancePct,
    rejectedToday: rejectedToday,
    countDeltaPct: pctDelta(todayCount, prevCount),
    revenueDeltaPct: pctDelta(todayRevenue, prevRevenue),
    acceptanceDeltaPct: (acceptancePct != null && prevAcceptancePct != null)
        ? acceptancePct - prevAcceptancePct
        : null,
  );
}

class _DayPoint {
  final DateTime day;
  final int revenueTiyin;
  final bool isRef;
  _DayPoint(this.day, this.revenueTiyin, this.isRef);
}

List<_DayPoint> _computeRevenueSeries(
    List<Map<String, dynamic>> orders, DateTime refDate, int days) {
  final dayList = List.generate(days,
      (i) => DateTime(refDate.year, refDate.month, refDate.day)
          .subtract(Duration(days: days - 1 - i)));
  final totals = {for (final d in dayList) d: 0};
  for (final o in orders) {
    final createdAt = _parseAt(o['created_at']);
    if (createdAt == null) continue;
    final status = o['status'] as String? ?? '';
    if (_terminalBad.contains(status)) continue;
    final key = DateTime(createdAt.year, createdAt.month, createdAt.day);
    if (totals.containsKey(key)) {
      totals[key] = totals[key]! + (o['total_tiyin'] ?? 0) as int;
    }
  }
  return dayList
      .map((d) => _DayPoint(d, totals[d] ?? 0, _isSameDay(d, refDate)))
      .toList();
}

/// Kunlik son-hisoblagich (buyurtmalar soni / bekor qilinganlar) — sparkline
/// va bottom mini-kartochkalar uchun.
List<double> _computeCountSeries(List<Map<String, dynamic>> orders,
    DateTime refDate, int days, bool Function(String status) include) {
  final dayList = List.generate(days,
      (i) => DateTime(refDate.year, refDate.month, refDate.day)
          .subtract(Duration(days: days - 1 - i)));
  final counts = {for (final d in dayList) d: 0};
  for (final o in orders) {
    final createdAt = _parseAt(o['created_at']);
    if (createdAt == null) continue;
    final status = o['status'] as String? ?? '';
    if (!include(status)) continue;
    final key = DateTime(createdAt.year, createdAt.month, createdAt.day);
    if (counts.containsKey(key)) counts[key] = counts[key]! + 1;
  }
  return dayList.map((d) => (counts[d] ?? 0).toDouble()).toList();
}

List<Map<String, dynamic>> _computeActiveOrders(
    List<Map<String, dynamic>> orders) {
  final active = orders
      .where((o) => _activeStatuses.contains(o['status'] as String? ?? ''))
      .toList();
  active.sort((a, b) {
    final da = _parseAt(a['updated_at']) ?? _parseAt(a['created_at']) ?? DateTime(0);
    final db = _parseAt(b['updated_at']) ?? _parseAt(b['created_at']) ?? DateTime(0);
    return db.compareTo(da);
  });
  return active.take(6).toList();
}

class _ProductAgg {
  final String name;
  final String imageUrl;
  int qty = 0;
  int revenueTiyin = 0;
  _ProductAgg(this.name, this.imageUrl);
}

List<_ProductAgg> _computeTopProducts(
    List<Map<String, dynamic>> orders, int limit) {
  final byKey = <String, _ProductAgg>{};
  for (final o in orders) {
    final status = o['status'] as String? ?? '';
    if (status == 'rejected' || status == 'cancelled') continue;
    final items = (o['items'] as List?) ?? [];
    for (final raw in items) {
      if (raw is! Map) continue;
      final item = raw.cast<String, dynamic>();
      final key = (item['product_id'] as String?) ?? (item['name'] as String? ?? '');
      if (key.isEmpty) continue;
      final agg = byKey.putIfAbsent(
          key,
          () => _ProductAgg(
              item['name'] as String? ?? '?', item['image_url'] as String? ?? ''));
      agg.qty += (item['qty'] ?? 0) as int;
      agg.revenueTiyin += ((item['price_tiyin'] ?? 0) as int) * ((item['qty'] ?? 0) as int);
    }
  }
  final list = byKey.values.toList()..sort((a, b) => b.qty.compareTo(a.qty));
  return list.take(limit).toList();
}

List<Map<String, dynamic>> _recentOrders(
    List<Map<String, dynamic>> orders, int n) {
  final sorted = [...orders];
  sorted.sort((a, b) {
    final da = _parseAt(a['created_at']) ?? DateTime(0);
    final db = _parseAt(b['created_at']) ?? DateTime(0);
    return db.compareTo(da);
  });
  return sorted.take(n).toList();
}

/// O'rtacha davomiylik (daqiqada) ikkita status o'tishi orasida — masalan
/// accepted→ready (tayyorlash vaqti) yoki picked_up→delivered (yetkazish
/// vaqti). Ikkalasi ham `history` massividagi HAQIQIY vaqt belgilaridan
/// hisoblanadi, hech narsa taxmin qilinmaydi. Yetarli ma'lumot yo'q bo'lsa
/// (hali birorta ham to'liq siklga ega buyurtma bo'lmasa) — null.
int? _computeAvgDuration(
    List<Map<String, dynamic>> orders, String fromStatus, String toStatus) {
  final minutes = <int>[];
  for (final o in orders) {
    final history = (o['history'] as List?) ?? [];
    DateTime? fromAt;
    DateTime? toAt;
    for (final h in history) {
      if (h is! Map) continue;
      if (h['to'] == fromStatus) fromAt = _parseAt(h['at']);
      if (h['to'] == toStatus) toAt = _parseAt(h['at']);
    }
    if (fromAt != null && toAt != null) {
      final m = toAt.difference(fromAt).inMinutes;
      if (m >= 0 && m < 180) minutes.add(m);
    }
  }
  if (minutes.isEmpty) return null;
  return (minutes.reduce((a, b) => a + b) / minutes.length).round();
}

/// Qaytgan mijozlar foizi: tanlangan davrda buyurtma bergan mijozlar orasida
/// nechtasi (fetch qilingan buyurtmalar tarixi bo'yicha) BIRDAN ORTIQ
/// buyurtma bergani. Butun tarix emas — faqat serverdan olingan so'nggi
/// buyurtmalar ro'yxati (100 tagacha) asosida, shuning uchun aniqlik shu
/// hajm bilan chegaralangan (soxta emas, faqat mavjud ma'lumot ko'lami).
double? _computeRepeatCustomerPct(
    List<Map<String, dynamic>> orders, DateTime refDate, int days) {
  final start = DateTime(refDate.year, refDate.month, refDate.day)
      .subtract(Duration(days: days - 1));
  final ordersPerCustomer = <String, int>{};
  for (final o in orders) {
    final cust = o['customer_id'] as String? ?? '';
    if (cust.isEmpty) continue;
    ordersPerCustomer[cust] = (ordersPerCustomer[cust] ?? 0) + 1;
  }
  final periodCustomers = <String>{};
  for (final o in orders) {
    final createdAt = _parseAt(o['created_at']);
    if (createdAt == null || createdAt.isBefore(start)) continue;
    final cust = o['customer_id'] as String? ?? '';
    if (cust.isNotEmpty) periodCustomers.add(cust);
  }
  if (periodCustomers.isEmpty) return null;
  final repeat =
      periodCustomers.where((c) => (ordersPerCustomer[c] ?? 0) > 1).length;
  return (repeat / periodCustomers.length) * 100;
}

String _itemsSummary(Map<String, dynamic> o) {
  final items = (o['items'] as List?) ?? [];
  return items.map((i) => '${i['qty']}x ${i['name']}').join(', ');
}

String _shortOrderNumber(Map<String, dynamic> o) =>
    '#${(o['order_number'] ?? '').toString().split('-').last}';

// ---------------------------------------------------------------------------
// Vidjetlar
// ---------------------------------------------------------------------------

class _StatCard extends StatelessWidget {
  final String label;
  final String value;
  final double? delta;
  final IconData icon;
  final Color iconColor;
  final Color iconBg;
  const _StatCard({
    required this.label,
    required this.value,
    required this.icon,
    required this.iconColor,
    required this.iconBg,
    this.delta,
  });

  @override
  Widget build(BuildContext context) {
    // Delta bo'lmagan kartochka (masalan "O'rtacha chek") ham xuddi
    // deltali kartochkalar bilan BIR XIL balandlikda bo'lishi uchun —
    // footer satri doim band qilinadi, faqat ko'rinmas (Opacity 0) holda.
    // Aks holda Row+Expanded bilan barobar cho'zilgan kartochkalar orasida
    // ichki balandlik farqi "qandaydir bo'sh joy qolgandek" tuyular edi.
    final up = (delta ?? 0) >= 0;
    final deltaColor = up ? OnDexColors.success : OnDexColors.danger;
    final footer = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(up ? Icons.arrow_upward : Icons.arrow_downward, size: 12, color: deltaColor),
        const SizedBox(width: 3),
        Text(
          delta == null ? '—' : '${delta!.abs().round()}% kechagiga nisbatan',
          style: TextStyle(fontSize: 11.5, color: deltaColor, fontWeight: FontWeight.w700),
        ),
      ],
    );

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Text(label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 12.5, color: OnDexColors.inkDim)),
              ),
              Container(
                width: 30,
                height: 30,
                decoration: BoxDecoration(color: iconBg, shape: BoxShape.circle),
                child: Icon(icon, size: 15, color: iconColor),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(value,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                  fontSize: 22, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
          const SizedBox(height: 6),
          Opacity(opacity: delta == null ? 0 : 1, child: footer),
        ],
      ),
    );
  }
}

class _RevenueChartCard extends StatelessWidget {
  final List<_DayPoint> series;
  final int rangeDays;
  final ValueChanged<int> onRangeChanged;
  const _RevenueChartCard(
      {required this.series, required this.rangeDays, required this.onRangeChanged});

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
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text('Tushum statistikasi',
                  style: TextStyle(
                      fontSize: 15.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
              const SizedBox(width: 6),
              const Icon(Icons.info_outline_rounded, size: 15, color: OnDexColors.inkFaint),
              const Spacer(),
              _RangeDropdown(value: rangeDays, onChanged: onRangeChanged),
            ],
          ),
          const SizedBox(height: 12),
          SizedBox(
            height: 260,
            child: _RevenueChart(series: series),
          ),
        ],
      ),
    );
  }
}

class _RangeDropdown extends StatelessWidget {
  final int value;
  final ValueChanged<int> onChanged;
  const _RangeDropdown({required this.value, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        border: Border.all(color: OnDexColors.cardBorder),
        borderRadius: BorderRadius.circular(8),
      ),
      child: DropdownButtonHideUnderline(
        child: DropdownButton<int>(
          value: value,
          isDense: true,
          icon: const Icon(Icons.keyboard_arrow_down_rounded, size: 18),
          style: const TextStyle(
              fontSize: 12.5, color: OnDexColors.ink, fontWeight: FontWeight.w600),
          items: const [
            DropdownMenuItem(value: 7, child: Text('7 kun')),
            DropdownMenuItem(value: 30, child: Text('30 kun')),
          ],
          onChanged: (v) {
            if (v != null) onChanged(v);
          },
        ),
      ),
    );
  }
}

/// Haftalik/oylik tushum chizig'i — namunadagi kabi silliqlangan
/// egri chiziq + gradient bilan to'ldirilgan maydon + har nuqtada nuqta,
/// oxirgi (tanlangan sana) nuqtasi katta va apelsin rangda ta'kidlangan.
/// Piksel darajasida qo'lda CustomPainter bilan chizilgan — tashqi
/// grafik kutubxonasi shart emas. Sichqoncha nuqta ustiga kelganda
/// (MouseRegion.onHover) shu kundagi aniq summa tooltip sifatida
/// ko'rsatiladi — shuning uchun StatefulWidget (hover indeksi holati).
class _RevenueChart extends StatefulWidget {
  final List<_DayPoint> series;
  const _RevenueChart({required this.series});

  @override
  State<_RevenueChart> createState() => _RevenueChartState();
}

class _RevenueChartState extends State<_RevenueChart> {
  int? _hoverIndex;

  @override
  Widget build(BuildContext context) {
    final series = widget.series;
    if (series.every((d) => d.revenueTiyin == 0)) {
      return const Center(
        child: Text('Hali tushum ma\'lumoti yo\'q',
            style: TextStyle(color: OnDexColors.inkDim)),
      );
    }
    return LayoutBuilder(builder: (context, constraints) {
      final size = Size(constraints.maxWidth, constraints.maxHeight);
      return MouseRegion(
        onHover: (event) {
          final index = _RevenueChartPainter.indexForDx(
              event.localPosition.dx, size.width, series.length);
          if (index != _hoverIndex) setState(() => _hoverIndex = index);
        },
        onExit: (_) {
          if (_hoverIndex != null) setState(() => _hoverIndex = null);
        },
        child: CustomPaint(
          size: size,
          painter: _RevenueChartPainter(series: series, hoverIndex: _hoverIndex),
        ),
      );
    });
  }
}

double _niceStep(double roughStep) {
  if (roughStep <= 0) return 1;
  final magnitude = math.pow(10, (math.log(roughStep) / math.ln10).floor()).toDouble();
  final residual = roughStep / magnitude;
  double niceResidual;
  if (residual > 5) {
    niceResidual = 10;
  } else if (residual > 2) {
    niceResidual = 5;
  } else if (residual > 1) {
    niceResidual = 2;
  } else {
    niceResidual = 1;
  }
  return niceResidual * magnitude;
}

class _RevenueChartPainter extends CustomPainter {
  final List<_DayPoint> series;
  final int? hoverIndex;
  _RevenueChartPainter({required this.series, this.hoverIndex});

  static const _leftAxisWidth = 54.0;
  static const _bottomAxisHeight = 24.0;
  static const _gridLines = 5;

  /// Sichqoncha kursorining lokal x-koordinatasidan ENG YAQIN kunlik
  /// nuqta indeksini topadi — paint()dagi `dx = chartRect.width /
  /// (series.length-1)` bilan BIR XIL formuladan foydalanadi (geometriya
  /// ikki joyda mos kelmasligining oldini olish uchun shu yerda,
  /// widget'ning `onHover'idan chaqiriladi).
  static int? indexForDx(double localDx, double totalWidth, int seriesLength) {
    if (seriesLength < 2) return null;
    const chartLeft = _leftAxisWidth;
    final chartWidth = totalWidth - _leftAxisWidth;
    if (chartWidth <= 0) return null;
    final dx = chartWidth / (seriesLength - 1);
    final raw = (localDx - chartLeft) / dx;
    if (raw < -0.5 || raw > seriesLength - 0.5) return null; // grafikdan tashqarida
    return raw.round().clamp(0, seriesLength - 1);
  }

  @override
  void paint(Canvas canvas, Size size) {
    final maxRaw = series.map((d) => d.revenueTiyin / 100).fold<double>(0, math.max);
    final step = _niceStep(maxRaw / _gridLines == 0 ? 1 : maxRaw / _gridLines);
    final niceMax = step * _gridLines;

    final chartRect = Rect.fromLTWH(
      _leftAxisWidth,
      0,
      size.width - _leftAxisWidth,
      size.height - _bottomAxisHeight,
    );

    final gridPaint = Paint()
      ..color = OnDexColors.cardBorder
      ..strokeWidth = 1;
    const labelStyle = TextStyle(
        fontSize: 10.5, color: OnDexColors.inkFaint, fontFamily: 'Roboto');

    for (var i = 0; i <= _gridLines; i++) {
      final y = chartRect.bottom - chartRect.height * (i / _gridLines);
      canvas.drawLine(Offset(chartRect.left, y), Offset(chartRect.right, y), gridPaint);
      final labelVal = step * i;
      final tp = TextPainter(
        text: TextSpan(text: _axisLabel(labelVal), style: labelStyle),
        textDirection: TextDirection.ltr,
      )..layout();
      tp.paint(canvas, Offset(_leftAxisWidth - 10 - tp.width, y - tp.height / 2));
    }

    if (series.length < 2 || niceMax <= 0) return;

    final dx = chartRect.width / (series.length - 1);
    Offset pointAt(int i) {
      final val = series[i].revenueTiyin / 100;
      final y = chartRect.bottom - chartRect.height * (val / niceMax).clamp(0, 1);
      return Offset(chartRect.left + dx * i, y);
    }

    final points = [for (var i = 0; i < series.length; i++) pointAt(i)];
    final linePath = _smoothPath(points);

    final areaPath = Path.from(linePath)
      ..lineTo(points.last.dx, chartRect.bottom)
      ..lineTo(points.first.dx, chartRect.bottom)
      ..close();
    final areaPaint = Paint()
      ..shader = LinearGradient(
        begin: Alignment.topCenter,
        end: Alignment.bottomCenter,
        colors: [
          OnDexColors.primary.withValues(alpha: 0.28),
          OnDexColors.primary.withValues(alpha: 0.0),
        ],
      ).createShader(chartRect);
    canvas.drawPath(areaPath, areaPaint);

    final linePaint = Paint()
      ..color = OnDexColors.primary
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2.6
      ..strokeCap = StrokeCap.round;
    canvas.drawPath(linePath, linePaint);

    for (var i = 0; i < points.length; i++) {
      final isLast = i == points.length - 1;
      final dotPaint = Paint()..color = OnDexColors.primary;
      canvas.drawCircle(points[i], isLast ? 5 : 3.5, Paint()..color = Colors.white);
      canvas.drawCircle(points[i], isLast ? 5 : 3.5, dotPaint
        ..style = PaintingStyle.stroke
        ..strokeWidth = 2);
      if (isLast) {
        canvas.drawCircle(points[i], 3, Paint()..color = OnDexColors.primary);
      }
    }

    // X o'qi kunlar
    final showEvery = series.length > 10 ? (series.length / 7).ceil() : 1;
    for (var i = 0; i < series.length; i++) {
      if (i % showEvery != 0 && i != series.length - 1) continue;
      final d = series[i].day;
      final label = '${d.day}-${_monthNamesShort[d.month - 1]}';
      final isRef = series[i].isRef;
      final tp = TextPainter(
        text: TextSpan(
          text: label,
          style: TextStyle(
            fontSize: 10.5,
            color: isRef ? OnDexColors.primary : OnDexColors.inkFaint,
            fontWeight: isRef ? FontWeight.w700 : FontWeight.w400,
          ),
        ),
        textDirection: TextDirection.ltr,
      )..layout();
      tp.paint(canvas,
          Offset(points[i].dx - tp.width / 2, size.height - _bottomAxisHeight + 6));
    }

    // Hover tooltip — sichqoncha kursori nuqta ustida bo'lganda shu
    // kundagi ANIQ summani ko'rsatadi. Vertikal yo'naltiruvchi chiziq +
    // ta'kidlangan nuqta + sana/summa yozilgan qutича — barchasi ustma-ust
    // chizilganda oxirgi bo'lishi uchun paint()ning ENG OXIRIDA.
    if (hoverIndex != null && hoverIndex! < points.length) {
      final hp = points[hoverIndex!];
      final guidePaint = Paint()
        ..color = OnDexColors.inkFaint.withValues(alpha: 0.35)
        ..strokeWidth = 1;
      canvas.drawLine(Offset(hp.dx, chartRect.top), Offset(hp.dx, chartRect.bottom), guidePaint);

      canvas.drawCircle(hp, 7, Paint()..color = Colors.white);
      canvas.drawCircle(
          hp, 7, Paint()..color = OnDexColors.primary..style = PaintingStyle.stroke..strokeWidth = 2.5);
      canvas.drawCircle(hp, 3, Paint()..color = OnDexColors.primary);

      final d = series[hoverIndex!].day;
      final dateLabel = '${d.day}-${_monthNamesShort[d.month - 1]}';
      final sumLabel = formatSum(series[hoverIndex!].revenueTiyin);

      final dateTp = TextPainter(
        text: const TextSpan(text: '', style: TextStyle()),
        textDirection: TextDirection.ltr,
      );
      dateTp.text = TextSpan(
        text: dateLabel,
        style: const TextStyle(
            fontSize: 10.5, color: OnDexColors.sidebarTextDim, fontWeight: FontWeight.w500),
      );
      dateTp.layout();
      final sumTp = TextPainter(
        text: TextSpan(
          text: sumLabel,
          style: const TextStyle(
              fontSize: 12.5, color: Colors.white, fontWeight: FontWeight.w800),
        ),
        textDirection: TextDirection.ltr,
      )..layout();

      const hPad = 10.0, vPad = 7.0, gap = 2.0;
      final boxWidth = math.max(dateTp.width, sumTp.width) + hPad * 2;
      final boxHeight = dateTp.height + sumTp.height + gap + vPad * 2;

      var boxLeft = hp.dx - boxWidth / 2;
      boxLeft = boxLeft.clamp(chartRect.left, chartRect.right - boxWidth);
      var boxTop = hp.dy - boxHeight - 14;
      if (boxTop < chartRect.top) {
        boxTop = hp.dy + 14; // yuqorida joy yo'q — nuqtadan pastda ko'rsatiladi
      }
      final boxRect = Rect.fromLTWH(boxLeft, boxTop, boxWidth, boxHeight);
      final rrect = RRect.fromRectAndRadius(boxRect, const Radius.circular(8));
      canvas.drawRRect(
        rrect,
        Paint()
          ..color = OnDexColors.sidebarBg
          ..style = PaintingStyle.fill,
      );
      dateTp.paint(canvas, Offset(boxLeft + hPad, boxTop + vPad));
      sumTp.paint(canvas, Offset(boxLeft + hPad, boxTop + vPad + dateTp.height + gap));
    }
  }

  String _axisLabel(double sum) {
    if (sum == 0) return '0';
    if (sum >= 1000000) {
      final m = sum / 1000000;
      return '${m == m.roundToDouble() ? m.toStringAsFixed(0) : m.toStringAsFixed(1)}M';
    }
    if (sum >= 1000) return '${(sum / 1000).toStringAsFixed(0)}k';
    return sum.toStringAsFixed(0);
  }

  Path _smoothPath(List<Offset> points) {
    final path = Path()..moveTo(points.first.dx, points.first.dy);
    if (points.length == 2) {
      path.lineTo(points[1].dx, points[1].dy);
      return path;
    }
    for (var i = 0; i < points.length - 1; i++) {
      final p0 = points[i == 0 ? 0 : i - 1];
      final p1 = points[i];
      final p2 = points[i + 1];
      final p3 = points[i + 2 < points.length ? i + 2 : i + 1];
      final cp1 = Offset(p1.dx + (p2.dx - p0.dx) / 6, p1.dy + (p2.dy - p0.dy) / 6);
      final cp2 = Offset(p2.dx - (p3.dx - p1.dx) / 6, p2.dy - (p3.dy - p1.dy) / 6);
      path.cubicTo(cp1.dx, cp1.dy, cp2.dx, cp2.dy, p2.dx, p2.dy);
    }
    return path;
  }

  @override
  bool shouldRepaint(covariant _RevenueChartPainter oldDelegate) =>
      oldDelegate.series != series || oldDelegate.hoverIndex != hoverIndex;
}

class _RecentOrdersTable extends StatelessWidget {
  final List<Map<String, dynamic>> orders;
  final VoidCallback onSeeAll;
  const _RecentOrdersTable({required this.orders, required this.onSeeAll});

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
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text('So\'nggi buyurtmalar',
                  style: TextStyle(
                      fontSize: 15.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
              const Spacer(),
              _SeeAllLink(onTap: onSeeAll),
            ],
          ),
          const SizedBox(height: 14),
          if (orders.isEmpty)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 24),
              child: Text('Hali buyurtma yo\'q',
                  style: TextStyle(color: OnDexColors.inkDim)),
            )
          else ...[
            const _TableHeaderRow(),
            const Divider(height: 18, color: OnDexColors.cardBorder),
            for (final o in orders) _TableRow(order: o),
          ],
        ],
      ),
    );
  }
}

class _TableHeaderRow extends StatelessWidget {
  const _TableHeaderRow();
  static const _style = TextStyle(
      fontSize: 11.5, color: OnDexColors.inkFaint, fontWeight: FontWeight.w700);

  @override
  Widget build(BuildContext context) {
    return const Row(
      children: [
        SizedBox(width: 90, child: Text('ID', style: _style)),
        SizedBox(width: 48, child: Text('Vaqt', style: _style)),
        SizedBox(width: 108, child: Text('Mijoz', style: _style)),
        Expanded(child: Text('Mahsulotlar', style: _style)),
        SizedBox(width: 90, child: Text('Summa', style: _style, textAlign: TextAlign.right)),
        SizedBox(width: 100, child: Text('Holat', style: _style, textAlign: TextAlign.right)),
      ],
    );
  }
}

class _TableRow extends StatelessWidget {
  final Map<String, dynamic> order;
  const _TableRow({required this.order});

  @override
  Widget build(BuildContext context) {
    final createdAt = _parseAt(order['created_at']);
    final timeStr = createdAt == null
        ? ''
        : '${createdAt.hour.toString().padLeft(2, '0')}:${createdAt.minute.toString().padLeft(2, '0')}';
    final phone = order['customer_phone'] as String? ?? '';
    final (label, color, bg) = orderStatusStyle(order['status'] as String? ?? '');
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 9),
      child: Row(
        children: [
          SizedBox(
              width: 90,
              child: Text(_shortOrderNumber(order),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 12.5, color: OnDexColors.ink))),
          SizedBox(
              width: 48,
              child: Text(timeStr,
                  style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkFaint))),
          SizedBox(
              width: 108,
              child: Text(phone.isEmpty ? '—' : phone,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim))),
          Expanded(
            child: Text(_itemsSummary(order),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 12.5, color: OnDexColors.ink)),
          ),
          SizedBox(
            width: 90,
            child: Text(
              formatSum((order['total_tiyin'] ?? 0) as int),
              textAlign: TextAlign.right,
              style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 12.5, color: OnDexColors.ink),
            ),
          ),
          SizedBox(
            width: 100,
            child: Align(
              alignment: Alignment.centerRight,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
                decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(999)),
                child: Text(label,
                    style: TextStyle(fontSize: 11, color: color, fontWeight: FontWeight.w700)),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SeeAllLink extends StatelessWidget {
  final VoidCallback onTap;
  const _SeeAllLink({required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: const Text('Barchasini ko\'rish →',
          style: TextStyle(
              fontSize: 12.5, color: OnDexColors.primary, fontWeight: FontWeight.w700)),
    );
  }
}

class _ActiveOrdersCard extends StatelessWidget {
  final List<Map<String, dynamic>> orders;
  final VoidCallback onSeeAll;
  const _ActiveOrdersCard({required this.orders, required this.onSeeAll});

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
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text('Faol buyurtmalar',
                  style: TextStyle(
                      fontSize: 15.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
              const SizedBox(width: 8),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                decoration: BoxDecoration(
                    color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(999)),
                child: Text('${orders.length}',
                    style: const TextStyle(
                        fontSize: 11.5,
                        color: OnDexColors.primaryPressed,
                        fontWeight: FontWeight.w800)),
              ),
              const Spacer(),
              _SeeAllLink(onTap: onSeeAll),
            ],
          ),
          const SizedBox(height: 12),
          if (orders.isEmpty)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 20),
              child: Text('Hozircha faol buyurtma yo\'q',
                  style: TextStyle(color: OnDexColors.inkDim, fontSize: 13)),
            )
          else
            for (final o in orders) _ActiveOrderRow(order: o),
        ],
      ),
    );
  }
}

class _ActiveOrderRow extends StatelessWidget {
  final Map<String, dynamic> order;
  const _ActiveOrderRow({required this.order});

  @override
  Widget build(BuildContext context) {
    final createdAt = _parseAt(order['created_at']);
    final timeStr = createdAt == null
        ? ''
        : '${createdAt.hour.toString().padLeft(2, '0')}:${createdAt.minute.toString().padLeft(2, '0')}';
    final courierName = order['courier_name'] as String? ?? '';
    final hasCourier = (order['courier_id'] as String? ?? '').isNotEmpty;
    // Stol buyurtmasida kuryer yo'q va BO'LMAYDI — backend u uchun
    // dispatch'ni umuman ishga tushirmaydi (`routes_orders.go`:
    // `if !o.IsDineIn()`). Bu tekshiruvsiz qator "Kuryer qidirilmoqda"
    // deb turardi va restoran xodimi tizim ishlamayapti deb o'ylardi.
    final dineIn = isDineInOrder(order);
    final tableLabel = order['table_label'] as String? ?? '';
    final (label, color, bg) = orderStatusStyle(order['status'] as String? ?? '');
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 9),
      child: Row(
        children: [
          SizedBox(
            width: 68,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(_shortOrderNumber(order),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 12, color: OnDexColors.ink, fontWeight: FontWeight.w600)),
                Text(timeStr,
                    style: const TextStyle(fontSize: 11, color: OnDexColors.inkFaint)),
              ],
            ),
          ),
          Icon(
            dineIn
                ? Icons.room_service_rounded
                : (hasCourier
                    ? Icons.pedal_bike_rounded
                    : Icons.hourglass_empty_rounded),
            size: 17,
            color: dineIn
                ? OnDexColors.primary
                : (hasCourier ? OnDexColors.info : OnDexColors.inkFaint),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              dineIn
                  ? '${tableText(tableLabel)} · affitsiant'
                  : (hasCourier
                      ? (courierName.isEmpty ? 'Kuryer yo\'lda' : courierName)
                      : 'Kuryer qidirilmoqda'),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 12.5,
                color: dineIn ? OnDexColors.primary : OnDexColors.ink,
                fontWeight: dineIn ? FontWeight.w600 : FontWeight.normal,
              ),
            ),
          ),
          Text(formatSum((order['total_tiyin'] ?? 0) as int),
              style: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
          const SizedBox(width: 10),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
            decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(999)),
            child: Text(label,
                style: TextStyle(fontSize: 10.5, color: color, fontWeight: FontWeight.w700)),
          ),
        ],
      ),
    );
  }
}

class _TopProductsCard extends StatelessWidget {
  final List<_ProductAgg> products;
  const _TopProductsCard({required this.products});

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
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('Eng ko\'p sotilgan mahsulotlar',
              style: TextStyle(
                  fontSize: 15.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
          const SizedBox(height: 14),
          if (products.isEmpty)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 20),
              child: Text('Hali sotilgan mahsulot yo\'q',
                  style: TextStyle(color: OnDexColors.inkDim, fontSize: 13)),
            )
          else
            for (var i = 0; i < products.length; i++)
              _TopProductRow(rank: i + 1, product: products[i]),
        ],
      ),
    );
  }
}

class _TopProductRow extends StatelessWidget {
  final int rank;
  final _ProductAgg product;
  const _TopProductRow({required this.rank, required this.product});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        children: [
          SizedBox(
              width: 18,
              child: Text('$rank',
                  style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkFaint, fontWeight: FontWeight.w700))),
          ClipRRect(
            borderRadius: BorderRadius.circular(8),
            child: SizedBox(
              width: 34,
              height: 34,
              child: product.imageUrl.isEmpty
                  ? Container(
                      color: OnDexColors.pageBg,
                      child: const Icon(Icons.restaurant, size: 16, color: OnDexColors.inkFaint))
                  : Image.network(
                      fullImageUrl(product.imageUrl),
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) => Container(
                          color: OnDexColors.pageBg,
                          child: const Icon(Icons.restaurant, size: 16, color: OnDexColors.inkFaint)),
                    ),
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text(product.name,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 13, color: OnDexColors.ink)),
          ),
          SizedBox(
              width: 44,
              child: Text('${product.qty} ta',
                  style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim))),
          SizedBox(
            width: 90,
            child: Text(formatSum(product.revenueTiyin),
                textAlign: TextAlign.right,
                style: const TextStyle(
                    fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
          ),
        ],
      ),
    );
  }
}

class _MiniStatCard extends StatelessWidget {
  final String label;
  final String value;
  final String subLabel;
  final IconData icon;
  final Color color;
  final Color bg;
  final List<double>? sparkline;

  const _MiniStatCard({
    required this.label,
    required this.value,
    required this.subLabel,
    required this.icon,
    required this.color,
    required this.bg,
    this.sparkline,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 28,
                height: 28,
                decoration: BoxDecoration(color: bg, shape: BoxShape.circle),
                child: Icon(icon, size: 14, color: color),
              ),
              const SizedBox(width: 9),
              Expanded(
                child: Text(label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(value,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                            fontSize: 18, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                    const SizedBox(height: 2),
                    Text(subLabel,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontSize: 11, color: OnDexColors.inkFaint)),
                  ],
                ),
              ),
              if (sparkline != null && sparkline!.any((v) => v > 0))
                SizedBox(
                  width: 46,
                  height: 28,
                  child: CustomPaint(painter: _SparklinePainter(values: sparkline!, color: color)),
                ),
            ],
          ),
        ],
      ),
    );
  }
}

class _SparklinePainter extends CustomPainter {
  final List<double> values;
  final Color color;
  _SparklinePainter({required this.values, required this.color});

  @override
  void paint(Canvas canvas, Size size) {
    if (values.length < 2) return;
    final maxV = values.fold<double>(0, math.max);
    if (maxV <= 0) return;
    final dx = size.width / (values.length - 1);
    final points = [
      for (var i = 0; i < values.length; i++)
        Offset(dx * i, size.height - size.height * (values[i] / maxV).clamp(0, 1))
    ];
    final path = Path()..moveTo(points.first.dx, points.first.dy);
    for (final p in points.skip(1)) {
      path.lineTo(p.dx, p.dy);
    }
    canvas.drawPath(
      path,
      Paint()
        ..color = color
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1.8
        ..strokeCap = StrokeCap.round
        ..strokeJoin = StrokeJoin.round,
    );
    canvas.drawCircle(points.last, 2, Paint()..color = color);
  }

  @override
  bool shouldRepaint(covariant _SparklinePainter oldDelegate) =>
      oldDelegate.values != values;
}
