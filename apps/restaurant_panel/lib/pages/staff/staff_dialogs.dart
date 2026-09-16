part of '../staff_page.dart';

// ─── Umumiy ───────────────────────────────────────────────────────────

InputDecoration _input(String label, {String? hint, String? prefix, Widget? suffix, String? suffixText}) =>
    InputDecoration(
      labelText: label,
      hintText: hint,
      prefixText: prefix,
      suffixIcon: suffix,
      suffixText: suffixText,
      isDense: true,
      filled: true,
      fillColor: OnDexColors.cardBg,
      contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 14),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(10),
        borderSide: const BorderSide(color: OnDexColors.cardBorder),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(10),
        borderSide: const BorderSide(color: OnDexColors.primary, width: 1.4),
      ),
    );

Widget _pair(Widget a, Widget b) => LayoutBuilder(builder: (context, box) {
      if (box.maxWidth >= 460) {
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [Expanded(child: a), const SizedBox(width: 12), Expanded(child: b)],
        );
      }
      return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [a, const SizedBox(height: 12), b]);
    });

Widget _sectionTitle(String text) => Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Text(text, style: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
    );

class _DialogFrame extends StatelessWidget {
  const _DialogFrame({required this.title, this.subtitle, required this.body, required this.actions, this.maxWidth = 620});

  final String title;
  final String? subtitle;
  final Widget body;
  final List<Widget> actions;
  final double maxWidth;

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: OnDexColors.cardBg,
      surfaceTintColor: Colors.transparent,
      insetPadding: const EdgeInsets.all(20),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: maxWidth),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(22, 16, 12, 8),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(title,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                        if (subtitle != null)
                          Text(subtitle!,
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
                      ],
                    ),
                  ),
                  IconButton(
                    tooltip: 'Yopish',
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close_rounded, color: OnDexColors.inkDim),
                  ),
                ],
              ),
            ),
            const Divider(height: 1, color: OnDexColors.cardBorder),
            Flexible(child: SingleChildScrollView(padding: const EdgeInsets.fromLTRB(22, 18, 22, 18), child: body)),
            const Divider(height: 1, color: OnDexColors.cardBorder),
            Padding(
              padding: const EdgeInsets.fromLTRB(22, 12, 22, 14),
              child: Wrap(alignment: WrapAlignment.end, spacing: 8, runSpacing: 8, children: actions),
            ),
          ],
        ),
      ),
    );
  }
}

/// 9 raqam, "90 123 45 67" ko'rinishida.
class _UzPhoneFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(TextEditingValue oldValue, TextEditingValue newValue) {
    var digits = newValue.text.replaceAll(RegExp(r'\D'), '');
    if (digits.length > 9) digits = digits.substring(0, 9);
    final b = StringBuffer();
    for (var i = 0; i < digits.length; i++) {
      if (i == 2 || i == 5 || i == 7) b.write(' ');
      b.write(digits[i]);
    }
    final text = b.toString();
    return TextEditingValue(text: text, selection: TextSelection.collapsed(offset: text.length));
  }
}

/// Faqat raqam, minglik bo'shliq bilan: "3 500 000" (so'm, 10 xonagacha).
class _MoneyFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(TextEditingValue oldValue, TextEditingValue newValue) {
    var digits = newValue.text.replaceAll(RegExp(r'\D'), '').replaceFirst(RegExp(r'^0+(?=\d)'), '');
    if (digits.length > 10) digits = digits.substring(0, 10);
    final text = digits.isEmpty ? '' : _groupThousands(int.parse(digits));
    return TextEditingValue(text: text, selection: TextSelection.collapsed(offset: text.length));
  }
}

String _moneyInput(int? tiyin) => tiyin == null ? '' : _groupThousands((tiyin / 100).round());

String _digits(String s) => s.replaceAll(RegExp(r'\D'), '');

String _phoneLocal(String? e164) {
  if (e164 == null || !e164.startsWith('+998') || e164.length != 13) return '';
  final d = e164.substring(4);
  return '${d.substring(0, 2)} ${d.substring(2, 5)} ${d.substring(5, 7)} ${d.substring(7)}';
}

// ─── Qo'shish / tahrirlash ────────────────────────────────────────────

class _StaffFormDialog extends StatefulWidget {
  const _StaffFormDialog({required this.positions, this.member});

  final List<StaffPositionInfo> positions;
  final StaffMember? member;

  @override
  State<_StaffFormDialog> createState() => _StaffFormDialogState();
}

class _StaffFormDialogState extends State<_StaffFormDialog> {
  late final _first = TextEditingController(text: widget.member?.firstName ?? '');
  late final _last = TextEditingController(text: widget.member?.lastName ?? '');
  late final _phone = TextEditingController(text: _phoneLocal(widget.member?.phone));
  late final _note = TextEditingController(text: widget.member?.note ?? '');
  late final _salary = TextEditingController(text: _moneyInput(widget.member?.salaryTiyin));
  late String _position = widget.member?.position ?? '';
  late DateTime? _hiredOn = widget.member?.hiredOn;
  late bool _hasSchedule = widget.member == null || widget.member!.schedule != null;
  late final Set<int> _days = {...(widget.member?.schedule?.days ?? const [1, 2, 3, 4, 5, 6])};
  late String _start = widget.member?.schedule?.start ?? '09:00';
  late String _end = widget.member?.schedule?.end ?? '18:00';
  late bool _appAccess = widget.member?.appAccess ?? false;
  bool _saving = false;
  String? _error;

  bool get _editing => widget.member != null;

  bool get _accessAllowed => widget.positions.where((p) => p.key == _position).firstOrNull?.appAccess ?? false;

  @override
  void dispose() {
    _first.dispose();
    _last.dispose();
    _phone.dispose();
    _note.dispose();
    _salary.dispose();
    super.dispose();
  }

  /// So'mda kiritiladi, serverga tiyinda ketadi; bo'sh — kiritilmagan.
  int? get _salaryTiyin {
    final d = _digits(_salary.text);
    return d.isEmpty ? null : int.parse(d) * 100;
  }

  Map<String, dynamic>? get _schedule =>
      _hasSchedule ? {'days': (_days.toList()..sort()), 'start': _start, 'end': _end} : null;

  Map<String, dynamic> _body() => {
        'first_name': _first.text.trim(),
        'last_name': _last.text.trim(),
        'phone': '+998${_digits(_phone.text)}',
        'position': _position,
        'schedule': _schedule,
        'hired_on': _hiredOn == null ? null : _ymd(_hiredOn!),
        'note': _note.text.trim(),
        'app_access': _accessAllowed && _appAccess,
        'monthly_salary_tiyin': _salaryTiyin,
      };

  /// Faqat o'zgargan maydonlar — server ham shuni kutadi.
  Map<String, dynamic> _patch() {
    final m = widget.member!;
    final b = _body();
    String schedKey(Object? s) => s is Map ? '${s['days']} ${s['start']} ${s['end']}' : '';
    return {
      if (b['first_name'] != m.firstName) 'first_name': b['first_name'],
      if (b['last_name'] != m.lastName) 'last_name': b['last_name'],
      if (b['phone'] != m.phone) 'phone': b['phone'],
      if (b['position'] != m.position) 'position': b['position'],
      if (schedKey(b['schedule']) != schedKey(m.schedule?.toJson())) 'schedule': b['schedule'],
      if (b['hired_on'] != (m.hiredOn == null ? null : _ymd(m.hiredOn!))) 'hired_on': b['hired_on'],
      if (b['note'] != m.note) 'note': b['note'],
      if (b['app_access'] != m.appAccess) 'app_access': b['app_access'],
      if (b['monthly_salary_tiyin'] != m.salaryTiyin) 'monthly_salary_tiyin': b['monthly_salary_tiyin'],
    };
  }

  String? _validate() {
    if (_first.text.trim().isEmpty) return 'Xodimning ismini kiriting';
    if (_digits(_phone.text).length != 9) return 'Telefon raqamini to\'liq kiriting (9 ta raqam)';
    if (_position.isEmpty) return 'Lavozimni tanlang';
    if (_hasSchedule && _days.isEmpty) return 'Kamida bitta ish kunini tanlang';
    final salary = _salaryTiyin;
    if (salary != null && salary <= 0) return 'Oylik maosh 0 dan katta bo\'lsin yoki bo\'sh qoldiring';
    if (salary != null && salary > 100000000000) return 'Oylik maosh 1 mlrd so\'mdan oshmasin';
    return null;
  }

  Future<void> _submit() async {
    final problem = _validate();
    if (problem != null) {
      setState(() => _error = problem);
      return;
    }
    final patch = _editing ? _patch() : const <String, dynamic>{};
    if (_editing && patch.isEmpty) {
      Navigator.of(context).pop(false);
      return;
    }
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      if (_editing) {
        await api.updateStaff(widget.member!.id, patch);
      } else {
        await api.createStaff(_body());
      }
      if (mounted) Navigator.of(context).pop(true);
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } catch (_) {
      if (mounted) setState(() => _error = 'Saqlab bo\'lmadi. Internet aloqasini tekshirib, qayta urinib ko\'ring.');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Future<void> _pickTime({required bool start}) async {
    final cur = start ? _start : _end;
    final picked = await showOnDexTimePicker(
      context: context,
      initial: TimeOfDay(hour: int.tryParse(cur.substring(0, 2)) ?? 9, minute: int.tryParse(cur.substring(3)) ?? 0),
      title: start ? 'Smena boshlanishi' : 'Smena tugashi',
    );
    if (picked == null || !mounted) return;
    setState(() {
      final v = '${_two(picked.hour)}:${_two(picked.minute)}';
      if (start) {
        _start = v;
      } else {
        _end = v;
      }
    });
  }

  Future<void> _pickDate() async {
    final now = DateTime.now();
    final picked = await showOnDexDatePicker(
      context: context,
      initialDate: _hiredOn ?? now,
      firstDate: DateTime(1950),
      lastDate: now.add(const Duration(days: 365)),
      title: 'Ishga kirgan sana',
    );
    if (picked != null && mounted) setState(() => _hiredOn = DateTime(picked.year, picked.month, picked.day));
  }

  @override
  Widget build(BuildContext context) {
    final m = widget.member;
    Widget timeButton(String key, String label, String value, VoidCallback onTap) => Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(label, style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
            const SizedBox(height: 4),
            OutlinedButton.icon(
              key: ValueKey(key),
              onPressed: onTap,
              style: OutlinedButton.styleFrom(
                foregroundColor: OnDexColors.ink,
                side: const BorderSide(color: OnDexColors.cardBorder),
                minimumSize: const Size(110, 40),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                textStyle: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700),
              ),
              icon: const Icon(Icons.schedule_rounded, size: 16),
              label: Text(value),
            ),
          ],
        );

    final body = Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _sectionTitle('Shaxsiy ma\'lumotlar'),
        _pair(
          TextField(
            key: const ValueKey('form-first'),
            controller: _first,
            autofocus: !_editing,
            textCapitalization: TextCapitalization.words,
            maxLength: 50,
            decoration: _input('Ism *').copyWith(counterText: ''),
          ),
          TextField(
            key: const ValueKey('form-last'),
            controller: _last,
            textCapitalization: TextCapitalization.words,
            maxLength: 50,
            decoration: _input('Familiya').copyWith(counterText: ''),
          ),
        ),
        const SizedBox(height: 12),
        TextField(
          key: const ValueKey('form-phone'),
          controller: _phone,
          keyboardType: TextInputType.phone,
          inputFormatters: [_UzPhoneFormatter()],
          decoration: _input('Telefon raqam *', hint: '90 123 45 67', prefix: '+998 '),
        ),
        const SizedBox(height: 18),
        _sectionTitle('Ish ma\'lumotlari'),
        _pair(
          DropdownButtonFormField<String>(
            key: const ValueKey('form-position'),
            initialValue: _position.isEmpty ? null : _position,
            isExpanded: true,
            decoration: _input('Lavozim *'),
            items: [
              for (final p in widget.positions)
                DropdownMenuItem(
                  value: p.key,
                  child: Row(
                    children: [
                      Icon(positionIcon(p.key), size: 18, color: OnDexColors.inkDim),
                      const SizedBox(width: 8),
                      Flexible(child: Text(p.title, overflow: TextOverflow.ellipsis)),
                    ],
                  ),
                ),
            ],
            onChanged: (v) => setState(() {
              _position = v ?? '';
              if (!_accessAllowed) _appAccess = false;
            }),
          ),
          InkWell(
            key: const ValueKey('form-hired'),
            borderRadius: BorderRadius.circular(10),
            onTap: _pickDate,
            child: InputDecorator(
              decoration: _input('Ishga kirgan sana',
                  suffix: _hiredOn == null
                      ? const Icon(Icons.calendar_today_rounded, size: 18)
                      : IconButton(
                          tooltip: 'Olib tashlash',
                          icon: const Icon(Icons.close_rounded, size: 18),
                          onPressed: () => setState(() => _hiredOn = null),
                        )),
              child: Text(_hiredOn == null ? 'Tanlanmagan' : _dmy(_hiredOn!),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 14, color: _hiredOn == null ? OnDexColors.inkFaint : OnDexColors.ink)),
            ),
          ),
        ),
        const SizedBox(height: 12),
        TextField(
          key: const ValueKey('form-salary'),
          controller: _salary,
          keyboardType: TextInputType.number,
          inputFormatters: [_MoneyFormatter()],
          decoration: _input('Oylik maosh (ixtiyoriy)', hint: 'Masalan: 3 500 000', suffixText: 'so\'m'),
        ),
        const Padding(
          padding: EdgeInsets.fromLTRB(4, 4, 4, 0),
          child: Text('Maxfiy: faqat restoran egasiga ko\'rinadi va hisobotda ishlatiladi.',
              style: TextStyle(fontSize: 11.5, color: OnDexColors.inkFaint)),
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.fromLTRB(14, 10, 10, 14),
          decoration: BoxDecoration(
            color: const Color(0xFFFAF6F0),
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: OnDexColors.cardBorder),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  const Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text('Ish jadvali',
                            style: TextStyle(fontSize: 13.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                        Text('Ish kunlari va smena vaqti (Toshkent vaqti)',
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
                      ],
                    ),
                  ),
                  Switch(
                    key: const ValueKey('form-has-schedule'),
                    value: _hasSchedule,
                    activeTrackColor: OnDexColors.success,
                    onChanged: (v) => setState(() => _hasSchedule = v),
                  ),
                ],
              ),
              if (_hasSchedule) ...[
                const SizedBox(height: 8),
                Wrap(
                  spacing: 6,
                  runSpacing: 6,
                  children: [
                    for (var d = 1; d <= 7; d++)
                      FilterChip(
                        key: ValueKey('form-day-$d'),
                        label: Text(_weekdayShort[d - 1]),
                        selected: _days.contains(d),
                        showCheckmark: false,
                        selectedColor: OnDexColors.primaryTint,
                        side: BorderSide(color: _days.contains(d) ? OnDexColors.primary : OnDexColors.cardBorder),
                        labelStyle: TextStyle(
                          fontWeight: FontWeight.w700,
                          color: _days.contains(d) ? OnDexColors.primary : OnDexColors.inkDim,
                        ),
                        onSelected: (v) => setState(() => v ? _days.add(d) : _days.remove(d)),
                      ),
                  ],
                ),
                const SizedBox(height: 10),
                Wrap(
                  spacing: 12,
                  runSpacing: 8,
                  crossAxisAlignment: WrapCrossAlignment.end,
                  children: [
                    timeButton('form-start', 'Boshlanishi', _start, () => _pickTime(start: true)),
                    timeButton('form-end', 'Tugashi', _end, () => _pickTime(start: false)),
                    if (_end.compareTo(_start) <= 0)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 10),
                        child: Text(_end == _start ? 'kun bo\'yi' : 'ertasi kuni tugaydi',
                            style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                      ),
                  ],
                ),
              ],
            ],
          ),
        ),
        if (_accessAllowed) ...[
          const SizedBox(height: 12),
          Container(
            key: const ValueKey('form-access-block'),
            padding: const EdgeInsets.fromLTRB(14, 10, 10, 10),
            decoration: BoxDecoration(
              color: OnDexColors.infoBg.withValues(alpha: 0.5),
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: OnDexColors.info.withValues(alpha: 0.25)),
            ),
            child: Row(
              children: [
                const Icon(Icons.phone_iphone_rounded, color: OnDexColors.info),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text('${staffAppName(_position)} ilovasiga kirish',
                          style: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                      Text(
                        'Xodim shu raqam bilan SMS/Telegram kod orqali kiradi — parol kerak emas. '
                        'Ta\'til yoki ishdan bo\'shatishda kirish avtomatik yopiladi.'
                        '${_position == 'courier' ? ' Yetkazish buyurtmalari unga faqat sizning restoraningizdan keladi.' : ''}',
                        style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim),
                      ),
                    ],
                  ),
                ),
                Switch(
                  key: const ValueKey('form-app-access'),
                  value: _appAccess,
                  activeTrackColor: OnDexColors.success,
                  onChanged: (v) => setState(() => _appAccess = v),
                ),
              ],
            ),
          ),
        ],
        const SizedBox(height: 12),
        TextField(
          key: const ValueKey('form-note'),
          controller: _note,
          minLines: 2,
          maxLines: 4,
          maxLength: 300,
          decoration: _input('Izoh', hint: 'Masalan: kechki smena, stajyor...'),
        ),
        if (_error != null) ...[
          const SizedBox(height: 4),
          Container(
            key: const ValueKey('form-error'),
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(color: OnDexColors.dangerBg, borderRadius: BorderRadius.circular(10)),
            child: Row(
              children: [
                const Icon(Icons.error_outline_rounded, size: 18, color: OnDexColors.danger),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(_error!,
                      style: const TextStyle(fontSize: 12.5, color: OnDexColors.danger, fontWeight: FontWeight.w600)),
                ),
              ],
            ),
          ),
        ],
      ],
    );

    return _DialogFrame(
      title: _editing ? 'Xodimni tahrirlash' : 'Yangi xodim qo\'shish',
      subtitle: _editing ? '${m!.fullName} · #${m.code}' : 'Kafe, restoran, oshxona va choyxona uchun universal',
      body: body,
      actions: [
        TextButton(
          onPressed: _saving ? null : () => Navigator.of(context).pop(false),
          style: TextButton.styleFrom(foregroundColor: OnDexColors.inkDim, minimumSize: const Size(0, 44)),
          child: const Text('Bekor qilish'),
        ),
        FilledButton.icon(
          key: const ValueKey('form-save'),
          onPressed: _saving ? null : _submit,
          style: FilledButton.styleFrom(
            backgroundColor: OnDexColors.primary,
            minimumSize: const Size(0, 44),
            padding: const EdgeInsets.symmetric(horizontal: 20),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
          ),
          icon: _saving
              ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
              : const Icon(Icons.check_rounded, size: 18),
          label: Text(_editing ? 'Saqlash' : 'Qo\'shish'),
        ),
      ],
    );
  }
}

// ─── Xodim kartochkasi ────────────────────────────────────────────────

class _StaffDetailsDialog extends StatefulWidget {
  const _StaffDetailsDialog({required this.member});

  final StaffMember member;

  @override
  State<_StaffDetailsDialog> createState() => _StaffDetailsDialogState();
}

class _StaffDetailsDialogState extends State<_StaffDetailsDialog> {
  List<StaffEvent>? _events;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadActivity();
  }

  Future<void> _loadActivity() async {
    try {
      final list = await api.staffActivity(widget.member.id);
      if (!mounted) return;
      setState(() => _events = [
            for (final e in list)
              if (e is Map) StaffEvent.fromJson(Map<String, dynamic>.from(e)),
          ]);
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } catch (_) {
      if (mounted) setState(() => _error = 'Faoliyatni yuklab bo\'lmadi');
    }
  }

  @override
  Widget build(BuildContext context) {
    final m = widget.member;
    Widget info(IconData icon, String label, String value) => Padding(
          padding: const EdgeInsets.symmetric(vertical: 6),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(icon, size: 18, color: OnDexColors.inkDim),
              const SizedBox(width: 10),
              SizedBox(
                width: 130,
                child: Text(label, style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
              ),
              Expanded(
                child: Text(value, style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: OnDexColors.ink)),
              ),
            ],
          ),
        );
    final access = !m.appAccessActive && !m.appAccess
        ? (m.position == 'waiter' || m.position == 'courier' ? 'Yopiq' : 'Bu lavozim uchun ilova yo\'q')
        : (m.appAccessActive
            ? 'Ochiq — ${staffAppName(m.position)}'
            : 'Vaqtincha yopiq (${m.status == 'on_leave' ? 'ta\'tilda' : 'faol emas'})');

    final events = _events;
    final body = Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            _Avatar(member: m, size: 56),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(m.fullName,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                  Text('#${m.code} · ${m.positionTitle}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 13, color: OnDexColors.inkDim)),
                  const SizedBox(height: 6),
                  _StatusPill(status: m.status),
                ],
              ),
            ),
          ],
        ),
        const SizedBox(height: 14),
        info(Icons.phone_outlined, 'Telefon', formatUzPhone(m.phone)),
        info(Icons.schedule_rounded, 'Ish vaqti',
            m.schedule == null ? 'Belgilanmagan' : '${m.schedule!.range} · ${m.schedule!.daysText}'),
        if (m.schedule != null)
          info(Icons.weekend_outlined, 'Dam olish kunlari',
              m.schedule!.offDays.isEmpty ? 'Yo\'q' : [for (final d in m.schedule!.offDays) _weekdayFull[d - 1]].join(', ')),
        info(Icons.payments_outlined, 'Oylik maosh', m.salaryTiyin == null ? 'Kiritilmagan' : formatMoney(m.salaryTiyin!)),
        info(Icons.event_outlined, 'Ishga kirgan sana', m.hiredOn == null ? 'Kiritilmagan' : _dmy(m.hiredOn!)),
        info(Icons.phone_iphone_rounded, 'Ilovaga kirish', access),
        info(Icons.person_add_alt_outlined, 'Qo\'shilgan', _dmy(m.createdAt)),
        if (m.note.isNotEmpty) info(Icons.notes_rounded, 'Izoh', m.note),
        const SizedBox(height: 12),
        const Text('Faoliyat', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
        const SizedBox(height: 6),
        if (_error != null)
          Text(_error!, style: const TextStyle(fontSize: 12.5, color: OnDexColors.danger))
        else if (events == null)
          const Padding(
            padding: EdgeInsets.all(12),
            child: Center(child: SizedBox(width: 22, height: 22, child: CircularProgressIndicator(strokeWidth: 2))),
          )
        else if (events.isEmpty)
          const Text('Hali faoliyat yo\'q', style: TextStyle(fontSize: 12.5, color: OnDexColors.inkDim))
        else
          for (final e in events) _EventRow(event: e),
      ],
    );

    return _DialogFrame(
      title: 'Xodim ma\'lumotlari',
      maxWidth: 560,
      body: body,
      actions: [
        for (final (key, label, _, danger) in _statusActions(m.status))
          TextButton(
            key: ValueKey('details-$key'),
            onPressed: () => Navigator.of(context).pop(key),
            style: TextButton.styleFrom(
              foregroundColor: danger ? OnDexColors.danger : OnDexColors.ink,
              minimumSize: const Size(0, 44),
            ),
            child: Text(label),
          ),
        FilledButton.icon(
          key: const ValueKey('details-edit'),
          onPressed: () => Navigator.of(context).pop('edit'),
          style: FilledButton.styleFrom(backgroundColor: OnDexColors.primary, minimumSize: const Size(0, 44)),
          icon: const Icon(Icons.edit_outlined, size: 18),
          label: const Text('Tahrirlash'),
        ),
      ],
    );
  }
}

// ─── Holatni tasdiqlash ───────────────────────────────────────────────

Future<bool> _confirmStatus(BuildContext context, StaffMember m, String status) async {
  if (status == 'active') return true;
  final dismiss = status == 'dismissed';
  final appName = staffAppName(m.position);
  final appNote = m.appAccessActive
      ? (dismiss
          ? '\n\n$appName ilovasiga kirish va barcha sessiyalar DARHOL yopiladi.'
          : '\n\n$appName ilovasiga kirish ta\'til davomida yopiladi va ishga qaytganda qayta ochiladi.')
      : '';
  final ok = await showDialog<bool>(
    context: context,
    builder: (ctx) => AlertDialog(
      icon: Icon(dismiss ? Icons.logout_rounded : Icons.beach_access_rounded,
          color: dismiss ? OnDexColors.danger : OnDexColors.amber, size: 30),
      title: Text(dismiss ? 'Ishdan bo\'shatish' : 'Ta\'tilga chiqarish'),
      content: Text(dismiss
          ? '${m.fullName} ishdan bo\'shagan deb belgilanadi. Yozuv va faoliyat tarixi saqlanadi, '
              'kerak bo\'lsa qayta ishga olish mumkin.$appNote'
          : '${m.fullName} ta\'tilda deb belgilanadi.$appNote'),
      actions: [
        TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Bekor qilish')),
        FilledButton(
          key: const ValueKey('confirm-status'),
          style: FilledButton.styleFrom(backgroundColor: dismiss ? OnDexColors.danger : OnDexColors.primary),
          onPressed: () => Navigator.of(ctx).pop(true),
          child: Text(dismiss ? 'Ishdan bo\'shatish' : 'Ta\'tilga chiqarish'),
        ),
      ],
    ),
  );
  return ok == true;
}

// ─── Haftalik ish jadvali ─────────────────────────────────────────────

class _ScheduleDialog extends StatelessWidget {
  const _ScheduleDialog({required this.members});

  final List<StaffMember> members;

  @override
  Widget build(BuildContext context) {
    final scheduled = members.where((m) => m.schedule != null).toList()
      ..sort((a, b) => a.positionTitle != b.positionTitle
          ? a.positionTitle.compareTo(b.positionTitle)
          : a.fullName.compareTo(b.fullName));
    final unscheduled = members.length - scheduled.length;
    const head = TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: OnDexColors.inkDim);

    // Yon tomonga aylantirish YO'Q: kunlar kengligi oynaga moslashadi,
    // tor joyda matn kichrayadi (FittedBox).
    Widget cell(String text, {bool on = false, bool bold = false}) => Expanded(
          child: Container(
            height: 40,
            margin: const EdgeInsets.all(2),
            padding: const EdgeInsets.symmetric(horizontal: 4),
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: on ? OnDexColors.primaryTint : const Color(0xFFFAF6F0),
              borderRadius: BorderRadius.circular(8),
            ),
            child: FittedBox(
              fit: BoxFit.scaleDown,
              child: Text(text,
                  style: TextStyle(
                    fontSize: 11.5,
                    fontWeight: bold || on ? FontWeight.w700 : FontWeight.w400,
                    color: on ? OnDexColors.primary : OnDexColors.inkFaint,
                  )),
            ),
          ),
        );

    Widget grid(double width) {
      final nameW = width >= 700 ? 190.0 : 110.0;
      return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(children: [
          SizedBox(width: nameW, child: const Text('Xodim', style: head)),
          for (final d in _weekdayShort) Expanded(child: Center(child: Text(d, style: head))),
        ]),
        const SizedBox(height: 6),
        for (final m in scheduled)
          Row(children: [
            SizedBox(
              width: nameW,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(m.fullName,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                  Text(m.positionTitle,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
                ],
              ),
            ),
            for (var d = 1; d <= 7; d++)
              // Jadvalda yo'q kun — dam olish kuni.
              m.schedule!.worksOn(d) ? cell(m.schedule!.range.replaceAll(' ', ''), on: true) : cell('Dam olish'),
          ]),
        const SizedBox(height: 6),
        Row(children: [
          SizedBox(
            width: nameW,
            child: const Text('Ishlaydiganlar', maxLines: 1, overflow: TextOverflow.ellipsis, style: head),
          ),
          for (var d = 1; d <= 7; d++)
            cell('${scheduled.where((m) => m.schedule!.worksOn(d)).length} kishi', bold: true),
        ]),
      ],
    );
    }

    return _DialogFrame(
      title: 'Haftalik ish jadvali',
      subtitle: 'Faol xodimlar · Toshkent vaqti',
      maxWidth: 960,
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (scheduled.isEmpty)
            const Text('Hali hech bir faol xodimga ish jadvali belgilanmagan.',
                style: TextStyle(fontSize: 13, color: OnDexColors.inkDim))
          else
            LayoutBuilder(builder: (context, c) => grid(c.maxWidth)),
          if (unscheduled > 0) ...[
            const SizedBox(height: 12),
            Text('Jadvali belgilanmagan: $unscheduled ta xodim (tahrirlash oynasida belgilanadi).',
                style: const TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
          ],
        ],
      ),
      actions: [
        FilledButton(
          onPressed: () => Navigator.of(context).pop(),
          style: FilledButton.styleFrom(backgroundColor: OnDexColors.primary, minimumSize: const Size(0, 44)),
          child: const Text('Yopish'),
        ),
      ],
    );
  }
}
