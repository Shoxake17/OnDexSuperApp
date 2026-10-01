/// Restoran mijozga qaysi to'lov usullarini ko'rsatishi.
///
/// ┌─ FAQAT RESTORAN YOQQAN USULLAR KO'RINADI ─────────────────────────┐
/// Server `payment_options` ni hisoblaydi: restoran paneldagi "To'lov
/// usullari" da yoqqan VA to'lov hisobi ulangan usullar. Kalitlar:
/// `cash`, `card`, `payme`, `click`, `uzum`.
///
/// O'chirilgan usul KULRANG ko'rinmaydi — umuman chiqmaydi.
///
/// `payment_options` yo'q bo'lsa (restoran hali yuklanmagan yoki eski
/// server) usul OCHIQ qoladi: oxirgi so'z baribir serverniki (`/pay` 409
/// qaytaradi), yuklanish paytida hamma usulni yashirish mijozni
/// chalg'itardi. Kalit yo'q bo'lsa ham xuddi shunday.
/// └───────────────────────────────────────────────────────────────────┘
bool paymentOptionVisible(Object? options, String key) {
  if (options is! Map) return true;
  final v = options[key];
  return v is bool ? v : true;
}
