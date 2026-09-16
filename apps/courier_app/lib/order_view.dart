import 'package:ondex_core/ondex_core.dart';

/// Kuryer mijozdan pul olishi kerakmi.
enum PaymentKind { collectCash, paidOnline, unknown }

/// "Joriy buyurtma" kartochkasidagi to'lov ko'rsatmasi.
class PaymentInstruction {
  final PaymentKind kind;
  final String title;
  final String subtitle;
  const PaymentInstruction(this.kind, this.title, this.subtitle);
}

/// Buyurtmaning to'lov turi va holatidan kuryer uchun ko'rsatma.
///
/// ┌─ NEGA KERAK (kuryer ilovasi auditi, 1-band) ──────────────────────┐
/// Server `payment_method` (`cash`/`card`) va `payment_state` ni doim
/// berardi, mijoz ilovasida "Naqd pul" tanlovi ham ishlaydi — lekin
/// kuryer kartochkasi faqat SUMMANI ko'rsatardi. Natija: naqd
/// buyurtmada pul olinmay qolishi yoki karta bilan to'langan mijozdan
/// yana pul so'ralishi mumkin edi.
///
/// Noma'lum holatda "pul oling" DEYILMAYDI: xato ko'rsatma to'g'ridan-
/// to'g'ri pul yo'qotishi yoki mijozdan ikki marta olish demak, shuning
/// uchun kuryer restoranga qo'ng'iroq qilib aniqlaydi.
/// └───────────────────────────────────────────────────────────────────┘
PaymentInstruction paymentInstructionFor(Map<String, dynamic> order) {
  final method = (order['payment_method'] as String?)?.trim() ?? '';
  final state = (order['payment_state'] as String?)?.trim() ?? '';
  final total = (order['total_tiyin'] as num?)?.toInt() ?? 0;

  if (method == 'cash') {
    return PaymentInstruction(
      PaymentKind.collectCash,
      'Naqd: ${formatSum(total)} oling',
      'Mijoz buyurtmani olganda naqd to\'laydi',
    );
  }
  if (method == 'card') {
    // `held` — pul kartada ushlab qolingan, `paid` — yechilgan. Ikkalasi
    // ham "to'langan" (server `PaymentState.Settled`).
    if (state == 'paid' || state == 'held') {
      return const PaymentInstruction(
        PaymentKind.paidOnline,
        'Onlayn to\'langan',
        'Mijozdan pul OLMANG',
      );
    }
    return const PaymentInstruction(
      PaymentKind.unknown,
      'Karta to\'lovi tasdiqlanmagan',
      'Pul olmang — restoranga qo\'ng\'iroq qilib aniqlang',
    );
  }
  return const PaymentInstruction(
    PaymentKind.unknown,
    'To\'lov turi noma\'lum',
    'Pul olishdan oldin restoranga qo\'ng\'iroq qiling',
  );
}

/// Paneldagi "pul" kartasi uchun qisqa xulosa.
class CashSummary {
  final String label;
  final String value;

  /// `null` — joriy buyurtma yo'q.
  final PaymentKind? kind;
  const CashSummary(this.label, this.value, this.kind);
}

/// Joriy buyurtma bo'yicha kuryer mijozdan qancha pul olishi.
///
/// Foydalanuvchi qarori (2026-09-15): kartada maosh EMAS — buyurtma naqd
/// bo'lsa AYNAN o'sha buyurtmaning narxi. Karta bilan to'langan yoki turi
/// noma'lum buyurtmada summa yozilmaydi: "oling" deb o'qilishi mumkin va
/// mijozdan ikki marta pul olinishiga olib keladi.
CashSummary cashSummaryFor(Map<String, dynamic>? order) {
  if (order == null) {
    return const CashSummary('Naqd', 'Buyurtma yo\'q', null);
  }
  final pay = paymentInstructionFor(order);
  switch (pay.kind) {
    case PaymentKind.collectCash:
      final total = (order['total_tiyin'] as num?)?.toInt() ?? 0;
      return CashSummary('Naqd olinadi', formatSum(total), pay.kind);
    case PaymentKind.paidOnline:
      return CashSummary('To\'lov', 'Onlayn to\'langan', pay.kind);
    case PaymentKind.unknown:
      return CashSummary('To\'lov', 'Aniqlanmagan', pay.kind);
  }
}
