import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../api.dart';
import '../../theme.dart';
import 'reservations_api.dart';

/// Bron sozlamalari (RS-3): yoqish, kishi boshiga zaklad (umumiy va zona
/// bo'yicha), bron davomiyligi, eng kamida qancha oldin, necha kun oldinga.
/// Saqlansa `true`.
Future<bool?> showReservationSettingsDialog(BuildContext context) =>
    showDialog<bool>(context: context, builder: (_) => const _SettingsDialog());

class _SettingsDialog extends StatefulWidget {
  const _SettingsDialog();

  @override
  State<_SettingsDialog> createState() => _SettingsDialogState();
}

int _som(dynamic tiyin) => ((tiyin as num?)?.toInt() ?? 0) ~/ 100;

class _SettingsDialogState extends State<_SettingsDialog> {
  bool _loading = true;
  bool _saving = false;
  String? _error;

  bool _enabled = false;
  final _deposit = TextEditingController(text: '0');
  int _slot = 120;
  int _advance = 60;
  int _days = 30;
  final Map<String, TextEditingController> _zones = {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _deposit.dispose();
    for (final c in _zones.values) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final s = await api.reservationSettings();
      final tables = await api.tables();
      final zones = <String>{
        for (final t in tables)
          if ('${(t as Map)['zone'] ?? ''}'.trim().isNotEmpty) '${t['zone']}'.trim(),
      };
      final zd = Map<String, dynamic>.from((s['zone_deposits'] as Map?) ?? {});
      zones.addAll(zd.keys);
      if (!mounted) return;
      setState(() {
        _enabled = s['enabled'] == true;
        _deposit.text = '${_som(s['deposit_per_person_tiyin'])}';
        _slot = (s['slot_minutes'] as num?)?.toInt() ?? 120;
        _advance = (s['min_advance_minutes'] as num?)?.toInt() ?? 60;
        _days = (s['max_days_ahead'] as num?)?.toInt() ?? 30;
        for (final z in zones.toList()..sort()) {
          _zones[z] = TextEditingController(text: zd.containsKey(z) ? '${_som(zd[z])}' : '');
        }
        _loading = false;
      });
    } on ApiException catch (e) {
      if (mounted) {
        setState(() {
          _loading = false;
          _error = e.message;
        });
      }
    }
  }

  Future<void> _save() async {
    final dep = int.tryParse(_deposit.text.trim()) ?? -1;
    if (dep < 0 || dep > 10000000) {
      setState(() => _error = 'Zaklad 0 dan 10 000 000 so\'mgacha bo\'lishi kerak');
      return;
    }
    final zones = <String, int>{};
    for (final e in _zones.entries) {
      final v = e.value.text.trim();
      if (v.isEmpty) continue; // umumiy qiymat ishlatiladi
      final n = int.tryParse(v);
      if (n == null || n < 0 || n > 10000000) {
        setState(() => _error = '"${e.key}" zonasi zakladi noto\'g\'ri');
        return;
      }
      zones[e.key] = n * 100;
    }
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await api.saveReservationSettings({
        'enabled': _enabled,
        'deposit_per_person_tiyin': dep * 100,
        'zone_deposits': zones,
        'slot_minutes': _slot,
        'min_advance_minutes': _advance,
        'max_days_ahead': _days,
      });
      if (mounted) Navigator.pop(context, true);
    } on ApiException catch (e) {
      if (mounted) {
        setState(() {
          _saving = false;
          _error = e.message;
        });
      }
    }
  }

  Widget _dropdown(String label, int value, List<(int, String)> items, ValueChanged<int> on) => Row(children: [
        Expanded(child: Text(label, style: const TextStyle(fontSize: 13.5, color: OnDexColors.ink))),
        DropdownButton<int>(
          value: items.any((i) => i.$1 == value) ? value : items.first.$1,
          items: [for (final (v, t) in items) DropdownMenuItem(value: v, child: Text(t))],
          onChanged: (v) => v == null ? null : setState(() => on(v)),
        ),
      ]);

  Widget _sumField(TextEditingController c, String label, {String? hint}) => TextField(
        controller: c,
        keyboardType: TextInputType.number,
        inputFormatters: [FilteringTextInputFormatter.digitsOnly],
        decoration: InputDecoration(labelText: label, hintText: hint, suffixText: 'so\'m', border: const OutlineInputBorder(), isDense: true),
      );

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Bron sozlamalari'),
      content: SizedBox(
        width: 460,
        child: _loading
            ? const SizedBox(height: 120, child: Center(child: CircularProgressIndicator()))
            : SingleChildScrollView(
                child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                  SwitchListTile(
                    contentPadding: EdgeInsets.zero,
                    value: _enabled,
                    activeTrackColor: OnDexColors.success,
                    onChanged: (v) => setState(() => _enabled = v),
                    title: const Text('Mijozlar stol bron qila oladi'),
                    subtitle: const Text('Faqat sig\'imi kiritilgan faol stollar bronga beriladi'),
                  ),
                  const SizedBox(height: 8),
                  _sumField(_deposit, 'Zaklad, kishi boshiga', hint: '0 — zakladsiz'),
                  const SizedBox(height: 6),
                  const Text(
                    'Bron vaqtidan 2+ soat oldin bekor qilinsa zaklad mijozga to\'liq qaytadi; kechroq yoki '
                    'kelmasa — sizda qoladi. Mijoz kelsa zakladni yakuniy hisobdan ayiring.',
                    style: TextStyle(fontSize: 12, color: OnDexColors.inkDim, height: 1.4),
                  ),
                  if (_zones.isNotEmpty) ...[
                    const SizedBox(height: 14),
                    const Text('Zona bo\'yicha zaklad (bo\'sh — umumiy qiymat)',
                        style: TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: OnDexColors.ink)),
                    const SizedBox(height: 8),
                    for (final e in _zones.entries)
                      Padding(padding: const EdgeInsets.only(bottom: 8), child: _sumField(e.value, e.key)),
                  ],
                  const SizedBox(height: 8),
                  _dropdown('Bron davomiyligi', _slot,
                      const [(60, '1 soat'), (90, '1,5 soat'), (120, '2 soat'), (150, '2,5 soat'), (180, '3 soat'), (240, '4 soat')],
                      (v) => _slot = v),
                  _dropdown('Eng kamida qancha oldin', _advance,
                      const [(0, 'Istalgan payt'), (30, '30 daqiqa'), (60, '1 soat'), (120, '2 soat'), (180, '3 soat'), (1440, '1 kun')],
                      (v) => _advance = v),
                  _dropdown('Necha kun oldinga', _days,
                      const [(7, '7 kun'), (14, '14 kun'), (30, '30 kun'), (60, '60 kun'), (90, '90 kun')], (v) => _days = v),
                  if (_error != null) ...[
                    const SizedBox(height: 8),
                    Text(_error!, style: const TextStyle(color: OnDexColors.danger)),
                  ],
                ]),
              ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: const Text('Bekor')),
        FilledButton(onPressed: _loading || _saving ? null : _save, child: const Text('Saqlash')),
      ],
    );
  }
}
