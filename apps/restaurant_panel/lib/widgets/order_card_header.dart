import 'package:flutter/material.dart';
import 'package:ondex_core/ondex_core.dart' show tableText;

import '../theme.dart';

/// Buyurtma kartochkasining yuqori qismi: raqam va vaqt, stol
/// buyurtmasida esa ULARNING OSTIDA katta stol belgisi.
///
/// ┌─ NEGA ALOHIDA FAYLDA VA OCHIQ (public) ───────────────────────────┐
/// Bu qator bir marta Flutter'ning sariq-qora "RIGHT OVERFLOWED BY 15
/// PIXELS" chizig'ini chiqargan edi: stol belgisi qat'iy edi va
/// ustun torayganda qisqara olmasdi (`image/buyurtma.png`).
///
/// Bunday xato TEST bilan qulflanishi kerak, test esa faqat OCHIQ
/// widget'ni ko'ra oladi — sahifa ichidagi maxfiy (`_`) sinfni
/// import qilib bo'lmaydi. Shuning uchun qator shu yerga chiqarildi
/// va `test/order_card_header_test.dart` uni tor kenglikda chizib,
/// hech qanday overflow bo'lmasligini tekshiradi.
/// └───────────────────────────────────────────────────────────────────┘
class OrderCardHeader extends StatelessWidget {
  final Map<String, dynamic> order;
  const OrderCardHeader({super.key, required this.order});

  @override
  Widget build(BuildContext context) {
    final createdAt = parseOrderAt(order['created_at']);
    final dineIn = isDineInOrder(order);
    final tableLabel = order['table_label'] as String? ?? '';
    final partySize = order['party_size'] as int? ?? 0;

    // Raqam chapda (qisqara oladi), vaqt HAR DOIM o'ng chekkada.
    final numberRow = Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
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
        const SizedBox(width: 8),
        Text(orderTimeOfDay(createdAt),
            maxLines: 1,
            softWrap: false,
            style: const TextStyle(
                fontSize: 12.5,
                color: OnDexColors.inkFaint,
                fontWeight: FontWeight.w600)),
      ],
    );
    if (!dineIn) return numberRow;

    // ┌─ NEGA STOL ALOHIDA QATORDA ─────────────────────────────────────┐
    // Avval stol belgisi raqam YONIDA, kichik (12 px) yozilardi va tor
    // Kanban ustunida yana kichraytirilardi — oshxonada uzoqdan
    // o'qib bo'lmasdi. Stol buyurtmasida xodim uchun eng muhim narsa
    // taom QAYSI STOLGA ketishi, shuning uchun u raqamdan keyin, alohida
    // qatorda va katta shriftda turadi. Tartib: raqam → stol → taomlar
    // → mijoz raqami → jami summa.
    // └─────────────────────────────────────────────────────────────────┘
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        numberRow,
        const SizedBox(height: 8),
        TableChip(tableLabel: tableLabel, partySize: partySize),
      ],
    );
  }
}

/// Stol buyurtmasi belgisi: "Asosiy zal · 5-stol · 4 kishi" — ikonkasiz,
/// raqamdan keyin alohida qatorda.
///
/// Tor joyda butun belgi proporsional kichrayadi, lekin HECH QACHON
/// ota-elementdan chiqib ketmaydi.
class TableChip extends StatelessWidget {
  final String tableLabel;
  final int partySize;
  const TableChip(
      {super.key, required this.tableLabel, required this.partySize});

  @override
  Widget build(BuildContext context) {
    return Container(
      key: const ValueKey('order-table-chip'),
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
      decoration: BoxDecoration(
        color: OnDexColors.primaryTint,
        borderRadius: BorderRadius.circular(8),
      ),
      // ┌─ NEGA `FittedBox` ────────────────────────────────────────┐
      // Uch nuqta (`ellipsis`) bilan qisqartirish YETARLI EMAS edi:
      // uch nuqtaning o'zi ham joy egallaydi va belgi (ikonka +
      // qisqargan nom + "N kishi") ma'lum eng kichik kenglikdan
      // pastga tusha olmasdi — test aynan shu 3-7 pikselni ushladi.
      //
      // `FittedBox(scaleDown)` esa kerak bo'lganda BUTUN belgini
      // proporsional kichraytiradi: joy yetarli bo'lsa (odatiy holat)
      // hech narsa o'zgarmaydi — `scaleDown` faqat kichraytiradi.
      // └───────────────────────────────────────────────────────────┘
      child: FittedBox(
        fit: BoxFit.scaleDown,
        alignment: Alignment.centerLeft,
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Ikonka ATAYLAB yo'q (foydalanuvchi talabi): stol nomi
            // o'zi yetarlicha aniq, ikonka esa tor ustunda joy yeb,
            // yozuvni kichraytirib yuborardi.
            Text(
              // `tableText` — `ondex_core` da. Ilgari bu yerda
              // "$tableLabel-stol" deb yozilardi va restoran stolni
              // "Stol-1" deb nomlagan bo'lsa "Stol-1-stol" chiqardi.
              tableText(tableLabel),
              maxLines: 1,
              softWrap: false,
              style: const TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w800,
                  color: OnDexColors.primary),
            ),
            if (partySize > 0) ...[
              const SizedBox(width: 6),
              Text('· $partySize kishi',
                  maxLines: 1,
                  softWrap: false,
                  style: const TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: OnDexColors.primary)),
            ],
          ],
        ),
      ),
    );
  }
}

/// Buyurtma stol (QR kod) buyurtmasimi.
///
/// ┌─ NEGA BITTA JOYDA ────────────────────────────────────────────────┐
/// Bu tekshiruv ilgari faqat `OrderCardHeader` ichida edi, kuryer holati
/// ko'rsatiladigan joylarda esa YO'Q edi. Natijada stol buyurtmasi
/// ustida "Kuryer qidirilmoqda..." aylanuvchi indikatori ABADIY turardi:
/// backend stol buyurtmasi uchun kuryer qidirmaydi (`routes_orders.go` —
/// `if !o.IsDineIn()`), ya'ni `courier_id` hech qachon to'lmaydi va
/// indikator hech qachon to'xtamasdi.
///
/// `type` maydoni BO'SH bo'lishi mumkin (`json:"type,omitempty"` — eski
/// va yetkazish buyurtmalari uni yubormaydi), shuning uchun tekshiruv
/// aynan `== 'dine_in'`: bo'sh = yetkazish. Bu backenddagi
/// `Type.Normalized()` qoidasi bilan bir xil.
/// └───────────────────────────────────────────────────────────────────┘
bool isDineInOrder(Map<String, dynamic> order) => order['type'] == 'dine_in';

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
