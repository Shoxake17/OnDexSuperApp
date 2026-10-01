import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../api.dart';
import '../theme.dart';
import '../widgets/charts.dart';
import '../widgets/date_range_dialog.dart';
import '../widgets/page_header.dart';
import '../widgets/panel_cards.dart';

/// "Moliya" sahifasi — joylashuv `image/Moliya.png` namunasi bo'yicha.
///
/// ┌─ RAQAM QAYERDAN KELADI ───────────────────────────────────────────┐
/// `GET /restaurants/{id}/finance` → to'lov shlyuzi (Multicard/Rahmat)
/// reestri. BIZNING buyurtmalar jadvalidan EMAS: restoran uchun haqiqiy
/// pul — hisobiga tushgan pul, "bajarilgan buyurtma" esa to'lov o'tganini
/// kafolatlamaydi (naqd, bekor qilingan to'lov, qaytarish).
///
/// Shu sababdan bu sahifadagi summalar "Statistika" dagi tushumdan farq
/// qilishi MUMKIN va bu xato emas — ikkisi turli narsani o'lchaydi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// ┌─ NAMUNADAN ATAYLAB FARQ QILADIGAN JOYLAR ─────────────────────────┐
/// Maketdagi "Jami xarajatlar", "Sof foyda" va "Xarajatlar taqsimoti"
/// CHIZILMAYDI: tizimda xarajat tushunchasi yo'q. Ularni ko'rsatish
/// uchun o'ylab topilgan raqam kerak bo'lardi, bu esa moliyaviy
/// sahifada — yolg'on. O'rniga shlyuzdan KELADIGAN haqiqiy
/// ko'rsatkichlar turadi: qaytarilgan summa, shlyuz komissiyasi va
/// to'lov turlari bo'yicha taqsimot.
///
/// "Oylik hisobot" bloki ham yo'q — hisobot fayli hali yaratilmaydi;
/// tugma bosilib hech narsa bo'lmasligidan ko'ra, yo'qligi ma'qul.
/// └───────────────────────────────────────────────────────────────────┘
class FinancePage extends StatefulWidget {
  const FinancePage({super.key});

  /// Sahifa xotirasini tozalaydi. FAQAT testlar uchun: xotira sahifa
  /// ob'ektidan tashqarida (`static`) turgani uchun testlar bir-biriga
  /// ta'sir qilmasligi kerak.
  @visibleForTesting
  static void debugClearMemory() => _FinanceMemory.clear();

  @override
  State<FinancePage> createState() => _FinancePageState();
}

// ─── Sozlamalar ─────────────────────────────────────────────────────────

const _pagePadding = EdgeInsets.fromLTRB(20, 18, 20, 18);
const _chartsRowHeight = 292.0;
const _tableHeight = 320.0;

/// Server chegarasi (`financeMaxDays`) — undan uzun davr 400 qaytaradi,
/// shuning uchun kalendar oyna ham shu chegarada ushlaydi.
const _maxDays = 366;

/// To'lov turlari halqasining ranglari.
const _methodPalette = <Color>[
  OnDexColors.primary,
  Color(0xFF2F6FDE),
  OnDexColors.success,
  Color(0xFFF5C21B),
  OnDexColors.purple,
];
const _otherMethodColor = Color(0xFFB9AFA3);

const _monthsFull = [
  'yanvar', 'fevral', 'mart', 'aprel', 'may', 'iyun', //
  'iyul', 'avgust', 'sentabr', 'oktabr', 'noyabr', 'dekabr',
];
const _monthsShort = [
  'yan', 'fev', 'mar', 'apr', 'may', 'iyun', //
  'iyul', 'avg', 'sen', 'okt', 'noy', 'dek',
];

String _dateLong(DateTime d) => '${d.day}-${_monthsFull[d.month - 1]}, ${d.year}';

String _dateShort(DateTime d) => '${d.day}-${_monthsShort[d.month - 1]}';

String _count(int n) {
  final digits = n
      .abs()
      .toString()
      .replaceAllMapped(RegExp(r'(\d)(?=(\d{3})+$)'), (m) => '${m[1]} ');
  return '${n < 0 ? '-$digits' : digits} ta';
}

String _compactSom(double som) {
  String trim(double v) =>
      v == v.roundToDouble() ? v.toStringAsFixed(0) : v.toStringAsFixed(1);
  if (som <= 0) return '0';
  if (som >= 1000000) return '${trim(som / 1000000)}M';
  if (som >= 1000) return '${trim(som / 1000)}k';
  return som.toStringAsFixed(0);
}

DateTime _dateOnly(DateTime d) => DateTime(d.year, d.month, d.day);

/// `"2026-09-30"` yoki `"2026-09-30 14:36:31"` → `DateTime`. Shlyuz
/// vaqtni Toshkent mintaqasida beradi va mintaqa belgisisiz — shuning
/// uchun MAHALLIY vaqt deb o'qiladi (panel ham Toshkentda ishlaydi).
DateTime? _parseAt(String s) {
  final t = s.trim();
  if (t.length < 10) return null;
  return DateTime.tryParse(t.replaceFirst(' ', 'T'));
}

String _clock(DateTime t) =>
    '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';

/// To'lov tizimi kodi → ko'rinadigan nom. Noma'lum kod O'ZGARTIRILMAY
/// chiqadi: shlyuz yangi tizim qo'shsa, u "Boshqa" bo'lib yo'qolmaydi.
String _methodLabel(String ps) => switch (ps.toLowerCase()) {
      'uzcard' => 'UzCard',
      'humo' => 'Humo',
      'payme' => 'Payme',
      'click' => 'Click',
      'visa' => 'Visa',
      'mastercard' => 'Mastercard',
      'rahmat' => 'Rahmat',
      'boshqa' || '' => 'Boshqa',
      _ => ps,
    };

/// Tranzaksiya holati → (yorliq, matn rangi, fon rangi).
(String, Color, Color) _txnStatusStyle(String status) => switch (status) {
      'success' => ('Tushdi', OnDexColors.success, OnDexColors.successBg),
      'revert' => ('Qaytarildi', OnDexColors.warning, OnDexColors.warningBg),
      'cancelled' => ('Bekor qilindi', OnDexColors.danger, OnDexColors.dangerBg),
      'error' => ('Xato', OnDexColors.danger, OnDexColors.dangerBg),
      'hold' => ('Ushlab turilgan', OnDexColors.info, OnDexColors.infoBg),
      'draft' || 'billing' || 'progress' => (
          'Jarayonda',
          OnDexColors.info,
          OnDexColors.infoBg
        ),
      _ => (status, OnDexColors.inkDim, OnDexColors.pageBg),
    };

// ─── Ma'lumot ───────────────────────────────────────────────────────────

int _int(Object? v) => switch (v) {
      int() => v,
      num() => v.toInt(),
      String() => int.tryParse(v) ?? 0,
      _ => 0,
    };

class _Day {
  final DateTime date;
  final int amountTiyin;
  final int count;

  const _Day(this.date, this.amountTiyin, this.count);

  static _Day fromJson(Map<String, dynamic> j) => _Day(
        DateTime.tryParse(j['date'] as String? ?? '') ?? DateTime(2000),
        _int(j['amount_tiyin']),
        _int(j['count']),
      );
}

class _Method {
  final String ps;
  final int amountTiyin;
  final int count;

  const _Method(this.ps, this.amountTiyin, this.count);

  static _Method fromJson(Map<String, dynamic> j) => _Method(
        j['ps'] as String? ?? '',
        _int(j['amount_tiyin']),
        _int(j['count']),
      );
}

class _Txn {
  final String uuid;
  final String status;
  final String ps;
  final String orderRef;
  final int amountTiyin;
  final int commissionTiyin;
  final DateTime? at;

  const _Txn({
    required this.uuid,
    required this.status,
    required this.ps,
    required this.orderRef,
    required this.amountTiyin,
    required this.commissionTiyin,
    required this.at,
  });

  static _Txn fromJson(Map<String, dynamic> j) => _Txn(
        uuid: j['uuid'] as String? ?? '',
        status: j['status'] as String? ?? '',
        ps: j['ps'] as String? ?? '',
        orderRef: j['order_ref'] as String? ?? '',
        amountTiyin: _int(j['amount_tiyin']),
        commissionTiyin: _int(j['commission_tiyin']),
        at: _parseAt(j['at'] as String? ?? ''),
      );
}

class _Finance {
  final bool connected;
  final bool available;
  final String storeId;

  /// Restoran O'Z shartnomasi bilan ulanganmi (`false` + `connected` —
  /// OnDex shartnomasi ostida).
  final bool ownCredentials;
  final String appId;
  final String storeTitle;
  final String accountMasked;

  final DateTime from;
  final DateTime to;
  final int turnoverTiyin;
  final int refundedTiyin;
  final int commissionTiyin;
  final int netTiyin;
  final int transactions;
  final int prevTurnoverTiyin;
  final int prevTransactions;
  final List<_Day> daily;
  final List<_Method> byMethod;
  final List<_Txn> recent;

  const _Finance({
    required this.connected,
    required this.available,
    required this.storeId,
    required this.ownCredentials,
    required this.appId,
    required this.storeTitle,
    required this.accountMasked,
    required this.from,
    required this.to,
    required this.turnoverTiyin,
    required this.refundedTiyin,
    required this.commissionTiyin,
    required this.netTiyin,
    required this.transactions,
    required this.prevTurnoverTiyin,
    required this.prevTransactions,
    required this.daily,
    required this.byMethod,
    required this.recent,
  });

  static _Finance fromJson(Map<String, dynamic> j) {
    List<Map<String, dynamic>> list(Object? v) => (v as List<dynamic>? ?? [])
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .toList();
    final today = _dateOnly(DateTime.now());
    return _Finance(
      connected: j['connected'] == true,
      available: j['available'] == true,
      storeId: j['store_id'] as String? ?? '',
      ownCredentials: j['own_credentials'] == true,
      appId: j['app_id'] as String? ?? '',
      storeTitle: j['store_title'] as String? ?? '',
      accountMasked: j['account_masked'] as String? ?? '',
      from: DateTime.tryParse(j['from'] as String? ?? '') ?? today,
      to: DateTime.tryParse(j['to'] as String? ?? '') ?? today,
      turnoverTiyin: _int(j['turnover_tiyin']),
      refundedTiyin: _int(j['refunded_tiyin']),
      commissionTiyin: _int(j['commission_tiyin']),
      netTiyin: _int(j['net_tiyin']),
      transactions: _int(j['transactions']),
      prevTurnoverTiyin: _int(j['prev_turnover_tiyin']),
      prevTransactions: _int(j['prev_transactions']),
      daily: [for (final d in list(j['daily'])) _Day.fromJson(d)],
      byMethod: [for (final m in list(j['by_method'])) _Method.fromJson(m)],
      recent: [for (final t in list(j['recent'])) _Txn.fromJson(t)],
    );
  }
}

// ─── Sahifa xotirasi ────────────────────────────────────────────────────

/// Sahifa QAYTA YARATILGANDA ham saqlanadigan oxirgi natijalar.
///
/// ┌─ NEGA "FAQAT MOLIYA AYLANADI" ────────────────────────────────────┐
/// `RestaurantShell` har bir bo'limni kirganda YANGIDAN quradi
/// (`IndexedStack` yo'q), ya'ni sahifaning holati har safar yo'qoladi.
/// Boshqa sahifalar uchun bu sezilmaydi: ularning ma'lumoti bizning
/// serverdan 0–70 ms da keladi. Moliya esa TASHQI shlyuzdan oladi
/// (7 kun ~0.8 s, 30 kun ~4–5 s) va `data == null` bo'lgani uchun har
/// kirganda to'liq ekranli aylanuvchi chiqardi.
///
/// Endi oxirgi javob shu yerda saqlanadi: sahifa darhol ESKI raqamlarni
/// ko'rsatadi, yangisi esa orqada yuklanadi (tepada ingichka chiziq).
/// Tanlangan davr ham eslab qolinadi — qaytganda "Bugun"ga tushib
/// ketmaydi.
///
/// Kalit restoran IDsini o'z ichiga oladi: boshqa restoran bilan
/// kirilsa, eskisining raqamlari ko'rinmaydi.
/// └───────────────────────────────────────────────────────────────────┘
class _FinanceMemory {
  _FinanceMemory._();

  static final Map<String, _Finance> _byKey = {};
  static String? _rid;
  static int? _preset;
  static DateTimeRange? _customRange;

  static String _iso(DateTime d) =>
      '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

  static String key(DateTimeRange r) => '${api.rid}|${_iso(r.start)}|${_iso(r.end)}';

  static _Finance? read(DateTimeRange r) => _byKey[key(r)];

  static void write(DateTimeRange r, _Finance data) => _byKey[key(r)] = data;

  /// Oxirgi tanlangan davr (faqat shu restoran uchun).
  static int? presetFor() => _rid == api.rid ? _preset : null;
  static DateTimeRange? customRangeFor() => _rid == api.rid ? _customRange : null;

  static void rememberSelection(int preset, DateTimeRange range, {required bool custom}) {
    _rid = api.rid;
    _preset = preset;
    _customRange = custom ? range : null;
  }

  static void clear() {
    _byKey.clear();
    _rid = null;
    _preset = null;
    _customRange = null;
  }
}

// ─── Sahifa ─────────────────────────────────────────────────────────────

class _FinancePageState extends State<FinancePage> {
  /// Tayyor davrlar (namunadagi tugmalar). `null` — "Maxsus".
  static const _presets = <int?>[1, 7, 30, 90, null];
  static const _presetLabels = <String>[
    'Bugun',
    '7 kun',
    '30 kun',
    '90 kun',
    'Maxsus'
  ];

  int _preset = 1; // 7 kun
  late DateTimeRange _range = _rangeOfDays(7);

  _Finance? _data;

  /// Ko'rsatilayotgan ma'lumot QAYSI davrga tegishli (`_FinanceMemory.key`).
  /// Tanlangan davrdan farq qilsa — raqamlar eskirgan va xira chiziladi;
  /// bir xil bo'lsa (qaytib kirganda) — to'liq ko'rinadi, orqada
  /// jimgina yangilanadi.
  String? _dataKey;
  bool _loading = true;
  String? _error;

  /// Oxirgi ma'lum kassa raqami va `application_id`.
  ///
  /// Xato yuz berganda `_data` bo'sh bo'lishi mumkin, lekin sababi
  /// ko'pincha AYNAN shu qiymatlar bo'ladi — shuning uchun ularni
  /// sozlamalardan alohida olamiz va tuzatish tugmasini ko'rsatamiz.
  String _storeId = '';
  String _appId = '';

  /// Yuklashlar ketma-ketligi: kech kelgan eski javob yangisini
  /// almashtirib qo'ymasin (davr tez almashtirilganda ko'rinadigan xato).
  int _seq = 0;

  static DateTimeRange _rangeOfDays(int days) {
    final to = _dateOnly(DateTime.now());
    return DateTimeRange(start: to.subtract(Duration(days: days - 1)), end: to);
  }

  @override
  void initState() {
    super.initState();
    // Oxirgi tanlangan davrni tiklaymiz: nisbiy davrlar ("7 kun") BUGUNGI
    // sanadan qayta hisoblanadi, "Maxsus" esa o'z oralig'i bilan.
    final saved = _FinanceMemory.presetFor();
    if (saved != null) {
      _preset = saved;
      final days = _presets[saved];
      final custom = _FinanceMemory.customRangeFor();
      if (days != null) {
        _range = _rangeOfDays(days);
      } else if (custom != null) {
        _range = custom;
      }
    }
    _adoptMemory();
    _load();
  }

  /// Tanlangan davr uchun xotirada natija bo'lsa — darhol ko'rsatadi.
  void _adoptMemory() {
    final cached = _FinanceMemory.read(_range);
    if (cached == null) return;
    _data = cached;
    _dataKey = _FinanceMemory.key(_range);
    _storeId = cached.storeId;
    _appId = cached.appId;
  }

  /// Ulanish o'zgargach (ulandi/uzildi) eski natijalar YOLG'ON bo'ladi:
  /// ular boshqa hisobga tegishli edi. Xotira tozalanadi va sahifa
  /// ma'lumotni noldan yuklaydi.
  void _reloadAfterConnectionChange() {
    _FinanceMemory.clear();
    setState(() {
      _data = null;
      _dataKey = null;
    });
    _load();
  }

  Future<void> _load() async {
    final seq = ++_seq;
    final requested = _range;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final j = await api.finance(from: requested.start, to: requested.end);
      if (!mounted || seq != _seq) return;
      final data = _Finance.fromJson(j);
      _FinanceMemory.write(requested, data);
      setState(() {
        _data = data;
        _dataKey = _FinanceMemory.key(requested);
        _storeId = data.storeId;
        _appId = data.appId;
        _loading = false;
      });
    } catch (e) {
      // Raqamni sozlamalardan olamiz: xato sababi ko'pincha shu raqam
      // bo'ladi va foydalanuvchiga uni tuzatish yo'li ko'rsatilishi
      // kerak. Bu qo'shimcha so'rov FAQAT xato yo'lida ketadi va o'zi
      // yiqilsa e'tiborsiz qoldiriladi — asosiy xatoni yashirmasin.
      var store = _storeId;
      var app = _appId;
      if (store.isEmpty) {
        try {
          final s = await api.restaurantSettings();
          store = s['multicard_store_id'] as String? ?? '';
          app = s['multicard_app_id'] as String? ?? '';
        } catch (_) {}
      }
      if (!mounted || seq != _seq) return;
      setState(() {
        _error = '$e';
        _storeId = store;
        _appId = app;
        _loading = false;
      });
    }
  }

  void _selectPreset(int i) {
    final days = _presets[i];
    if (days == null) {
      _pickCustom();
      return;
    }
    setState(() {
      _preset = i;
      _range = _rangeOfDays(days);
      _adoptMemory(); // oldin ochilgan davr bo'lsa — aylanmasdan
    });
    _FinanceMemory.rememberSelection(i, _range, custom: false);
    _load();
  }

  Future<void> _pickCustom() async {
    final picked = await showOnDexDateRangePicker(
      context: context,
      initialRange: _range,
      firstDate: DateTime(2023),
      lastDate: _dateOnly(DateTime.now()),
      maxDays: _maxDays,
    );
    if (picked == null || !mounted) return;
    setState(() {
      _preset = _presets.length - 1;
      _range = DateTimeRange(
          start: _dateOnly(picked.start), end: _dateOnly(picked.end));
      _adoptMemory();
    });
    _FinanceMemory.rememberSelection(_preset, _range, custom: true);
    _load();
  }

  @override
  Widget build(BuildContext context) {
    final data = _data;
    return Stack(
      children: [
        SingleChildScrollView(
          padding: _pagePadding,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              PageHeader(
                title: 'Moliya',
                subtitle: 'Hisobingizga tushgan to\'lovlar, qaytarishlar va '
                    'to\'lov turlari',
                action: _PeriodChips(
                  labels: _presetLabels,
                  selected: _preset,
                  onSelect: _selectPreset,
                  customLabel: _preset == _presets.length - 1
                      ? '${_dateShort(_range.start)} – ${_dateShort(_range.end)}'
                      : null,
                ),
              ),
              const SizedBox(height: 18),
              if (data == null)
                _loading
                    ? const SizedBox(
                        height: 220,
                        child: Center(child: CircularProgressIndicator()))
                    : ErrorPanel(
                        icon: Icons.account_balance_wallet_rounded,
                        message: 'Moliya ma\'lumotini yuklab bo\'lmadi: '
                            '${_error ?? 'noma\'lum xato'}',
                        onRetry: _load,
                        // Sabab noto'g'ri kalit yoki kassa raqami bo'lishi
                        // mumkin — uni qayta urinish TUZATMAYDI.
                        action: _storeId.isEmpty
                            ? null
                            : OutlinedButton.icon(
                                onPressed: () => _showConnectDialog(context,
                                    appId: _appId,
                                    storeId: _storeId,
                                    onSaved: _reloadAfterConnectionChange),
                                icon: const Icon(Icons.edit_outlined, size: 17),
                                label: const Text('Kalitlarni o\'zgartirish'),
                              ),
                      )
              else ...[
                if (_error != null)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 12),
                    child: ErrorBanner(
                      message: 'Yangi ma\'lumot yuklanmadi: $_error. '
                          'Ko\'rsatilayotgan davr: ${_dateLong(data.from)} — '
                          '${_dateLong(data.to)}.',
                      onRetry: _load,
                    ),
                  ),
                // Yangi davr yuklanayotganda eski raqamlar xira: ular
                // tanlangan davrga tegishli emasligi ko'rinib tursin.
                AnimatedOpacity(
                  opacity: _loading && _dataKey != _FinanceMemory.key(_range) ? 0.55 : 1,
                  duration: const Duration(milliseconds: 150),
                  child: data.connected
                      ? _buildConnected(data)
                      : _ConnectCard(
                          available: data.available,
                          storeId: data.storeId,
                          onConnected: _reloadAfterConnectionChange,
                        ),
                ),
              ],
            ],
          ),
        ),
        if (_loading && _data != null)
          const Positioned(
            top: 0,
            left: 0,
            right: 0,
            child: LinearProgressIndicator(minHeight: 2),
          ),
      ],
    );
  }

  Widget _buildConnected(_Finance d) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _buildKpis(d),
        const SizedBox(height: kPanelGap),
        SplitRow(
          height: _chartsRowHeight,
          leftFlex: 13,
          rightFlex: 7,
          left: _DailyChartCard(days: d.daily),
          right: _MethodsCard(methods: d.byMethod),
        ),
        const SizedBox(height: kPanelGap),
        SizedBox(height: _tableHeight, child: _TxnTable(txns: d.recent)),
        const SizedBox(height: kPanelGap),
        _NotShownNote(data: d, onChanged: _reloadAfterConnectionChange),
      ],
    );
  }

  Widget _buildKpis(_Finance d) {
    // ┌─ NEGA "SOF FOYDA" EMAS ────────────────────────────────────────┐
    // Maketda "Sof foyda" bor. Foyda = tushum − xarajat, xarajat esa
    // tizimda YO'Q. Shuning uchun kartochka "Sof tushum" deb ataladi va
    // ta'rifi izohda aniq yozilgan: tushum − qaytarilgan.
    // └────────────────────────────────────────────────────────────────┘
    return ResponsiveWrap(
      minItemWidth: 190,
      spacing: 12,
      children: [
        KpiCard(
          label: 'Umumiy tushum',
          value: formatSum(d.turnoverTiyin),
          icon: Icons.payments_outlined,
          color: OnDexColors.primary,
          background: OnDexColors.primaryTint,
          delta: Delta(d.turnoverTiyin, d.prevTurnoverTiyin),
          hint: 'To\'lov shlyuzida MUVAFFAQIYATLI o\'tgan to\'lovlar '
              'yig\'indisi. Naqd to\'lovlar bu yerga kirmaydi — ular '
              'shlyuzdan o\'tmaydi.',
        ),
        KpiCard(
          label: 'Qaytarilgan',
          value: formatSum(d.refundedTiyin),
          icon: Icons.undo_rounded,
          color: OnDexColors.warning,
          background: OnDexColors.warningBg,
          delta: null,
          caption: 'Shu davrda qaytarilgan to\'lovlar',
          hint: 'Mijozga qaytarilgan summa (shlyuzdagi `revert` holati).',
        ),
        KpiCard(
          label: 'Sof tushum',
          value: formatSum(d.netTiyin),
          icon: Icons.account_balance_wallet_outlined,
          color: OnDexColors.success,
          background: OnDexColors.successBg,
          delta: null,
          caption: 'Tushum − qaytarilgan',
          hint: 'Bu SOF FOYDA emas: xarajatlar (mahsulot, ish haqi, '
              'kommunal) tizimda yuritilmaydi, shuning uchun hisobga '
              'olinmagan.',
        ),
        KpiCard(
          label: 'Tranzaksiyalar',
          value: _count(d.transactions),
          icon: Icons.receipt_long_outlined,
          color: OnDexColors.info,
          background: OnDexColors.infoBg,
          delta: Delta(d.transactions, d.prevTransactions),
          hint: 'Muvaffaqiyatli to\'lovlar soni.',
        ),
      ],
    );
  }
}

// ─── Davr tugmalari ─────────────────────────────────────────────────────

class _PeriodChips extends StatelessWidget {
  final List<String> labels;
  final int selected;
  final ValueChanged<int> onSelect;

  /// "Maxsus" tanlangan bo'lsa — tanlangan sanalar oralig'i.
  final String? customLabel;

  const _PeriodChips({
    required this.labels,
    required this.selected,
    required this.onSelect,
    this.customLabel,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      // `Wrap`, `Row` emas: tor oynada sarlavha ostiga tushganda beshta
      // tugma bir qatorga sig'masdi va o'ng chetdan chiqib ketardi.
      child: Wrap(
        spacing: 4,
        runSpacing: 4,
        children: [
          for (var i = 0; i < labels.length; i++)
            _Chip(
              label: i == labels.length - 1 && customLabel != null
                  ? customLabel!
                  : labels[i],
              icon: i == labels.length - 1 ? Icons.calendar_today_rounded : null,
              active: i == selected,
              onTap: () => onSelect(i),
            ),
        ],
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  final String label;
  final IconData? icon;
  final bool active;
  final VoidCallback onTap;

  const _Chip({
    required this.label,
    required this.active,
    required this.onTap,
    this.icon,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(9),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        decoration: BoxDecoration(
          color: active ? OnDexColors.primary : Colors.transparent,
          borderRadius: BorderRadius.circular(9),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (icon != null) ...[
              Icon(icon,
                  size: 14, color: active ? Colors.white : OnDexColors.inkDim),
              const SizedBox(width: 7),
            ],
            Text(label,
                style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w700,
                    color: active ? Colors.white : OnDexColors.inkDim)),
          ],
        ),
      ),
    );
  }
}

// ─── Kunlik tushum grafigi ──────────────────────────────────────────────

class _DailyChartCard extends StatelessWidget {
  final List<_Day> days;

  const _DailyChartCard({required this.days});

  @override
  Widget build(BuildContext context) {
    return PanelCard(
      padding: const EdgeInsets.fromLTRB(16, 14, 14, 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const CardTitle(
            icon: Icons.show_chart_rounded,
            title: 'Kunlik tushum',
            hint: 'Muvaffaqiyatli to\'lovlar, to\'lov kuni bo\'yicha '
                '(Toshkent vaqti). Qaytarishlar bu grafikdan ayirilmaydi — '
                'ular alohida ko\'rsatkichda.',
          ),
          const SizedBox(height: 8),
          Expanded(
            child: OnDexLineChart(
              points: [
                for (final d in days)
                  ChartPoint(
                    axisLabel: _dateShort(d.date),
                    value: d.amountTiyin / 100,
                    tooltipTitle: _dateLong(d.date),
                    tooltipValue: '${formatSum(d.amountTiyin)} · '
                        '${_count(d.count)}',
                  ),
              ],
              color: OnDexColors.primary,
              axisLabel: _compactSom,
              unitLabel: 'so\'m',
              emptyText: 'Bu davrda karta orqali to\'lov bo\'lmagan',
            ),
          ),
        ],
      ),
    );
  }
}

// ─── To'lov turlari ─────────────────────────────────────────────────────

class _MethodsCard extends StatelessWidget {
  final List<_Method> methods;

  const _MethodsCard({required this.methods});

  /// Eng ko'p ishlatilganlar, qolgani bitta "Boshqa" bo'lagi.
  static List<LegendEntry> _entries(List<_Method> methods) {
    final slots = _methodPalette.length;
    final shown =
        methods.length <= slots ? methods : methods.take(slots - 1).toList();
    final entries = [
      for (var i = 0; i < shown.length; i++)
        LegendEntry(
            _methodLabel(shown[i].ps), shown[i].amountTiyin, _methodPalette[i]),
    ];
    final total = methods.fold<int>(0, (s, m) => s + m.amountTiyin);
    final rest = total - shown.fold<int>(0, (s, m) => s + m.amountTiyin);
    if (rest > 0) {
      entries.add(LegendEntry(
          'Boshqa',
          rest,
          shown.length < slots
              ? _methodPalette[shown.length]
              : _otherMethodColor));
    }
    return entries;
  }

  @override
  Widget build(BuildContext context) {
    final total = methods.fold<int>(0, (s, m) => s + m.amountTiyin);
    final entries = _entries(methods);
    return PanelCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const CardTitle(
            icon: Icons.pie_chart_outline_rounded,
            title: 'To\'lov turlari',
            hint: 'Muvaffaqiyatli to\'lovlarning to\'lov tizimi bo\'yicha '
                'taqsimoti. Turni shlyuz aniqlaydi.',
          ),
          const SizedBox(height: 10),
          Expanded(
            child: total == 0
                ? const EmptyText('Bu davrda karta orqali to\'lov bo\'lmagan')
                : LayoutBuilder(builder: (context, c) {
                    final diameter =
                        math.min(170.0, math.min(c.maxHeight, c.maxWidth * 0.42));
                    return Row(
                      children: [
                        OnDexDonutChart(
                          diameter: diameter,
                          thickness: diameter * 0.18,
                          segments: [
                            for (final e in entries)
                              DonutSegment(e.value.toDouble(), e.color),
                          ],
                          center: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              const Text('Jami',
                                  style: TextStyle(
                                      fontSize: 12, color: OnDexColors.inkDim)),
                              const SizedBox(height: 2),
                              Text(formatSum(total),
                                  style: const TextStyle(
                                      fontSize: 16,
                                      fontWeight: FontWeight.w800,
                                      color: OnDexColors.ink)),
                            ],
                          ),
                        ),
                        const SizedBox(width: 18),
                        Expanded(
                          child: SingleChildScrollView(
                            child: Column(
                              children: [
                                for (final e in entries)
                                  LegendLine(
                                    entry: e,
                                    total: total,
                                    valueText: formatSum(e.value.toInt()),
                                    valueWidth: 108,
                                  ),
                              ],
                            ),
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

// ─── So'nggi operatsiyalar ──────────────────────────────────────────────

class _TxnTable extends StatelessWidget {
  final List<_Txn> txns;

  const _TxnTable({required this.txns});

  static const _wDate = 132.0;
  static const _wMethod = 108.0;
  static const _wStatus = 128.0;
  static const _wAmount = 150.0;

  /// Shundan tor bo'lganda beshta ustun sig'maydi (132+108+150+128 va
  /// ustiga "Buyurtma") — qator IKKI SATRLI ko'rinishga o'tadi. Ilgari
  /// u shunchaki o'ng chetdan chiqib ketardi.
  static const _compactBelow = 700.0;

  @override
  Widget build(BuildContext context) {
    return PanelCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const CardTitle(
            icon: Icons.receipt_long_rounded,
            title: 'So\'nggi operatsiyalar',
            hint: 'Shlyuzdagi oxirgi yozuvlar — HAMMA holat bilan, '
                'jumladan o\'tmagan to\'lovlar.',
          ),
          const SizedBox(height: 10),
          Expanded(
            child: LayoutBuilder(builder: (context, c) {
              final compact = c.maxWidth < _compactBelow;
              return Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (!compact) ...[
                    const _TxnHeader(
                        wDate: _wDate,
                        wMethod: _wMethod,
                        wAmount: _wAmount,
                        wStatus: _wStatus),
                    const Divider(height: 14, color: OnDexColors.cardBorder),
                  ],
                  Expanded(
                    child: txns.isEmpty
                        ? const EmptyText('Bu davrda operatsiya yo\'q')
                        : ListView.separated(
                            itemCount: txns.length,
                            separatorBuilder: (_, __) => const Divider(
                                height: 12, color: OnDexColors.cardBorder),
                            itemBuilder: (_, i) => compact
                                ? _TxnRowCompact(txn: txns[i])
                                : _TxnRow(
                                    txn: txns[i],
                                    wDate: _wDate,
                                    wMethod: _wMethod,
                                    wAmount: _wAmount,
                                    wStatus: _wStatus,
                                  ),
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

class _TxnHeader extends StatelessWidget {
  final double wDate;
  final double wMethod;
  final double wAmount;
  final double wStatus;

  const _TxnHeader({
    required this.wDate,
    required this.wMethod,
    required this.wAmount,
    required this.wStatus,
  });

  @override
  Widget build(BuildContext context) => Row(
        children: [
          panelCell(wDate, const Text('Sana / Vaqt', style: kTableHeadStyle)),
          panelCell(wMethod, const Text('Turi', style: kTableHeadStyle)),
          const Expanded(child: Text('Buyurtma', style: kTableHeadStyle)),
          panelCell(wAmount, const Text('Summa', style: kTableHeadStyle)),
          panelCell(wStatus, const Text('Holat', style: kTableHeadStyle)),
        ],
      );
}

/// Qaytarilgan to'lov MINUS bilan yoziladi: jadvalda ular tushum
/// qatorlari bilan aralashib turadi va belgisiz farqi ko'rinmasdi.
String _signedAmount(_Txn t) => t.status == 'revert'
    ? '-${formatSum(t.amountTiyin)}'
    : formatSum(t.amountTiyin);

Color _amountColor(_Txn t) =>
    t.status == 'revert' ? OnDexColors.warning : OnDexColors.ink;

String _whenText(_Txn t) =>
    t.at == null ? '—' : '${_dateShort(t.at!)}, ${_clock(t.at!)}';

class _StatusPill extends StatelessWidget {
  final String status;

  const _StatusPill(this.status);

  @override
  Widget build(BuildContext context) {
    final (label, color, background) = _txnStatusStyle(status);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
          color: background, borderRadius: BorderRadius.circular(20)),
      child: Text(label,
          style: TextStyle(
              fontSize: 11.5, fontWeight: FontWeight.w700, color: color)),
    );
  }
}

class _TxnRow extends StatelessWidget {
  final _Txn txn;
  final double wDate;
  final double wMethod;
  final double wAmount;
  final double wStatus;

  const _TxnRow({
    required this.txn,
    required this.wDate,
    required this.wMethod,
    required this.wAmount,
    required this.wStatus,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        panelCell(
          wDate,
          Text(_whenText(txn),
              style: const TextStyle(fontSize: 12.5, color: OnDexColors.ink)),
        ),
        panelCell(
          wMethod,
          Text(_methodLabel(txn.ps),
              style:
                  const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
        ),
        Expanded(
          child: Text(
            txn.orderRef.isEmpty ? '—' : txn.orderRef,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim),
          ),
        ),
        panelCell(
          wAmount,
          Text(_signedAmount(txn),
              style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w700,
                  color: _amountColor(txn))),
        ),
        panelCell(wStatus, _StatusPill(txn.status)),
      ],
    );
  }
}

/// Tor oyna uchun: ikki satr — yuqorida sana va summa, pastda buyurtma
/// havolasi, to'lov turi va holat.
class _TxnRowCompact extends StatelessWidget {
  final _Txn txn;

  const _TxnRowCompact({required this.txn});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(_whenText(txn),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 12.5,
                        fontWeight: FontWeight.w600,
                        color: OnDexColors.ink)),
              ),
              const SizedBox(width: 8),
              Text(_signedAmount(txn),
                  style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w700,
                      color: _amountColor(txn))),
            ],
          ),
          const SizedBox(height: 4),
          Row(
            children: [
              Expanded(
                child: Text(
                  txn.orderRef.isEmpty
                      ? _methodLabel(txn.ps)
                      : '${_methodLabel(txn.ps)} · ${txn.orderRef}',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      fontSize: 12, color: OnDexColors.inkDim),
                ),
              ),
              const SizedBox(width: 8),
              _StatusPill(txn.status),
            ],
          ),
        ],
      ),
    );
  }
}

// ─── Nima ko'rsatilmaydi ────────────────────────────────────────────────

/// Sahifaning oxiridagi halol izoh: nima YO'Q va nega.
///
/// Bu bezak emas. Restoran "hisobimda qancha pul bor" degan savol bilan
/// keladi va sahifada u raqam yo'qligini TUSHUNTIRISH kerak — aks holda
/// boshqa raqamlardan birini shunday deb o'qib oladi.
class _NotShownNote extends StatelessWidget {
  final _Finance data;
  final VoidCallback onChanged;

  const _NotShownNote({required this.data, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    return PanelCard(
      padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const CardTitle(
              icon: Icons.info_outline_rounded, title: 'Qo\'shimcha ma\'lumot'),
          const SizedBox(height: 10),
          const _Bullet(
              'Bank hisobidagi QOLDIQ bu yerda ko\'rsatilmaydi — unga '
              'OnDex\'ning kirishi yo\'q. Sahifada faqat "shu davrda '
              'qancha tushdi" ko\'rinadi.'),
          const _Bullet(
              'Xarajatlar va sof foyda hisoblanmaydi: tizimda xarajat '
              'daftari yo\'q. O\'ylab topilgan raqam chizilmaydi.'),
          const _Bullet(
              'Naqd to\'lovlar shlyuzdan o\'tmaydi, shuning uchun bu '
              'sahifada emas — ular "Statistika" bo\'limidagi tushumda.'),
          const SizedBox(height: 14),
          const Divider(height: 1, color: OnDexColors.cardBorder),
          const SizedBox(height: 12),
          _ConnectionSummary(data: data, onChanged: onChanged),
        ],
      ),
    );
  }
}

/// Ulanish holati: qaysi kassa, qaysi hisob, kim nomidan.
///
/// ┌─ NEGA HISOB RAQAMI KO'RSATILADI ──────────────────────────────────┐
/// Restoran "pulim qayerga tushyapti" degan savolga javob olishi
/// kerak. Noto'g'ri kassa raqami kiritilsa pul BEGONA hisobga ketadi
/// va buni faqat shu yozuv ko'rsatib bera oladi. To'liq raqam
/// saqlanmaydi — oxirgi to'rt raqam tanish uchun yetadi.
/// └───────────────────────────────────────────────────────────────────┘
class _ConnectionSummary extends StatelessWidget {
  final _Finance data;
  final VoidCallback onChanged;

  const _ConnectionSummary({required this.data, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    final lines = <String>[
      if (data.storeTitle.isNotEmpty)
        'Kassa: ${data.storeTitle} (№${data.storeId})'
      else
        'Kassa: №${data.storeId}',
      if (data.accountMasked.isNotEmpty) 'Hisob raqami: ${data.accountMasked}',
      data.ownCredentials
          ? 'Ulanish: o\'z shartnomangiz (${data.appId}) — '
              'pul to\'g\'ridan-to\'g\'ri sizning hisobingizga tushadi'
          : 'Ulanish: OnDex shartnomasi ostida',
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final l in lines)
          Padding(
            padding: const EdgeInsets.only(bottom: 4),
            child: Text(l,
                style: const TextStyle(
                    fontSize: 12.5, color: OnDexColors.inkDim)),
          ),
        const SizedBox(height: 8),
        Wrap(
          spacing: 10,
          runSpacing: 4,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            TextButton.icon(
              onPressed: () => _showConnectDialog(context,
                  appId: data.appId, storeId: data.storeId, onSaved: onChanged),
              icon: const Icon(Icons.edit_outlined, size: 16),
              label: const Text('Kalitlarni o\'zgartirish'),
            ),
            if (data.ownCredentials)
              TextButton.icon(
                onPressed: () => _confirmDisconnect(context, onDone: onChanged),
                icon: const Icon(Icons.link_off_rounded, size: 16),
                label: const Text('Uzish'),
                style: TextButton.styleFrom(
                    foregroundColor: OnDexColors.danger),
              ),
          ],
        ),
      ],
    );
  }
}

/// Uzish — tasdiqsiz bajarilmaydi: keyin Moliya bo'limi bo'shab qoladi
/// va kalitlarni qaytadan kiritish kerak bo'ladi.
Future<void> _confirmDisconnect(BuildContext context,
    {required VoidCallback onDone}) async {
  final yes = await showDialog<bool>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: const Text('Ulanishni uzish'),
      content: const Text(
        'To\'lov kalitlari o\'chiriladi va Moliya bo\'limi bo\'shab qoladi. '
        'Buyurtmalar va boshqa bo\'limlarga ta\'sir qilmaydi. '
        'Keyinroq kalitlarni qaytadan kiritishingiz mumkin.',
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Bekor qilish')),
        FilledButton(
          onPressed: () => Navigator.of(ctx).pop(true),
          style: FilledButton.styleFrom(backgroundColor: OnDexColors.danger),
          child: const Text('Uzish'),
        ),
      ],
    ),
  );
  if (yes != true || !context.mounted) return;
  try {
    await api.disconnectFinance();
    onDone();
  } catch (e) {
    if (!context.mounted) return;
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text('Uzib bo\'lmadi: $e')));
  }
}

class _Bullet extends StatelessWidget {
  final String text;

  const _Bullet(this.text);

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(bottom: 6),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Padding(
              padding: EdgeInsets.only(top: 6, right: 8),
              child: Icon(Icons.circle, size: 5, color: OnDexColors.inkFaint),
            ),
            Expanded(
              child: Text(text,
                  style: const TextStyle(
                      fontSize: 13, height: 1.45, color: OnDexColors.inkDim)),
            ),
          ],
        ),
      );
}

// ─── Ulash ──────────────────────────────────────────────────────────────

/// Do'kon ulanmagan holat. Bu XATO EMAS — restoran hali to'lov tizimiga
/// ulanmagan, sahifa unga nima qilish kerakligini aytadi.
class _ConnectCard extends StatelessWidget {
  /// Shlyuz OnDex tomonida sozlanganmi. `false` bo'lsa restoran o'zi
  /// hech narsa qila olmaydi.
  final bool available;
  final String storeId;
  final VoidCallback onConnected;

  const _ConnectCard({
    required this.available,
    required this.storeId,
    required this.onConnected,
  });

  @override
  Widget build(BuildContext context) {
    return PanelCard(
      padding: const EdgeInsets.fromLTRB(28, 28, 28, 28),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 46,
                height: 46,
                decoration: BoxDecoration(
                    color: OnDexColors.primaryTint,
                    borderRadius: BorderRadius.circular(12)),
                child: const Icon(Icons.link_rounded,
                    size: 24, color: OnDexColors.primary),
              ),
              const SizedBox(width: 14),
              const Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('To\'lov hisobi ulanmagan',
                        style: TextStyle(
                            fontSize: 17,
                            fontWeight: FontWeight.w800,
                            color: OnDexColors.ink)),
                    SizedBox(height: 3),
                    Text(
                        'Karta orqali to\'lovlar to\'g\'ridan-to\'g\'ri '
                        'restoran hisobiga tushadi. OnDex buyurtmalardan '
                        'foiz olmaydi.',
                        style:
                            TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 20),
          if (!available)
            const _Warning(
              'To\'lov tizimiga ulanish OnDex tomonida hali sozlanmagan. '
              'Kalitlarni kiritish hozir foyda bermaydi — OnDex bilan '
              'bog\'laning.',
            )
          else ...[
            const Text(
              'Ulash uchun nima kerak:',
              style: TextStyle(
                  fontSize: 13.5,
                  fontWeight: FontWeight.w700,
                  color: OnDexColors.ink),
            ),
            const SizedBox(height: 8),
            const _Step(1,
                'Multicard (Rahmat) bilan shartnoma tuzing — hisob '
                'raqamingiz va tashkilot rekvizitlaringiz bo\'yicha.'),
            const _Step(2,
                'Ular sizga `application_id`, `secret` va kassa raqamini '
                'beradi.'),
            const _Step(3,
                'Shu uchtasini quyidagi formaga kiriting. OnDex ularni '
                'tekshirib, pul qaysi hisobga tushishini ko\'rsatadi.'),
            const SizedBox(height: 16),
            FilledButton.icon(
              onPressed: () => _showConnectDialog(context,
                  appId: '', storeId: storeId, onSaved: onConnected),
              icon: const Icon(Icons.add_link_rounded, size: 19),
              label: const Text('Hisobni ulash'),
            ),
          ],
        ],
      ),
    );
  }
}

class _Warning extends StatelessWidget {
  final String text;

  const _Warning(this.text);

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.fromLTRB(14, 12, 14, 12),
        decoration: BoxDecoration(
            color: OnDexColors.warningBg,
            borderRadius: BorderRadius.circular(10)),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Icon(Icons.warning_amber_rounded,
                size: 18, color: OnDexColors.warning),
            const SizedBox(width: 10),
            Expanded(
              child: Text(text,
                  style:
                      const TextStyle(fontSize: 13, color: OnDexColors.ink)),
            ),
          ],
        ),
      );
}

/// Ulash bosqichi raqami bilan.
class _Step extends StatelessWidget {
  final int n;
  final String text;

  const _Step(this.n, this.text);

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(bottom: 7),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: 20,
              height: 20,
              alignment: Alignment.center,
              decoration: const BoxDecoration(
                  color: OnDexColors.primaryTint, shape: BoxShape.circle),
              child: Text('$n',
                  style: const TextStyle(
                      fontSize: 11.5,
                      fontWeight: FontWeight.w800,
                      color: OnDexColors.primary)),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Text(text,
                  style: const TextStyle(
                      fontSize: 13, height: 1.45, color: OnDexColors.inkDim)),
            ),
          ],
        ),
      );
}
// ─── Hisobni ulash oynasi ───────────────────────────────────────────────

/// To'lov hisobini ulash/kalitlarni yangilash.
///
/// ┌─ NIMA UCHUN UCHTA MAYDON ─────────────────────────────────────────┐
/// Multicard'da do'kon yaratish API'si yo'q: restoran shartnomani o'zi
/// tuzadi va `application_id`, `secret`, kassa raqamini oladi. Panel
/// aynan shularni so'raydi — bank rekvizitlarini EMAS: ular Multicard
/// tomonda, shartnomada.
///
/// ⚠️ `secret` serverga bir marta yuboriladi, u yerda SHIFRLANIB
/// saqlanadi va HECH QACHON qaytarilmaydi. Shuning uchun tahrirlashda
/// maydon bo'sh keladi va uni qaytadan kiritish kerak.
/// └───────────────────────────────────────────────────────────────────┘
Future<void> _showConnectDialog(
  BuildContext context, {
  required String appId,
  required String storeId,
  required VoidCallback onSaved,
}) async {
  final saved = await showDialog<bool>(
    context: context,
    builder: (_) => _ConnectDialog(appId: appId, storeId: storeId),
  );
  if (saved == true) onSaved();
}

class _ConnectDialog extends StatefulWidget {
  final String appId;
  final String storeId;

  const _ConnectDialog({required this.appId, required this.storeId});

  @override
  State<_ConnectDialog> createState() => _ConnectDialogState();
}

class _ConnectDialogState extends State<_ConnectDialog> {
  late final _app = TextEditingController(text: widget.appId);
  final _secret = TextEditingController();
  late final _store = TextEditingController(text: widget.storeId);
  bool _busy = false;
  bool _showSecret = false;
  String? _error;

  /// Server tekshiruvidan qaytgan natija: pul qayerga tushadi.
  /// `null` — hali ulanmagan.
  Map<String, dynamic>? _result;

  @override
  void dispose() {
    _app.dispose();
    _secret.dispose();
    _store.dispose();
    super.dispose();
  }

  Future<void> _connect() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final r = await api.connectFinance(
        applicationId: _app.text.trim(),
        secret: _secret.text.trim(),
        storeId: _store.text.trim(),
      );
      if (!mounted) return;
      // Darhol yopmaymiz: restoran PUL QAYERGA tushishini ko'rib,
      // o'zi tasdiqlashi kerak. Noto'g'ri kassa raqami aynan shu
      // qadamda tutiladi.
      setState(() {
        _busy = false;
        _result = r;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = '$e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final result = _result;
    return AlertDialog(
      title: Text(result == null ? 'To\'lov hisobini ulash' : 'Ulandi'),
      content: SizedBox(
        width: 460,
        child: SingleChildScrollView(
          child: result == null ? _form() : _summary(result),
        ),
      ),
      actions: result == null
          ? [
              TextButton(
                onPressed:
                    _busy ? null : () => Navigator.of(context).pop(false),
                child: const Text('Bekor qilish'),
              ),
              FilledButton(
                onPressed: _busy ? null : _connect,
                child: _busy
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2))
                    : const Text('Tekshirib ulash'),
              ),
            ]
          : [
              FilledButton(
                onPressed: () => Navigator.of(context).pop(true),
                child: const Text('Yopish'),
              ),
            ],
    );
  }

  Widget _form() {
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text(
          'Kalitlarni Multicard (Rahmat) shartnoma tuzilgandan keyin '
          'beradi. OnDex ularni saqlashdan oldin tekshiradi.',
          style: TextStyle(fontSize: 12.5, color: OnDexColors.inkDim),
        ),
        const SizedBox(height: 14),
        TextField(
          controller: _app,
          autofocus: true,
          enabled: !_busy,
          decoration: const InputDecoration(
            labelText: 'application_id',
            hintText: 'masalan: bookcafe_prod',
            border: OutlineInputBorder(),
          ),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: _secret,
          enabled: !_busy,
          obscureText: !_showSecret,
          decoration: InputDecoration(
            labelText: 'secret',
            helperText: widget.appId.isEmpty
                ? null
                : 'Saqlangan kalit ko\'rsatilmaydi — qaytadan kiriting',
            border: const OutlineInputBorder(),
            suffixIcon: IconButton(
              tooltip: _showSecret ? 'Yashirish' : 'Ko\'rsatish',
              icon: Icon(_showSecret
                  ? Icons.visibility_off_outlined
                  : Icons.visibility_outlined),
              onPressed: () => setState(() => _showSecret = !_showSecret),
            ),
          ),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: _store,
          enabled: !_busy,
          decoration: const InputDecoration(
            labelText: 'Kassa raqami (store_id)',
            hintText: 'masalan: 6',
            border: OutlineInputBorder(),
          ),
        ),
        const SizedBox(height: 12),
        const _Warning(
          'Kalitlar pul harakatiga kirish huquqini beradi. Ularni '
          'faqat Multicard bergan ko\'rinishda kiriting va hech kimga '
          'bermang.',
        ),
        if (_error != null) ...[
          const SizedBox(height: 12),
          Text(_error!,
              style:
                  const TextStyle(fontSize: 12.5, color: OnDexColors.danger)),
        ],
      ],
    );
  }

  Widget _summary(Map<String, dynamic> r) {
    String s(String k) => (r[k] as String?)?.trim() ?? '';
    final rows = <(String, String)>[
      ('Kassa', s('store_title').isEmpty ? '№${s('store_id')}' : '${s('store_title')} (№${s('store_id')})'),
      ('Tashkilot', s('merchant_name')),
      ('STIR', s('merchant_tin')),
      ('Hisob raqami', s('account_masked')),
      ('Shartnoma', s('contract_num')),
      ('Shlyuz komissiyasi', s('fee_percent').isEmpty ? '' : '${s('fee_percent')}%'),
    ];
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text(
          'Kalitlar tekshirildi. Karta orqali to\'lovlar quyidagi '
          'hisobga tushadi — tekshiring:',
          style: TextStyle(fontSize: 13, color: OnDexColors.ink),
        ),
        const SizedBox(height: 14),
        for (final (label, value) in rows)
          if (value.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: 150,
                    child: Text(label,
                        style: const TextStyle(
                            fontSize: 12.5, color: OnDexColors.inkDim)),
                  ),
                  Expanded(
                    child: Text(value,
                        style: const TextStyle(
                            fontSize: 13.5,
                            fontWeight: FontWeight.w700,
                            color: OnDexColors.ink)),
                  ),
                ],
              ),
            ),
        const SizedBox(height: 6),
        const Text(
          'Rekvizitlar sizniki bo\'lmasa — darhol uzing va kassa '
          'raqamini tekshiring.',
          style: TextStyle(fontSize: 12.5, color: OnDexColors.inkDim),
        ),
      ],
    );
  }
}
