import '../api.dart';
import '../data/address_scope.dart';
import '../data/catalog_repository.dart';
import '../data/qurt_cart.dart';
import '../live.dart';
import '../screens/web_session.dart';
import 'app_pin.dart';
import 'firebase_phone.dart';
import 'google_auth.dart';
import 'push.dart';

/// Qurilmadagi sessiyani TO'LIQ tozalaydi — chiqishning YAGONA yo'li.
///
/// ┌─ NEGA BITTA FUNKSIYA (2026-11-21) ────────────────────────────────┐
/// Avval ikki nusxa bor edi va ular ajralib ketgan edi:
///   * profil -> "Chiqish" hammasini tozalardi;
///   * sessiya bekor bo'lganda (`lock_gate`, 401 / akkaunt o'chirildi)
///     shaxsiy kesh (buyurtmalar, manzil), qurt savati va Google/
///     Firebase sessiyasi QOLIB KETARDI — keyingi foydalanuvchiga
///     oldingisining ma'lumoti ko'rinardi.
/// Ikkalasida ham jonli kanal (`customerLive`) to'xtatilmasdi: soket
/// eski foydalanuvchi nomidan ochiq qolib, boshqa akkaunt bilan
/// kirilganda ham oldingisining hodisalarini olishi mumkin edi.
/// └───────────────────────────────────────────────────────────────────┘
///
/// `notifyServer: false` — sessiya serverda allaqachon bekor qilingan
/// (`POST /auth/logout` faqat yana bitta 401 qaytarardi).
Future<void> signOutLocally({required bool notifyServer}) async {
  // Push tokeni ENG BIRINCHI — logout'dan keyin `DELETE /me/push-token`
  // 401 olib, yozuv serverda qolardi (push.dart izohi).
  await PushService.instance.stop();
  // Jonli soket — sessiya tokeni o'chirilishidan OLDIN yopiladi.
  await customerLive.stop();
  if (notifyServer) {
    try {
      await api.logout();
    } catch (_) {
      // Tarmoq bo'lmasa ham lokal tozalash BAJARILADI.
    }
  }
  await api.clearSession();
  // Google va Firebase O'Z sessiyalarini saqlaydi (profile_screen izohi).
  await GoogleAuth.signOut();
  await FirebasePhoneAuth.signOut();
  await AppPin.clear();
  await clearMiniAppSession();
  // Shaxsiy kesh (buyurtmalar, sevimlilar); katalog keshi ataylab qoladi.
  await Repos.clearOnLogout();
  AddressScope.instance.clear();
  await QurtCart.instance.reset();
}
