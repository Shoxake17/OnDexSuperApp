import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// "Dori qo'shish" — ShoxPos `PharmAdd.tsx`.
///
/// 1:1: shtrix-kod + Enter — dorixona va global bazadan qidirib to'ldiradi;
/// majburiy: nom, shtrix-kod, ulgurji va sotuv narxi (ulgurji > sotuv —
/// xato); kategoriya bo'sh bo'lsa "Dorilar"; doza, ishlab chiqaruvchi,
/// yaroqlilik muddati, partiya, saqlash sharti, dori shakli, retsept, shkaf
/// va qator raqami, tavsif. Mavjud dori qayta qo'shilsa qoldiq va kirim
/// OSHADI (server). Saqlangach "Dorilar bazasi" ga o'tadi.
class PharmAddPage extends StatefulWidget {
  const PharmAddPage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<PharmAddPage> createState() => _PharmAddPageState();
}

class _PharmAddPageState extends State<PharmAddPage> {
  final _c = <String, TextEditingController>{
    for (final k in [
      'name', 'barcode', 'base', 'price', 'category', 'quantity', 'dosage', 'manufacturer', 'batch', //
      'storage', 'form', 'shelf', 'row', 'description',
    ])
      k: TextEditingController(text: k == 'quantity' ? '1' : ''),
  };
  DateTime? _expiry;
  bool _rx = false;
  bool _saving = false;
  (Color, String)? _msg;

  @override
  void dispose() {
    for (final c in _c.values) {
      c.dispose();
    }
    super.dispose();
  }

  void _say(Color color, String text) => setState(() => _msg = (color, text));

  Future<void> _lookup(String code) async {
    code = code.trim();
    if (code.isEmpty) return;
    _say(OnDexColors.info, 'Qidirilmoqda...');
    try {
      final hits = await api.pharmSearch(code);
      if (hits.isNotEmpty) {
        final fm = hits.where((h) => (h['barcode'] as String).toLowerCase() == code.toLowerCase()).firstOrNull ?? hits.first;
        String s(String k) => (fm[k] as String?) ?? '';
        final bp = intOf(fm['base_price_tiyin']), sp = intOf(fm['price_tiyin']);
        setState(() {
          _c['name']!.text = s('name');
          _c['base']!.text = bp > 0 ? '${bp ~/ 100}' : '';
          _c['price']!.text = sp > 0 ? '${sp ~/ 100}' : '';
          if (s('category').isNotEmpty) _c['category']!.text = s('category');
          _c['dosage']!.text = s('dosage');
          _c['manufacturer']!.text = s('manufacturer');
          _c['storage']!.text = s('storage_condition');
          _c['description']!.text = s('description');
          _c['form']!.text = s('dosage_form');
          _c['shelf']!.text = s('shelf_number');
          _c['row']!.text = s('row_number');
          _rx = fm['prescription_required'] == true;
        });
        _say(OnDexColors.success, 'Topildi: ${fm['name']}');
        return;
      }
      _c['name']!.clear();
      _c['base']!.clear();
      _c['price']!.clear();
      _say(OnDexColors.info, 'Dori topilmadi. Yangi ma\'lumotlarni kiriting.');
    } catch (e) {
      _say(OnDexColors.danger, errText(e));
    }
  }

  Future<void> _submit() async {
    String v(String k) => _c[k]!.text.trim();
    if (v('name').isEmpty) return _say(OnDexColors.danger, 'Dori nomini kiriting');
    if (v('barcode').isEmpty) return _say(OnDexColors.danger, 'Shtrix-kodni kiriting');
    if (v('base').isEmpty) return _say(OnDexColors.danger, 'Ulgurji narxini kiriting');
    if (v('price').isEmpty) return _say(OnDexColors.danger, 'Sotuv narxini kiriting');
    final bp = tiyinFromSomText(v('base')), sp = tiyinFromSomText(v('price'));
    if (bp > sp) return _say(OnDexColors.danger, 'Ulgurji narx sotuv narxidan yuqori bo\'lishi mumkin emas!');
    setState(() {
      _saving = true;
      _msg = null;
    });
    String two(int x) => x.toString().padLeft(2, '0');
    try {
      await api.pharmAdd({
        'barcode': v('barcode'),
        'name': v('name'),
        'category': v('category').isEmpty ? 'Dorilar' : v('category'),
        'price_tiyin': sp,
        'base_price_tiyin': bp,
        'quantity': double.tryParse(v('quantity')) ?? 0,
        'dosage': v('dosage'),
        'manufacturer': v('manufacturer'),
        'expiry_date': _expiry == null ? '' : '${_expiry!.year}-${two(_expiry!.month)}-${two(_expiry!.day)}',
        'batch_number': v('batch'),
        'prescription_required': _rx,
        'storage_condition': v('storage'),
        'description': v('description'),
        'dosage_form': v('form'),
        'shelf_number': v('shelf'),
        'row_number': v('row'),
      });
      if (!mounted) return;
      _say(OnDexColors.success, 'Dori muvaffaqiyatli qo\'shildi!');
      await Future<void>.delayed(const Duration(milliseconds: 1200));
      if (mounted) widget.ctx.go(StorePage.pharmBase);
    } catch (e) {
      if (mounted) _say(OnDexColors.danger, errText(e));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Widget _f(String key, String label, String hint, {bool number = false, String? suffix, int lines = 1, ValueChanged<String>? onSubmitted}) =>
      Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        FieldLabel(label),
        TextField(
          key: ValueKey('pharm-$key'),
          controller: _c[key],
          maxLines: lines,
          inputFormatters: number ? kNumberInput : null,
          onSubmitted: onSubmitted,
          decoration: shopInput(hint, suffix: suffix),
        ),
      ]);

  Widget _row(List<Widget> children, bool wide) => wide
      ? Padding(
          padding: const EdgeInsets.only(bottom: 14),
          child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
            for (var i = 0; i < children.length; i++) ...[
              if (i > 0) const SizedBox(width: 12),
              Expanded(child: children[i]),
            ],
          ]),
        )
      : Column(children: [for (final c in children) Padding(padding: const EdgeInsets.only(bottom: 14), child: c)]);

  @override
  Widget build(BuildContext context) {
    final wide = MediaQuery.sizeOf(context).width >= 760;
    String two(int x) => x.toString().padLeft(2, '0');
    return ShopPage(maxWidth: 900, children: [
      ShopCard(
        padding: const EdgeInsets.all(24),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          if (_msg != null)
            Container(
              margin: const EdgeInsets.only(bottom: 16),
              padding: const EdgeInsets.all(12),
              decoration:
                  BoxDecoration(color: _msg!.$1.withValues(alpha: .1), borderRadius: BorderRadius.circular(12)),
              child: Text(_msg!.$2, style: TextStyle(color: _msg!.$1, fontWeight: FontWeight.w600)),
            ),
          _row([
            _f('name', 'Dori nomi', 'Dori nomi...'),
            _f('barcode', 'Shtrix-kod', 'Skanerlang yoki kiriting va Enter', onSubmitted: _lookup),
          ], wide),
          _row([
            _f('base', 'Ulgurji narx', '0', number: true, suffix: 'so\'m'),
            _f('price', 'Sotuv narxi', '0', number: true, suffix: 'so\'m'),
          ], wide),
          _row([
            _f('category', 'Kategoriya', 'Kategoriya...'),
            _f('quantity', 'Miqdor', '1', number: true),
          ], wide),
          _row([
            _f('dosage', 'Doza', 'Doza (masalan: 500mg)...'),
            _f('manufacturer', 'Ishlab chiqaruvchi', 'Ishlab chiqaruvchi...'),
          ], wide),
          _row([
            Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              const FieldLabel('Yaroqlilik muddati'),
              InkWell(
                key: const ValueKey('pharm-expiry'),
                borderRadius: BorderRadius.circular(12),
                onTap: () async {
                  final now = DateTime.now();
                  final d = await showDatePicker(
                      context: context,
                      initialDate: _expiry ?? now.add(const Duration(days: 365)),
                      firstDate: DateTime(now.year - 5),
                      lastDate: DateTime(now.year + 20));
                  if (d != null) setState(() => _expiry = d);
                },
                child: InputDecorator(
                  decoration: shopInput('').copyWith(suffixIcon: const Icon(Icons.calendar_month_rounded)),
                  child: Text(_expiry == null ? 'Tanlang' : '${two(_expiry!.day)}.${two(_expiry!.month)}.${_expiry!.year}'),
                ),
              ),
            ]),
            _f('batch', 'Partiya raqami', 'Partiya raqami...'),
          ], wide),
          _row([
            _f('storage', 'Saqlash sharti', 'Saqlash sharti (masalan: 2-8°C)...'),
            _f('form', 'Dori shakli', 'Tabletka, sirop, vial...'),
          ], wide),
          _row([
            _f('shelf', 'Shkaf raqami', 'Shkaf raqami (masalan: A1)...'),
            _f('row', 'Qator raqami', 'Qator raqami...'),
          ], wide),
          SwitchListTile(
            key: const ValueKey('pharm-rx'),
            contentPadding: EdgeInsets.zero,
            title: const Text('Retsept talab qilinadimi?', style: TextStyle(fontWeight: FontWeight.w700)),
            subtitle: Text(_rx ? 'Ha' : 'Yo\'q'),
            value: _rx,
            onChanged: (v) => setState(() => _rx = v),
          ),
          const SizedBox(height: 8),
          _f('description', 'Tavsif', 'Qo\'shimcha ma\'lumot...', lines: 3),
          const SizedBox(height: 20),
          Row(children: [
            Expanded(
              child: OutlinedButton(
                onPressed: () => widget.ctx.go(StorePage.pharmBase),
                style: OutlinedButton.styleFrom(padding: const EdgeInsets.symmetric(vertical: 16)),
                child: const Text('Bekor qilish'),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              flex: 2,
              child: FilledButton(
                key: const ValueKey('pharm-save'),
                onPressed: _saving ? null : _submit,
                child: Text(_saving ? 'Saqlanmoqda...' : 'Dorini saqlash'),
              ),
            ),
          ]),
        ]),
      ),
    ]);
  }
}
