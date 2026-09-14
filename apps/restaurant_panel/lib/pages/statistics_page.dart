import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;
import 'package:printing/printing.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/charts.dart';
import '../widgets/date_range_dialog.dart';
import '../widgets/order_card_header.dart'
    show isDineInOrder, parseOrderAt, shortOrderNumber;

part 'statistics_history.dart';

/// "Statistika" sahifasi — joylashuv image/Statistika1.png namunasi bo'yicha.
///
/// ┌─ HAMMA RAQAM SERVERDA HISOBLANADI ────────────────────────────────┐
/// `api.orders()` eng so'nggi 100 ta buyurtmani qaytaradi — bir oylik
/// davr uchun undan hisoblash jimgina YOLG'ON raqam berardi. Shuning
/// uchun ko'rsatkichlar `GET /restaurants/{id}/stats` (`internal/stats`)
/// dan tayyor keladi. Faqat "So'nggi buyurtmalar" jadvali — ta'rifiga
/// ko'ra eng oxirgi buyurtmalar — `api.orders()` dan oziqlanadi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ BITTA EKRANGA SIG'ISH ───────────────────────────────────────────┐
/// Qatorlar balandligi qat'iy va 1920×1080 ekranda (yon menyu va yuqori
/// panel chiqarib tashlangandan keyin) sahifa scroll'siz sig'adi — bu
/// `test/statistics_page_test.dart` da qulflangan. Kichikroq ekranda
/// scroll zaxira sifatida qoladi: ma'lumotni kesib tashlashdan yaxshi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ NAMUNADAN ATAYLAB FARQ QILADIGAN JOYLAR ─────────────────────────┐
///  * "Mijozlar bahosi" o'rniga "O'rtacha tayyorlash vaqti": restoran
///    reytingi hozircha admin tomonidan QO'LDA kiritiladi — uni mijozlar
///    bergan baho deb ko'rsatish yolg'on bo'lardi.
///  * Jadvaldagi ustun "Tushum" emas "Summa": u menyu narxida, buyurtma
///    darajasidagi aksiya chegirmasi taomlarga bo'linmaydi.
///  * Buyurtma holati yorlig'i panelning YAGONA manbaidan
///    (`orderStatusStyle`) — boshqa sahifalar bilan bir xil.
/// └───────────────────────────────────────────────────────────────────┘
class StatisticsPage extends StatefulWidget {
  final StatisticsController controller;

  const StatisticsPage({super.key, required this.controller});

  @override
  State<StatisticsPage> createState() => _StatisticsPageState();
}

// ─── Doimiylar ────────────────────────────────────────────────────────────

const _monthsFull = [
  'yanvar', 'fevral', 'mart', 'aprel', 'may', 'iyun', //
  'iyul', 'avgust', 'sentabr', 'oktabr', 'noyabr', 'dekabr',
];
const _monthsShort = [
  'yan', 'fev', 'mar', 'apr', 'may', 'iyun', //
  'iyul', 'avg', 'sen', 'okt', 'noy', 'dek',
];
const _weekdays = [
  'Dushanba', 'Seshanba', 'Chorshanba', 'Payshanba', //
  'Juma', 'Shanba', 'Yakshanba',
];

/// Server bilan bir xil chegara (`internal/stats.MaxRangeDays`). Bu yerda
/// faqat foydalanuvchiga darhol aytish uchun — haqiqiy tekshiruv serverda.
const _maxRangeDays = 366;

const _pagePadding = EdgeInsets.fromLTRB(20, 18, 20, 18);
const _gap = 14.0;
const _chartsRowHeight = 272.0;
const _tablesRowHeight = 292.0;

/// Shundan tor bo'lganda juft kartochkalar ustma-ust tushadi.
const _splitBreakpoint = 1000.0;

/// Donut ranglari — namunadagi tartibda. Oxirgisi "Boshqa" uchun ham.
const _productPalette = <Color>[
  OnDexColors.primary,
  Color(0xFFF5C21B),
  OnDexColors.success,
  Color(0xFF2F6FDE),
  OnDexColors.purple,
];
const _otherProductColor = Color(0xFFB9AFA3);
const _insightsBg = Color(0xFFFFF8F1);

const _headStyle = TextStyle(
    fontSize: 11.5, color: OnDexColors.inkFaint, fontWeight: FontWeight.w600);

// ─── Formatlash ───────────────────────────────────────────────────────────

String _groupDigits(int n) {
  final digits = n
      .abs()
      .toString()
      .replaceAllMapped(RegExp(r'(\d)(?=(\d{3})+$)'), (m) => '${m[1]} ');
  return n < 0 ? '-$digits' : digits;
}

String _count(int n) => '${_groupDigits(n)} ta';

String _dateLong(DateTime d) => '${d.day}-${_monthsFull[d.month - 1]}, ${d.year}';

String _dateShort(DateTime d) => '${d.day}-${_monthsShort[d.month - 1]}';

String _hour(int h) => '${h.toString().padLeft(2, '0')}:00';

String _clock(DateTime t) =>
    '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';

String _fileDate(DateTime d) =>
    '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

String _percent(int part, int total) =>
    total <= 0 ? '0%' : '${(part * 100 / total).toStringAsFixed(1)}%';

String _decimal(double v) =>
    v == v.roundToDouble() ? v.round().toString() : v.toStringAsFixed(1);

DateTime _dateOnly(DateTime d) => DateTime(d.year, d.month, d.day);

/// Ikki sana orasidagi kunlar (ikkalasi ham kiradi). UTC orqali — yozgi
/// vaqtga o'tadigan mintaqada ham kun 23 yoki 25 soat bo'lib xato bermasin.
int _daysInclusive(DateTime from, DateTime to) =>
    DateTime.utc(to.year, to.month, to.day)
        .difference(DateTime.utc(from.year, from.month, from.day))
        .inDays +
    1;

String _compactSom(double som) {
  String trim(double v) =>
      v == v.roundToDouble() ? v.toStringAsFixed(0) : v.toStringAsFixed(1);
  if (som <= 0) return '0';
  if (som >= 1000000) return '${trim(som / 1000000)}M';
  if (som >= 1000) return '${trim(som / 1000)}k';
  return som.toStringAsFixed(0);
}

String _mealTime(int startHour) {
  if (startHour >= 5 && startHour <= 10) return 'Nonushta vaqti';
  if (startHour >= 11 && startHour <= 14) return 'Tushlik vaqti';
  if (startHour >= 15 && startHour <= 17) return 'Tushdan keyin';
  if (startHour >= 18 && startHour <= 22) return 'Kechki ovqat vaqti';
  return 'Tungi vaqt';
}

String _granularityLabel(String g) => switch (g) {
      'week' => 'Haftalik',
      'month' => 'Oylik',
      _ => 'Kunlik',
    };

String _rangeText(DateTime start, DateTime end, {String dash = '–'}) {
  if (start == end) return _dateLong(start);
  if (start.year != end.year) return '${_dateLong(start)} $dash ${_dateLong(end)}';
  return '${_dateShort(start)} $dash ${_dateLong(end)}';
}

/// Yuqori paneldagi ixcham yozuv: "15-avg – 13-sen, 2026".
String _rangeLabel(DateTimeRange r) {
  final s = r.start;
  final e = r.end;
  if (s == e) return '${_dateShort(s)}, ${s.year}';
  if (s.year == e.year) return '${_dateShort(s)} – ${_dateShort(e)}, ${e.year}';
  return '${_dateShort(s)}, ${s.year} – ${_dateShort(e)}, ${e.year}';
}

String _axisLabelOf(_Point p, String granularity) => granularity == 'month'
    ? '${_monthsShort[p.start.month - 1]} ${p.start.year}'
    : _dateShort(p.start);

String _pointTitle(_Point p, String granularity) =>
    granularity == 'day' ? _dateLong(p.start) : _rangeText(p.start, p.end);

/// Bugungi buyurtma — faqat soat; eskisi — sana bilan (aks holda
/// kechagi "16:22" bugungidek o'qilardi).
String _orderTime(DateTime? t) {
  if (t == null) return '—';
  final now = DateTime.now();
  if (t.year == now.year && t.month == now.month && t.day == now.day) return _clock(t);
  return '${_dateShort(t)} ${_clock(t)}';
}

String _customerText(Map<String, dynamic> o) {
  final phone = _str(o['customer_phone']);
  if (phone.isNotEmpty) return phone;
  if (isDineInOrder(o)) return tableText(_str(o['table_label']));
  return '—';
}

String _itemsText(Map<String, dynamic> o) {
  final items = [for (final i in (o['items'] is List ? o['items'] as List : const [])) if (i is Map) i];
  if (items.isEmpty) return '—';
  final first = items.first;
  final more = items.length - 1;
  return '${_int(first['qty'])}x ${_str(first['name'])}${more > 0 ? ' +$more' : ''}';
}

/// "Barcha buyurtmalar" jadvali uchun — har doim sana bilan.
String _orderDateTime(DateTime? t) =>
    t == null ? '—' : '${_dateShort(t)}, ${t.year} · ${_clock(t)}';

/// Buyurtmaning barcha taomlari: ["2x Lavash", "3x Cola"].
List<String> _itemLines(Map<String, dynamic> o) => [
      for (final i in (o['items'] is List ? o['items'] as List : const []))
        if (i is Map) '${_int(i['qty'])}x ${_str(i['name'])}',
    ];

// ─── JSON ─────────────────────────────────────────────────────────────────
//
// Yordamchilar HECH QACHON xato tashlamaydi: kutilmagan maydon turi
// "internet yo'q" degan chalg'ituvchi xabarga aylanib ketmasin.

int _int(Object? v) => v is num ? v.toInt() : 0;

int? _intOrNull(Object? v) => v is num ? v.toInt() : null;

double _double(Object? v) => v is num ? v.toDouble() : 0;

String _str(Object? v) => v is String ? v : '';

DateTime _day(Object? v) {
  final d = v is String ? DateTime.tryParse(v) : null;
  return d == null ? DateTime(1970) : _dateOnly(d);
}

Map<String, dynamic> _map(Object? v) =>
    v is Map ? Map<String, dynamic>.from(v) : const <String, dynamic>{};

List<Map<String, dynamic>> _maps(Object? v) => v is List
    ? [for (final e in v) if (e is Map) Map<String, dynamic>.from(e)]
    : const [];

class _Summary {
  final int revenueTiyin;
  final int orders;
  final int completed;
  final int inProgress;

  /// Hali yakunlanmagan buyurtmalar summasi — tushumga QO'SHILMAYDI,
  /// faqat alohida ko'rsatiladi.
  final int inProgressTiyin;
  final int cancelled;
  final int newCustomers;
  final int returningCustomers;
  final int? avgOrderTiyin;
  final int? avgPrepMinutes;

  const _Summary({
    required this.revenueTiyin,
    required this.orders,
    required this.completed,
    required this.inProgress,
    required this.inProgressTiyin,
    required this.cancelled,
    required this.newCustomers,
    required this.returningCustomers,
    required this.avgOrderTiyin,
    required this.avgPrepMinutes,
  });

  factory _Summary.fromJson(Map<String, dynamic> j) => _Summary(
        revenueTiyin: _int(j['revenue_tiyin']),
        orders: _int(j['orders']),
        completed: _int(j['completed']),
        inProgress: _int(j['in_progress']),
        inProgressTiyin: _int(j['in_progress_tiyin']),
        cancelled: _int(j['cancelled']),
        newCustomers: _int(j['new_customers']),
        returningCustomers: _int(j['returning_customers']),
        avgOrderTiyin: _intOrNull(j['avg_order_tiyin']),
        avgPrepMinutes: _intOrNull(j['avg_prep_minutes']),
      );
}

class _Point {
  final DateTime start;
  final DateTime end;
  final int revenueTiyin;
  final int orders;

  const _Point(this.start, this.end, this.revenueTiyin, this.orders);
}

class _Category {
  final String name;
  final int qty;
  final int salesTiyin;

  const _Category(this.name, this.qty, this.salesTiyin);
}

class _Product {
  final String name;
  final String category;
  final String imageUrl;
  final int qty;
  final int salesTiyin;

  const _Product(this.name, this.category, this.imageUrl, this.qty, this.salesTiyin);
}

class _Stats {
  final DateTime from;
  final DateTime to;
  final DateTime previousFrom;
  final DateTime previousTo;
  final String granularity;
  final _Summary current;
  final _Summary previous;
  final List<_Point> series;
  final List<_Category> categories;
  final List<_Product> topProducts;
  final int topProductsTotal;
  final int? busyStartHour;
  final int? busyEndHour;

  /// 1 = dushanba ... 7 = yakshanba; `null` — buyurtma bo'lmagan.
  final int? busyWeekday;
  final double busyWeekdayAvg;

  const _Stats({
    required this.from,
    required this.to,
    required this.previousFrom,
    required this.previousTo,
    required this.granularity,
    required this.current,
    required this.previous,
    required this.series,
    required this.categories,
    required this.topProducts,
    required this.topProductsTotal,
    required this.busyStartHour,
    required this.busyEndHour,
    required this.busyWeekday,
    required this.busyWeekdayAvg,
  });

  /// Barcha sotilgan dona (turkumlar butun taomlarni qamraydi, eng
  /// ko'p sotilganlar ro'yxati esa 50 ta bilan cheklangan).
  int get soldQty => categories.fold<int>(0, (sum, c) => sum + c.qty);

  factory _Stats.fromJson(Map<String, dynamic> j) {
    final hours = _map(j['busiest_hours']);
    final weekday = _map(j['busiest_weekday']);
    final start = _intOrNull(hours['start_hour']);
    final end = _intOrNull(hours['end_hour']);
    final validHours =
        start != null && end != null && start >= 0 && start < 24 && end >= 0 && end < 24;
    final wd = _intOrNull(weekday['weekday']);
    return _Stats(
      from: _day(j['from']),
      to: _day(j['to']),
      previousFrom: _day(j['previous_from']),
      previousTo: _day(j['previous_to']),
      granularity: _str(j['granularity']),
      current: _Summary.fromJson(_map(j['current'])),
      previous: _Summary.fromJson(_map(j['previous'])),
      series: [
        for (final p in _maps(j['series']))
          _Point(_day(p['start']), _day(p['end']), _int(p['revenue_tiyin']),
              _int(p['orders'])),
      ],
      categories: [
        for (final c in _maps(j['categories']))
          _Category(_str(c['name']), _int(c['qty']), _int(c['sales_tiyin'])),
      ],
      topProducts: [
        for (final p in _maps(j['top_products']))
          _Product(_str(p['name']), _str(p['category']), _str(p['image_url']),
              _int(p['qty']), _int(p['sales_tiyin'])),
      ],
      topProductsTotal: _int(j['top_products_total']),
      busyStartHour: validHours ? start : null,
      busyEndHour: validHours ? end : null,
      busyWeekday: wd != null && wd >= 1 && wd <= 7 ? wd : null,
      busyWeekdayAvg: _double(weekday['avg_orders']),
    );
  }
}

// ─── Holat (sahifa + yuqori panel) ───────────────────────────────────────

/// Statistika sahifasi va yuqori paneldagi davr/Eksport tugmalarining
/// UMUMIY holati.
///
/// ┌─ NEGA ALOHIDA OBYEKT ─────────────────────────────────────────────┐
/// Davr tanlagich va Eksport yuqori panelda (`TopBar`), grafiklar esa
/// sahifada — ular turli vidjet daraxtlarida, lekin BITTA holatni ko'rishi
/// shart: panelda davr almashsa sahifa qayta yuklanadi, Eksport esa aynan
/// ekrandagi ma'lumotni chop etadi. Obyekt `RestaurantShell` da yashaydi,
/// ya'ni boshqa sahifaga o'tib qaytganda tanlangan davr yo'qolmaydi.
/// └───────────────────────────────────────────────────────────────────┘
class StatisticsController extends ChangeNotifier {
  StatisticsController() : _range = _defaultRange();

  static DateTimeRange _defaultRange() {
    final today = _dateOnly(DateTime.now());
    return DateTimeRange(
        start: DateTime(today.year, today.month, today.day - 29), end: today);
  }

  DateTimeRange _range;
  String _granularity = 'day';
  _Stats? _data;
  bool _loading = false;
  String? _error;
  bool _exporting = false;
  List<Map<String, dynamic>>? _recentOrders;
  bool _showingHistory = false;
  DateTimeRange? _historyRange;
  String _historyBackLabel = 'Statistika';
  VoidCallback? _historyOnBack;
  bool _disposed = false;

  /// Eskirgan javob yangisini bosib qolmasin: davr tez-tez almashtirilganda
  /// oldingi so'rov keyinroq qaytishi mumkin.
  int _requestSeq = 0;

  DateTimeRange get range => _range;
  String get granularity => _granularity;
  bool get loading => _loading;
  String? get error => _error;
  bool get exporting => _exporting;
  bool get hasData => _data != null;

  /// "So'nggi buyurtmalar" → "Barchasini ko'rish" ochiqmi. Holat shu
  /// yerda: yuqori paneldagi davr/Eksport tugmalari ham unga qarab
  /// yashiriladi.
  bool get showingHistory => _showingHistory;

  /// "Barcha buyurtmalar" davr filtri (yuqori panelda); `null` — butun
  /// tarix (restoran ochilgandan beri).
  DateTimeRange? get historyRange => _historyRange;

  /// Qaytish tugmasining yozuvi — sahifa qayerdan ochilganiga qarab.
  String get historyBackLabel => _historyBackLabel;

  /// [backLabel]/[onBack] — qaytish tugmasi qayerga olib borishi (masalan
  /// "Buyurtmalar" sahifasidagi "Tarix" dan ochilganda — o'sha sahifaga).
  /// Har ochilishda davr filtri tozalanadi: sahifa butun tarixdan
  /// boshlanadi.
  void openHistory({String backLabel = 'Statistika', VoidCallback? onBack}) {
    _showingHistory = true;
    _historyRange = null;
    _historyBackLabel = backLabel;
    _historyOnBack = onBack;
    _notify();
  }

  void closeHistory() {
    if (!_showingHistory) return;
    _showingHistory = false;
    _historyOnBack = null;
    _notify();
  }

  /// Qaytish tugmasi: yopadi va (berilgan bo'lsa) ochilgan joyga qaytaradi.
  void leaveHistory() {
    final back = _historyOnBack;
    closeHistory();
    back?.call();
  }

  Future<void> pickHistoryRange(BuildContext context) async {
    final pick = await showOnDexPeriodPicker(
      context: context,
      initialRange: _historyRange,
      firstDate: DateTime(2020, 1, 1),
      lastDate: _dateOnly(DateTime.now()),
    );
    if (pick == null || !context.mounted) return;
    final r = pick.range;
    final next = r == null
        ? null
        : DateTimeRange(start: _dateOnly(r.start), end: _dateOnly(r.end));
    if (next == _historyRange) return;
    _historyRange = next;
    _notify();
  }

  void clearHistoryRange() {
    if (_historyRange == null) return;
    _historyRange = null;
    _notify();
  }

  void _notify() {
    if (!_disposed) notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }

  Future<void> load() async {
    final seq = ++_requestSeq;
    _loading = true;
    _error = null;
    _notify();
    try {
      final json = await api.stats(
          from: _range.start, to: _range.end, granularity: _granularity);
      if (_disposed || seq != _requestSeq) return;
      _data = _Stats.fromJson(json);
      _loading = false;
      _notify();
    } on ApiException catch (e) {
      if (_disposed || seq != _requestSeq) return;
      _error = e.message;
      _loading = false;
      _notify();
    } catch (_) {
      if (_disposed || seq != _requestSeq) return;
      _error =
          'Ma\'lumotni yuklab bo\'lmadi. Internet aloqasini tekshirib, qayta urinib ko\'ring.';
      _loading = false;
      _notify();
    }
  }

  void setGranularity(String value) {
    if (value == _granularity) return;
    _granularity = value;
    load();
  }

  /// `RestaurantShell._pollOrders` beradi — yon paneldagi hisoblagich
  /// uchun ALLAQACHON olingan ro'yxat. Qo'shimcha so'rov ham, taymer ham
  /// yo'q; yangi buyurtma kelishi bilan jadval o'zi yangilanadi.
  void setRecentOrders(List<Map<String, dynamic>> orders) {
    _recentOrders = orders;
    _notify();
  }

  Future<void> pickRange(BuildContext context) async {
    final today = _dateOnly(DateTime.now());
    final initial = _range.end.isAfter(today)
        ? DateTimeRange(
            start: _range.start.isAfter(today) ? today : _range.start, end: today)
        : _range;
    final picked = await showOnDexDateRangePicker(
      context: context,
      initialRange: initial,
      firstDate: DateTime(2020, 1, 1),
      lastDate: today,
      maxDays: _maxRangeDays,
    );
    if (picked == null || !context.mounted) return;
    final range = DateTimeRange(
        start: _dateOnly(picked.start), end: _dateOnly(picked.end));
    if (_daysInclusive(range.start, range.end) > _maxRangeDays) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
          content:
              Text('Davr $_maxRangeDays kundan oshmasligi kerak — qisqaroq davr tanlang')));
      return;
    }
    _range = range;
    load();
  }

  Future<void> export(BuildContext context, String restaurantName) async {
    final data = _data;
    if (data == null || _exporting) return;
    final messenger = ScaffoldMessenger.maybeOf(context);
    _exporting = true;
    _notify();
    try {
      final doc = await _buildReport(data, restaurantName);
      await Printing.layoutPdf(
        name: 'OnDex-statistika-${_fileDate(data.from)}_${_fileDate(data.to)}.pdf',
        onLayout: (_) => doc.save(),
      );
    } catch (e) {
      messenger?.showSnackBar(
          SnackBar(content: Text('Hisobotni tayyorlab bo\'lmadi: $e')));
    } finally {
      _exporting = false;
      _notify();
    }
  }
}

/// Yuqori panel uchun: davr tanlagich + Eksport. Faqat Statistika
/// sahifasida ko'rsatiladi (`RestaurantShell`), boshqa sahifalarda odatiy
/// bir kunlik sana tugmasi qoladi.
class StatisticsToolbar extends StatelessWidget {
  final StatisticsController controller;
  final String restaurantName;

  const StatisticsToolbar({
    super.key,
    required this.controller,
    required this.restaurantName,
  });

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: controller,
      // "Barcha buyurtmalar" ochiq bo'lsa — uning O'Z davr filtri
      // ("Barcha vaqt" bilan); statistika davri va Eksport unga tegishli emas.
      builder: (context, _) => controller.showingHistory
          ? Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Flexible(
                  child: _RangeButton(
                    label: controller.historyRange == null
                        ? 'Barcha vaqt'
                        : _rangeLabel(controller.historyRange!),
                    onTap: () => controller.pickHistoryRange(context),
                  ),
                ),
                if (controller.historyRange != null) ...[
                  const SizedBox(width: 4),
                  IconButton(
                    tooltip: 'Filtrni tozalash',
                    onPressed: controller.clearHistoryRange,
                    icon: const Icon(Icons.filter_alt_off_rounded, size: 19),
                    color: OnDexColors.inkDim,
                  ),
                ],
              ],
            )
          : Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Flexible(
                  child: _RangeButton(
                    label: _rangeLabel(controller.range),
                    onTap: () => controller.pickRange(context),
                  ),
                ),
                const SizedBox(width: 10),
                _ExportButton(
                  busy: controller.exporting,
                  onPressed: !controller.hasData || controller.loading || controller.exporting
                      ? null
                      : () => controller.export(context, restaurantName),
                ),
              ],
            ),
    );
  }
}

// ─── Sahifa ───────────────────────────────────────────────────────────────

class _StatisticsPageState extends State<StatisticsPage> {
  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_onChanged);
    // Sahifaga har kirilganda yangilanadi (server natijani 60 s keshlaydi,
    // ya'ni qimmat emas). Kadrdan KEYIN: yuqori paneldagi tugmalar ham shu
    // obyektni tinglaydi va ularni qurilish paytida qayta chizishga
    // majburlab bo'lmaydi.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) widget.controller.load();
    });
  }

  @override
  void didUpdateWidget(covariant StatisticsPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller != widget.controller) {
      oldWidget.controller.removeListener(_onChanged);
      widget.controller.addListener(_onChanged);
    }
  }

  @override
  void dispose() {
    widget.controller.removeListener(_onChanged);
    super.dispose();
  }

  void _onChanged() {
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final c = widget.controller;
    if (c.showingHistory) return _OrderHistoryView(controller: c);
    final data = c._data;
    if (data == null) {
      if (c.error != null && !c.loading) {
        return SingleChildScrollView(
          padding: _pagePadding,
          child: _ErrorPanel(message: c.error!, onRetry: c.load),
        );
      }
      return const Center(child: CircularProgressIndicator());
    }

    return Stack(
      children: [
        SingleChildScrollView(
          padding: _pagePadding,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (c.error != null)
                Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: _ErrorBanner(
                    message: 'Yangi ma\'lumot yuklanmadi: ${c.error}. '
                        'Ko\'rsatilayotgan davr: ${_dateLong(data.from)} - ${_dateLong(data.to)}.',
                    onRetry: c.load,
                  ),
                ),
              // Yangi davr yuklanayotganda eski raqamlar xira: ular
              // tanlangan davrga tegishli emasligi ko'rinib tursin.
              AnimatedOpacity(
                opacity: c.loading ? 0.55 : 1,
                duration: const Duration(milliseconds: 150),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    _buildKpis(data),
                    const SizedBox(height: _gap),
                    _SplitRow(
                      height: _chartsRowHeight,
                      leftFlex: 13,
                      rightFlex: 7,
                      left: _buildRevenueCard(c, data),
                      right: _ProductsDonutCard(data: data),
                    ),
                    const SizedBox(height: _gap),
                    _SplitRow(
                      height: _tablesRowHeight,
                      leftFlex: 9,
                      rightFlex: 11,
                      left: _TopProductsTable(
                        products: data.topProducts,
                        inProgress: data.current.inProgress,
                      ),
                      right: _RecentOrdersTable(
                          orders: c._recentOrders, onSeeAll: c.openHistory),
                    ),
                    const SizedBox(height: _gap),
                    _InsightsStrip(data: data),
                  ],
                ),
              ),
            ],
          ),
        ),
        if (c.loading)
          const Positioned(
              top: 0, left: 0, right: 0, child: LinearProgressIndicator(minHeight: 2)),
      ],
    );
  }

  Widget _buildKpis(_Stats d) {
    final c = d.current;
    final p = d.previous;
    // ┌─ "0 so'm" NEGA 0 EKANI KO'RINSIN ────────────────────────────────┐
    // Tushum faqat YAKUNLANGAN buyurtmalardan (buyurtma hali bekor
    // bo'lishi mumkin). Lekin jarayondagi buyurtmalar bo'lsa-yu kartochkada
    // shunchaki "0 so'm" tursa, bu xato yoki soxta ma'lumotdek ko'rinardi.
    // Shuning uchun ularning summasi tushumga qo'shilmasdan, ALOHIDA
    // yoziladi.
    //
    // Qator bo'lsa, HAMMA kartochkada joy ajratiladi — aks holda bitta
    // qatordagi kartochkalar turli balandlikda chiqardi.
    // └──────────────────────────────────────────────────────────────────┘
    final revenueNote = c.inProgress > 0
        ? '+${formatSum(c.inProgressTiyin)} jarayonda (${_count(c.inProgress)})'
        : null;
    final reserveNote = revenueNote != null;
    return _ResponsiveWrap(
      minItemWidth: 180,
      spacing: 12,
      children: [
        _KpiCard(
          label: 'Jami tushum',
          value: formatSum(c.revenueTiyin),
          icon: Icons.payments_outlined,
          color: OnDexColors.primary,
          background: OnDexColors.primaryTint,
          delta: _Delta(c.revenueTiyin, p.revenueTiyin),
          note: revenueNote,
          reserveNote: reserveNote,
          hint: 'Faqat bajarilgan (yetkazilgan yoki stolga berilgan) buyurtmalar — '
              'mijoz haqiqatan to\'lagan, chegirmadan keyingi summa. Hali yakunlanmagan '
              'buyurtmalar summasi alohida ko\'rsatiladi va tushumga qo\'shilmaydi.',
        ),
        _KpiCard(
          label: 'Jami buyurtmalar',
          value: _count(c.orders),
          icon: Icons.inventory_2_outlined,
          color: OnDexColors.info,
          background: OnDexColors.infoBg,
          delta: _Delta(c.orders, p.orders),
          reserveNote: reserveNote,
          hint: 'Davrda kelgan barcha buyurtmalar. To\'lanmagan karta buyurtmalari kirmaydi.',
        ),
        _KpiCard(
          label: 'O\'rtacha buyurtma',
          // Bajarilgan buyurtma bo'lmasa — bo'sh chiziq emas, "0 so'm";
          // nega nol ekani solishtirish qatorida yoziladi.
          value: formatSum(c.avgOrderTiyin ?? 0),
          icon: Icons.trending_up_rounded,
          color: OnDexColors.purple,
          background: OnDexColors.purpleBg,
          delta: c.avgOrderTiyin == null
              ? const _Delta.missing('Bajarilgan buyurtma yo\'q')
              : p.avgOrderTiyin == null
                  ? _Delta.none
                  : _Delta(c.avgOrderTiyin!, p.avgOrderTiyin!),
          reserveNote: reserveNote,
          hint: 'Jami tushum ÷ bajarilgan buyurtmalar soni. Bajarilgan buyurtma bo\'lmasa 0.',
        ),
        _KpiCard(
          label: 'Bajarilgan buyurtmalar',
          value: _count(c.completed),
          icon: Icons.check_circle_outline_rounded,
          color: OnDexColors.success,
          background: OnDexColors.successBg,
          delta: _Delta(c.completed, p.completed),
          reserveNote: reserveNote,
        ),
        _KpiCard(
          label: 'Bekor qilinganlar',
          value: _count(c.cancelled),
          icon: Icons.highlight_off_rounded,
          color: OnDexColors.danger,
          background: OnDexColors.dangerBg,
          delta: _Delta(c.cancelled, p.cancelled, higherIsBetter: false),
          reserveNote: reserveNote,
          hint: 'Restoran rad etgan yoki bekor qilingan buyurtmalar.',
        ),
      ],
    );
  }

  Widget _buildRevenueCard(StatisticsController c, _Stats d) {
    return _ChartCard(
      title: 'Tushum statistikasi',
      hint: 'Bajarilgan buyurtmalar tushumi. Hisob Toshkent vaqti bo\'yicha.',
      granularity: c.granularity,
      onGranularityChanged: c.setGranularity,
      chart: OnDexLineChart(
        points: [
          for (final p in d.series)
            ChartPoint(
              axisLabel: _axisLabelOf(p, d.granularity),
              value: p.revenueTiyin / 100,
              tooltipTitle: _pointTitle(p, d.granularity),
              tooltipValue: 'Tushum: ${formatSum(p.revenueTiyin)}',
            ),
        ],
        color: OnDexColors.primary,
        axisLabel: _compactSom,
        unitLabel: 'so\'m',
        // Jarayonda buyurtma bo'lsa — bo'shlik sababi aytiladi (aks holda
        // grafik "buzilgan"dek ko'rinardi).
        emptyText: d.current.inProgress > 0
            ? 'Yakunlangan tushum yo\'q · ${formatSum(d.current.inProgressTiyin)} jarayonda'
            : 'Bu davrda tushum yo\'q',
      ),
    );
  }
}

// ─── Solishtirish (o'tgan davrga nisbatan) ──────────────────────────────

class _Delta {
  final num current;
  final num previous;

  /// Qaysi yo'nalish yaxshi: tushum o'sishi yaxshi, bekor qilinganlar
  /// yoki tayyorlash vaqti o'sishi — yomon.
  final bool higherIsBetter;
  final bool available;

  /// Solishtirib bo'lmaganda foiz o'rniga yoziladigan sabab.
  final String reason;

  const _Delta(this.current, this.previous, {this.higherIsBetter = true})
      : available = true,
        reason = '';

  const _Delta.missing(this.reason)
      : current = 0,
        previous = 0,
        higherIsBetter = true,
        available = false;

  static const none = _Delta.missing('O\'tgan davrda ma\'lumot yo\'q');
}

class _DeltaLine extends StatelessWidget {
  final _Delta delta;

  const _DeltaLine({required this.delta});

  static const _rest = TextStyle(fontSize: 11, color: OnDexColors.inkDim);

  @override
  Widget build(BuildContext context) {
    if (!delta.available) {
      return Text(delta.reason, maxLines: 1, overflow: TextOverflow.ellipsis, style: _rest);
    }
    if (delta.previous == 0) {
      // Nolga bo'lib foiz chiqarib bo'lmaydi — "+∞%" o'rniga halol matn.
      return Text(
          delta.current == 0 ? 'O\'tgan davr bilan bir xil' : 'O\'tgan davrda ma\'lumot yo\'q',
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: _rest);
    }
    final pct = (delta.current - delta.previous) / delta.previous.abs() * 100;
    final rounded = pct.round();
    if (rounded == 0) {
      return const Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.remove_rounded, size: 12, color: OnDexColors.inkDim),
          SizedBox(width: 3),
          Flexible(
            child: Text('0% o\'tgan davrga',
                maxLines: 1, overflow: TextOverflow.ellipsis, style: _rest),
          ),
        ],
      );
    }
    final up = pct > 0;
    final good = up == delta.higherIsBetter;
    final color = good ? OnDexColors.success : OnDexColors.danger;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(up ? Icons.arrow_upward_rounded : Icons.arrow_downward_rounded,
            size: 12, color: color),
        const SizedBox(width: 3),
        Flexible(
          child: Text.rich(
            TextSpan(children: [
              TextSpan(
                text: '${up ? '+' : '-'}${rounded.abs()}%',
                style: TextStyle(color: color, fontWeight: FontWeight.w700),
              ),
              const TextSpan(text: ' o\'tgan davrga'),
            ]),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: _rest,
          ),
        ),
      ],
    );
  }
}

// ─── Qurilish bloklari ──────────────────────────────────────────────────

class _Panel extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry padding;

  const _Panel({
    required this.child,
    this.padding = const EdgeInsets.fromLTRB(16, 14, 16, 14),
  });

  @override
  Widget build(BuildContext context) => Container(
        padding: padding,
        decoration: BoxDecoration(
          color: OnDexColors.cardBg,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: OnDexColors.cardBorder),
        ),
        child: child,
      );
}

/// Elementlarni sig'adigan qadar teng ustunlarga taqsimlaydi.
///
/// Kenglik PASTGA yaxlitlanadi: aks holda suzuvchi nuqta xatosi tufayli
/// ustunlar yig'indisi konteynerdan bir necha pikselga oshib, oxirgi
/// kartochka sababsiz keyingi qatorga tushib ketardi.
class _ResponsiveWrap extends StatelessWidget {
  final double minItemWidth;
  final double spacing;
  final List<Widget> children;

  const _ResponsiveWrap({
    required this.minItemWidth,
    required this.spacing,
    required this.children,
  });

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, c) {
      final columns = ((c.maxWidth + spacing) / (minItemWidth + spacing))
          .floor()
          .clamp(1, children.length);
      final width = ((c.maxWidth - spacing * (columns - 1)) / columns).floorToDouble();
      return Wrap(
        spacing: spacing,
        runSpacing: spacing,
        children: [for (final child in children) SizedBox(width: width, child: child)],
      );
    });
  }
}

/// Qat'iy balandlikdagi ikki kartochka yonma-yon (namunadagi nisbatda);
/// tor ekranda ustma-ust.
class _SplitRow extends StatelessWidget {
  final double height;
  final int leftFlex;
  final int rightFlex;
  final Widget left;
  final Widget right;

  const _SplitRow({
    required this.height,
    required this.leftFlex,
    required this.rightFlex,
    required this.left,
    required this.right,
  });

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, c) {
      if (c.maxWidth >= _splitBreakpoint) {
        return SizedBox(
          height: height,
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Expanded(flex: leftFlex, child: left),
              const SizedBox(width: _gap),
              Expanded(flex: rightFlex, child: right),
            ],
          ),
        );
      }
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SizedBox(height: height, child: left),
          const SizedBox(height: _gap),
          SizedBox(height: height, child: right),
        ],
      );
    });
  }
}

class _HintIcon extends StatelessWidget {
  final String message;

  const _HintIcon({required this.message});

  @override
  Widget build(BuildContext context) => Tooltip(
        message: message,
        waitDuration: const Duration(milliseconds: 250),
        child: const Icon(Icons.info_outline_rounded,
            size: 15, color: OnDexColors.inkFaint),
      );
}

class _CardTitle extends StatelessWidget {
  final IconData icon;
  final String title;
  final String? hint;
  final Widget? trailing;

  const _CardTitle({
    required this.icon,
    required this.title,
    this.hint,
    this.trailing,
  });

  @override
  Widget build(BuildContext context) {
    // Sarlavha ICHKI qatorda, butun qolgan joy `Expanded` da: aks holda
    // `Flexible` + `Spacer` bo'sh joyni teng bo'lib, o'ngdagi tugma chetga
    // yetmay qolardi (`top_bar.dart` dagi xuddi shu xato).
    return SizedBox(
      height: 32,
      child: Row(
        children: [
          Expanded(
            child: Row(
              children: [
                Icon(icon, size: 19, color: OnDexColors.primary),
                const SizedBox(width: 8),
                Flexible(
                  child: Text(title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                          fontSize: 15,
                          fontWeight: FontWeight.w700,
                          color: OnDexColors.ink)),
                ),
                if (hint != null) ...[
                  const SizedBox(width: 6),
                  _HintIcon(message: hint!),
                ],
              ],
            ),
          ),
          if (trailing != null) ...[const SizedBox(width: 8), trailing!],
        ],
      ),
    );
  }
}

class _EmptyText extends StatelessWidget {
  final String text;

  const _EmptyText(this.text);

  @override
  Widget build(BuildContext context) => Center(
        child: Text(text,
            textAlign: TextAlign.center,
            style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
      );
}

/// Qat'iy kenglikdagi jadval katagi. Matn sig'masa KESILMAYDI, biroz
/// kichrayadi — summa va sonlar to'liq o'qilishi shart.
Widget _cell(double width, Widget child) => SizedBox(
      width: width,
      child: Align(
        alignment: Alignment.centerLeft,
        child: FittedBox(
            fit: BoxFit.scaleDown, alignment: Alignment.centerLeft, child: child),
      ),
    );

class _RangeButton extends StatelessWidget {
  final String label;
  final VoidCallback onTap;

  const _RangeButton({required this.label, required this.onTap});

  @override
  Widget build(BuildContext context) {
    // Ko'rinishi yuqori paneldagi odatiy sana tugmasi bilan bir xil.
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(10),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(
          border: Border.all(color: OnDexColors.cardBorder),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.calendar_today_rounded,
                size: 15, color: OnDexColors.inkDim),
            const SizedBox(width: 9),
            Flexible(
              child: Text(label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      fontSize: 13,
                      color: OnDexColors.ink,
                      fontWeight: FontWeight.w600)),
            ),
            const SizedBox(width: 6),
            const Icon(Icons.keyboard_arrow_down_rounded,
                size: 17, color: OnDexColors.inkDim),
          ],
        ),
      ),
    );
  }
}

class _ExportButton extends StatelessWidget {
  final bool busy;
  final VoidCallback? onPressed;

  const _ExportButton({required this.busy, required this.onPressed});

  @override
  Widget build(BuildContext context) {
    return OutlinedButton.icon(
      onPressed: onPressed,
      style: OutlinedButton.styleFrom(
        foregroundColor: OnDexColors.ink,
        side: const BorderSide(color: OnDexColors.cardBorder),
        padding: const EdgeInsets.symmetric(horizontal: 14),
        minimumSize: const Size(0, 38),
        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
        textStyle: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
      ),
      icon: busy
          ? const SizedBox(
              width: 15, height: 15, child: CircularProgressIndicator(strokeWidth: 2))
          : const Icon(Icons.file_download_outlined, size: 17),
      label: const Text('Eksport'),
    );
  }
}

class _ErrorPanel extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;

  const _ErrorPanel({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return _Panel(
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 48),
      child: Column(
        children: [
          const Icon(Icons.bar_chart_rounded, size: 40, color: OnDexColors.inkFaint),
          const SizedBox(height: 12),
          Text(message,
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 14, color: OnDexColors.ink)),
          const SizedBox(height: 16),
          FilledButton(onPressed: onRetry, child: const Text('Qayta urinish')),
        ],
      ),
    );
  }
}

class _ErrorBanner extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;

  const _ErrorBanner({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(14, 8, 8, 8),
      decoration: BoxDecoration(
        color: OnDexColors.dangerBg,
        borderRadius: BorderRadius.circular(10),
      ),
      child: Row(
        children: [
          const Icon(Icons.error_outline_rounded, size: 18, color: OnDexColors.danger),
          const SizedBox(width: 10),
          Expanded(
            child: Text(message,
                style: const TextStyle(fontSize: 13, color: OnDexColors.ink)),
          ),
          TextButton(onPressed: onRetry, child: const Text('Qayta urinish')),
        ],
      ),
    );
  }
}

class _GranularityDropdown extends StatelessWidget {
  final String value;
  final ValueChanged<String> onChanged;

  const _GranularityDropdown({required this.value, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        border: Border.all(color: OnDexColors.cardBorder),
        borderRadius: BorderRadius.circular(8),
      ),
      child: DropdownButtonHideUnderline(
        child: DropdownButton<String>(
          value: value,
          isDense: true,
          borderRadius: BorderRadius.circular(10),
          icon: const Icon(Icons.keyboard_arrow_down_rounded, size: 18),
          style: const TextStyle(
              fontSize: 12.5, color: OnDexColors.ink, fontWeight: FontWeight.w600),
          items: const [
            DropdownMenuItem(value: 'day', child: Text('Kunlik')),
            DropdownMenuItem(value: 'week', child: Text('Haftalik')),
            DropdownMenuItem(value: 'month', child: Text('Oylik')),
          ],
          onChanged: (v) {
            if (v != null) onChanged(v);
          },
        ),
      ),
    );
  }
}

class _ProductThumb extends StatelessWidget {
  final String url;

  const _ProductThumb({required this.url});

  static const _size = 30.0;

  @override
  Widget build(BuildContext context) {
    const placeholder = ColoredBox(
      color: OnDexColors.pageBg,
      child: Center(
        child: Icon(Icons.restaurant_rounded, size: 16, color: OnDexColors.inkFaint),
      ),
    );
    return ClipRRect(
      borderRadius: BorderRadius.circular(7),
      child: SizedBox(
        width: _size,
        height: _size,
        child: url.isEmpty
            ? placeholder
            : Image.network(
                fullImageUrl(url),
                fit: BoxFit.cover,
                // Kichik ko'rinish uchun to'liq o'lchamli rasmni xotiraga
                // yuklamaslik.
                cacheWidth: 96,
                errorBuilder: (_, __, ___) => placeholder,
              ),
      ),
    );
  }
}

// ─── Kartochkalar ───────────────────────────────────────────────────────

class _KpiCard extends StatelessWidget {
  final String label;
  final String value;
  final IconData icon;
  final Color color;
  final Color background;
  /// `null` — solishtiriladigan davr yo'q (masalan butun tarix); o'rniga
  /// [caption] yoziladi.
  final _Delta? delta;
  final String? caption;
  final String? hint;

  /// Qo'shimcha izoh (masalan "+144 000 so'm jarayonda (2 ta)").
  final String? note;

  /// Izoh qatori uchun joy ajratilsinmi — bitta qatordagi kartochkalar
  /// bir xil balandlikda turishi uchun, izohi bo'lmaganlarida ham.
  final bool reserveNote;

  const _KpiCard({
    required this.label,
    required this.value,
    required this.icon,
    required this.color,
    required this.background,
    required this.delta,
    this.caption,
    this.hint,
    this.note,
    this.reserveNote = false,
  });

  @override
  Widget build(BuildContext context) {
    final badge = Container(
      width: 38,
      height: 38,
      decoration: BoxDecoration(color: background, borderRadius: BorderRadius.circular(10)),
      child: Icon(icon, size: 20, color: color),
    );
    // Ikonka CHAPDA (namunadagi kabi): tepada alohida qator egallamaydi,
    // kartochka balandligi shu sababli deyarli uchdan biriga kamaydi.
    return _Panel(
      padding: const EdgeInsets.fromLTRB(14, 12, 12, 12),
      child: Row(
        children: [
          if (hint == null)
            badge
          else
            Tooltip(
              message: hint!,
              waitDuration: const Duration(milliseconds: 250),
              child: badge,
            ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 12.5,
                        color: OnDexColors.inkDim,
                        fontWeight: FontWeight.w500)),
                const SizedBox(height: 3),
                // Katta summa tor kartochkaga sig'masa kesilmaydi —
                // kichrayadi ("24 180 000 so'm" to'liq o'qilishi SHART).
                SizedBox(
                  width: double.infinity,
                  child: FittedBox(
                    fit: BoxFit.scaleDown,
                    alignment: Alignment.centerLeft,
                    child: Text(value,
                        maxLines: 1,
                        style: const TextStyle(
                            fontSize: 19,
                            fontWeight: FontWeight.w800,
                            color: OnDexColors.ink)),
                  ),
                ),
                const SizedBox(height: 3),
                if (delta != null)
                  _DeltaLine(delta: delta!)
                else
                  Text(caption ?? '',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 11, color: OnDexColors.inkDim)),
                if (reserveNote) ...[
                  const SizedBox(height: 3),
                  SizedBox(
                    height: 14,
                    child: note == null
                        ? null
                        : Text(note!,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                                fontSize: 11,
                                color: OnDexColors.amber,
                                fontWeight: FontWeight.w600)),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _ChartCard extends StatelessWidget {
  final String title;
  final String hint;
  final String granularity;
  final ValueChanged<String> onGranularityChanged;
  final Widget chart;

  const _ChartCard({
    required this.title,
    required this.hint,
    required this.granularity,
    required this.onGranularityChanged,
    required this.chart,
  });

  @override
  Widget build(BuildContext context) {
    return _Panel(
      padding: const EdgeInsets.fromLTRB(16, 14, 14, 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _CardTitle(
            icon: Icons.show_chart_rounded,
            title: title,
            hint: hint,
            trailing: _GranularityDropdown(
                value: granularity, onChanged: onGranularityChanged),
          ),
          const SizedBox(height: 8),
          Expanded(child: chart),
        ],
      ),
    );
  }
}

class _LegendEntry {
  final String label;
  final int value;
  final Color color;

  const _LegendEntry(this.label, this.value, this.color);
}

/// "Buyurtmalar bo'yicha mahsulotlar" — sotilgan donalar ulushi.
class _ProductsDonutCard extends StatelessWidget {
  final _Stats data;

  const _ProductsDonutCard({required this.data});

  /// Eng ko'p sotilgan mahsulotlar, qolgani bitta "Boshqa" bo'lagi.
  static List<_LegendEntry> _entries(_Stats d) {
    final total = d.soldQty;
    final products = d.topProducts;
    final slots = _productPalette.length;
    final shown = products.length <= slots ? products : products.take(slots - 1).toList();
    final entries = [
      for (var i = 0; i < shown.length; i++)
        _LegendEntry(shown[i].name.isEmpty ? 'Nomsiz taom' : shown[i].name, shown[i].qty,
            _productPalette[i]),
    ];
    final rest = total - shown.fold<int>(0, (sum, p) => sum + p.qty);
    if (rest > 0) {
      entries.add(_LegendEntry('Boshqa', rest,
          shown.length < slots ? _productPalette[shown.length] : _otherProductColor));
    }
    return entries;
  }

  @override
  Widget build(BuildContext context) {
    final total = data.soldQty;
    final entries = _entries(data);
    return _Panel(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const _CardTitle(
            icon: Icons.bar_chart_rounded,
            title: 'Buyurtmalar bo\'yicha mahsulotlar',
            hint: 'Bajarilgan buyurtmalarda sotilgan taomlar soni (dona) va ulushi.',
          ),
          const SizedBox(height: 10),
          Expanded(
            child: total == 0
                ? _EmptyText(data.current.inProgress > 0
                    ? 'Yakunlangan sotuv yo\'q · ${_count(data.current.inProgress)} buyurtma jarayonda'
                    : 'Bu davrda sotuv yo\'q')
                : LayoutBuilder(builder: (context, c) {
                    final diameter = math.min(170.0, math.min(c.maxHeight, c.maxWidth * 0.42));
                    return Row(
                      children: [
                        OnDexDonutChart(
                          diameter: diameter,
                          thickness: diameter * 0.18,
                          segments: [
                            for (final e in entries) DonutSegment(e.value.toDouble(), e.color),
                          ],
                          center: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              const Text('Jami',
                                  style: TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                              const SizedBox(height: 2),
                              Text(_count(total),
                                  style: const TextStyle(
                                      fontSize: 18,
                                      fontWeight: FontWeight.w800,
                                      color: OnDexColors.ink)),
                            ],
                          ),
                        ),
                        const SizedBox(width: 18),
                        Expanded(
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              for (final e in entries) _LegendLine(entry: e, total: total),
                            ],
                          ),
                        ),
                      ],
                    );
                  }),
          ),
        ],
      ),
    );
  }
}

class _LegendLine extends StatelessWidget {
  final _LegendEntry entry;
  final int total;

  const _LegendLine({required this.entry, required this.total});

  @override
  Widget build(BuildContext context) {
    const numbers = TextStyle(fontSize: 12.5, color: OnDexColors.inkDim);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        children: [
          Container(
            width: 10,
            height: 10,
            decoration: BoxDecoration(color: entry.color, shape: BoxShape.circle),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text(entry.label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 13, color: OnDexColors.ink)),
          ),
          const SizedBox(width: 8),
          _cell(58, Text(_count(entry.value), style: numbers)),
          _cell(42, Text('${(entry.value * 100 / total).round()}%', style: numbers)),
        ],
      ),
    );
  }
}

class _TopProductsTable extends StatelessWidget {
  final List<_Product> products;

  /// Davrdagi yakunlanmagan buyurtmalar soni — ro'yxat bo'sh bo'lganda
  /// sababini aytish uchun.
  final int inProgress;

  const _TopProductsTable({required this.products, required this.inProgress});

  static const _visible = 5;

  @override
  Widget build(BuildContext context) {
    return _Panel(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const _CardTitle(
            icon: Icons.emoji_events_rounded,
            title: 'Eng ko\'p sotilgan mahsulotlar',
            hint: 'Bajarilgan buyurtmalar. Summa menyu narxida — buyurtma darajasidagi '
                'aksiya chegirmalarisiz, shuning uchun jami tushumdan farq qilishi mumkin.',
          ),
          const SizedBox(height: 10),
          Expanded(
            child: products.isEmpty
                ? _EmptyText(inProgress > 0
                    ? 'Yakunlangan sotuv yo\'q · ${_count(inProgress)} buyurtma jarayonda'
                    : 'Bu davrda sotilgan mahsulot yo\'q')
                : LayoutBuilder(builder: (context, c) {
                    final showCategory = c.maxWidth >= 480;
                    const headerHeight = 28.0;
                    final rows = products.take(_visible).toList();
                    final rowHeight = math.min(44.0, (c.maxHeight - headerHeight) / _visible);
                    return Column(
                      children: [
                        SizedBox(
                          height: headerHeight,
                          child: DecoratedBox(
                            decoration: const BoxDecoration(
                              border:
                                  Border(bottom: BorderSide(color: OnDexColors.cardBorder)),
                            ),
                            child: Row(
                              children: [
                                _cell(26, const Text('#', style: _headStyle)),
                                const SizedBox(width: 40),
                                const Expanded(
                                  child: Text('Mahsulot nomi',
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                      style: _headStyle),
                                ),
                                if (showCategory)
                                  const SizedBox(
                                    width: 118,
                                    child: Text('Kategoriya',
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: _headStyle),
                                  ),
                                _cell(70, const Text('Sotilgan', style: _headStyle)),
                                _cell(112, const Text('Summa', style: _headStyle)),
                              ],
                            ),
                          ),
                        ),
                        for (var i = 0; i < rows.length; i++)
                          SizedBox(
                            height: rowHeight,
                            child: _TopProductRow(
                              rank: i + 1,
                              product: rows[i],
                              showCategory: showCategory,
                              last: i == rows.length - 1,
                            ),
                          ),
                      ],
                    );
                  }),
          ),
        ],
      ),
    );
  }
}

class _TopProductRow extends StatelessWidget {
  final int rank;
  final _Product product;
  final bool showCategory;
  final bool last;

  const _TopProductRow({
    required this.rank,
    required this.product,
    required this.showCategory,
    required this.last,
  });

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        border: last
            ? null
            : Border(
                bottom: BorderSide(color: OnDexColors.cardBorder.withValues(alpha: 0.55))),
      ),
      child: Row(
        children: [
          _cell(
              26,
              Text('$rank',
                  style: const TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w700,
                      color: OnDexColors.primary))),
          _ProductThumb(url: product.imageUrl),
          const SizedBox(width: 10),
          Expanded(
            child: Padding(
              padding: const EdgeInsets.only(right: 8),
              child: Text(product.name.isEmpty ? 'Nomsiz taom' : product.name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      fontSize: 13, fontWeight: FontWeight.w600, color: OnDexColors.ink)),
            ),
          ),
          if (showCategory)
            SizedBox(
              width: 118,
              child: Padding(
                padding: const EdgeInsets.only(right: 8),
                child: Text(product.category.isEmpty ? '—' : product.category,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
              ),
            ),
          _cell(
              70,
              Text(_count(product.qty),
                  style: const TextStyle(fontSize: 12.5, color: OnDexColors.ink))),
          _cell(
              112,
              Text(formatSum(product.salesTiyin),
                  style: const TextStyle(
                      fontSize: 13, fontWeight: FontWeight.w700, color: OnDexColors.ink))),
        ],
      ),
    );
  }
}

class _RecentOrdersTable extends StatelessWidget {
  /// `null` — ro'yxat hali kelmagan (yon panel so'rovi davom etmoqda).
  final List<Map<String, dynamic>>? orders;
  final VoidCallback onSeeAll;

  const _RecentOrdersTable({required this.orders, required this.onSeeAll});

  static const _visible = 5;

  @override
  Widget build(BuildContext context) {
    final source = orders;
    final list = source == null
        ? null
        : ([...source]
              ..sort((a, b) => (parseOrderAt(b['created_at']) ?? DateTime(0))
                  .compareTo(parseOrderAt(a['created_at']) ?? DateTime(0))))
            .take(_visible)
            .toList();
    return _Panel(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _CardTitle(
            icon: Icons.schedule_rounded,
            title: 'So\'nggi buyurtmalar',
            hint: 'Tanlangan davrdan qat\'i nazar — eng oxirgi kelgan buyurtmalar.',
            trailing: TextButton(
              onPressed: onSeeAll,
              style: TextButton.styleFrom(
                foregroundColor: OnDexColors.primary,
                padding: const EdgeInsets.symmetric(horizontal: 8),
                minimumSize: const Size(0, 32),
                tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                visualDensity: VisualDensity.compact,
                textStyle: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700),
              ),
              child: const Text('Barchasini ko\'rish →'),
            ),
          ),
          const SizedBox(height: 10),
          Expanded(
            child: list == null
                ? const Center(
                    child: SizedBox(
                        width: 22, height: 22, child: CircularProgressIndicator(strokeWidth: 2)))
                : list.isEmpty
                    ? const _EmptyText('Hali buyurtma yo\'q')
                    : LayoutBuilder(builder: (context, c) {
                        final showCustomer = c.maxWidth >= 640;
                        final showTime = c.maxWidth >= 440;
                        const headerHeight = 28.0;
                        final rowHeight =
                            math.min(44.0, (c.maxHeight - headerHeight) / _visible);
                        return Column(
                          children: [
                            SizedBox(
                              height: headerHeight,
                              child: _OrderTableHeader(
                                  showCustomer: showCustomer, showTime: showTime),
                            ),
                            for (var i = 0; i < list.length; i++)
                              SizedBox(
                                height: rowHeight,
                                child: _RecentOrderRow(
                                  order: list[i],
                                  showCustomer: showCustomer,
                                  showTime: showTime,
                                  last: i == list.length - 1,
                                ),
                              ),
                          ],
                        );
                      }),
          ),
        ],
      ),
    );
  }
}

// Jadval ustunlari — "So'nggi buyurtmalar" va "Barcha buyurtmalar" da
// bir xil: sarlavha va qatorlar shu kengliklardan chiziladi.
const _colId = 86.0;
const _colTime = 80.0;
const _colDateTime = 138.0;
const _colCustomer = 128.0;
const _colSum = 102.0;
const _colStatus = 110.0;

class _OrderTableHeader extends StatelessWidget {
  final bool showCustomer;
  final bool showTime;

  /// Vaqt ustuni sana bilan ("Barcha buyurtmalar").
  final bool fullDate;

  const _OrderTableHeader({
    required this.showCustomer,
    required this.showTime,
    this.fullDate = false,
  });

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: const BoxDecoration(
        border: Border(bottom: BorderSide(color: OnDexColors.cardBorder)),
      ),
      child: Row(
        children: [
          _cell(_colId, const Text('ID', style: _headStyle)),
          if (showTime)
            fullDate
                ? _cell(_colDateTime, const Text('Sana va vaqt', style: _headStyle))
                : _cell(_colTime, const Text('Vaqt', style: _headStyle)),
          if (showCustomer) _cell(_colCustomer, const Text('Mijoz', style: _headStyle)),
          const Expanded(
            child: Text('Mahsulotlar',
                maxLines: 1, overflow: TextOverflow.ellipsis, style: _headStyle),
          ),
          _cell(_colSum, const Text('Summa', style: _headStyle)),
          _cell(_colStatus, const Text('Holat', style: _headStyle)),
        ],
      ),
    );
  }
}

class _RecentOrderRow extends StatelessWidget {
  final Map<String, dynamic> order;
  final bool showCustomer;
  final bool showTime;
  final bool last;

  /// "Barcha buyurtmalar": vaqt sana bilan, taomlar to'liq (sichqoncha
  /// ustida — ro'yxat).
  final bool fullDate;

  const _RecentOrderRow({
    required this.order,
    required this.showCustomer,
    required this.showTime,
    required this.last,
    this.fullDate = false,
  });

  @override
  Widget build(BuildContext context) {
    const dim = TextStyle(fontSize: 12.5, color: OnDexColors.inkDim);
    final (label, color, bg) = orderStatusStyle(_str(order['status']));
    return DecoratedBox(
      decoration: BoxDecoration(
        border: last
            ? null
            : Border(
                bottom: BorderSide(color: OnDexColors.cardBorder.withValues(alpha: 0.55))),
      ),
      child: Row(
        children: [
          _cell(
              _colId,
              Text(shortOrderNumber(order),
                  style: const TextStyle(
                      fontSize: 12.5, fontWeight: FontWeight.w600, color: OnDexColors.ink))),
          if (showTime)
            fullDate
                ? _cell(_colDateTime,
                    Text(_orderDateTime(parseOrderAt(order['created_at'])), style: dim))
                : _cell(_colTime, Text(_orderTime(parseOrderAt(order['created_at'])), style: dim)),
          if (showCustomer)
            SizedBox(
              width: _colCustomer,
              child: Padding(
                padding: const EdgeInsets.only(right: 8),
                child: Text(_customerText(order),
                    maxLines: 1, overflow: TextOverflow.ellipsis, style: dim),
              ),
            ),
          Expanded(
            child: Padding(
              padding: const EdgeInsets.only(right: 8),
              child: fullDate
                  ? _ItemsCell(order: order)
                  : Text(_itemsText(order),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 12.5, color: OnDexColors.ink)),
            ),
          ),
          _cell(
              _colSum,
              Text(formatSum(_int(order['total_tiyin'])),
                  style: const TextStyle(
                      fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.ink))),
          _cell(
            _colStatus,
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(999)),
              child: Text(label,
                  style: TextStyle(fontSize: 11.5, color: color, fontWeight: FontWeight.w700)),
            ),
          ),
        ],
      ),
    );
  }
}

class _InsightsStrip extends StatelessWidget {
  final _Stats data;

  const _InsightsStrip({required this.data});

  static const _dim = TextStyle(fontSize: 11.5, color: OnDexColors.inkDim);

  @override
  Widget build(BuildContext context) {
    final c = data.current;
    final p = data.previous;
    final start = data.busyStartHour;
    final end = data.busyEndHour;
    final weekday = data.busyWeekday;

    final items = <Widget>[
      _Insight(
        icon: Icons.schedule_rounded,
        label: 'Eng gavjum vaqt',
        value: start == null || end == null ? '—' : '${_hour(start)} - ${_hour(end)}',
        footer: Text(start == null ? 'Buyurtma yo\'q' : _mealTime(start),
            maxLines: 1, overflow: TextOverflow.ellipsis, style: _dim),
      ),
      _Insight(
        icon: Icons.calendar_month_outlined,
        label: 'Eng faol kun',
        value: weekday == null ? '—' : _weekdays[weekday - 1],
        footer: Text(
            weekday == null
                ? 'Buyurtma yo\'q'
                : 'Kuniga o\'rtacha ${_decimal(data.busyWeekdayAvg)} ta buyurtma',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: _dim),
      ),
      _Insight(
        icon: Icons.person_add_alt_outlined,
        label: 'Yangi mijozlar',
        value: _count(c.newCustomers),
        footer: _DeltaLine(delta: _Delta(c.newCustomers, p.newCustomers)),
      ),
      _Insight(
        icon: Icons.groups_outlined,
        label: 'Qaytgan mijozlar',
        value: _count(c.returningCustomers),
        footer: _DeltaLine(delta: _Delta(c.returningCustomers, p.returningCustomers)),
      ),
      _Insight(
        icon: Icons.timer_outlined,
        label: 'O\'rtacha tayyorlash vaqti',
        value: '${c.avgPrepMinutes ?? 0} daqiqa',
        footer: _DeltaLine(
          delta: c.avgPrepMinutes == null
              ? const _Delta.missing('Tayyorlangan buyurtma yo\'q')
              : p.avgPrepMinutes == null
                  ? _Delta.none
                  : _Delta(c.avgPrepMinutes!, p.avgPrepMinutes!, higherIsBetter: false),
        ),
      ),
    ];

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 14),
      decoration: BoxDecoration(
        color: _insightsBg,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: LayoutBuilder(builder: (context, constraints) {
        if (constraints.maxWidth >= 1040) {
          return Row(
            children: [
              for (var i = 0; i < items.length; i++) ...[
                if (i > 0) Container(width: 1, height: 54, color: OnDexColors.cardBorder),
                Expanded(child: items[i]),
              ],
            ],
          );
        }
        return _ResponsiveWrap(minItemWidth: 230, spacing: 10, children: items);
      }),
    );
  }
}

class _Insight extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;
  final Widget footer;

  const _Insight({
    required this.icon,
    required this.label,
    required this.value,
    required this.footer,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 2),
      child: Row(
        children: [
          Container(
            width: 42,
            height: 42,
            decoration: BoxDecoration(
              color: OnDexColors.primaryTint,
              borderRadius: BorderRadius.circular(12),
            ),
            child: Icon(icon, size: 21, color: OnDexColors.primary),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                const SizedBox(height: 2),
                FittedBox(
                  fit: BoxFit.scaleDown,
                  alignment: Alignment.centerLeft,
                  child: Text(value,
                      maxLines: 1,
                      style: const TextStyle(
                          fontSize: 17,
                          fontWeight: FontWeight.w800,
                          color: OnDexColors.ink)),
                ),
                const SizedBox(height: 2),
                footer,
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ─── PDF hisobot ────────────────────────────────────────────────────────

/// Eksport — PDF hisobot, tizimning chop etish oynasi orqali (u yerda
/// "PDF sifatida saqlash" ham bor). Stollar sahifasidagi QR chop etish
/// bilan bir xil yo'l (`Printing.layoutPdf`) — Windows va veb'da sinalgan.
///
/// ┌─ SHRIFT ───────────────────────────────────────────────────────────┐
/// PDF'ning standart shrifti (Helvetica) faqat lotin harflarini biladi,
/// taom nomlarida esa kirill ham uchraydi ("Кока-кола"). Shuning uchun
/// avval Roboto yuklanadi (`printing` uni keshlaydi). Internet bo'lmasa
/// hisobot baribir tayyorlanadi: lotindan tashqari belgilar "?" bilan
/// almashtiriladi — eksport butunlay ishlamay qolgandan yaxshi.
/// └────────────────────────────────────────────────────────────────────┘
Future<pw.Document> _buildReport(_Stats d, String restaurantName) async {
  pw.ThemeData? theme;
  try {
    final fonts = await Future.wait([
      PdfGoogleFonts.robotoRegular(),
      PdfGoogleFonts.robotoBold(),
    ]).timeout(const Duration(seconds: 10));
    theme = pw.ThemeData.withFont(base: fonts[0], bold: fonts[1]);
  } catch (_) {
    theme = null;
  }
  final unicode = theme != null;
  String safe(String s) =>
      unicode ? s : String.fromCharCodes(s.runes.map((r) => r <= 0xFF ? r : 0x3F));

  final c = d.current;
  final p = d.previous;
  final generatedAt = DateTime.now();
  final name = restaurantName.trim().isEmpty ? 'Restoran' : restaurantName.trim();

  String change(int? current, int? previous) {
    if (current == null || previous == null || previous == 0) return '-';
    final v = ((current - previous) * 100 / previous.abs()).round();
    return v > 0 ? '+$v%' : '$v%';
  }

  String sum(int? tiyin) => tiyin == null ? '-' : formatSum(tiyin);
  String minutes(int? m) => m == null ? '-' : '$m daqiqa';

  final busyHours = d.busyStartHour == null || d.busyEndHour == null
      ? '-'
      : '${_hour(d.busyStartHour!)} - ${_hour(d.busyEndHour!)}';
  final busyDay = d.busyWeekday == null
      ? '-'
      : '${_weekdays[d.busyWeekday! - 1]} (o\'rtacha ${_decimal(d.busyWeekdayAvg)} ta)';

  final doc = pw.Document(
    theme: theme,
    title: 'OnDex statistika',
    author: 'OnDex',
    creator: 'OnDex restoran paneli',
  );
  doc.addPage(
    pw.MultiPage(
      pageFormat: PdfPageFormat.a4,
      margin: const pw.EdgeInsets.fromLTRB(32, 32, 32, 28),
      // Standart chegara 20 sahifa: bir yillik kunlik jadvalda oshib,
      // eksport xato bilan to'xtardi.
      maxPages: 100,
      footer: (context) => pw.Container(
        alignment: pw.Alignment.centerRight,
        margin: const pw.EdgeInsets.only(top: 8),
        child: pw.Text('OnDex  ·  ${context.pageNumber} / ${context.pagesCount}',
            style: const pw.TextStyle(fontSize: 8, color: PdfColors.grey600)),
      ),
      build: (context) => [
        pw.Text(safe(name),
            style: const pw.TextStyle(fontSize:18, fontWeight: pw.FontWeight.bold)),
        pw.SizedBox(height: 3),
        pw.Text('Statistika hisoboti: ${_dateLong(d.from)} - ${_dateLong(d.to)}',
            style: const pw.TextStyle(fontSize: 11)),
        pw.SizedBox(height: 2),
        pw.Text(
          'Solishtirish davri: ${_dateLong(d.previousFrom)} - ${_dateLong(d.previousTo)}. '
          'Tayyorlangan: ${_dateLong(generatedAt)}, ${_clock(generatedAt)}. '
          'Vaqt: Toshkent (UTC+5).',
          style: const pw.TextStyle(fontSize: 8.5, color: PdfColors.grey700),
        ),
        _pdfSection('Asosiy ko\'rsatkichlar'),
        _pdfTable(
          headers: const ['Ko\'rsatkich', 'Joriy davr', 'O\'tgan davr', 'O\'zgarish'],
          rows: [
            ['Jami tushum', sum(c.revenueTiyin), sum(p.revenueTiyin), change(c.revenueTiyin, p.revenueTiyin)],
            ['Jami buyurtmalar', _count(c.orders), _count(p.orders), change(c.orders, p.orders)],
            ['O\'rtacha buyurtma', sum(c.avgOrderTiyin ?? 0), sum(p.avgOrderTiyin ?? 0), change(c.avgOrderTiyin, p.avgOrderTiyin)],
            ['Bajarilgan buyurtmalar', _count(c.completed), _count(p.completed), change(c.completed, p.completed)],
            ['Jarayonda', _count(c.inProgress), _count(p.inProgress), change(c.inProgress, p.inProgress)],
            ['Jarayondagi summa (tushumga kirmagan)', sum(c.inProgressTiyin), sum(p.inProgressTiyin), change(c.inProgressTiyin, p.inProgressTiyin)],
            ['Bekor qilinganlar', _count(c.cancelled), _count(p.cancelled), change(c.cancelled, p.cancelled)],
            ['Yangi mijozlar', _count(c.newCustomers), _count(p.newCustomers), change(c.newCustomers, p.newCustomers)],
            ['Qaytgan mijozlar', _count(c.returningCustomers), _count(p.returningCustomers), change(c.returningCustomers, p.returningCustomers)],
            ['O\'rtacha tayyorlash vaqti', minutes(c.avgPrepMinutes ?? 0), minutes(p.avgPrepMinutes ?? 0), change(c.avgPrepMinutes, p.avgPrepMinutes)],
            ['Eng gavjum vaqt', busyHours, '-', '-'],
            ['Eng faol kun', busyDay, '-', '-'],
          ],
        ),
        _pdfSection('${_granularityLabel(d.granularity)} dinamika'),
        _pdfTable(
          headers: const ['Davr', 'Buyurtmalar', 'Tushum'],
          rows: [
            for (final pt in d.series)
              [
                d.granularity == 'day'
                    ? _dateLong(pt.start)
                    : _rangeText(pt.start, pt.end, dash: '-'),
                _count(pt.orders),
                formatSum(pt.revenueTiyin),
              ],
          ],
        ),
        _pdfSection('Sotuvlar bo\'yicha kategoriyalar'),
        if (d.categories.isEmpty)
          _pdfNote('Bu davrda sotuv yo\'q.')
        else
          _pdfTable(
            headers: const ['Turkum', 'Soni', 'Ulushi', 'Sotuv (menyu narxida)'],
            rows: [
              for (final cat in d.categories)
                [
                  safe(cat.name),
                  _count(cat.qty),
                  _percent(cat.qty, d.soldQty),
                  formatSum(cat.salesTiyin),
                ],
            ],
          ),
        _pdfSection('Eng ko\'p sotilgan mahsulotlar'),
        if (d.topProducts.isEmpty)
          _pdfNote('Bu davrda sotilgan mahsulot yo\'q.')
        else
          _pdfTable(
            headers: const ['#', 'Mahsulot', 'Kategoriya', 'Soni', 'Summa (menyu narxida)'],
            rightAlignFrom: 3,
            rows: [
              for (var i = 0; i < d.topProducts.length; i++)
                [
                  '${i + 1}',
                  safe(d.topProducts[i].name.isEmpty ? 'Nomsiz taom' : d.topProducts[i].name),
                  safe(d.topProducts[i].category.isEmpty ? '-' : d.topProducts[i].category),
                  _count(d.topProducts[i].qty),
                  formatSum(d.topProducts[i].salesTiyin),
                ],
            ],
          ),
        if (d.topProductsTotal > d.topProducts.length)
          _pdfNote('Eng ko\'p sotilgan ${d.topProducts.length} tasi ko\'rsatilgan '
              '(jami ${d.topProductsTotal} ta mahsulot).'),
        pw.SizedBox(height: 14),
        _pdfNote(
          'Izoh: tushum va sotuvlar faqat bajarilgan (yetkazilgan yoki stolga berilgan) '
          'buyurtmalardan hisoblanadi. To\'lanmagan karta buyurtmalari hisobga olinmaydi. '
          '"Summa (menyu narxida)" buyurtma darajasidagi aksiya chegirmalarini o\'z ichiga '
          'olmaydi, shuning uchun jami tushumdan farq qilishi mumkin.',
        ),
      ],
    ),
  );
  return doc;
}

pw.Widget _pdfSection(String title) => pw.Padding(
      padding: const pw.EdgeInsets.only(top: 16, bottom: 6),
      child: pw.Text(title,
          style: const pw.TextStyle(fontSize:12.5, fontWeight: pw.FontWeight.bold)),
    );

pw.Widget _pdfNote(String text) => pw.Padding(
      padding: const pw.EdgeInsets.only(top: 4),
      child: pw.Text(text,
          style: const pw.TextStyle(fontSize: 8.5, color: PdfColors.grey700)),
    );

pw.Widget _pdfTable({
  required List<String> headers,
  required List<List<String>> rows,
  int rightAlignFrom = 1,
}) {
  final right = {
    for (var i = rightAlignFrom; i < headers.length; i++) i: pw.Alignment.centerRight,
  };
  return pw.TableHelper.fromTextArray(
    headers: headers,
    data: rows,
    border: const pw.TableBorder(
      horizontalInside: pw.BorderSide(color: PdfColors.grey300, width: 0.5),
      bottom: pw.BorderSide(color: PdfColors.grey400, width: 0.5),
    ),
    headerStyle: const pw.TextStyle(
        fontSize: 9, fontWeight: pw.FontWeight.bold, color: PdfColors.grey800),
    headerDecoration: const pw.BoxDecoration(color: PdfColors.grey200),
    cellStyle: const pw.TextStyle(fontSize: 9),
    cellHeight: 18,
    cellAlignment: pw.Alignment.centerLeft,
    headerAlignment: pw.Alignment.centerLeft,
    cellAlignments: right,
    headerAlignments: right,
  );
}
