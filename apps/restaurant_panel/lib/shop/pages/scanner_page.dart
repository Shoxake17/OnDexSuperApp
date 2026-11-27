import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../api.dart';
import '../../printing/receipt_model.dart';
import '../../printing/receipt_printer.dart';
import '../../theme.dart';
import '../shop_api.dart';
import '../shop_widgets.dart';
import '../store_pages.dart';

/// Kassa qatori.
class CartLine {
  CartLine({
    required this.barcode,
    required this.name,
    required this.priceTiyin,
    required this.count,
    required this.category,
    required this.max,
    required this.global,
  });
  final String barcode;
  final String name;
  int priceTiyin;
  double count;
  final String category;

  /// Do'kondagi qoldiq; global mahsulotda cheksiz.
  final double max;
  final bool global;

  int get totalTiyin => (priceTiyin * count).round();
}

/// "Savdo terminali" — ShoxPos `Scanner.tsx` (+ `PrinterButton.tsx`).
///
/// 1:1: shtrix-kod yoki nom + Enter (raqamli 4+ belgi — faqat shtrix-kod,
/// aks holda nom bo'yicha ham), avval do'kon mahsulotlari, keyin server
/// qidiruvi (do'kon + global). Narxi 0 yoki global mahsulot — narx so'raladi
/// va do'konga yoziladi. Qoldiqdan ko'p qo'shilmaydi. To'lov: naqd (F9),
/// plastik (F10), qarz; F8 — chek chiqarib naqd sotish. Sotuvdan so'ng
/// "Sotuvlar" sahifasiga o'tiladi.
///
/// Farq: ShoxPos narxi 0 bo'lgan O'Z mahsulotiga narx qo'yganda qoldiqni ham
/// 1 ga tushirib yuborardi — bu yerda faqat narx yangilanadi. Chek WebUSB
/// o'rniga panelning tizim printeri orqali (Sozlamalar > Printer).
class ScannerPage extends StatefulWidget {
  const ScannerPage({super.key, required this.ctx});
  final StoreContext ctx;

  @override
  State<ScannerPage> createState() => ScannerPageState();
}

class ScannerPageState extends State<ScannerPage> {
  final _input = TextEditingController();
  final _focus = FocusNode();
  List<Map<String, dynamic>> _products = [];
  final List<CartLine> cart = [];
  String _status = 'Tayyor';
  bool _statusError = false;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _loadProducts();
  }

  @override
  void dispose() {
    _input.dispose();
    _focus.dispose();
    super.dispose();
  }

  Future<void> _loadProducts() async {
    try {
      final list = await api.shopProducts();
      if (mounted) setState(() => _products = list);
    } catch (_) {
      // Mahalliy kesh bo'lmasa ham server qidiruvi ishlaydi.
    }
  }

  int get totalTiyin => cart.fold(0, (s, l) => s + l.totalTiyin);

  void _say(String text, {bool error = false}) => setState(() {
        _status = text;
        _statusError = error;
      });

  static bool _barcodeLike(String q) => RegExp(r'^\d{4,}$').hasMatch(q.trim());

  /// (mahsulot, global?) yoki null.
  Future<(Map<String, dynamic>, bool)?> _find(String query) async {
    final q = query.trim().toLowerCase();
    if (q.isEmpty) return null;
    final barcodeMode = _barcodeLike(q);
    final local = _products.where((p) {
      if ((p['barcode'] as String).toLowerCase() == q) return true;
      return !barcodeMode && ((p['name'] as String?) ?? '').toLowerCase().contains(q);
    }).firstOrNull;
    if (local != null) return (local, false);
    final hits = await api.shopSearch(query.trim().replaceAll(RegExp(r'\s+'), ' '));
    if (hits.isEmpty) return null;
    final found = hits.first;
    if (found['is_global'] == true) return (found, true);
    setState(() => _products = [..._products, found]);
    return (found, false);
  }

  Future<int?> _askPrice(String name) =>
      showDialog<int>(context: context, builder: (_) => _PriceDialog(name: name));

  Future<void> addProduct(Map<String, dynamic> p, bool global, {double count = 1}) async {
    var price = intOf(p['price_tiyin']);
    final barcode = p['barcode'] as String;
    final name = (p['name'] as String?) ?? barcode;
    if (price == 0 || global) {
      final v = await _askPrice(name);
      if (v == null || v <= 0) {
        _say('Bekor qilindi', error: true);
        return;
      }
      price = v;
      try {
        if (global) {
          // ShoxPos: global mahsulot shu narx va sotilayotgan miqdor bilan do'konga yoziladi.
          await api.shopUpsert(barcode,
              name: name,
              category: (p['category'] as String?) ?? '',
              priceTiyin: price,
              basePriceTiyin: 0,
              quantity: count);
        } else {
          await api.shopUpdateField(barcode, 'price', price);
        }
      } catch (e) {
        _say(errText(e), error: true);
        return;
      }
    }
    final max = global ? double.infinity : numOf(p['quantity']);
    final existing = cart.where((l) => l.barcode == barcode).firstOrNull;
    if (existing != null) {
      if (!existing.global && existing.count + count > existing.max) {
        _say('Omborda yetarli emas', error: true);
        return;
      }
      setState(() => existing.count += count);
    } else {
      if (!global && count > max) {
        _say('Omborda yetarli emas', error: true);
        return;
      }
      setState(() => cart.add(CartLine(
          barcode: barcode,
          name: name,
          priceTiyin: price,
          count: count,
          category: (p['category'] as String?) ?? '',
          max: max,
          global: global)));
    }
    _say('$name — ${fmtQty(count)} ta qo\'shildi (${somFromTiyin(price)})');
    _input.clear();
  }

  Future<void> _onEnter(String v) async {
    final q = v.trim();
    if (q.isEmpty) return;
    try {
      final r = await _find(q);
      if (r == null) {
        _say('Mahsulot topilmadi', error: true);
      } else {
        await addProduct(r.$1, r.$2);
      }
    } catch (e) {
      _say(errText(e), error: true);
    }
    _input.clear();
    _focus.requestFocus();
  }

  void setCount(CartLine l, double v) {
    if (v < 1) {
      setState(() => cart.remove(l));
      return;
    }
    if (!l.global && v > l.max) {
      _say('Omborda yetarli emas', error: true);
      return;
    }
    setState(() => l.count = v);
  }

  Future<void> finalize(String payment) async {
    if (_busy) return;
    if (cart.isEmpty) {
      _say('Savat bo\'sh', error: true);
      return;
    }
    if (cart.any((l) => !l.global && l.count > l.max)) {
      _say('Omborda yetarli emas', error: true);
      return;
    }
    setState(() => _busy = true);
    try {
      await api.shopSell([
        for (final l in cart)
          {
            'barcode': l.barcode,
            'name': l.name,
            'quantity': l.count,
            'price_tiyin': l.priceTiyin,
            'total_tiyin': l.totalTiyin,
            'category': l.category,
          }
      ], payment);
      if (!mounted) return;
      setState(() => cart.clear());
      _say('Sotuv amalga oshirildi');
      await _loadProducts();
      await Future<void>.delayed(const Duration(seconds: 1));
      if (mounted) widget.ctx.go(StorePage.sales);
    } catch (e) {
      if (mounted) _say(errText(e), error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// F8 — chek chiqarish, so'ng naqd sotuv (ShoxPos PrinterButton bilan 1:1).
  Future<void> printAndSell() async {
    if (cart.isEmpty || _busy) return;
    _say('Chek tayyorlanmoqda...');
    try {
      await ReceiptPrinter.printReceipt(buildPosReceipt(widget.ctx.name, cart, DateTime.now()));
    } catch (e) {
      _say('Chek chiqmadi: printerni tekshiring', error: true);
      return;
    }
    await finalize('naqd');
  }

  @override
  Widget build(BuildContext context) {
    final narrow = MediaQuery.sizeOf(context).width < 900;
    return CallbackShortcuts(
      bindings: {
        const SingleActivator(LogicalKeyboardKey.f8): printAndSell,
        const SingleActivator(LogicalKeyboardKey.f9): () => finalize('naqd'),
        const SingleActivator(LogicalKeyboardKey.f10): () => finalize('plastik'),
      },
      child: Focus(
        autofocus: true,
        child: ShopPage(children: [
          Row(children: [
            Expanded(
              child: SearchBox(
                controller: _input,
                focusNode: _focus,
                autofocus: true,
                hint: 'Shtrix-kod raqamini yoki nomini kiriting',
                onChanged: (_) {},
                onSubmitted: _onEnter,
              ),
            ),
            if (!narrow) ...[
              const SizedBox(width: 16),
              Text('Jami: ', style: const TextStyle(fontSize: 18, color: OnDexColors.inkDim)),
              Text(somFromTiyin(totalTiyin),
                  key: const ValueKey('cart-total'),
                  style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w900, color: OnDexColors.primary)),
            ],
          ]),
          const SizedBox(height: 12),
          Container(
            key: const ValueKey('scanner-status'),
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            decoration: BoxDecoration(
              color: _statusError ? OnDexColors.dangerBg : OnDexColors.successBg,
              borderRadius: BorderRadius.circular(12),
            ),
            child: Row(children: [
              Icon(_statusError ? Icons.error_outline : Icons.check_circle_outline,
                  size: 18, color: _statusError ? OnDexColors.danger : OnDexColors.success),
              const SizedBox(width: 8),
              Expanded(
                  child: Text(_status,
                      style: TextStyle(
                          fontWeight: FontWeight.w600, color: _statusError ? OnDexColors.danger : OnDexColors.success))),
            ]),
          ),
          const SizedBox(height: 16),
          if (narrow) ...[
            Text('Jami: ${somFromTiyin(totalTiyin)}',
                key: const ValueKey('cart-total'),
                style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w900, color: OnDexColors.primary)),
            const SizedBox(height: 10),
            _cartList(),
            const SizedBox(height: 16),
            _payments(),
          ] else
            Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Expanded(child: _cartTable()),
              const SizedBox(width: 16),
              SizedBox(width: 300, child: _payments()),
            ]),
        ]),
      ),
    );
  }

  Widget _qtyStepper(CartLine l) => Row(mainAxisSize: MainAxisSize.min, children: [
        IconButton(
          key: ValueKey('minus-${l.barcode}'),
          visualDensity: VisualDensity.compact,
          onPressed: () => setCount(l, l.count - 1),
          icon: const Icon(Icons.remove_circle_outline, size: 20),
        ),
        SizedBox(
            width: 44,
            child: Text(fmtQty(l.count),
                textAlign: TextAlign.center, style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 15))),
        IconButton(
          key: ValueKey('plus-${l.barcode}'),
          visualDensity: VisualDensity.compact,
          onPressed: () => setCount(l, l.count + 1),
          icon: const Icon(Icons.add_circle_outline, size: 20),
        ),
      ]);

  Widget _cartTable() => ShopCard(
        padding: EdgeInsets.zero,
        child: cart.isEmpty
            ? const EmptyState(loading: false, text: 'Savat bo\'sh — mahsulotni skanerlang')
            : SizedBox(
                width: double.infinity,
                child: DataTable(
                  headingRowColor: WidgetStateProperty.all(OnDexColors.pageBg),
                  headingTextStyle:
                      const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w700, color: OnDexColors.inkDim),
                  columns: const [
                    DataColumn(label: Text('MAHSULOT')),
                    DataColumn(label: Text('SHTRIX-KOD')),
                    DataColumn(label: Text('MIQDOR')),
                    DataColumn(label: Text('NARX')),
                    DataColumn(label: Text('SUMMA')),
                  ],
                  rows: [
                    for (final l in cart)
                      DataRow(cells: [
                        DataCell(Text(l.name, style: const TextStyle(fontWeight: FontWeight.w600))),
                        DataCell(Text(l.barcode, style: const TextStyle(fontFamily: 'monospace', color: OnDexColors.info))),
                        DataCell(_qtyStepper(l)),
                        DataCell(Text(somFromTiyin(l.priceTiyin), style: const TextStyle(color: OnDexColors.inkDim))),
                        DataCell(Text(somFromTiyin(l.totalTiyin), style: const TextStyle(fontWeight: FontWeight.w800))),
                      ]),
                  ],
                ),
              ),
      );

  Widget _cartList() => cart.isEmpty
      ? const ShopCard(child: EmptyState(loading: false, text: 'Savat bo\'sh — mahsulotni skanerlang'))
      : Column(children: [
          for (final l in cart)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: ShopCard(
                padding: const EdgeInsets.all(12),
                child: Row(children: [
                  Expanded(
                    child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Text(l.name, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(fontWeight: FontWeight.w700)),
                      Text('${somFromTiyin(l.priceTiyin)} · ${somFromTiyin(l.totalTiyin)}',
                          style: const TextStyle(color: OnDexColors.inkDim, fontSize: 12.5)),
                    ]),
                  ),
                  _qtyStepper(l),
                ]),
              ),
            ),
        ]);

  Widget _payBtn(String key, IconData icon, Color color, String title, String sub, VoidCallback onTap) => Padding(
        padding: const EdgeInsets.only(bottom: 10),
        child: Material(
          color: OnDexColors.cardBg,
          shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(14), side: const BorderSide(color: OnDexColors.cardBorder)),
          child: InkWell(
            key: ValueKey(key),
            borderRadius: BorderRadius.circular(14),
            onTap: _busy ? null : onTap,
            child: Padding(
              padding: const EdgeInsets.all(14),
              child: Row(children: [
                Container(
                  width: 42,
                  height: 42,
                  decoration: BoxDecoration(color: color.withValues(alpha: .12), borderRadius: BorderRadius.circular(10)),
                  child: Icon(icon, color: color),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                    Text(title, style: const TextStyle(fontWeight: FontWeight.w800)),
                    Text(sub, style: const TextStyle(fontSize: 12, color: OnDexColors.inkDim)),
                  ]),
                ),
              ]),
            ),
          ),
        ),
      );

  Widget _payments() => Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        _payBtn('pay-print', Icons.print_rounded, const Color(0xFF06B6D4), 'Chek chiqarish  F8',
            ReceiptPrefs.printerName.value.isEmpty ? 'Tizim printeri' : ReceiptPrefs.printerName.value, printAndSell),
        _payBtn('pay-naqd', Icons.payments_rounded, OnDexColors.success, 'Naqd  F9', 'Naqd sotish', () => finalize('naqd')),
        _payBtn('pay-plastik', Icons.credit_card_rounded, OnDexColors.info, 'Plastik  F10', 'Plastik sotish',
            () => finalize('plastik')),
        _payBtn('pay-qarz', Icons.account_balance_wallet_rounded, OnDexColors.amber, 'Qarz', 'Qarzga sotish',
            () => finalize('qarz')),
      ]);
}

/// Kassa cheki (ShoxPos: "ONDEX MERCHANT", sana, chek ID, qatorlar, JAMI).
Receipt buildPosReceipt(String storeName, List<CartLine> cart, DateTime at) {
  final id = '${at.millisecondsSinceEpoch.toRadixString(36)}${Random().nextInt(1296).toRadixString(36)}'.toUpperCase();
  String two(int v) => v.toString().padLeft(2, '0');
  return Receipt(
    kind: ReceiptKind.customer,
    header: storeName.isEmpty ? 'ONDEX MERCHANT' : storeName,
    orderNumber: '#$id',
    badge: 'KASSA',
    meta: ['Sana: ${two(at.day)}.${two(at.month)}.${at.year} ${two(at.hour)}:${two(at.minute)}'],
    items: [
      for (final l in cart)
        l.count == l.count.roundToDouble()
            ? ReceiptItem(name: l.name, qty: l.count.toInt(), amount: somFromTiyin(l.totalTiyin))
            : ReceiptItem(name: '${l.name} (${fmtQty(l.count)})', qty: 1, amount: somFromTiyin(l.totalTiyin)),
    ],
    totals: [ReceiptTotal('JAMI', somFromTiyin(cart.fold(0, (s, l) => s + l.totalTiyin)), strong: true)],
    footer: const ['Xaridingiz uchun rahmat!'],
  );
}

/// Narx so'rash oynasi — controller oynaning o'zida (yopilish animatsiyasi
/// tugagach yo'q qilinadi).
class _PriceDialog extends StatefulWidget {
  const _PriceDialog({required this.name});
  final String name;

  @override
  State<_PriceDialog> createState() => _PriceDialogState();
}

class _PriceDialogState extends State<_PriceDialog> {
  final _c = TextEditingController();

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  void _ok() => Navigator.pop(context, tiyinFromSomText(_c.text));

  @override
  Widget build(BuildContext context) => AlertDialog(
        title: Text(widget.name),
        content: TextField(
          key: const ValueKey('ask-price'),
          controller: _c,
          autofocus: true,
          inputFormatters: kNumberInput,
          decoration: shopInput('Narxini kiriting', suffix: 'so\'m'),
          onSubmitted: (_) => _ok(),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text('Bekor qilish')),
          FilledButton(onPressed: _ok, child: const Text('Qo\'shish')),
        ],
      );
}