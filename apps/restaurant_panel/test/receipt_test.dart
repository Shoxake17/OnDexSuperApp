import 'package:flutter_test/flutter_test.dart';
import 'package:chust_restaurant/printing/receipt_model.dart';
import 'package:chust_restaurant/printing/receipt_pdf.dart';
import 'package:chust_restaurant/printing/receipt_printer.dart';
import 'package:shared_preferences/shared_preferences.dart';

Map<String, dynamic> _delivery() => {
      'id': 'o1',
      'order_number': '151126-0000123',
      'status': 'accepted',
      'created_at': '2026-11-15T09:05:00Z',
      'customer_phone': '+998901234567',
      'preparation_minutes': 20,
      'items': [
        {'name': 'Osh', 'qty': 2, 'price_tiyin': 4500000},
        {'name': 'Somsa', 'qty': 3, 'price_tiyin': 800000, 'discount_price_tiyin': 700000},
      ],
      'subtotal_tiyin': 11100000,
      'discount_tiyin': 1000000,
      'promotion_name': 'Kuz aksiyasi',
      'promotion_discount_tiyin': 1000000,
      'delivery_fee_tiyin': 1000000,
      'total_tiyin': 11100000,
      'payment_method': 'card',
      'payment_state': 'paid',
      'delivery_address': {'text': 'Chust, Navoiy ko\'chasi 5', 'floor': '3', 'apartment': '12'},
    };

void main() {
  final now = DateTime(2026, 11, 15, 14, 30);

  test('oshxona cheki: narxsiz, stol/yetkazish belgisi, tayyorlash vaqti', () {
    final r = buildReceipt(_delivery(), kind: ReceiptKind.kitchen, restaurantName: 'Chust Osh', now: now);
    expect(r.header, 'OSHXONA');
    expect(r.orderNumber, '#0000123');
    expect(r.badge, 'YETKAZISH');
    expect(r.items.map((i) => '${i.qty}x ${i.name}'), ['2x Osh', '3x Somsa']);
    expect(r.items.every((i) => i.amount == null), isTrue);
    expect(r.totals, isEmpty);
    expect(r.meta, contains('Tayyorlash: 20 daqiqa'));
    // Oshxonaga mijoz telefoni va manzili kerak emas.
    expect(r.meta.join(' '), isNot(contains('+998')));
  });

  test('mijoz cheki: narxlar, chegirma narxi, aksiya nomi, yetkazish, to\'lov', () {
    final r = buildReceipt(_delivery(), kind: ReceiptKind.customer, restaurantName: 'Chust Osh', now: now);
    expect(r.header, 'Chust Osh');
    expect(r.items[0].amount, '90 000 so\'m');
    expect(r.items[1].amount, '21 000 so\'m'); // chegirma narxi 7 000 * 3
    final t = {for (final x in r.totals) x.label: x.value};
    expect(t['Taomlar'], '111 000 so\'m');
    expect(t['Kuz aksiyasi'], '-10 000 so\'m');
    expect(t['Yetkazish'], '10 000 so\'m');
    expect(t['JAMI'], '111 000 so\'m');
    expect(t['To\'lov'], 'Karta (to\'langan)');
    expect(r.meta.any((m) => m.startsWith('Manzil: Chust') && m.contains('qavat 3')), isTrue);
    expect(r.meta, contains('Tel: +998901234567'));
  });

  test('stol buyurtmasi: stol belgisi, kishi soni, manzil yo\'q, naqd', () {
    final o = {
      'id': 'o2',
      'order_number': '151126-0000124',
      'type': 'dine_in',
      'table_label': '5',
      'party_size': 4,
      'items': [
        {'name': 'Choy', 'qty': 1, 'price_tiyin': 500000},
      ],
      'subtotal_tiyin': 500000,
      'total_tiyin': 500000,
    };
    final r = buildReceipt(o, kind: ReceiptKind.customer, restaurantName: '', now: now);
    expect(r.header, 'Restoran');
    expect(r.badge, '5-stol');
    expect(r.meta, contains('4 kishi'));
    expect(r.meta.any((m) => m.startsWith('Manzil')), isFalse);
    final t = {for (final x in r.totals) x.label: x.value};
    expect(t.containsKey('Taomlar'), isFalse); // chegirma/yetkazish yo'q — faqat JAMI
    expect(t['JAMI'], '5 000 so\'m');
    expect(t['To\'lov'], 'Naqd');
  });

  test('PDF 58 va 80 mm uchun yaratiladi (kirill bilan)', () async {
    TestWidgetsFlutterBinding.ensureInitialized();
    final o = _delivery();
    (o['items'] as List).add({'name': 'Кока-кола', 'qty': 1, 'price_tiyin': 1200000});
    for (final w in [58, 80]) {
      final bytes = await buildReceiptPdf(
          buildReceipt(o, kind: ReceiptKind.customer, restaurantName: 'Чуст Ош', now: now),
          widthMm: w);
      expect(String.fromCharCodes(bytes.take(5)), '%PDF-');
    }
  });

  test('avtomatik chop: faqat qabul qilingan va hali chop etilmaganlar', () async {
    SharedPreferences.setMockInitialValues({});
    ReceiptPrefs.resetForTest();
    await ReceiptPrefs.load();
    final orders = [
      {'id': 'a', 'status': 'created'},
      {'id': 'b', 'status': 'accepted'},
      {'id': 'c', 'status': 'preparing'},
      {'id': 'd', 'status': 'delivered'},
    ];
    expect(pendingAutoPrint(orders).map((o) => o['id']), ['b', 'c']);
    await ReceiptPrefs.markPrinted(['b']);
    expect(pendingAutoPrint(orders).map((o) => o['id']), ['c']);
    // O'chiq bo'lsa hech narsa chiqmaydi va belgilanmaydi.
    expect(await ReceiptPrinter.autoPrint(orders, restaurantName: 'x'), 0);
    expect(ReceiptPrefs.wasPrinted('c'), isFalse);
  });
}
