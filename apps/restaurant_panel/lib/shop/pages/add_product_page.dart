import 'dart:math';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../api.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// "Mahsulot qo'shish" / "Mahsulotni tahrirlash" — ShoxPos `MaxQoshish.tsx`.
///
/// Oddiy do'kon: shtrix-kod (Enter — avval do'kon, keyin baza/global
/// qidiruv bilan to'ldiriladi), nom, kategoriya, ulgurji va sotuv narxi
/// (ulgurji > sotuv — xato), miqdor (−/+). Mavjud shtrix-kod qayta
/// qo'shilmaydi.
///
/// Qurt: shtrix-kod maydoni YO'Q (kalit `QURT-...` o'zi yaratiladi), rasm,
/// gramm (1..100000), tavsif, tarkib va "Mavjud / Mavjud emas" (mavjud emas =
/// 0 dona — mijozlarga ko'rinmaydi). Tahrir: `ctx.arg` = shtrix-kod.
///
/// Kamera skaneri yo'q: kompyuterdagi USB shtrix-kod skaneri klaviatura
/// kabi yozadi va Enter bosadi — aynan shu oqim ishlaydi.
class AddProductPage extends StatefulWidget {
  const AddProductPage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<AddProductPage> createState() => _AddProductPageState();
}

enum _MsgType { success, error, info }

class _AddProductPageState extends State<AddProductPage> {
  final _barcode = TextEditingController();
  final _name = TextEditingController();
  final _base = TextEditingController();
  final _price = TextEditingController();
  final _qty = TextEditingController(text: '1');
  final _weight = TextEditingController();
  final _desc = TextEditingController();
  final _comp = TextEditingController();
  final _newCat = TextEditingController();

  List<String> _cats = [];
  List<String> _extraCats = [];
  String _category = '';
  String _imageUrl = '';
  bool _available = true;
  bool _uploading = false;
  bool _saving = false;
  bool _lookingUp = false;
  (_MsgType, String)? _msg;

  bool get _isQurt => widget.ctx.isQurt;
  String? get _edit => _isQurt ? widget.ctx.arg : null;
  bool get _isEdit => _edit != null && _edit!.isNotEmpty;

  static const _qurtCatsKey = 'qurtCats';

  @override
  void initState() {
    super.initState();
    _base.addListener(_priceGuard);
    _price.addListener(_priceGuard);
    _init();
  }

  @override
  void dispose() {
    for (final c in [_barcode, _name, _base, _price, _qty, _weight, _desc, _comp, _newCat]) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> _init() async {
    try {
      final cats = await api.shopCategories();
      if (_isQurt) {
        final prefs = await SharedPreferences.getInstance();
        _extraCats = prefs.getStringList(_qurtCatsKey) ?? [];
      }
      if (!mounted) return;
      setState(() {
        _cats = cats;
        if (_category.isEmpty && !_isQurt && cats.isNotEmpty) _category = cats.first;
      });
      if (_isEdit) await _loadEdit();
    } catch (e) {
      if (mounted) _show(_MsgType.error, errText(e));
    }
  }

  Future<void> _loadEdit() async {
    final all = await api.shopProducts();
    final p = all.where((x) => x['barcode'] == _edit).firstOrNull;
    if (!mounted) return;
    if (p == null) {
      widget.ctx.go(StorePage.products);
      return;
    }
    final q = numOf(p['quantity']);
    setState(() {
      _name.text = (p['name'] as String?) ?? '';
      _price.text = '${intOf(p['price_tiyin']) ~/ 100}';
      _base.text = intOf(p['base_price_tiyin']) > 0 ? '${intOf(p['base_price_tiyin']) ~/ 100}' : '';
      _qty.text = q > 0 ? fmtQty(q) : '1';
      _available = q > 0;
      _category = (p['category'] as String?) ?? '';
      _weight.text = intOf(p['weight_grams']) > 0 ? '${intOf(p['weight_grams'])}' : '';
      _desc.text = (p['description'] as String?) ?? '';
      _comp.text = (p['composition'] as String?) ?? '';
      _imageUrl = (p['image_url'] as String?) ?? '';
    });
  }

  void _show(_MsgType t, String text) => setState(() => _msg = (t, text));

  /// ShoxPos: ulgurji narx sotuv narxidan yuqori bo'lsa darhol ogohlantirish.
  void _priceGuard() {
    final bp = double.tryParse(_base.text) ?? 0, sp = double.tryParse(_price.text) ?? 0;
    if (bp > sp && bp > 0 && sp > 0) {
      _show(_MsgType.error, 'Ulgurji narx sotuv narxidan yuqori bo\'lishi mumkin emas!');
    } else if (_msg?.$1 == _MsgType.error && _msg!.$2.contains('Ulgurji')) {
      setState(() => _msg = null);
    }
  }

  Future<void> _lookup(String code) async {
    code = code.trim();
    if (code.isEmpty) return;
    setState(() => _lookingUp = true);
    _show(_MsgType.info, 'Qidirilmoqda...');
    try {
      // 1) Do'konning o'z mahsuloti.
      try {
        final own = await api.shopScan(barcode: code);
        if (own['source'] == 'user') {
          _fill(own);
          _show(_MsgType.success, 'Do\'kon bazasida topildi: ${own['name']}');
          return;
        }
      } on ApiException catch (e) {
        if (e.statusCode != 404) rethrow;
      }
      // 2) Qidiruv (do'kon + global): aniq shtrix-kod, bo'lmasa birinchisi.
      final hits = await api.shopSearch(code);
      if (hits.isNotEmpty) {
        final fm = hits.where((h) => (h['barcode'] as String).toLowerCase() == code.toLowerCase()).firstOrNull ?? hits.first;
        _fill(fm);
        _show(_MsgType.success, '${fm['is_global'] == true ? 'Global bazada topildi:' : 'Bazada topildi:'} ${fm['name']}');
        return;
      }
      _name.clear();
      _base.clear();
      _price.clear();
      _show(_MsgType.info, 'Mahsulot topilmadi. Yangi ma\'lumotlarni kiriting.');
    } catch (e) {
      _show(_MsgType.error, errText(e));
    } finally {
      if (mounted) setState(() => _lookingUp = false);
    }
  }

  void _fill(Map<String, dynamic> p) {
    _name.text = (p['name'] as String?) ?? '';
    final bp = intOf(p['base_price_tiyin']), sp = intOf(p['price_tiyin']);
    _base.text = bp > 0 ? '${bp ~/ 100}' : '';
    _price.text = sp > 0 ? '${sp ~/ 100}' : '';
    final c = (p['category'] as String?) ?? '';
    if (c.isNotEmpty) setState(() => _category = c);
  }

  Future<void> _pickImage() async {
    final r = await FilePicker.platform.pickFiles(
        type: FileType.custom, allowedExtensions: ['jpg', 'jpeg', 'png', 'webp'], withData: true);
    final f = r?.files.single;
    if (f == null || f.bytes == null) return;
    final ext = (f.extension ?? '').toLowerCase();
    if (!['jpg', 'jpeg', 'png', 'webp'].contains(ext)) {
      _show(_MsgType.error, 'Faqat jpg, png yoki webp rasm tanlang');
      return;
    }
    if (f.size > 5 * 1024 * 1024) {
      _show(_MsgType.error, 'Rasm juda katta (maks 5 MB)');
      return;
    }
    setState(() {
      _uploading = true;
      _msg = null;
    });
    try {
      final res = await api.sendMultipart('POST', '/merchant/qurt/uploads', f.bytes!, f.name,
          errorText: 'Rasm yuklab bo\'lmadi');
      if (!mounted) return;
      setState(() => _imageUrl = (res is Map ? res['url'] as String? : null) ?? '');
    } catch (e) {
      if (mounted) _show(_MsgType.error, errText(e));
    } finally {
      if (mounted) setState(() => _uploading = false);
    }
  }

  Future<void> _addQurtCategory() async {
    final v = _newCat.text.trim();
    if (v.isEmpty) return;
    final value = v.length > 64 ? v.substring(0, 64) : v;
    final known = [..._cats, ..._extraCats].where((x) => x.toLowerCase() == value.toLowerCase()).firstOrNull;
    final fin = known ?? value;
    if (known == null) {
      _extraCats = [..._extraCats, fin];
      final prefs = await SharedPreferences.getInstance();
      await prefs.setStringList(_qurtCatsKey, _extraCats);
    }
    setState(() {
      _category = fin;
      _newCat.clear();
    });
  }

  String _qurtKey() {
    final r = Random.secure();
    final rnd = List.generate(4, (_) => '0123456789abcdefghijklmnopqrstuvwxyz'[r.nextInt(36)]).join();
    return 'QURT-${DateTime.now().millisecondsSinceEpoch.toRadixString(36)}$rnd'.toUpperCase();
  }

  Future<void> _submit() async {
    final code = _isEdit ? _edit! : _isQurt ? _qurtKey() : _barcode.text.trim();
    if (_name.text.trim().isEmpty) return _show(_MsgType.error, 'Mahsulot nomini kiriting');
    if (code.isEmpty) return _show(_MsgType.error, 'Shtrix-kodni kiriting');
    if (!_isQurt && _base.text.trim().isEmpty) return _show(_MsgType.error, 'Ulgurji narxni kiriting');
    if (_price.text.trim().isEmpty) return _show(_MsgType.error, 'Sotuv narxini kiriting');
    final bp = tiyinFromSomText(_base.text), sp = tiyinFromSomText(_price.text);
    if (bp > sp) return _show(_MsgType.error, 'Ulgurji narx sotuv narxidan yuqori bo\'lishi mumkin emas!');

    int? grams;
    if (_isQurt) {
      grams = int.tryParse(_weight.text.trim());
      if (grams == null || grams < 1 || grams > 100000) {
        return _show(_MsgType.error, 'Nechi grammligini kiriting (1 dan 100000 gacha)');
      }
      if (_category.isEmpty) return _show(_MsgType.error, 'Kategoriyani tanlang');
      if (_available && !((double.tryParse(_qty.text) ?? 0) >= 1)) {
        return _show(_MsgType.error,
            'Mavjud mahsulot uchun miqdor kamida 1 bo\'lsin (yoki «Mavjud emas» ni tanlang)');
      }
      if (_uploading) return _show(_MsgType.error, 'Rasm yuklanishini kuting');
      if (_imageUrl.isEmpty) return _show(_MsgType.error, 'Mahsulot rasmini tanlang');
    }

    setState(() {
      _saving = true;
      _msg = null;
    });
    try {
      if (!_isEdit) {
        // ShoxPos: mavjud shtrix-kod qayta qo'shilmaydi.
        try {
          final dup = await api.shopScan(barcode: code);
          if (dup['source'] == 'user') {
            _show(_MsgType.error, 'Bu shtrix-kod bilan mahsulot allaqachon bor: ${dup['name']}');
            return;
          }
        } on ApiException catch (e) {
          if (e.statusCode != 404) rethrow;
        }
      }
      await api.shopUpsert(
        code,
        name: _name.text.trim(),
        category: _category.isEmpty ? 'general' : _category,
        priceTiyin: sp,
        basePriceTiyin: bp,
        quantity: _isQurt && !_available ? 0 : (double.tryParse(_qty.text) ?? 0),
        weightGrams: grams,
        description: _isQurt ? _desc.text.trim() : null,
        composition: _isQurt ? _comp.text.trim() : null,
        imageUrl: _isQurt ? _imageUrl : null,
      );
      if (!mounted) return;
      _show(_MsgType.success, 'Mahsulot muvaffaqiyatli saqlandi');
      await Future<void>.delayed(const Duration(milliseconds: 1200));
      if (mounted) widget.ctx.go(StorePage.products);
    } catch (e) {
      if (mounted) _show(_MsgType.error, errText(e));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Widget _field(String label, TextEditingController c,
          {String hint = '', bool number = false, String? suffix, int maxLines = 1, int? maxLength, Key? key, ValueChanged<String>? onSubmitted, String? labelHint}) =>
      Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        FieldLabel(label, hint: labelHint),
        TextField(
          key: key,
          controller: c,
          maxLines: maxLines,
          maxLength: maxLength,
          inputFormatters: number ? kNumberInput : null,
          onSubmitted: onSubmitted,
          decoration: shopInput(hint, suffix: suffix).copyWith(counterText: ''),
        ),
      ]);

  Widget _qtyField() => Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        const FieldLabel('Miqdor'),
        Opacity(
          opacity: _isQurt && !_available ? .5 : 1,
          child: Row(children: [
            IconButton.filledTonal(
              key: const ValueKey('qty-minus'),
              onPressed: () => setState(() {
                final v = (double.tryParse(_qty.text) ?? 0) - 1;
                _qty.text = fmtQty(v < 0 ? 0 : v);
              }),
              icon: const Icon(Icons.remove),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: TextField(
                key: const ValueKey('qty'),
                controller: _qty,
                textAlign: TextAlign.center,
                inputFormatters: kNumberInput,
                decoration: shopInput('0'),
              ),
            ),
            const SizedBox(width: 8),
            IconButton.filled(
              key: const ValueKey('qty-plus'),
              onPressed: () => setState(() => _qty.text = fmtQty((double.tryParse(_qty.text) ?? 0) + 1)),
              icon: const Icon(Icons.add),
            ),
          ]),
        ),
      ]);

  Widget _categoryField() {
    if (!_isQurt) {
      return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        const FieldLabel('Kategoriya'),
        InkWell(
          key: const ValueKey('add-category'),
          borderRadius: BorderRadius.circular(12),
          onTap: () async {
            final v = await pickCategory(context, _cats, _category);
            if (v != null) setState(() => _category = v);
          },
          child: InputDecorator(
            decoration: shopInput(''),
            child: Row(children: [
              Text(_category.isEmpty ? '📦' : categoryEmoji(_category), style: const TextStyle(fontSize: 18)),
              const SizedBox(width: 10),
              Expanded(child: Text(_category.isEmpty ? 'Tanlang' : _category)),
              const Icon(Icons.expand_more_rounded),
            ]),
          ),
        ),
      ]);
    }
    final all = {..._cats, ..._extraCats}.toList();
    return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      const FieldLabel('Kategoriya *'),
      InputDecorator(
        decoration: shopInput('').copyWith(contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4)),
        child: DropdownButtonHideUnderline(
          child: DropdownButton<String>(
            key: const ValueKey('qurt-category'),
            isExpanded: true,
            value: all.contains(_category) ? _category : null,
            hint: const Text('Tanlang'),
            items: [for (final c in all) DropdownMenuItem(value: c, child: Text(c))],
            onChanged: (v) => setState(() => _category = v ?? ''),
          ),
        ),
      ),
      const SizedBox(height: 8),
      Row(children: [
        Expanded(
          child: TextField(
            key: const ValueKey('qurt-new-category'),
            controller: _newCat,
            maxLength: 64,
            onSubmitted: (_) => _addQurtCategory(),
            decoration: shopInput('Yangi kategoriya').copyWith(counterText: ''),
          ),
        ),
        const SizedBox(width: 8),
        IconButton.filled(
            tooltip: 'Kategoriya qo\'shish', onPressed: _addQurtCategory, icon: const Icon(Icons.add)),
      ]),
    ]);
  }

  Widget _imageBox() => Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        const FieldLabel('Rasm *'),
        InkWell(
          key: const ValueKey('qurt-image'),
          borderRadius: BorderRadius.circular(14),
          onTap: _uploading ? null : _pickImage,
          child: AspectRatio(
            aspectRatio: 2.4,
            child: Container(
              decoration: BoxDecoration(
                color: OnDexColors.pageBg,
                borderRadius: BorderRadius.circular(14),
                border: Border.all(color: OnDexColors.cardBorder),
              ),
              clipBehavior: Clip.antiAlias,
              child: Stack(fit: StackFit.expand, children: [
                if (_imageUrl.isNotEmpty)
                  Image.network(fullImageUrl(_imageUrl), fit: BoxFit.contain, errorBuilder: (_, __, ___) => const SizedBox()),
                Align(
                  alignment: Alignment.bottomCenter,
                  child: Container(
                    width: double.infinity,
                    color: Colors.black.withValues(alpha: _imageUrl.isEmpty ? 0 : .45),
                    padding: const EdgeInsets.all(8),
                    child: Text(
                      _uploading
                          ? 'Yuklanmoqda...'
                          : _imageUrl.isNotEmpty
                              ? 'Almashtirish uchun bosing'
                              : 'Rasm tanlash',
                      textAlign: TextAlign.center,
                      style: TextStyle(
                          color: _imageUrl.isEmpty ? OnDexColors.inkDim : Colors.white, fontWeight: FontWeight.w600),
                    ),
                  ),
                ),
                if (_imageUrl.isEmpty && !_uploading)
                  const Center(child: Icon(Icons.add_photo_alternate_outlined, size: 40, color: OnDexColors.inkFaint)),
                if (_uploading) const Center(child: CircularProgressIndicator()),
              ]),
            ),
          ),
        ),
        const Padding(
          padding: EdgeInsets.only(top: 6),
          child: Text('jpg, png yoki webp — maks 5 MB', style: TextStyle(fontSize: 11.5, color: OnDexColors.inkDim)),
        ),
      ]);

  Widget _availability() => Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        const FieldLabel('Mavjudlik *', hint: '«Mavjud emas» bo\'lsa mahsulot mijozlarga ko\'rinmaydi'),
        SegmentedButton<bool>(
          key: const ValueKey('qurt-available'),
          segments: const [
            ButtonSegment(value: true, label: Text('Mavjud'), icon: Icon(Icons.check_circle_outline)),
            ButtonSegment(value: false, label: Text('Mavjud emas'), icon: Icon(Icons.block)),
          ],
          selected: {_available},
          onSelectionChanged: (s) => setState(() => _available = s.first),
        ),
      ]);

  @override
  Widget build(BuildContext context) {
    final wide = MediaQuery.sizeOf(context).width >= 1100;
    const gap = SizedBox(height: 16);
    final msg = _msg;
    final message = msg == null
        ? const SizedBox.shrink()
        : Container(
            key: const ValueKey('add-message'),
            margin: const EdgeInsets.only(bottom: 16),
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: switch (msg.$1) {
                _MsgType.success => OnDexColors.successBg,
                _MsgType.error => OnDexColors.dangerBg,
                _MsgType.info => OnDexColors.infoBg,
              },
              borderRadius: BorderRadius.circular(12),
            ),
            child: Text(msg.$2,
                style: TextStyle(
                  fontWeight: FontWeight.w600,
                  color: switch (msg.$1) {
                    _MsgType.success => OnDexColors.success,
                    _MsgType.error => OnDexColors.danger,
                    _MsgType.info => OnDexColors.info,
                  },
                )),
          );

    final prices = Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Expanded(
          child: _field(_isQurt ? 'Sotuv narxi *' : 'Sotuv narxi', _price,
              hint: '0', number: true, suffix: 'so\'m', key: const ValueKey('add-price'))),
      const SizedBox(width: 12),
      Expanded(
          child: _field(_isQurt ? 'Ulgurji narx' : 'Ulgurji narx', _base,
              hint: '0',
              number: true,
              suffix: 'so\'m',
              key: const ValueKey('add-base'),
              labelHint: _isQurt ? 'Ixtiyoriy — faqat siz ko\'rasiz' : null)),
    ]);

    final buttons = Row(children: [
      Expanded(
        child: OutlinedButton(
          onPressed: () => widget.ctx.go(StorePage.products),
          style: OutlinedButton.styleFrom(padding: const EdgeInsets.symmetric(vertical: 16)),
          child: const Text('Bekor qilish'),
        ),
      ),
      const SizedBox(width: 12),
      Expanded(
        flex: 2,
        child: FilledButton(
          key: const ValueKey('add-save'),
          onPressed: _saving || _uploading ? null : _submit,
          child: _saving
              ? const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
              : Text(_isEdit ? 'Saqlash' : 'Mahsulotni saqlash'),
        ),
      ),
    ]);

    if (!_isQurt) {
      return ShopPage(maxWidth: 850, children: [
        ShopCard(
          padding: const EdgeInsets.all(28),
          child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            message,
            _field('Shtrix-kod', _barcode,
                hint: 'Skanerlang yoki kiriting va Enter bosing',
                key: const ValueKey('add-barcode'),
                onSubmitted: _lookup,
                labelHint: _lookingUp ? 'Qidirilmoqda...' : null),
            gap,
            _field('Mahsulot nomi', _name, hint: 'Mahsulot nomini kiriting', key: const ValueKey('add-name')),
            gap,
            _categoryField(),
            gap,
            prices,
            gap,
            _qtyField(),
            const SizedBox(height: 24),
            buttons,
          ]),
        ),
      ]);
    }

    final left = Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      ShopCard(
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          _field('Mahsulot nomi *', _name, hint: 'Masalan: Sho\'r qurt', maxLength: 120, key: const ValueKey('add-name')),
          gap,
          _categoryField(),
        ]),
      ),
      gap,
      ShopCard(
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          prices,
          gap,
          _field('Nechi gramm *', _weight, hint: 'Masalan: 250', number: true, suffix: 'g', key: const ValueKey('add-weight')),
          gap,
          _qtyField(),
        ]),
      ),
    ]);
    final right = Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      ShopCard(child: _imageBox()),
      gap,
      ShopCard(child: _availability()),
      gap,
      ShopCard(
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          _field('Tavsif', _desc, hint: 'Mahsulot haqida qisqacha', maxLines: 3, maxLength: 1000),
          gap,
          _field('Tarkib', _comp, hint: 'Masalan: qatiq, tuz', maxLines: 2, maxLength: 1000),
        ]),
      ),
    ]);
    return ShopPage(maxWidth: 1180, children: [
      message,
      if (wide)
        Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Expanded(child: left),
          const SizedBox(width: 16),
          Expanded(child: right),
        ])
      else ...[
        left,
        gap,
        right,
      ],
      const SizedBox(height: 20),
      buttons,
    ]);
  }
}
