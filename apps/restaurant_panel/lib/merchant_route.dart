/// Kirgandan keyin qaysi panel ochilishi — YAGONA qoida.
///
/// Avval bu shart ikki joyda (`main.dart` sessiyani tiklash va
/// `login_screen.dart` kirishdan keyin) alohida yozilgan edi: do'kon
/// tomonga o'zgartirish kiritilganda biri yangilanib, ikkinchisi eski
/// qolib, restoran egasi noto'g'ri ekranga tushishi mumkin edi. Endi
/// ikkalasi shu funksiyani chaqiradi va `test/guard_restaurant_test.dart`
/// uni qulflaydi.
enum MerchantHome { restaurant, store, none }

MerchantHome merchantHomeFor(Map<String, dynamic> user) {
  final role = user['role'] as String? ?? '';
  final entityId = user['entity_id'] as String? ?? '';
  if (role == 'restaurant' && entityId.isNotEmpty) {
    return MerchantHome.restaurant;
  }
  if (role == 'store') return MerchantHome.store;
  return MerchantHome.none;
}
