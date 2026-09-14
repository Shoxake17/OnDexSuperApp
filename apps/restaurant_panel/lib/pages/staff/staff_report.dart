part of '../staff_page.dart';

/// Oylik hisobot: har bir xodimning oylik maoshi, ish kunlari va
/// soatlari, qaysi kunlari ishlagani (kunlar tasmasi).
///
/// ┌─ HALOL YOZUV ─────────────────────────────────────────────────────┐
/// Davomat (keldi/ketdi) tizimda qayd etilmaydi — hisobot SERVERDA ish
/// jadvali va holatlar tarixi (ta'til, ishdan bo'shatish, jadval
/// o'zgarishi) bo'yicha hisoblanadi va oynada aynan shunday yoziladi.
/// └───────────────────────────────────────────────────────────────────┘
class _StaffReportDialog extends StatefulWidget {
  const _StaffReportDialog();

  @override
  State<_StaffReportDialog> createState() => _StaffReportDialogState();
}

class _StaffReportDialogState extends State<_StaffReportDialog> {
  late DateTime _month = DateTime(DateTime.now().year, DateTime.now().month);
  StaffReport? _data;
  bool _loading = true;
  String? _error;

  String get _key => '${_month.year}-${_two(_month.month)}';

  /// Server kelgusi oydan keyingisini qabul qilmaydi.
  bool get _canNext {
    final now = DateTime.now();
    return _month.isBefore(DateTime(now.year, now.month + 1));
  }

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final key = _key;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final json = await api.staffReport(key);
      if (!mounted || key != _key) return;
      setState(() {
        _data = StaffReport.fromJson(json);
        _loading = false;
      });
    } on ApiException catch (e) {
      if (!mounted || key != _key) return;
      setState(() {
        _loading = false;
        _error = e.message;
      });
    } catch (_) {
      if (!mounted || key != _key) return;
      setState(() {
        _loading = false;
        _error = 'Hisobotni yuklab bo\'lmadi. Internet aloqasini tekshirib, qayta urinib ko\'ring.';
      });
    }
  }

  void _shift(int delta) {
    setState(() => _month = DateTime(_month.year, _month.month + delta));
    _load();
  }

  @override
  Widget build(BuildContext context) {
    final screen = MediaQuery.sizeOf(context);
    final data = _data;
    final rows = <StaffReportRow>[...?data?.rows]..sort((a, b) => a.code.compareTo(b.code));

    final monthSwitcher = Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        IconButton(
          key: const ValueKey('report-prev'),
          tooltip: 'Oldingi oy',
          onPressed: () => _shift(-1),
          icon: const Icon(Icons.chevron_left_rounded),
        ),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
          decoration: BoxDecoration(
            color: OnDexColors.primaryTint,
            borderRadius: BorderRadius.circular(10),
          ),
          child: Text('${_monthNames[_month.month - 1]} ${_month.year}',
              key: const ValueKey('report-month'),
              style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: OnDexColors.primary)),
        ),
        IconButton(
          key: const ValueKey('report-next'),
          tooltip: 'Keyingi oy',
          onPressed: _canNext ? () => _shift(1) : null,
          icon: const Icon(Icons.chevron_right_rounded),
        ),
      ],
    );

    Widget content;
    if (_error != null) {
      content = Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(_error!, textAlign: TextAlign.center, style: const TextStyle(color: OnDexColors.danger)),
            const SizedBox(height: 12),
            FilledButton(onPressed: _load, child: const Text('Qayta urinish')),
          ],
        ),
      );
    } else if (data == null) {
      content = const Center(child: CircularProgressIndicator());
    } else {
      content = Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              _ReportStat(icon: Icons.groups_rounded, color: OnDexColors.primary, label: 'Xodimlar', value: '${data.members}'),
              _ReportStat(
                  icon: Icons.timer_outlined,
                  color: OnDexColors.info,
                  label: 'Ishlangan soat (bugungacha)',
                  value: formatHours(data.workedMinutes)),
              _ReportStat(
                  icon: Icons.event_note_rounded,
                  color: _purple,
                  label: 'Oy bo\'yicha reja',
                  value: formatHours(data.plannedMinutes)),
              _ReportStat(
                  icon: Icons.account_balance_wallet_outlined,
                  color: OnDexColors.amber,
                  label: 'Oylik maosh fondi',
                  value: formatMoney(data.salaryFundTiyin)),
              _ReportStat(
                  key: const ValueKey('report-accrued'),
                  icon: Icons.payments_outlined,
                  color: OnDexColors.success,
                  label: 'Hisoblangan maosh',
                  value: formatMoney(data.accruedTiyin)),
            ],
          ),
          const SizedBox(height: 14),
          Expanded(
            child: rows.isEmpty
                ? const Center(
                    child: Text('Bu oyda ishlagan xodim yo\'q', style: TextStyle(fontSize: 14, color: OnDexColors.inkDim)),
                  )
                : _ReportTable(rows: rows, daysInMonth: data.daysInMonth, month: _month),
          ),
          const SizedBox(height: 10),
          const Wrap(
            spacing: 14,
            runSpacing: 6,
            children: [
              _Legend(mark: 'W', label: 'Ishlagan'),
              _Legend(mark: 'P', label: 'Reja (bugun va keyin)'),
              _Legend(mark: 'O', label: 'Dam olish'),
              _Legend(mark: 'L', label: 'Ta\'til'),
              _Legend(mark: 'N', label: 'Ishda emas'),
            ],
          ),
          const SizedBox(height: 8),
          const Text(
            'Hisobot ish jadvali va holatlar tarixi (ta\'til, ishdan bo\'shatish, jadval o\'zgarishi) asosida '
            'hisoblanadi — davomat (keldi-ketdi) alohida qayd etilmaydi. Hisoblangan maosh = oylik maosh × '
            'ishda bo\'lgan ish kunlari ÷ oyning ish kunlari; ta\'til kunlari kiritilmagan.',
            style: TextStyle(fontSize: 11.5, color: OnDexColors.inkDim),
          ),
        ],
      );
    }

    return Dialog(
      backgroundColor: OnDexColors.cardBg,
      surfaceTintColor: Colors.transparent,
      insetPadding: const EdgeInsets.all(20),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 1320),
        child: SizedBox(
          height: math.max(420.0, screen.height - 40),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(22, 12, 10, 8),
                child: Wrap(
                  alignment: WrapAlignment.spaceBetween,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  runSpacing: 6,
                  children: [
                    const Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text('Xodimlar hisoboti',
                            style: TextStyle(fontSize: 19, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                        Text('Oylik maosh, ish kunlari va soatlari · Toshkent vaqti',
                            style: TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
                      ],
                    ),
                    Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        monthSwitcher,
                        const SizedBox(width: 6),
                        IconButton(
                          tooltip: 'Yopish',
                          onPressed: () => Navigator.of(context).pop(),
                          icon: const Icon(Icons.close_rounded, color: OnDexColors.inkDim),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              if (_loading && data != null) const LinearProgressIndicator(minHeight: 2) else const Divider(height: 2),
              Expanded(
                child: Padding(padding: const EdgeInsets.fromLTRB(22, 14, 22, 16), child: content),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ReportStat extends StatelessWidget {
  const _ReportStat({super.key, required this.icon, required this.color, required this.label, required this.value});

  final IconData icon;
  final Color color;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(minWidth: 170),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      decoration: BoxDecoration(
        color: _headBg,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: OnDexColors.cardBorder),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 20, color: color),
          const SizedBox(width: 10),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(label, style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
              Text(value, style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
            ],
          ),
        ],
      ),
    );
  }
}

/// Qaysi ustunlar sig'adi. Yon tomonga aylantirish YO'Q: tor joyda
/// lavozim va jadval ism ostiga ko'chadi, kunlar tasmasi qator ostiga
/// tushadi, keyin eng kam muhim ustunlar yashiriladi.
class _ReportCols {
  const _ReportCols(this.width);

  final double width;

  bool get full => width >= 1220;
  bool get stripInline => width >= 940;
  bool get index => width >= 600;
  bool get days => width >= 600;
  bool get salary => width >= 520;
}

/// Qat'iy sarlavhali hisobot jadvali: qatorlar ichkarida aylanadi.
class _ReportTable extends StatelessWidget {
  const _ReportTable({required this.rows, required this.daysInMonth, required this.month});

  final List<StaffReportRow> rows;
  final int daysInMonth;
  final DateTime month;

  static const _head = TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: OnDexColors.inkDim);
  static const _cell = TextStyle(fontSize: 12.5, color: OnDexColors.ink);
  static const _sub = TextStyle(fontSize: 11, color: OnDexColors.inkFaint);

  /// Kunlar tasmasi: har kun 6 px + 2 px oraliq.
  static const dayCell = 6.0;

  @override
  Widget build(BuildContext context) {
    final stripWidth = daysInMonth * (dayCell + 2) + 8;
    return LayoutBuilder(builder: (context, c) {
      final cols = _ReportCols(c.maxWidth - 20);
      Widget box(double w, Widget child) =>
          SizedBox(width: w, child: Padding(padding: const EdgeInsets.only(right: 8), child: child));
      Widget headText(String t) => Text(t, maxLines: 2, overflow: TextOverflow.ellipsis, style: _head);

      final header = Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 10),
        decoration: BoxDecoration(color: _headBg, borderRadius: BorderRadius.circular(10)),
        child: Row(
          children: [
            if (cols.index) box(30, headText('#')),
            Expanded(child: headText('Xodim')),
            if (cols.full) box(124, headText('Lavozim')),
            if (cols.full) box(128, headText('Ish jadvali')),
            if (cols.days) box(80, headText('Ish kunlari\n(ishlagan / oy)')),
            box(108, headText('Ish soatlari\n(ishlagan / oy)')),
            if (cols.stripInline) box(56, headText('Ta\'til')),
            if (cols.salary) box(112, headText('Oylik maosh')),
            box(120, headText('Hisoblangan')),
            if (cols.stripInline) SizedBox(width: stripWidth, child: headText('Qaysi kunlari (1–$daysInMonth)')),
          ],
        ),
      );

      return Column(
        children: [
          header,
          Expanded(
            child: ListView.separated(
              key: const ValueKey('report-rows'),
              primary: false,
              itemCount: rows.length,
              separatorBuilder: (_, __) => const Divider(height: 1, color: _rowDivider),
              itemBuilder: (_, i) {
                final r = rows[i];
                final statusNote = switch (r.status) {
                  'on_leave' => 'ta\'tilda',
                  'dismissed' => 'bo\'shagan',
                  _ => '',
                };
                final subtitle = [
                  '#${r.code}',
                  if (!cols.full) r.positionTitle,
                  if (!cols.full && statusNote.isNotEmpty) statusNote,
                  if (!cols.stripInline && r.leaveDays > 0) 'ta\'til ${r.leaveDays} kun',
                  if (!cols.days) '${r.workedDays}/${r.payableDays} kun',
                ].join(' · ');
                final line = Row(
                  children: [
                    if (cols.index)
                      box(30, Text('${i + 1}', style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim))),
                    Expanded(
                      child: Padding(
                        padding: const EdgeInsets.only(right: 8),
                        child: Row(
                          children: [
                            _InitialsAvatar(id: r.id, name: r.fullName, size: 32),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(r.fullName,
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                      style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700)),
                                  Text(subtitle, maxLines: 2, overflow: TextOverflow.ellipsis, style: _sub),
                                ],
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                    if (cols.full)
                      box(
                          124,
                          Text(statusNote.isEmpty ? r.positionTitle : '${r.positionTitle} · $statusNote',
                              maxLines: 2, overflow: TextOverflow.ellipsis, style: _cell)),
                    if (cols.full)
                      box(
                        128,
                        r.schedule == null
                            ? const Text('Belgilanmagan', style: _sub)
                            : Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(r.schedule!.range, maxLines: 1, overflow: TextOverflow.ellipsis, style: _cell),
                                  Text(r.schedule!.daysText, maxLines: 1, overflow: TextOverflow.ellipsis, style: _sub),
                                ],
                              ),
                      ),
                    if (cols.days) box(80, Text('${r.workedDays} / ${r.payableDays}', style: _cell)),
                    box(
                      108,
                      r.scheduled
                          ? Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(formatHours(r.workedMinutes), maxLines: 1, overflow: TextOverflow.ellipsis, style: _cell),
                                Text('/ ${formatHours(r.plannedMinutes)}',
                                    maxLines: 1, overflow: TextOverflow.ellipsis, style: _sub),
                              ],
                            )
                          : const Tooltip(
                              message: 'Ish jadvali belgilanmagan — soat hisoblanmaydi',
                              child: Text('—', style: _cell),
                            ),
                    ),
                    if (cols.stripInline) box(56, Text(r.leaveDays == 0 ? '—' : '${r.leaveDays} kun', style: _cell)),
                    if (cols.salary)
                      box(
                          112,
                          Text(r.salaryTiyin == null ? 'Kiritilmagan' : formatMoney(r.salaryTiyin!),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: r.salaryTiyin == null ? _sub : _cell)),
                    box(
                        120,
                        Text(r.accruedTiyin == null ? '—' : formatMoney(r.accruedTiyin!),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w800, color: OnDexColors.ink))),
                    if (cols.stripInline) SizedBox(width: stripWidth, child: _DayStrip(days: r.days, month: month)),
                  ],
                );
                return Padding(
                  key: ValueKey('report-row-${r.id}'),
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
                  child: cols.stripInline
                      ? line
                      : Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            line,
                            const SizedBox(height: 6),
                            Padding(
                              padding: EdgeInsets.only(left: cols.index ? 70 : 40),
                              child: _DayStrip(days: r.days, month: month),
                            ),
                          ],
                        ),
                );
              },
            ),
          ),
        ],
      );
    });
  }
}

BoxDecoration _dayBox(String mark) => switch (mark) {
      'W' => BoxDecoration(color: OnDexColors.success, borderRadius: BorderRadius.circular(2)),
      'P' => BoxDecoration(
          color: OnDexColors.successBg,
          borderRadius: BorderRadius.circular(2),
          border: Border.all(color: OnDexColors.success.withValues(alpha: 0.5)),
        ),
      'O' => BoxDecoration(color: const Color(0xFFE2D9CD), borderRadius: BorderRadius.circular(2)),
      'L' => BoxDecoration(color: OnDexColors.amber, borderRadius: BorderRadius.circular(2)),
      _ => BoxDecoration(borderRadius: BorderRadius.circular(2), border: Border.all(color: OnDexColors.cardBorder)),
    };

String _dayLabel(String mark) => switch (mark) {
      'W' => 'ishlagan',
      'P' => 'ish kuni (reja)',
      'O' => 'dam olish',
      'L' => 'ta\'til',
      _ => 'ishda emas',
    };

class _DayStrip extends StatelessWidget {
  const _DayStrip({required this.days, required this.month});

  final String days;
  final DateTime month;

  @override
  Widget build(BuildContext context) {
    final name = _monthNames[month.month - 1].toLowerCase();
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        for (var i = 0; i < days.length; i++)
          Tooltip(
            message: '${i + 1}-$name, ${_weekdayFull[DateTime(month.year, month.month, i + 1).weekday - 1]}: '
                '${_dayLabel(days[i])}',
            child: Container(
              width: _ReportTable.dayCell,
              height: 22,
              margin: const EdgeInsets.symmetric(horizontal: 1),
              decoration: _dayBox(days[i]),
            ),
          ),
      ],
    );
  }
}

class _Legend extends StatelessWidget {
  const _Legend({required this.mark, required this.label});

  final String mark;
  final String label;

  @override
  Widget build(BuildContext context) => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(width: 12, height: 12, decoration: _dayBox(mark)),
          const SizedBox(width: 6),
          Text(label, style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
        ],
      );
}
