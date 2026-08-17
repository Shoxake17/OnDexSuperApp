import 'package:flutter/material.dart';

import '../theme.dart';

/// Buyurtma kartochkasining yuqori qatori: raqam, stol belgisi va vaqt.
///
/// ┌─ NEGA ALOHIDA FAYLDA VA OCHIQ (public) ───────────────────────────┐
/// Bu qator bir marta Flutter'ning sariq-qora "RIGHT OVERFLOWED BY 15
/// PIXELS" chizig'ini chiqargan edi: stol belgisi qat'iy edi va
/// ustun torayganda qisqara olmasdi (`image/buyurtma.png`).
///
/// Bunday xato TEST bilan qulflanishi kerak, test esa faqat OCHIQ
/// widget'ni ko'ra oladi — sahifa ichidagi maxfiy (`_`) sinfni
/// import qilib bo'lmaydi. Shuning uchun qator shu yerga chiqarildi
/// va `test/order_card_header_test.dart` uni juda tor (60 px)
/// kenglikda chizib, hech qanday overflow bo'lmasligini tekshiradi.
/// └───────────────────────────────────────────────────────────────────┘
class OrderCardHeader extends StatelessWidget {
  final Map<String, dynamic> order;
  const OrderCardHeader({super.key, required this.order});

  @override
  Widget build(BuildContext context) {
    final createdAt = parseOrderAt(order['created_at']);
    // Stol buyurtmasi — oshxona uchun MUHIM farq: taom qayerga ketishi
    // (kuryerga emas, zalga) va nechta kishiga tayyorlash kerakligi shu
    // yerdan ko'rinadi.
    final dineIn = order['type'] == 'dine_in';
    final tableLabel = order['table_label'] as String? ?? '';
    final partySize = order['party_size'] as int? ?? 0;

    // ┌─ TARTIB QANDAY QURILGAN ──────────────────────────────────────┐
    // Avval bu qator qat'iy edi: raqam + stol belgisi + `Spacer` +
    // vaqt. Ustun torayganda belgi qisqara olmasdi va Flutter
    // kartochka ustiga sariq-qora chiziq chizardi.
    //
    // Endi:
    //   * tashqi `Row` — `spaceBetween`: vaqt HAR DOIM o'ng chekkada
    //     (`Spacer` bilan bo'lgani kabi), lekin u bo'sh joyni
    //     "egallab" turmaydi;
    //   * chap guruh (`raqam` + `stol belgisi`) — `Flexible`, ya'ni
    //     qolgan joyga sig'adi;
    //   * guruh ichidagi HAR IKKI element ham qisqara oladi (uch
    //     nuqta bilan), shuning uchun juda tor joyda ham overflow
    //     bo'lmaydi.
    //
    // Bu `test/order_card_header_test.dart` bilan qulflangan: 60 px
    // kenglikda ham xato chiqmasligi tekshiriladi.
    // └───────────────────────────────────────────────────────────────┘
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Flexible(
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Flexible(
                child: Text(
                  shortOrderNumber(order),
                  maxLines: 1,
                  softWrap: false,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      fontSize: 14.5,
                      fontWeight: FontWeight.w800,
                      color: OnDexColors.ink),
                ),
              ),
              if (dineIn)
                Flexible(
                  child: Padding(
                    padding: const EdgeInsets.only(left: 8, right: 8),
                    child:
                        TableChip(tableLabel: tableLabel, partySize: partySize),
                  ),
                ),
            ],
          ),
        ),
        Text(orderTimeOfDay(createdAt),
            maxLines: 1,
            softWrap: false,
            style: const TextStyle(
                fontSize: 12.5,
                color: OnDexColors.inkFaint,
                fontWeight: FontWeight.w600)),
      ],
    );
  }
}

/// Stol buyurtmasi belgisi: "🍽 5-stol · 4 kishi".
///
/// Tor joyda stol nomi uch nuqtaga qisqaradi, lekin belgi HECH QACHON
/// ota-elementdan chiqib ketmaydi.
class TableChip extends StatelessWidget {
  final String tableLabel;
  final int partySize;
  const TableChip(
      {super.key, required this.tableLabel, required this.partySize});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: OnDexColors.primaryTint,
        borderRadius: BorderRadius.circular(6),
      ),
      // ┌─ NEGA `FittedBox` ────────────────────────────────────────┐
      // Uch nuqta (`ellipsis`) bilan qisqartirish YETARLI EMAS edi:
      // uch nuqtaning o'zi ham joy egallaydi va belgi (ikonka +
      // qisqargan nom + "N kishi") ma'lum eng kichik kenglikdan
      // pastga tusha olmasdi — test aynan shu 3-7 pikselni ushladi.
      //
      // `FittedBox(scaleDown)` esa kerak bo'lganda BUTUN belgini
      // proporsional kichraytiradi: matn o'qilishda qoladi, ustun
      // qanchalik tor bo'lmasin overflow bo'lmaydi. Joy yetarli
      // bo'lsa (odatiy holat) hech narsa o'zgarmaydi — `scaleDown`
      // faqat kichraytiradi, kattalashtirmaydi.
      // └───────────────────────────────────────────────────────────┘
      child: FittedBox(
        fit: BoxFit.scaleDown,
        alignment: Alignment.centerLeft,
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.table_restaurant_rounded,
                size: 13, color: OnDexColors.primary),
            const SizedBox(width: 4),
            Text(
              tableLabel.isEmpty ? 'Stol' : '$tableLabel-stol',
              maxLines: 1,
              softWrap: false,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                  color: OnDexColors.primary),
            ),
            if (partySize > 0) ...[
              const SizedBox(width: 6),
              Text('· $partySize kishi',
                  maxLines: 1,
                  softWrap: false,
                  style: const TextStyle(
                      fontSize: 11.5, color: OnDexColors.primary)),
            ],
          ],
        ),
      ),
    );
  }
}

/// ISO satrni mahalliy vaqtga aylantiradi (noto'g'ri qiymatda `null`).
DateTime? parseOrderAt(dynamic iso) =>
    iso is String ? DateTime.tryParse(iso)?.toLocal() : null;

/// Buyurtma raqamining oxirgi bo'lagi: "#0000123".
String shortOrderNumber(Map<String, dynamic> o) =>
    '#${(o['order_number'] ?? '').toString().split('-').last}';

/// "14:05" ko'rinishidagi vaqt.
String orderTimeOfDay(DateTime? d) => d == null
    ? '—'
    : '${d.hour.toString().padLeft(2, '0')}:${d.minute.toString().padLeft(2, '0')}';
