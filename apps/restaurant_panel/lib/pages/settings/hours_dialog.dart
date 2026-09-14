part of '../restaurant_settings_page.dart';

/// Oyna natijasi: `days == null` — ish vaqti cheklanmagan (kun bo'yi).
class _HoursResult {
  const _HoursResult(this.days);

  final List<_DayHours>? days;
}

class _HoursDialog extends StatefulWidget {
  const _HoursDialog({required this.initial});

  final List<_DayHours> initial;

  @override
  State<_HoursDialog> createState() => _HoursDialogState();
}

class _HoursDialogState extends State<_HoursDialog> {
  late List<_DayHours> _days = [...widget.initial];

  Future<void> _pick(_DayHours d, {required bool open}) async {
    final picked = await showOnDexTimePicker(
      context: context,
      initial: _timeOf(open ? d.open : d.close),
      title: '${_dayNames[d.day - 1]} — ${open ? 'ochilish' : 'yopilish'}',
    );
    if (picked == null || !mounted) return;
    setState(() {
      _days = [
        for (final x in _days)
          x.day == d.day ? (open ? x.copyWith(open: _hhmm(picked)) : x.copyWith(close: _hhmm(picked))) : x,
      ];
    });
  }

  void _copyMondayToAll() {
    final monday = _days.first;
    setState(() {
      _days = [
        for (final x in _days) x.copyWith(enabled: monday.enabled, open: monday.open, close: monday.close),
      ];
    });
  }

  @override
  Widget build(BuildContext context) {
    final allOff = _days.every((d) => !d.enabled);
    return Dialog(
      backgroundColor: OnDexColors.cardBg,
      surfaceTintColor: Colors.transparent,
      insetPadding: const EdgeInsets.all(20),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 560),
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(22, 18, 16, 18),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Container(
                    width: 38,
                    height: 38,
                    decoration: BoxDecoration(color: OnDexColors.primaryTint, borderRadius: BorderRadius.circular(11)),
                    child: const Icon(Icons.schedule_rounded, color: OnDexColors.primary, size: 20),
                  ),
                  const SizedBox(width: 12),
                  const Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text('Haftalik ish vaqti',
                            style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: OnDexColors.ink)),
                        Text('Toshkent vaqti. Yopilish ochilishdan oldin bo\'lsa — ertasi kuni yopiladi.',
                            style: TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
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
              const SizedBox(height: 12),
              for (final d in _days)
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 3),
                  child: Wrap(
                    crossAxisAlignment: WrapCrossAlignment.center,
                    spacing: 8,
                    runSpacing: 4,
                    children: [
                      SizedBox(
                        width: 120,
                        child: Row(
                          children: [
                            Checkbox(
                              key: ValueKey('hours-day-${d.day}'),
                              value: d.enabled,
                              onChanged: (v) => setState(() {
                                _days = [for (final x in _days) x.day == d.day ? x.copyWith(enabled: v ?? false) : x];
                              }),
                            ),
                            Expanded(
                              child: Text(_dayNames[d.day - 1],
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w600)),
                            ),
                          ],
                        ),
                      ),
                      if (d.enabled) ...[
                        _TimeButton(
                          key: ValueKey('hours-open-${d.day}'),
                          text: d.open,
                          onTap: () => _pick(d, open: true),
                        ),
                        const Text('–', style: TextStyle(color: OnDexColors.inkDim)),
                        _TimeButton(
                          key: ValueKey('hours-close-${d.day}'),
                          text: d.close,
                          onTap: () => _pick(d, open: false),
                        ),
                        if (d.open == d.close || d.close.compareTo(d.open) < 0)
                          Text(d.open == d.close ? 'kun bo\'yi' : 'ertasi kuni',
                              style: const TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
                      ] else
                        const Text('Dam olish kuni', style: TextStyle(fontSize: 13, color: OnDexColors.inkFaint)),
                    ],
                  ),
                ),
              if (allOff) ...[
                const SizedBox(height: 8),
                const Text('Hamma kun dam olish — restoran buyurtma qabul qilmaydi.',
                    style: TextStyle(fontSize: 12.5, color: OnDexColors.danger, fontWeight: FontWeight.w600)),
              ],
              const SizedBox(height: 12),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  TextButton.icon(
                    onPressed: _copyMondayToAll,
                    icon: const Icon(Icons.copy_all_rounded, size: 18),
                    label: const Text('Dushanbani barcha kunlarga qo\'llash'),
                  ),
                  TextButton.icon(
                    key: const ValueKey('hours-unlimited'),
                    onPressed: () => Navigator.of(context).pop(const _HoursResult(null)),
                    icon: const Icon(Icons.all_inclusive_rounded, size: 18),
                    label: const Text('Cheklovni olib tashlash'),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Wrap(
                alignment: WrapAlignment.end,
                spacing: 8,
                runSpacing: 8,
                children: [
                  TextButton(
                    onPressed: () => Navigator.of(context).pop(),
                    child: const Text('Bekor qilish'),
                  ),
                  FilledButton(
                    key: const ValueKey('hours-apply'),
                    onPressed: () => Navigator.of(context).pop(_HoursResult(_days)),
                    style: FilledButton.styleFrom(
                      backgroundColor: OnDexColors.primary,
                      minimumSize: const Size(0, 42),
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

class _TimeButton extends StatelessWidget {
  const _TimeButton({super.key, required this.text, required this.onTap});

  final String text;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => OutlinedButton(
        onPressed: onTap,
        style: OutlinedButton.styleFrom(
          foregroundColor: OnDexColors.ink,
          side: const BorderSide(color: OnDexColors.cardBorder),
          minimumSize: const Size(76, 38),
          padding: const EdgeInsets.symmetric(horizontal: 10),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
          textStyle: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700),
        ),
        child: Text(text),
      );
}
