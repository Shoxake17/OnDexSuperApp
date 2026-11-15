import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:printing/printing.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'receipt_model.dart';
import 'receipt_pdf.dart';

/// Chek printeri sozlamalari va chop etish (RS-8).
///
/// ┌─ NEGA SHU KOMPYUTERDA (`PanelPrefs` kabi) ───────────────────────────┐
/// Restoranda bir nechta kompyuter bo'ladi; chek faqat printer ULANGAN
/// kassa kompyuteridan chiqishi kerak. Server sozlamasi bo'lsa, menejer
/// noutbuki ham har buyurtmada chop etish oynasini ochardi.
/// └──────────────────────────────────────────────────────────────────────┘
///
/// ┌─ QANDAY CHIQADI ─────────────────────────────────────────────────────┐
/// * Windows ilova: sozlamada printer tanlangan bo'lsa — OYNASIZ
///   (`Printing.directPrintPdf`); tanlanmagan bo'lsa tizim oynasi.
/// * Web: brauzer chop etish oynasi. Kassa kompyuterida Chrome
///   `--kiosk-printing` bilan ochilsa — OYNASIZ, standart printerga
///   (yo'riqnoma: sozlamalardagi "Printer" kartasi).
/// └──────────────────────────────────────────────────────────────────────┘
///
/// ┌─ AVTOMATIK CHOP — BIR MARTA ─────────────────────────────────────────┐
/// Buyurtma qabul qilingani (`accepted`/`preparing`) ro'yxat yangilanganda
/// aniqlanadi — qaysi kompyuter yoki ilova qabul qilganidan qat'i nazar.
/// Chop etilgan buyurtmalar ID'si shu kompyuterda saqlanadi (oxirgi 300 ta):
/// sahifa qayta ochilsa yoki panel qayta ishga tushsa takror chiqmaydi,
/// panel yopiq paytda qabul qilingan buyurtma esa ochilganda chiqadi.
/// Avtomatik chop YOQILGAN paytda ekrandagi faol buyurtmalar "chop
/// etilgan" deb belgilanadi — eski buyurtmalar to'dasi chiqib ketmasin.
/// └──────────────────────────────────────────────────────────────────────┘
class ReceiptPrefs {
  ReceiptPrefs._();

  static const _kAutoKitchen = 'panel.print.auto_kitchen';
  static const _kAutoCustomer = 'panel.print.auto_customer';
  static const _kWidth = 'panel.print.width_mm';
  static const _kKitchenCopies = 'panel.print.kitchen_copies';
  static const _kCustomerCopies = 'panel.print.customer_copies';
  static const _kPrinterUrl = 'panel.print.printer_url';
  static const _kPrinterName = 'panel.print.printer_name';
  static const _kPrinted = 'panel.print.printed_ids';
  static const _printedMax = 300;

  /// Qabul qilinganda oshxona cheki.
  static final autoKitchen = ValueNotifier<bool>(false);

  /// Qabul qilinganda mijoz cheki ham.
  static final autoCustomer = ValueNotifier<bool>(false);

  /// Lenta kengligi: 58 yoki 80 mm.
  static final widthMm = ValueNotifier<int>(58);
  static final kitchenCopies = ValueNotifier<int>(1);
  static final customerCopies = ValueNotifier<int>(1);

  /// Windows: tanlangan printer (bo'sh — tizim oynasi).
  static final printerName = ValueNotifier<String>('');
  static String _printerUrl = '';

  static final List<String> _printed = [];
  static bool _loaded = false;

  static Future<void> load() async {
    if (_loaded) return;
    try {
      final p = await SharedPreferences.getInstance();
      autoKitchen.value = p.getBool(_kAutoKitchen) ?? false;
      autoCustomer.value = p.getBool(_kAutoCustomer) ?? false;
      widthMm.value = p.getInt(_kWidth) == 80 ? 80 : 58;
      kitchenCopies.value = (p.getInt(_kKitchenCopies) ?? 1).clamp(1, 3);
      customerCopies.value = (p.getInt(_kCustomerCopies) ?? 1).clamp(1, 3);
      _printerUrl = p.getString(_kPrinterUrl) ?? '';
      printerName.value = p.getString(_kPrinterName) ?? '';
      final raw = p.getString(_kPrinted);
      if (raw != null) {
        _printed
          ..clear()
          ..addAll((jsonDecode(raw) as List).map((e) => '$e'));
      }
      _loaded = true;
    } catch (_) {
      // Saqlash yo'q (maxfiy oyna) — standart: avtomatik chop o'chiq.
    }
  }

  static Future<SharedPreferences?> _prefs() async {
    try {
      return await SharedPreferences.getInstance();
    } catch (_) {
      return null;
    }
  }

  static Future<void> setBool(ValueNotifier<bool> which, bool v) async {
    which.value = v;
    final key = identical(which, autoKitchen) ? _kAutoKitchen : _kAutoCustomer;
    await (await _prefs())?.setBool(key, v);
  }

  static Future<void> setInt(ValueNotifier<int> which, int v) async {
    which.value = v;
    final key = identical(which, widthMm)
        ? _kWidth
        : identical(which, kitchenCopies)
            ? _kKitchenCopies
            : _kCustomerCopies;
    await (await _prefs())?.setInt(key, v);
  }

  static Future<void> setPrinter(Printer? printer) async {
    _printerUrl = printer?.url ?? '';
    printerName.value = printer?.name ?? '';
    final p = await _prefs();
    await p?.setString(_kPrinterUrl, _printerUrl);
    await p?.setString(_kPrinterName, printerName.value);
  }

  static bool wasPrinted(String id) => _printed.contains(id);

  static Future<void> markPrinted(Iterable<String> ids) async {
    var changed = false;
    for (final id in ids) {
      if (id.isEmpty || _printed.contains(id)) continue;
      _printed.add(id);
      changed = true;
    }
    if (!changed) return;
    if (_printed.length > _printedMax) _printed.removeRange(0, _printed.length - _printedMax);
    await (await _prefs())?.setString(_kPrinted, jsonEncode(_printed));
  }

  @visibleForTesting
  static void resetForTest() {
    _printed.clear();
    _loaded = false;
    autoKitchen.value = false;
    autoCustomer.value = false;
  }
}

/// Avtomatik chopga tushadigan holatlar (qabul qilingan, hali berilmagan).
const autoPrintStatuses = {'accepted', 'preparing', 'ready'};

/// Ro'yxatdan hali chop etilmagan, qabul qilingan buyurtmalar.
List<Map<String, dynamic>> pendingAutoPrint(List<Map<String, dynamic>> orders) => [
      for (final o in orders)
        if (autoPrintStatuses.contains(o['status']) && !ReceiptPrefs.wasPrinted('${o['id']}')) o,
    ];

class ReceiptPrinter {
  ReceiptPrinter._();

  /// Bitta chekni chop etadi ([copies] nusxa).
  static Future<void> printReceipt(Receipt r, {int copies = 1}) async {
    final width = ReceiptPrefs.widthMm.value;
    final bytes = await buildReceiptPdf(r, widthMm: width);
    final format = receiptFormat(width);
    final name = 'Chek ${r.orderNumber}';
    final url = ReceiptPrefs._printerUrl;
    for (var i = 0; i < copies; i++) {
      if (!kIsWeb && url.isNotEmpty) {
        await Printing.directPrintPdf(
          printer: Printer(url: url, name: ReceiptPrefs.printerName.value),
          onLayout: (_) async => bytes,
          name: name,
          format: format,
        );
      } else {
        await Printing.layoutPdf(onLayout: (_) async => bytes, name: name, format: format);
      }
    }
  }

  /// Buyurtma bo'yicha chek ([kind]).
  static Future<void> printOrder(Map<String, dynamic> order, ReceiptKind kind,
      {required String restaurantName}) {
    final r = buildReceipt(order, kind: kind, restaurantName: restaurantName);
    final copies = kind == ReceiptKind.kitchen
        ? ReceiptPrefs.kitchenCopies.value
        : ReceiptPrefs.customerCopies.value;
    return printReceipt(r, copies: copies);
  }

  /// Avtomatik chop: yangi qabul qilingan buyurtmalar uchun sozlamaga ko'ra
  /// oshxona va/yoki mijoz cheki. Chop etilganlar belgilanadi (xato bo'lsa
  /// ham — printer o'chiq bo'lsa har yangilanishda oyna ochilib turmasin;
  /// qo'lda "Chek" tugmasi bilan qayta chiqariladi).
  static Future<int> autoPrint(List<Map<String, dynamic>> orders,
      {required String restaurantName}) async {
    final kitchen = ReceiptPrefs.autoKitchen.value;
    final customer = ReceiptPrefs.autoCustomer.value;
    if (!kitchen && !customer) return 0;
    final pending = pendingAutoPrint(orders);
    if (pending.isEmpty) return 0;
    // Belgilash SINXRON qismda (birinchi await'dan oldin xotirada): ketma-ket
    // ikki yangilanish (soket + so'rov sikli) bir buyurtmani ikki marta
    // chiqarmasin.
    final saving = ReceiptPrefs.markPrinted(pending.map((o) => '${o['id']}'));
    await saving;
    var n = 0;
    for (final o in pending) {
      try {
        if (kitchen) await printOrder(o, ReceiptKind.kitchen, restaurantName: restaurantName);
        if (customer) await printOrder(o, ReceiptKind.customer, restaurantName: restaurantName);
        n++;
      } catch (_) {
        // Bitta chek xatosi qolganlarini to'xtatmaydi.
      }
    }
    return n;
  }

  /// Sinov cheki — sozlamalar kartasidagi tugma.
  static Future<void> printTest({required String restaurantName}) {
    final now = DateTime.now();
    final sample = <String, dynamic>{
      'id': 'test',
      'order_number': 'TEST-0000001',
      'type': 'dine_in',
      'table_label': '5',
      'party_size': 2,
      'created_at': now.toUtc().toIso8601String(),
      'items': [
        {'name': 'Osh (to\'y oshi)', 'qty': 2, 'price_tiyin': 4500000},
        {'name': 'Shashlik, qo\'y go\'shti', 'qty': 3, 'price_tiyin': 1800000},
        {'name': 'Кока-кола 1 л', 'qty': 1, 'price_tiyin': 1200000},
      ],
      'subtotal_tiyin': 15600000,
      'total_tiyin': 15600000,
      'payment_method': 'cash',
    };
    return printReceipt(
        buildReceipt(sample, kind: ReceiptKind.customer, restaurantName: restaurantName, now: now));
  }
}
