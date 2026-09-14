import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../theme.dart';

/// OnDex kalendari — panelning BARCHA sana tanlagichlari shu bitta oynadan
/// (bir xil ko'rinish, bir xil tezkor tanlovlar):
///
///  * [showOnDexDateRangePicker] — davr (Statistika);
///  * [showOnDexPeriodPicker] — davr yoki "Barcha vaqt" (Barcha buyurtmalar);
///  * [showOnDexDatePicker] — bitta sana (yuqori panel);
///  * [showOnDexDateTimePicker] — sana va vaqt (Aksiyalar).
///
/// ┌─ NEGA MATERIAL'NING `showDatePicker` I EMAS ──────────────────────┐
/// U telefon uchun yasalgan: oylar uzun ro'yxatda, tezkor tanlovlar
/// yo'q, vaqt esa ALOHIDA soat siferblatli ikkinchi oynada tanlanadi.
/// Bu oyna — ish stoli uchun: chapda tezkor tanlovlar, o'ngda yonma-yon
/// ikki oy, davr sichqoncha ustida oldindan bo'yaladi, vaqt shu oynaning
/// o'zida yoziladi. Tor oynada bitta oy va tanlovlar tugmachalar
/// ko'rinishida.
/// └───────────────────────────────────────────────────────────────────┘

/// [showOnDexPeriodPicker] natijasi.
class DateRangePick {
  /// `null` — "Barcha vaqt" (sana bo'yicha cheklovsiz).
  final DateTimeRange? range;

  const DateRangePick(this.range);
}

enum _Mode { range, date, dateTime }

/// Davr (sanadan — sanagacha). [maxDays] oshsa "Qo'llash" o'chadi va
/// sababi yoziladi — tanlab bo'lgandan keyin xato ko'rsatishdan ko'ra
/// oldindan aytish yaxshi.
Future<DateTimeRange?> showOnDexDateRangePicker({
  required BuildContext context,
  required DateTimeRange initialRange,
  required DateTime firstDate,
  required DateTime lastDate,
  int? maxDays,
}) async {
  final result = await showDialog<Object>(
    context: context,
    builder: (_) => _CalendarDialog(
      mode: _Mode.range,
      title: 'Davrni tanlang',
      initialStart: initialRange.start,
      initialEnd: initialRange.end,
      firstDate: firstDate,
      lastDate: lastDate,
      maxDays: maxDays,
    ),
  );
  return result is DateRangePick ? result.range : null;
}

/// Davr YOKI "Barcha vaqt". `null` — oyna bekor qilindi;
/// `pick.range == null` — butun tarix tanlandi.
Future<DateRangePick?> showOnDexPeriodPicker({
  required BuildContext context,
  required DateTimeRange? initialRange,
  required DateTime firstDate,
  required DateTime lastDate,
}) async {
  final result = await showDialog<Object>(
    context: context,
    builder: (_) => _CalendarDialog(
      mode: _Mode.range,
      title: 'Davrni tanlang',
      initialStart: initialRange?.start,
      initialEnd: initialRange?.end,
      firstDate: firstDate,
      lastDate: lastDate,
      allowAllTime: true,
    ),
  );
  return result is DateRangePick ? result : null;
}

/// Bitta sana (vaqtsiz, kun boshi).
Future<DateTime?> showOnDexDatePicker({
  required BuildContext context,
  required DateTime initialDate,
  required DateTime firstDate,
  required DateTime lastDate,
  String title = 'Sanani tanlang',
}) async {
  final result = await showDialog<Object>(
    context: context,
    builder: (_) => _CalendarDialog(
      mode: _Mode.date,
      title: title,
      initialStart: initialDate,
      initialEnd: initialDate,
      firstDate: firstDate,
      lastDate: lastDate,
    ),
  );
  return result is DateTime ? result : null;
}

/// Sana va vaqt (soat:daqiqa) — bitta oynada.
Future<DateTime?> showOnDexDateTimePicker({
  required BuildContext context,
  required DateTime initial,
  required DateTime firstDate,
  required DateTime lastDate,
  String title = 'Sana va vaqtni tanlang',
}) async {
  final result = await showDialog<Object>(
    context: context,
    builder: (_) => _CalendarDialog(
      mode: _Mode.dateTime,
      title: title,
      initialStart: initial,
      initialEnd: initial,
      initialTime: TimeOfDay.fromDateTime(initial),
      firstDate: firstDate,
      lastDate: lastDate,
    ),
  );
  return result is DateTime ? result : null;
}

/// Vaqt (soat:daqiqa) — kalendardagi bilan bir xil maydon va tezkor
/// tanlovlar bilan (ish vaqti jadvali uchun).
Future<TimeOfDay?> showOnDexTimePicker({
  required BuildContext context,
  required TimeOfDay initial,
  String title = 'Vaqtni tanlang',
}) =>
    showDialog<TimeOfDay>(
      context: context,
      builder: (_) => _TimeDialog(initial: initial, title: title),
    );

class _TimeDialog extends StatefulWidget {
  const _TimeDialog({required this.initial, required this.title});

  final TimeOfDay initial;
  final String title;

  @override
  State<_TimeDialog> createState() => _TimeDialogState();
}

class _TimeDialogState extends State<_TimeDialog> {
  late int _hour = widget.initial.hour;
  late int _minute = widget.initial.minute;

  static const _quick = ['00:00', '07:00', '08:00', '09:00', '10:00', '18:00', '20:00', '22:00', '23:00'];

  @override
  Widget build(BuildContext context) {
    final current = '${_two(_hour)}:${_two(_minute)}';
    return Dialog(
      backgroundColor: OnDexColors.cardBg,
      surfaceTintColor: Colors.transparent,
      insetPadding: const EdgeInsets.all(20),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 380),
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(20, 16, 20, 18),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Container(
                    width: 34,
                    height: 34,
                    decoration: BoxDecoration(
                        color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(10)),
                    child: const Icon(Icons.schedule_rounded, size: 19, color: OnDexColors.primary),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(widget.title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                            fontSize: 17, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                  ),
                  IconButton(
                    tooltip: 'Yopish',
                    visualDensity: VisualDensity.compact,
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close_rounded, size: 20, color: OnDexColors.inkDim),
                  ),
                ],
              ),
              const SizedBox(height: 14),
              Center(
                child: _TimeField(
                  hour: _hour,
                  minute: _minute,
                  onHour: (h) => setState(() => _hour = h),
                  onMinute: (m) => setState(() => _minute = m),
                ),
              ),
              const SizedBox(height: 14),
              Wrap(
                spacing: 6,
                runSpacing: 6,
                children: [
                  for (final q in _quick)
                    _PresetChip(
                      label: q,
                      selected: q == current,
                      onTap: () => setState(() {
                        _hour = int.parse(q.substring(0, 2));
                        _minute = int.parse(q.substring(3));
                      }),
                    ),
                ],
              ),
              const SizedBox(height: 16),
              Wrap(
                alignment: WrapAlignment.end,
                spacing: 8,
                runSpacing: 8,
                children: [
                  TextButton(
                    onPressed: () => Navigator.of(context).pop(),
                    style: TextButton.styleFrom(
                        foregroundColor: OnDexColors.inkDim, minimumSize: const Size(0, 40)),
                    child: const Text('Bekor qilish'),
                  ),
                  FilledButton(
                    key: const ValueKey('time-apply'),
                    onPressed: () => Navigator.of(context).pop(TimeOfDay(hour: _hour, minute: _minute)),
                    style: FilledButton.styleFrom(
                      backgroundColor: OnDexColors.primary,
                      foregroundColor: Colors.white,
                      minimumSize: const Size(0, 40),
                      padding: const EdgeInsets.symmetric(horizontal: 20),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                    ),
                    child: const Text('Qo\'llash'),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ─── Yordamchilar ─────────────────────────────────────────────────────────

const _monthNames = [
  'Yanvar', 'Fevral', 'Mart', 'Aprel', 'May', 'Iyun', //
  'Iyul', 'Avgust', 'Sentabr', 'Oktabr', 'Noyabr', 'Dekabr',
];
const _monthShort = [
  'yan', 'fev', 'mar', 'apr', 'may', 'iyun', //
  'iyul', 'avg', 'sen', 'okt', 'noy', 'dek',
];
const _monthLong = [
  'yanvar', 'fevral', 'mart', 'aprel', 'may', 'iyun', //
  'iyul', 'avgust', 'sentabr', 'oktabr', 'noyabr', 'dekabr',
];
const _weekdayHeads = ['Du', 'Se', 'Ch', 'Pa', 'Ju', 'Sh', 'Ya'];

const _cellW = 40.0;
const _cellH = 38.0;
const _sideWidth = 188.0;
const _presetsBg = Color(0xFFFBF6EF);

DateTime _day(DateTime d) => DateTime(d.year, d.month, d.day);

DateTime _monthOf(DateTime d) => DateTime(d.year, d.month);

DateTime _shiftDays(DateTime t, int days) => DateTime(t.year, t.month, t.day + days);

/// Oy qo'shish — 31-martdan bir oy oldin 3-mart emas, 28/29-fevral.
DateTime _addMonths(DateTime t, int months) {
  final first = DateTime(t.year, t.month + months);
  final dim = DateUtils.getDaysInMonth(first.year, first.month);
  return DateTime(first.year, first.month, math.min(t.day, dim));
}

/// Ikkala chet ham kiradi. UTC orqali — kun 23/25 soat bo'lgan
/// mintaqada ham to'g'ri.
int _daysInclusive(DateTime a, DateTime b) =>
    DateTime.utc(b.year, b.month, b.day)
        .difference(DateTime.utc(a.year, a.month, a.day))
        .inDays +
    1;

String _format(DateTime d) => '${d.day}-${_monthShort[d.month - 1]}, ${d.year}';

String _formatLong(DateTime d) => '${d.day}-${_monthLong[d.month - 1]}, ${d.year}';

String _two(int n) => n.toString().padLeft(2, '0');

String _iso(DateTime d) => '${d.year}-${_two(d.month)}-${_two(d.day)}';

DateTimeRange _one(DateTime d) => DateTimeRange(start: d, end: d);

DateTimeRange _span(DateTime a, DateTime b) => DateTimeRange(start: a, end: b);

class _Preset {
  final String label;

  /// `null` natija — "Barcha vaqt".
  final DateTimeRange? Function(DateTime today) range;

  const _Preset(this.label, this.range);
}

final _allTimePreset = _Preset('Barcha vaqt', (_) => null);

final _rangePresets = <_Preset>[
  const _Preset('Bugun', _one),
  _Preset('Kecha', (t) => _one(_shiftDays(t, -1))),
  _Preset('Oxirgi 7 kun', (t) => _span(_shiftDays(t, -6), t)),
  _Preset('Oxirgi 30 kun', (t) => _span(_shiftDays(t, -29), t)),
  _Preset('Bu oy', (t) => _span(DateTime(t.year, t.month), t)),
  _Preset('O\'tgan oy',
      (t) => _span(DateTime(t.year, t.month - 1), DateTime(t.year, t.month, 0))),
  _Preset('Oxirgi 90 kun', (t) => _span(_shiftDays(t, -89), t)),
  _Preset('Bu yil', (t) => _span(DateTime(t.year), t)),
];

/// Bitta sana uchun. Ruxsat etilgan oraliqdan tashqaridagilari
/// ko'rsatilmaydi (masalan o'tmish sanasida "Ertaga" yo'q).
final _datePresets = <_Preset>[
  const _Preset('Bugun', _one),
  _Preset('Kecha', (t) => _one(_shiftDays(t, -1))),
  _Preset('Ertaga', (t) => _one(_shiftDays(t, 1))),
  _Preset('Bir hafta oldin', (t) => _one(_shiftDays(t, -7))),
  _Preset('Bir haftadan keyin', (t) => _one(_shiftDays(t, 7))),
  _Preset('Bir oy oldin', (t) => _one(_addMonths(t, -1))),
  _Preset('Bir oydan keyin', (t) => _one(_addMonths(t, 1))),
];

// ─── Oyna ─────────────────────────────────────────────────────────────────

class _CalendarDialog extends StatefulWidget {
  final _Mode mode;
  final String title;
  final DateTime? initialStart;
  final DateTime? initialEnd;
  final TimeOfDay? initialTime;
  final DateTime firstDate;
  final DateTime lastDate;
  final int? maxDays;
  final bool allowAllTime;

  const _CalendarDialog({
    required this.mode,
    required this.title,
    required this.initialStart,
    required this.initialEnd,
    required this.firstDate,
    required this.lastDate,
    this.initialTime,
    this.maxDays,
    this.allowAllTime = false,
  });

  @override
  State<_CalendarDialog> createState() => _CalendarDialogState();
}

class _CalendarDialogState extends State<_CalendarDialog> {
  late final DateTime _first = _day(widget.firstDate);
  late final DateTime _last = _day(widget.lastDate);
  final DateTime _today = _day(DateTime.now());

  late final List<_Preset> _presets;

  DateTime? _start;
  DateTime? _end;

  /// "Barcha vaqt" tanlangan (faqat [showOnDexPeriodPicker]).
  bool _allTime = false;

  /// Sichqoncha ustida turgan kun — tugash sanasi tanlanayotganda davr
  /// shu kungacha oldindan bo'yaladi.
  DateTime? _hover;

  /// O'ngdagi (tor oynada — yagona) ko'rinadigan oy.
  late DateTime _month;

  int _hour = 0;
  int _minute = 0;

  bool get _isRange => widget.mode == _Mode.range;

  @override
  void initState() {
    super.initState();
    final s = widget.initialStart;
    final e = widget.initialEnd;
    if (s == null || e == null) {
      _allTime = widget.allowAllTime;
      _start = _allTime ? null : _clamp(_today);
      _end = _start;
    } else {
      _start = _clamp(_day(s));
      _end = _clamp(_day(e));
      if (_end!.isBefore(_start!)) _end = _start;
    }
    _month = _monthOf(_end ?? _clamp(_today));
    _hour = widget.initialTime?.hour ?? 0;
    _minute = widget.initialTime?.minute ?? 0;
    _presets = _isRange
        ? [if (widget.allowAllTime) _allTimePreset, ..._rangePresets]
        : [
            for (final p in _datePresets)
              if (_inBounds(p.range(_today)!.start)) p,
          ];
  }

  bool _inBounds(DateTime d) => !d.isBefore(_first) && !d.isAfter(_last);

  DateTime _clamp(DateTime d) => d.isBefore(_first) ? _first : (d.isAfter(_last) ? _last : d);

  /// Boshlanish tanlangan, tugash hali yo'q.
  bool get _pickingEnd => _isRange && _start != null && _end == null;

  /// Bo'yaladigan oxirgi kun: tanlangani yoki (tanlanayotganda) sichqoncha
  /// ustidagisi.
  DateTime? get _shownEnd {
    if (_end != null) return _end;
    final h = _hover;
    if (_pickingEnd && h != null && !h.isBefore(_start!)) return h;
    return null;
  }

  _Preset? get _activePreset {
    for (final p in _presets) {
      final r = p.range(_today);
      if (r == null) {
        if (_allTime) return p;
        continue;
      }
      if (_allTime || _start == null || _end == null) continue;
      if (_clamp(r.start) == _start && _clamp(r.end) == _end) return p;
    }
    return null;
  }

  void _tapDay(DateTime d) {
    setState(() {
      _allTime = false;
      _hover = null;
      if (!_isRange) {
        _start = d;
        _end = d;
      } else if (_start == null || _end != null) {
        _start = d;
        _end = null;
      } else if (d.isBefore(_start!)) {
        _start = d;
      } else {
        _end = d;
      }
    });
  }

  void _applyPreset(_Preset p) {
    final r = p.range(_today);
    setState(() {
      _hover = null;
      if (r == null) {
        _allTime = true;
        _start = null;
        _end = null;
        _month = _monthOf(_clamp(_today));
        return;
      }
      _allTime = false;
      _start = _clamp(r.start);
      _end = _clamp(r.end);
      _month = _monthOf(_end!);
    });
  }

  void _shiftMonth(int delta) =>
      setState(() => _month = DateTime(_month.year, _month.month + delta));

  int get _selectedDays {
    final s = _start;
    return s == null ? 0 : _daysInclusive(s, _end ?? s);
  }

  bool get _tooLong {
    final max = widget.maxDays;
    return _isRange && !_allTime && max != null && _selectedDays > max;
  }

  bool get _canApply => _isRange ? (_allTime || (_start != null && !_tooLong)) : _start != null;

  void _apply() {
    if (!_canApply) return;
    final nav = Navigator.of(context);
    final s = _start;
    switch (widget.mode) {
      case _Mode.range:
        // Tugash tanlanmagan bo'lsa — bir kunlik davr.
        nav.pop(DateRangePick(_allTime ? null : DateTimeRange(start: s!, end: _end ?? s)));
      case _Mode.date:
        nav.pop(s);
      case _Mode.dateTime:
        nav.pop(DateTime(s!.year, s.month, s.day, _hour, _minute));
    }
  }

  @override
  Widget build(BuildContext context) {
    final wide = MediaQuery.sizeOf(context).width >= 760;
    return Dialog(
      backgroundColor: OnDexColors.cardBg,
      surfaceTintColor: Colors.transparent,
      insetPadding: const EdgeInsets.all(20),
      clipBehavior: Clip.antiAlias,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: wide ? 820 : 400),
        child: SingleChildScrollView(child: wide ? _wideLayout() : _narrowLayout()),
      ),
    );
  }

  Widget _wideLayout() {
    final active = _activePreset;
    // Chap panel foni `Stack` bilan: `IntrinsicHeight` ishlatilmaydi —
    // vaqt maydonidagi matn kiritgich uni qo'llamaydi.
    return Stack(
      children: [
        const Positioned(
          left: 0,
          top: 0,
          bottom: 0,
          width: _sideWidth,
          child: ColoredBox(color: _presetsBg),
        ),
        const Positioned(
          left: _sideWidth,
          top: 0,
          bottom: 0,
          width: 1,
          child: ColoredBox(color: OnDexColors.cardBorder),
        ),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(
              width: _sideWidth,
              child: Padding(
                padding: const EdgeInsets.fromLTRB(12, 22, 12, 16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const Padding(
                      padding: EdgeInsets.fromLTRB(10, 0, 10, 10),
                      child: Text('TEZKOR TANLOV',
                          style: TextStyle(
                              fontSize: 11,
                              letterSpacing: 0.6,
                              fontWeight: FontWeight.w700,
                              color: OnDexColors.inkFaint)),
                    ),
                    for (final p in _presets)
                      _PresetTile(
                        label: p.label,
                        selected: identical(p, active),
                        onTap: () => _applyPreset(p),
                      ),
                  ],
                ),
              ),
            ),
            const SizedBox(width: 1),
            Expanded(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(24, 18, 18, 18),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    _header(),
                    const SizedBox(height: 14),
                    _selectionFields(),
                    const SizedBox(height: 16),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        _monthView(DateTime(_month.year, _month.month - 1),
                            withPrev: true, withNext: false),
                        const SizedBox(width: 24),
                        _monthView(_month, withPrev: false, withNext: true),
                      ],
                    ),
                    const SizedBox(height: 14),
                    const Divider(height: 1, color: OnDexColors.cardBorder),
                    const SizedBox(height: 14),
                    _footer(),
                  ],
                ),
              ),
            ),
          ],
        ),
      ],
    );
  }

  Widget _narrowLayout() {
    final active = _activePreset;
    return Padding(
      padding: const EdgeInsets.fromLTRB(18, 16, 18, 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          _header(),
          const SizedBox(height: 12),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: [
              for (final p in _presets)
                _PresetChip(
                  label: p.label,
                  selected: identical(p, active),
                  onTap: () => _applyPreset(p),
                ),
            ],
          ),
          const SizedBox(height: 14),
          _selectionFields(),
          const SizedBox(height: 12),
          Center(child: _monthView(_month, withPrev: true, withNext: true)),
          const SizedBox(height: 12),
          const Divider(height: 1, color: OnDexColors.cardBorder),
          const SizedBox(height: 12),
          _footer(compact: true),
        ],
      ),
    );
  }

  Widget _header() {
    final icon = switch (widget.mode) {
      _Mode.range => Icons.date_range_rounded,
      _Mode.date => Icons.event_rounded,
      _Mode.dateTime => Icons.schedule_rounded,
    };
    return Row(
      children: [
        Container(
          width: 34,
          height: 34,
          decoration: BoxDecoration(
            color: OnDexColors.primaryTint,
            borderRadius: BorderRadius.circular(10),
          ),
          child: Icon(icon, size: 19, color: OnDexColors.primary),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: Text(widget.title,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                  fontSize: 17, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
        ),
        IconButton(
          tooltip: 'Yopish',
          visualDensity: VisualDensity.compact,
          onPressed: () => Navigator.of(context).pop(),
          icon: const Icon(Icons.close_rounded, size: 20, color: OnDexColors.inkDim),
        ),
      ],
    );
  }

  Widget _selectionFields() {
    String text(DateTime? d) => d == null ? 'Tanlanmagan' : _format(d);
    return switch (widget.mode) {
      _Mode.range => Row(
          children: [
            Expanded(
              child: _DateField(
                label: 'Boshlanish',
                text: _allTime ? 'Boshidan' : text(_start),
                empty: !_allTime && _start == null,
                active: !_allTime && !_pickingEnd,
              ),
            ),
            const Padding(
              padding: EdgeInsets.symmetric(horizontal: 8),
              child: Icon(Icons.arrow_forward_rounded, size: 16, color: OnDexColors.inkFaint),
            ),
            Expanded(
              child: _DateField(
                label: 'Tugash',
                text: _allTime ? 'Bugungacha' : text(_end),
                empty: !_allTime && _end == null,
                active: _pickingEnd,
              ),
            ),
          ],
        ),
      _Mode.date => _DateField(
          label: 'Sana', text: text(_start), empty: _start == null, active: true),
      _Mode.dateTime => Row(
          children: [
            Expanded(
              child: _DateField(
                  label: 'Sana', text: text(_start), empty: _start == null, active: true),
            ),
            const SizedBox(width: 10),
            _TimeField(
              hour: _hour,
              minute: _minute,
              onHour: (h) => setState(() => _hour = h),
              onMinute: (m) => setState(() => _minute = m),
            ),
          ],
        ),
    };
  }

  Widget _monthView(DateTime month, {required bool withPrev, required bool withNext}) {
    // Oldingi oyga — faqat undan oldingi oy ruxsat etilgan chegarada
    // bo'lsa; keyingisiga — o'ngdagi oy oxirgi ruxsat etilgan oydan oldin
    // bo'lsa.
    final canPrev = month.isAfter(_monthOf(_first));
    final canNext = _month.isBefore(_monthOf(_last));
    final lead = DateTime(month.year, month.month, 1).weekday - 1;
    final count = DateUtils.getDaysInMonth(month.year, month.month);

    final rows = <Widget>[];
    var day = 1 - lead;
    // Har doim 6 qator: oy almashganda oyna balandligi sakramasin.
    for (var r = 0; r < 6; r++) {
      rows.add(Row(
        children: [
          for (var c = 0; c < 7; c++, day++)
            day < 1 || day > count
                ? const SizedBox(width: _cellW, height: _cellH)
                : _dayCell(DateTime(month.year, month.month, day)),
        ],
      ));
    }

    return SizedBox(
      width: _cellW * 7,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            height: 36,
            child: Row(
              children: [
                if (withPrev)
                  _NavButton(
                    icon: Icons.chevron_left_rounded,
                    tooltip: 'Oldingi oy',
                    onTap: canPrev ? () => _shiftMonth(-1) : null,
                  )
                else
                  const SizedBox(width: 34),
                Expanded(
                  child: Text('${_monthNames[month.month - 1]} ${month.year}',
                      textAlign: TextAlign.center,
                      style: const TextStyle(
                          fontSize: 14.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                ),
                if (withNext)
                  _NavButton(
                    icon: Icons.chevron_right_rounded,
                    tooltip: 'Keyingi oy',
                    onTap: canNext ? () => _shiftMonth(1) : null,
                  )
                else
                  const SizedBox(width: 34),
              ],
            ),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              for (final w in _weekdayHeads)
                SizedBox(
                  width: _cellW,
                  height: 22,
                  child: Center(
                    child: Text(w,
                        style: const TextStyle(
                            fontSize: 11.5,
                            fontWeight: FontWeight.w600,
                            color: OnDexColors.inkFaint)),
                  ),
                ),
            ],
          ),
          const SizedBox(height: 4),
          ...rows,
        ],
      ),
    );
  }

  Widget _dayCell(DateTime d) {
    final disabled = !_inBounds(d);
    final s = _start;
    final e = _shownEnd;
    // Tugash hali tanlanmagan — bo'yoq faqat oldindan ko'rinish.
    final preview = _end == null;
    final isStart = s != null && d == s;
    final isEnd = e != null && d == e;
    final inside = s != null && e != null && d.isAfter(s) && d.isBefore(e);
    final hasSpan = s != null && e != null && e != s;
    final band = preview
        ? OnDexColors.primaryTint.withValues(alpha: 0.55)
        : OnDexColors.primaryTint;

    final filled = isStart || (isEnd && !preview);
    final isToday = d == _today;
    final hovered = _hover == d && !filled && !disabled;

    Color? ringColor;
    if (!filled && isEnd) {
      ringColor = OnDexColors.primary;
    } else if (!filled && isToday) {
      ringColor = OnDexColors.primary.withValues(alpha: 0.55);
    }

    return SizedBox(
      key: ValueKey('range-day-${_iso(d)}'),
      width: _cellW,
      height: _cellH,
      child: Stack(
        children: [
          if (hasSpan && (inside || isStart || isEnd))
            Positioned(
              left: 0,
              right: 0,
              top: 3,
              bottom: 3,
              child: Row(
                children: [
                  Expanded(
                      child: ColoredBox(
                          color: inside || isEnd ? band : Colors.transparent)),
                  Expanded(
                      child: ColoredBox(
                          color: inside || isStart ? band : Colors.transparent)),
                ],
              ),
            ),
          Center(
            child: MouseRegion(
              cursor: disabled ? SystemMouseCursors.forbidden : SystemMouseCursors.click,
              onEnter: disabled ? null : (_) => setState(() => _hover = d),
              onExit: (_) {
                if (_hover == d) setState(() => _hover = null);
              },
              child: GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTap: disabled ? null : () => _tapDay(d),
                onDoubleTap: disabled || _isRange
                    ? null
                    : () {
                        _tapDay(d);
                        _apply();
                      },
                child: AnimatedContainer(
                  duration: const Duration(milliseconds: 120),
                  width: 34,
                  height: 32,
                  alignment: Alignment.center,
                  decoration: BoxDecoration(
                    color: filled
                        ? OnDexColors.primary
                        : hovered
                            ? OnDexColors.pageBg
                            : Colors.transparent,
                    borderRadius: BorderRadius.circular(10),
                    border: ringColor == null ? null : Border.all(color: ringColor, width: 1.4),
                  ),
                  child: Text(
                    '${d.day}',
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: filled || isToday ? FontWeight.w700 : FontWeight.w500,
                      color: disabled
                          ? OnDexColors.inkFaint.withValues(alpha: 0.45)
                          : filled
                              ? Colors.white
                              : OnDexColors.ink,
                    ),
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _footer({bool compact = false}) {
    final start = _start;
    final String info;
    var color = OnDexColors.inkDim;
    var icon = Icons.touch_app_outlined;

    if (_isRange) {
      final days = _selectedDays;
      if (_allTime) {
        info = 'Barcha vaqt — sana bo\'yicha cheklovsiz';
        color = OnDexColors.ink;
        icon = Icons.all_inclusive_rounded;
      } else if (start == null) {
        info = 'Boshlanish sanasini tanlang';
      } else if (_tooLong) {
        info = 'Davr ${widget.maxDays} kundan oshmasin — tanlangani $days kun';
        color = OnDexColors.danger;
        icon = Icons.error_outline_rounded;
      } else if (_pickingEnd) {
        info = 'Tugash sanasini tanlang';
      } else {
        info = '$days kun tanlandi';
        color = OnDexColors.ink;
        icon = Icons.event_available_rounded;
      }
    } else if (start == null) {
      info = 'Sanani tanlang';
    } else {
      final time = widget.mode == _Mode.dateTime ? ' · ${_two(_hour)}:${_two(_minute)}' : '';
      info = 'Tanlangan: ${_formatLong(start)}$time';
      color = OnDexColors.ink;
      icon = Icons.event_available_rounded;
    }

    final status = Row(
      children: [
        Icon(icon, size: 17, color: color),
        const SizedBox(width: 7),
        Expanded(
          child: Text(info,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: color)),
        ),
      ],
    );
    final cancel = TextButton(
      onPressed: () => Navigator.of(context).pop(),
      style: TextButton.styleFrom(
        foregroundColor: OnDexColors.inkDim,
        minimumSize: const Size(0, 40),
        padding: const EdgeInsets.symmetric(horizontal: 14),
      ),
      child: const Text('Bekor qilish'),
    );
    final apply = FilledButton(
      onPressed: _canApply ? _apply : null,
      style: FilledButton.styleFrom(
        backgroundColor: OnDexColors.primary,
        foregroundColor: Colors.white,
        minimumSize: const Size(0, 40),
        padding: const EdgeInsets.symmetric(horizontal: 20),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
        textStyle: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w700),
      ),
      child: const Text('Qo\'llash'),
    );

    // Tor oynada holat yozuvi va tugmalar alohida qatorda, tugmalar esa
    // sig'masa keyingi qatorga o'tadi: bitta qatorda ular yozuvni siqib,
    // oxirgisi oynadan chiqib ketardi (katta shrift/masshtabda ham).
    if (compact) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          status,
          const SizedBox(height: 10),
          Wrap(
            alignment: WrapAlignment.end,
            spacing: 6,
            runSpacing: 6,
            children: [cancel, apply],
          ),
        ],
      );
    }
    return Row(
      children: [
        Expanded(child: status),
        cancel,
        const SizedBox(width: 6),
        apply,
      ],
    );
  }
}

// ─── Kichik vidjetlar ─────────────────────────────────────────────────────

class _DateField extends StatelessWidget {
  final String label;
  final String text;
  final bool empty;
  final bool active;

  const _DateField({
    required this.label,
    required this.text,
    required this.empty,
    required this.active,
  });

  @override
  Widget build(BuildContext context) {
    return AnimatedContainer(
      duration: const Duration(milliseconds: 150),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: active ? OnDexColors.primaryTint.withValues(alpha: 0.4) : OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(
          color: active ? OnDexColors.primary : OnDexColors.cardBorder,
          width: active ? 1.5 : 1,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(label, style: const TextStyle(fontSize: 11, color: OnDexColors.inkDim)),
          const SizedBox(height: 2),
          Text(text,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                  fontSize: 13.5,
                  fontWeight: FontWeight.w700,
                  color: empty ? OnDexColors.inkFaint : OnDexColors.ink)),
        ],
      ),
    );
  }
}

/// Vaqt: soat va daqiqa — yoziladi yoki ▲▼ bilan o'zgartiriladi
/// (klaviaturada ↑/↓ ham ishlaydi).
class _TimeField extends StatelessWidget {
  final int hour;
  final int minute;
  final ValueChanged<int> onHour;
  final ValueChanged<int> onMinute;

  const _TimeField({
    required this.hour,
    required this.minute,
    required this.onHour,
    required this.onMinute,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 5, 6, 3),
      decoration: BoxDecoration(
        color: OnDexColors.cardBg,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          const Text('Vaqt', style: TextStyle(fontSize: 11, color: OnDexColors.inkDim)),
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              _NumberBox(fieldKey: 'time-hour', value: hour, max: 23, onChanged: onHour),
              const Padding(
                padding: EdgeInsets.symmetric(horizontal: 2),
                child: Text(':',
                    style: TextStyle(
                        fontSize: 15, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
              ),
              _NumberBox(fieldKey: 'time-minute', value: minute, max: 59, onChanged: onMinute),
            ],
          ),
        ],
      ),
    );
  }
}

class _NumberBox extends StatefulWidget {
  final String fieldKey;
  final int value;
  final int max;
  final ValueChanged<int> onChanged;

  const _NumberBox({
    required this.fieldKey,
    required this.value,
    required this.max,
    required this.onChanged,
  });

  @override
  State<_NumberBox> createState() => _NumberBoxState();
}

class _NumberBoxState extends State<_NumberBox> {
  late final TextEditingController _ctrl = TextEditingController(text: _two(widget.value));
  final _focus = FocusNode();

  @override
  void initState() {
    super.initState();
    // Maydondan chiqqanda "7" → "07": qiymat har doim ikki xonali ko'rinsin.
    _focus.addListener(() {
      if (!_focus.hasFocus) _ctrl.text = _two(widget.value);
    });
  }

  @override
  void didUpdateWidget(covariant _NumberBox oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!_focus.hasFocus && _ctrl.text != _two(widget.value)) {
      _ctrl.text = _two(widget.value);
    }
  }

  @override
  void dispose() {
    _ctrl.dispose();
    _focus.dispose();
    super.dispose();
  }

  void _step(int delta) {
    final span = widget.max + 1;
    final next = ((widget.value + delta) % span + span) % span;
    widget.onChanged(next);
    _ctrl.text = _two(next);
  }

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          width: 30,
          child: Focus(
            onKeyEvent: (_, event) {
              if (event is! KeyDownEvent && event is! KeyRepeatEvent) {
                return KeyEventResult.ignored;
              }
              if (event.logicalKey == LogicalKeyboardKey.arrowUp) {
                _step(1);
                return KeyEventResult.handled;
              }
              if (event.logicalKey == LogicalKeyboardKey.arrowDown) {
                _step(-1);
                return KeyEventResult.handled;
              }
              return KeyEventResult.ignored;
            },
            child: TextField(
              key: ValueKey(widget.fieldKey),
              controller: _ctrl,
              focusNode: _focus,
              textAlign: TextAlign.center,
              keyboardType: TextInputType.number,
              inputFormatters: [
                FilteringTextInputFormatter.digitsOnly,
                LengthLimitingTextInputFormatter(2),
              ],
              style: const TextStyle(
                  fontSize: 15, fontWeight: FontWeight.w700, color: OnDexColors.ink),
              decoration: const InputDecoration(
                isDense: true,
                border: InputBorder.none,
                contentPadding: EdgeInsets.symmetric(vertical: 6),
              ),
              onChanged: (t) {
                final v = int.tryParse(t);
                if (v != null && v >= 0 && v <= widget.max) widget.onChanged(v);
              },
            ),
          ),
        ),
        Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _TinyArrow(icon: Icons.keyboard_arrow_up_rounded, onTap: () => _step(1)),
            _TinyArrow(icon: Icons.keyboard_arrow_down_rounded, onTap: () => _step(-1)),
          ],
        ),
      ],
    );
  }
}

class _TinyArrow extends StatelessWidget {
  final IconData icon;
  final VoidCallback onTap;

  const _TinyArrow({required this.icon, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(4),
      child: SizedBox(
        width: 20,
        height: 15,
        child: Icon(icon, size: 16, color: OnDexColors.inkDim),
      ),
    );
  }
}

class _NavButton extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final VoidCallback? onTap;

  const _NavButton({required this.icon, required this.tooltip, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 34,
      height: 34,
      child: IconButton(
        tooltip: tooltip,
        padding: EdgeInsets.zero,
        onPressed: onTap,
        style: IconButton.styleFrom(
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(9)),
          side: const BorderSide(color: OnDexColors.cardBorder),
        ),
        icon: Icon(icon, size: 20),
        color: OnDexColors.ink,
        disabledColor: OnDexColors.inkFaint.withValues(alpha: 0.4),
      ),
    );
  }
}

class _PresetTile extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _PresetTile({required this.label, required this.selected, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 2),
      child: Material(
        color: selected ? OnDexColors.primaryTint : Colors.transparent,
        borderRadius: BorderRadius.circular(9),
        child: InkWell(
          borderRadius: BorderRadius.circular(9),
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
            child: Row(
              children: [
                Expanded(
                  child: Text(label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                          fontSize: 13,
                          fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                          color: selected ? OnDexColors.primary : OnDexColors.ink)),
                ),
                if (selected)
                  const Icon(Icons.check_rounded, size: 16, color: OnDexColors.primary),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _PresetChip extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _PresetChip({required this.label, required this.selected, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: selected ? OnDexColors.primary : OnDexColors.cardBg,
      shape: StadiumBorder(
          side: BorderSide(color: selected ? OnDexColors.primary : OnDexColors.cardBorder)),
      child: InkWell(
        customBorder: const StadiumBorder(),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
          child: Text(label,
              style: TextStyle(
                  fontSize: 12.5,
                  fontWeight: FontWeight.w600,
                  color: selected ? Colors.white : OnDexColors.ink)),
        ),
      ),
    );
  }
}
