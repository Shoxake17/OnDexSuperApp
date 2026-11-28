import 'package:flutter/material.dart';

import '../../api.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// "Dori ma'lumoti" — ShoxPos `PharmInformation.tsx`.
///
/// 1:1: shtrix-kod, nom yoki partiya raqami bo'yicha qidiruv (Enter yoki
/// skaner); topilgan dori kartasi — kategoriya, doza, ishlab chiqaruvchi,
/// yaroqlilik muddati, chiqim va sotish narxi, miqdor, shkaf va qator,
/// tavsif; "Tahrirlash" → saqlash (faqat shu maydonlar).
class PharmInfoPage extends StatefulWidget {
  const PharmInfoPage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<PharmInfoPage> createState() => _PharmInfoPageState();
}

class _PharmInfoPageState extends State<PharmInfoPage> {
  final _q = TextEditingController();
  Map<String, dynamic>? _p;
  bool _editing = false;
  bool _loading = false;
  bool _saving = false;
  final _e = <String, TextEditingController>{
    for (final k in ['name', 'category', 'dosage', 'manufacturer', 'base', 'price', 'quantity', 'shelf', 'row', 'description'])
      k: TextEditingController(),
  };
  DateTime? _expiry;

  @override
  void dispose() {
    _q.dispose();
    for (final c in _e.values) {
      c.dispose();
    }
    super.dispose();
  }

  void _fill(Map<String, dynamic> p) {
    String s(String k) => (p[k] as String?) ?? '';
    _e['name']!.text = s('name');
    _e['category']!.text = s('category');
    _e['dosage']!.text = s('dosage');
    _e['manufacturer']!.text = s('manufacturer');
    _e['base']!.text = '${intOf(p['base_price_tiyin']) ~/ 100}';
    _e['price']!.text = '${intOf(p['price_tiyin']) ~/ 100}';
    _e['quantity']!.text = fmtQty(numOf(p['quantity']));
    _e['shelf']!.text = s('shelf_number');
    _e['row']!.text = s('row_number');
    _e['description']!.text = s('description');
    _expiry = DateTime.tryParse(s('expiry_date'));
  }

  Future<void> _search() async {
    final q = _q.text.trim();
    if (q.isEmpty) {
      toast(context, 'Qidirish uchun ma\'lumot kiriting!');
      return;
    }
    setState(() => _loading = true);
    try {
      final p = await api.pharmInfo(q);
      if (!mounted) return;
      setState(() {
        _p = p;
        _editing = false;
        _fill(p);
      });
      toast(context, 'Dori topildi!');
    } catch (e) {
      if (!mounted) return;
      setState(() => _p = null);
      toast(context, e is ApiException && e.statusCode == 404 ? 'Dori topilmadi!' : errText(e), error: true);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _save() async {
    final p = _p;
    if (p == null) return;
    String v(String k) => _e[k]!.text.trim();
    String two(int x) => x.toString().padLeft(2, '0');
    setState(() => _saving = true);
    try {
      final u = await api.pharmUpdate(p['barcode'] as String, {
        'name': v('name'),
        'category': v('category'),
        'dosage': v('dosage'),
        'manufacturer': v('manufacturer'),
        'base_price_tiyin': tiyinFromSomText(v('base')),
        'price_tiyin': tiyinFromSomText(v('price')),
        'quantity': double.tryParse(v('quantity')) ?? 0,
        'expiry_date': _expiry == null ? null : '${_expiry!.year}-${two(_expiry!.month)}-${two(_expiry!.day)}',
        'shelf_number': v('shelf'),
        'row_number': v('row'),
        'description': v('description'),
      });
      if (!mounted) return;
      setState(() {
        _p = u;
        _editing = false;
        _fill(u);
      });
      toast(context, 'Ma\'lumotlar saqlandi!');
    } catch (e) {
      if (!mounted) return;
      setState(() => _fill(p));
      toast(context, errText(e), error: true);
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Widget _cell(String label, String key, {bool number = false, String view = ''}) => Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(label, style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim, fontWeight: FontWeight.w600)),
          const SizedBox(height: 6),
          _editing
              ? TextField(
                  key: ValueKey('info-$key'),
                  controller: _e[key],
                  inputFormatters: number ? kNumberInput : null,
                  decoration: shopInput(''),
                )
              : Text(view.isNotEmpty ? view : (_e[key]!.text.isEmpty ? '—' : _e[key]!.text),
                  style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 15)),
        ],
      );

  @override
  Widget build(BuildContext context) {
    final p = _p;
    final cols = MediaQuery.sizeOf(context).width >= 900 ? 3 : 1;
    String two(int x) => x.toString().padLeft(2, '0');
    final expText = _expiry == null ? '—' : '${two(_expiry!.day)}.${two(_expiry!.month)}.${_expiry!.year}';
    final expired = _expiry != null && _expiry!.isBefore(DateTime.now());
    final cells = p == null
        ? <Widget>[]
        : [
            _cell('Kategoriya', 'category'),
            _cell('Doza', 'dosage'),
            _cell('Ishlab chiqaruvchi', 'manufacturer'),
            Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              const Text('Yaroqlilik muddati',
                  style: TextStyle(fontSize: 12, color: OnDexColors.inkDim, fontWeight: FontWeight.w600)),
              const SizedBox(height: 6),
              _editing
                  ? Row(children: [
                      Expanded(
                        child: OutlinedButton.icon(
                          key: const ValueKey('info-expiry'),
                          onPressed: () async {
                            final now = DateTime.now();
                            final d = await showDatePicker(
                                context: context,
                                initialDate: _expiry ?? now,
                                firstDate: DateTime(now.year - 10),
                                lastDate: DateTime(now.year + 20));
                            if (d != null) setState(() => _expiry = d);
                          },
                          icon: const Icon(Icons.calendar_month_rounded, size: 18),
                          label: Text(expText),
                        ),
                      ),
                      if (_expiry != null)
                        IconButton(
                            tooltip: 'Tozalash',
                            onPressed: () => setState(() => _expiry = null),
                            icon: const Icon(Icons.clear_rounded)),
                    ])
                  : Text(expText,
                      style: TextStyle(
                          fontWeight: FontWeight.w700, fontSize: 15, color: expired ? OnDexColors.danger : null)),
            ]),
            _cell('Asosiy (chiqim) narx', 'base', number: true, view: somFromTiyin(intOf(p['base_price_tiyin']))),
            _cell('Sotish narxi', 'price', number: true, view: somFromTiyin(intOf(p['price_tiyin']))),
            _cell('Miqdor', 'quantity', number: true),
            _cell('Shkaf', 'shelf'),
            _cell('Qator', 'row'),
          ];

    return ShopPage(maxWidth: 1000, children: [
      Row(children: [
        Expanded(
          child: SearchBox(
            controller: _q,
            autofocus: true,
            hint: 'Shtrix-kod, nom yoki partiya raqami — Enter',
            onChanged: (_) {},
            onSubmitted: (_) => _search(),
          ),
        ),
        const SizedBox(width: 10),
        FilledButton.icon(
          key: const ValueKey('info-search'),
          onPressed: _loading ? null : _search,
          icon: const Icon(Icons.search_rounded, size: 18),
          label: const Text('Qidirish'),
        ),
      ]),
      const SizedBox(height: 16),
      if (p == null)
        ShopCard(child: EmptyState(loading: _loading, text: 'Dorini qidiring — ma\'lumotlari shu yerda chiqadi'))
      else
        ShopCard(
          padding: const EdgeInsets.all(20),
          child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            Row(children: [
              Expanded(
                child: _editing
                    ? TextField(key: const ValueKey('info-name'), controller: _e['name'], decoration: shopInput('Nomi'))
                    : Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                        Text((p['name'] as String?) ?? '', style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w900)),
                        Text('${p['barcode']}${((p['batch_number'] as String?) ?? '').isEmpty ? '' : ' · partiya ${p['batch_number']}'}',
                            style: const TextStyle(fontFamily: 'monospace', color: OnDexColors.info)),
                      ]),
              ),
              if (p['prescription_required'] == true)
                Container(
                  margin: const EdgeInsets.only(left: 8),
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(color: OnDexColors.warningBg, borderRadius: BorderRadius.circular(20)),
                  child: const Text('Retsept bilan', style: TextStyle(color: OnDexColors.warning, fontWeight: FontWeight.w700)),
                ),
            ]),
            const SizedBox(height: 18),
            LayoutBuilder(builder: (context, c) {
              final w = (c.maxWidth - (cols - 1) * 16) / cols;
              return Wrap(spacing: 16, runSpacing: 16, children: [for (final x in cells) SizedBox(width: w, child: x)]);
            }),
            const SizedBox(height: 16),
            _cell('Tavsif', 'description'),
            const SizedBox(height: 20),
            Row(mainAxisAlignment: MainAxisAlignment.end, children: [
              if (_editing) ...[
                TextButton(
                    onPressed: () => setState(() {
                          _editing = false;
                          _fill(p);
                        }),
                    child: const Text('Bekor qilish')),
                const SizedBox(width: 8),
                FilledButton.icon(
                  key: const ValueKey('info-save'),
                  onPressed: _saving ? null : _save,
                  icon: const Icon(Icons.save_rounded, size: 18),
                  label: Text(_saving ? 'Saqlanmoqda...' : 'Saqlash'),
                ),
              ] else
                OutlinedButton.icon(
                  key: const ValueKey('info-edit'),
                  onPressed: () => setState(() => _editing = true),
                  icon: const Icon(Icons.edit_rounded, size: 18),
                  label: const Text('Tahrirlash'),
                ),
            ]),
          ]),
        ),
    ]);
  }
}
