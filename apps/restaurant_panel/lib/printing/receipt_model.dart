import 'package:ondex_core/ondex_core.dart' show formatSum, tableText;

import '../widgets/order_card_header.dart'
    show isDineInOrder, isPickupOrder, parseOrderAt, pickupWhenText, shortOrderNumber;

/// Chek turi (RS-8).
///
/// * [kitchen] — oshxona cheki: narxsiz, katta shriftda nima va qancha
///   tayyorlanishi, qaysi stol/yetkazish. Buyurtma qabul qilinganda.
/// * [customer] — mijoz cheki: narxlar, chegirma, yetkazish haqi, jami,
///   to'lov usuli. Paket bilan yoki stolga beriladi.
enum ReceiptKind { kitchen, customer }

/// Chekning bitta qatori (taom).
class ReceiptItem {
  const ReceiptItem({required this.name, required this.qty, this.amount});
  final String name;
  final int qty;

  /// `qty * narx` (so'm matni). Oshxona chekida `null`.
  final String? amount;
}

/// Jami bo'limidagi qator ("Chegirma", "Yetkazish", "JAMI").
class ReceiptTotal {
  const ReceiptTotal(this.label, this.value, {this.strong = false});
  final String label;
  final String value;
  final bool strong;
}

/// Chop etishga tayyor chek — PDF/printerdan MUSTAQIL (testda shu
/// tekshiriladi, chizish `receipt_pdf.dart` da).
class Receipt {
  const Receipt({
    required this.kind,
    required this.header,
    required this.orderNumber,
    required this.badge,
    required this.meta,
    required this.items,
    required this.totals,
    required this.footer,
  });

  final ReceiptKind kind;

  /// Yuqori markazdagi sarlavha (restoran nomi yoki "OSHXONA").
  final String header;

  /// "#0000123".
  final String orderNumber;

  /// Katta belgi: "5-stol" / "YETKAZISH".
  final String badge;

  /// Kichik ma'lumot qatorlari (vaqt, kishi soni, telefon, manzil...).
  final List<String> meta;
  final List<ReceiptItem> items;
  final List<ReceiptTotal> totals;
  final List<String> footer;
}

String _two(int v) => v.toString().padLeft(2, '0');

String _dateTime(DateTime d) =>
    '${_two(d.day)}.${_two(d.month)}.${d.year} ${_two(d.hour)}:${_two(d.minute)}';

int _int(dynamic v) => v is int ? v : (v is num ? v.toInt() : int.tryParse('$v') ?? 0);

/// To'lov usuli matni (server `orders.PaymentMethod` / `PaymentState`).
String paymentText(Map<String, dynamic> o) {
  final method = '${o['payment_method'] ?? ''}';
  final state = '${o['payment_state'] ?? ''}';
  final paid = state == 'paid' || state == 'held';
  return switch (method) {
    'card' => paid ? 'Karta (to\'langan)' : 'Karta',
    'wallet' => paid ? 'OnDex Wallet (to\'langan)' : 'OnDex Wallet',
    _ => 'Naqd',
  };
}

/// Buyurtmadan chek tuzadi. [now] — chop etilgan vaqt (testda qat'iy).
Receipt buildReceipt(
  Map<String, dynamic> o, {
  required ReceiptKind kind,
  required String restaurantName,
  DateTime? now,
}) {
  final dineIn = isDineInOrder(o);
  final created = parseOrderAt(o['created_at']);
  final printed = now ?? DateTime.now();
  final party = _int(o['party_size']);
  final pickup = isPickupOrder(o);
  final badge = dineIn
      ? tableText('${o['table_label'] ?? ''}')
      : (pickup ? 'OLIB KETISH' : 'YETKAZISH');

  final items = <ReceiptItem>[];
  for (final raw in (o['items'] as List? ?? const [])) {
    if (raw is! Map) continue;
    final qty = _int(raw['qty']);
    final price = _int(raw['price_tiyin']);
    final discount = _int(raw['discount_price_tiyin']);
    final unit = discount > 0 && discount < price ? discount : price;
    items.add(ReceiptItem(
      name: '${raw['name'] ?? ''}'.trim(),
      qty: qty,
      amount: kind == ReceiptKind.customer ? formatSum(unit * qty) : null,
    ));
  }

  final meta = <String>[];
  if (created != null) meta.add('Buyurtma: ${_dateTime(created)}');
  if (dineIn && party > 0) meta.add('$party kishi');
  if (pickup) meta.add('Olib ketish: ${pickupWhenText(o)}');

  if (kind == ReceiptKind.kitchen) {
    final readyIn = _int(o['preparation_minutes']);
    if (readyIn > 0) meta.add('Tayyorlash: $readyIn daqiqa');
    return Receipt(
      kind: kind,
      header: 'OSHXONA',
      orderNumber: shortOrderNumber(o),
      badge: badge,
      meta: meta,
      items: items,
      totals: const [],
      footer: ['Chop etildi: ${_dateTime(printed)}'],
    );
  }

  if (!dineIn && !pickup) {
    final addr = o['delivery_address'];
    if (addr is Map) {
      final parts = <String>[
        '${addr['text'] ?? ''}'.trim(),
        if ('${addr['entrance'] ?? ''}'.trim().isNotEmpty) 'podyezd ${addr['entrance']}',
        if ('${addr['floor'] ?? ''}'.trim().isNotEmpty) 'qavat ${addr['floor']}',
        if ('${addr['apartment'] ?? ''}'.trim().isNotEmpty) 'xonadon ${addr['apartment']}',
      ].where((s) => s.isNotEmpty).toList();
      if (parts.isNotEmpty) meta.add('Manzil: ${parts.join(', ')}');
    }
    final phone = '${o['customer_phone'] ?? ''}'.trim();
    if (phone.isNotEmpty) meta.add('Tel: $phone');
  }

  final subtotal = _int(o['subtotal_tiyin']);
  final discount = _int(o['discount_tiyin']);
  final fee = _int(o['delivery_fee_tiyin']);
  final total = _int(o['total_tiyin']);
  final promo = '${o['promotion_name'] ?? ''}'.trim();
  final promoOnly = promo.isNotEmpty && _int(o['promotion_discount_tiyin']) == discount;

  final totals = <ReceiptTotal>[
    if (subtotal > 0 && (discount > 0 || fee > 0)) ReceiptTotal('Taomlar', formatSum(subtotal)),
    if (discount > 0) ReceiptTotal(promoOnly ? promo : 'Chegirma', '-${formatSum(discount)}'),
    if (fee > 0) ReceiptTotal('Yetkazish', formatSum(fee)),
    ReceiptTotal('JAMI', formatSum(total), strong: true),
    ReceiptTotal('To\'lov', paymentText(o)),
  ];

  final name = restaurantName.trim().isEmpty ? 'Restoran' : restaurantName.trim();
  return Receipt(
    kind: kind,
    header: name,
    orderNumber: shortOrderNumber(o),
    badge: badge,
    meta: meta,
    items: items,
    totals: totals,
    footer: [
      'Rahmat! Yoqimli ishtaha!',
      'OnDex orqali buyurtma',
      _dateTime(printed),
    ],
  );
}
