part of '../restaurant_settings_page.dart';

// ─── Olib ketish (Borish, Plane 477) ─────────────────────────────────────

/// Mijoz oldindan to'lab, restoranga O'ZI keladi. Mustaqil karta: faqat
/// `pickup` maydonini saqlaydi (sahifaning umumiy "Saqlash"iga bog'lanmagan).
///
/// Egasi qarori: to'lov faqat oldindan (karta / OnDex Wallet), chegirma 0..30%,
/// mijoz 2 soat kelmasa buyurtma "olib ketilmadi" bo'lib yopiladi (pul qaytmaydi).
class _PickupCard extends StatefulWidget {
  const _PickupCard({required this.initial, required this.onlineAvailable});

  final Map<String, dynamic> initial;
  final bool onlineAvailable;

  @override
  State<_PickupCard> createState() => _PickupCardState();
}

class _PickupCardState extends State<_PickupCard> {
  late bool _enabled;
  late double _prep;
  late double _discount;
  late final TextEditingController _note;
  bool _saving = false;
  bool _dirty = false;

  int _int(Object? v, int d) => v is num ? v.toInt() : d;

  @override
  void initState() {
    super.initState();
    final p = widget.initial;
    _enabled = p['enabled'] == true;
    _prep = _int(p['prep_minutes'], 15).clamp(5, 180).toDouble();
    _discount = _int(p['discount_percent'], 0).clamp(0, 30).toDouble();
    _note = TextEditingController(text: '${p['note'] ?? ''}')..addListener(_touch);
  }

  @override
  void dispose() {
    _note.dispose();
    super.dispose();
  }

  void _touch() {
    if (!_dirty) setState(() => _dirty = true);
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    try {
      await api.updateRestaurantSettings({
        'pickup': {
          'enabled': _enabled,
          'prep_minutes': _prep.round(),
          'discount_percent': _discount.round(),
          'note': _note.text.trim(),
        },
      });
      if (!mounted) return;
      setState(() {
        _saving = false;
        _dirty = false;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Olib ketish sozlamasi saqlandi')),
      );
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _saving = false);
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

  @override
  Widget build(BuildContext context) {
    return _SettingsCard(
      icon: Icons.storefront_outlined,
      title: 'Olib ketish',
      subtitle: 'Mijoz oldindan to\'laydi va restoranga o\'zi keladi',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // ListTile EMAS: kartaning rangli foni ostida ink ko'rinmaydi (test).
          Row(children: [
            const Expanded(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text('Olib ketishni qabul qilish', style: TextStyle(fontWeight: FontWeight.w700)),
                SizedBox(height: 2),
                Text('Mijoz ilovasida "Borish" bo\'limida ko\'rinadi',
                    style: TextStyle(fontSize: 12.5, color: OnDexColors.inkDim)),
              ]),
            ),
            Switch(
              key: const ValueKey('pickup-enabled'),
              value: _enabled,
              onChanged: (v) => setState(() {
                _enabled = v;
                _dirty = true;
              }),
            ),
          ]),
          const SizedBox(height: 8),
          if (_enabled && !widget.onlineAvailable)
            Container(
              margin: const EdgeInsets.only(bottom: 8),
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(color: OnDexColors.warningBg, borderRadius: BorderRadius.circular(8)),
              child: const Text(
                'Onlayn karta to\'lovi ulanmagan — mijozlar faqat OnDex Wallet bilan to\'lay oladi. '
                '"Moliya" bo\'limida to\'lov hisobini ulang.',
                style: TextStyle(color: OnDexColors.warning, fontSize: 12.5),
              ),
            ),
          AnimatedOpacity(
            duration: const Duration(milliseconds: 180),
            opacity: _enabled ? 1 : 0.45,
            child: IgnorePointer(
              ignoring: !_enabled,
              child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                const SizedBox(height: 6),
                Text('Tayyorlash vaqti: ${_prep.round()} daqiqa',
                    style: const TextStyle(fontWeight: FontWeight.w600)),
                Slider(
                  key: const ValueKey('pickup-prep'),
                  value: _prep,
                  min: 5,
                  max: 90,
                  divisions: 17,
                  label: '${_prep.round()} daq',
                  onChanged: (v) => setState(() {
                    _prep = v;
                    _dirty = true;
                  }),
                ),
                Text(
                  _discount.round() == 0 ? 'Chegirma: yo\'q' : 'Chegirma: ${_discount.round()}%',
                  style: const TextStyle(fontWeight: FontWeight.w600),
                ),
                Slider(
                  key: const ValueKey('pickup-discount'),
                  value: _discount,
                  min: 0,
                  max: 30,
                  divisions: 30,
                  label: '${_discount.round()}%',
                  onChanged: (v) => setState(() {
                    _discount = v;
                    _dirty = true;
                  }),
                ),
                const SizedBox(height: 6),
                TextField(
                  key: const ValueKey('pickup-note'),
                  controller: _note,
                  maxLength: 200,
                  decoration: const InputDecoration(
                    labelText: 'Mijozga ko\'rsatma (ixtiyoriy)',
                    hintText: 'Masalan: kassa 1-qavatda, kodni ayting',
                    border: OutlineInputBorder(),
                  ),
                ),
              ]),
            ),
          ),
          const Text(
            'Mijoz 2 soat ichida kelmasa buyurtma "olib ketilmadi" bo\'lib yopiladi va pul qaytarilmaydi.',
            style: TextStyle(fontSize: 12, color: OnDexColors.inkDim),
          ),
          const SizedBox(height: 10),
          Align(
            alignment: Alignment.centerRight,
            child: FilledButton(
              key: const ValueKey('pickup-save'),
              onPressed: _dirty && !_saving ? _save : null,
              child: _saving
                  ? const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
                  : const Text('Saqlash'),
            ),
          ),
        ],
      ),
    );
  }
}
